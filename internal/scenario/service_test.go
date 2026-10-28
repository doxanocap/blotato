package scenario

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/doxanocap/blotato/internal/adapter"
	"github.com/doxanocap/blotato/internal/fake"
	"github.com/doxanocap/blotato/internal/sqlite"
)

func setup(t *testing.T) (*sqlite.Store, *Service) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, New(db.DB, Settings{Reply: "reply", Question: "question", Link: "link", Wait: 24 * time.Hour})
}
func event(kind adapter.Kind, id, text string) adapter.Event {
	return adapter.Event{Platform: "fake", AccountID: "brand", ContactID: "person", Kind: kind, EventID: id, Text: text}
}
func receive(t *testing.T, s *Service, e adapter.Event) {
	t.Helper()
	if err := s.Receive(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}
func work(t *testing.T, s *Service, sender adapter.Sender) {
	t.Helper()
	ok, err := s.Work(context.Background(), sender)
	if err != nil || !ok {
		t.Fatalf("work: %v %v", ok, err)
	}
}
func state(t *testing.T, db *sql.DB) string {
	t.Helper()
	var state string
	if err := db.QueryRow(`SELECT state FROM runs LIMIT 1`).Scan(&state); err != nil {
		t.Fatal(err)
	}
	return state
}
func count(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHappyPathAndDuplicates(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	sender := fake.New()
	c := event(adapter.Comment, "c1", "pricing")
	receive(t, s, c)
	receive(t, s, c)
	receive(t, s, event(adapter.Comment, "c2", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	if got := state(t, db.DB); got != "waiting_email" {
		t.Fatal(got)
	}
	receive(t, s, event(adapter.Message, "bad", "invalid"))
	if got := state(t, db.DB); got != "waiting_email" {
		t.Fatal(got)
	}
	m := event(adapter.Message, "m1", "person@example.com")
	receive(t, s, m)
	receive(t, s, m)
	work(t, s, sender)
	if got := state(t, db.DB); got != "completed" {
		t.Fatal(got)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM runs`); n != 1 {
		t.Fatal(n)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions`); n != 3 {
		t.Fatal(n)
	}
}
func TestConcurrentAnswers(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	sender := fake.New()
	receive(t, s, event(adapter.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"m1", "m2"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			errs <- s.Receive(context.Background(), event(adapter.Message, id, "person@example.com"))
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n != 1 {
		t.Fatal(n)
	}
}
func TestRestartWaiting(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := New(db.DB, Settings{Reply: "r", Question: "q", Link: "l", Wait: time.Hour})
	sender := fake.New()
	receive(t, s, event(adapter.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s = New(db.DB, Settings{Wait: time.Hour})
	receive(t, s, event(adapter.Message, "m1", "person@example.com"))
	work(t, s, sender)
	if got := state(t, db.DB); got != "completed" {
		t.Fatal(got)
	}
}
func TestUnknownStops(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	sender := fake.New()
	sender.Set(adapter.AskEmail, adapter.Unknown)
	receive(t, s, event(adapter.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	if got := state(t, db.DB); got != "unknown" {
		t.Fatal(got)
	}
	worked, err := s.Work(context.Background(), sender)
	if err != nil || worked {
		t.Fatalf("retry: %v %v", worked, err)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE status='accepted'`); n != 1 {
		t.Fatal(n)
	}
}

type blockingSender struct {
	started chan struct{}
	release chan struct{}
}

func (b *blockingSender) Send(ctx context.Context, c adapter.Command) adapter.Result {
	close(b.started)
	select {
	case <-b.release:
		return adapter.Result{Outcome: adapter.Accepted}
	case <-ctx.Done():
		return adapter.Result{Outcome: adapter.Unknown}
	}
}
func TestEarlyAnswer(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	receive(t, s, event(adapter.Comment, "c1", "pricing"))
	work(t, s, fake.New())
	b := &blockingSender{make(chan struct{}), make(chan struct{})}
	done := make(chan error, 1)
	go func() { _, err := s.Work(context.Background(), b); done <- err }()
	<-b.started
	receive(t, s, event(adapter.Message, "m1", "person@example.com"))
	close(b.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if got := state(t, db.DB); got != "link_pending" {
		t.Fatal(got)
	}
	work(t, s, fake.New())
	if got := state(t, db.DB); got != "completed" {
		t.Fatal(got)
	}
}
func TestInterruptedSendBecomesUnknown(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "db")
	db, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := New(db.DB, Settings{Wait: time.Hour})
	receive(t, s, event(adapter.Comment, "c1", "pricing"))
	if _, err = s.claim(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := state(t, db.DB); got != "unknown" {
		t.Fatal(got)
	}
}

func TestExpiredAndIgnoredActiveComment(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	sender := fake.New()
	receive(t, s, event(adapter.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	receive(t, s, event(adapter.Comment, "c2", "pricing"))
	now = now.Add(24 * time.Hour)
	receive(t, s, event(adapter.Message, "m1", "person@example.com"))
	if _, err := s.Work(context.Background(), sender); err != nil {
		t.Fatal(err)
	}
	if got := state(t, db.DB); got != "expired" {
		t.Fatal(got)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n != 0 {
		t.Fatal(n)
	}
	receive(t, s, event(adapter.Comment, "c2", "pricing"))
	if n := count(t, db.DB, `SELECT count(*) FROM runs`); n != 1 {
		t.Fatal(n)
	}
}
