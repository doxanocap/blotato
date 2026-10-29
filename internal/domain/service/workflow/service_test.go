package workflow

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/doxanocap/blotato/internal/clients/fake"
	"github.com/doxanocap/blotato/internal/clients/instagram"
	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
	"github.com/doxanocap/blotato/internal/domain/service/resolver"
	"github.com/doxanocap/blotato/internal/domain/service/workflow/contracts"
	sqlitepkg "github.com/doxanocap/blotato/internal/pkg/sqlite"
	"github.com/doxanocap/blotato/internal/repo/sqlite"
)

type testRepository struct {
	*sqlite.Repository
	DB *sql.DB
}

func (r *testRepository) Close() error {
	return r.DB.Close()
}

func openTestRepository(filename string) (*testRepository, error) {
	ctx := context.Background()
	db, err := sqlitepkg.Open(ctx, filename)
	if err != nil {
		return nil, err
	}

	r := sqlite.New(db)
	if err := r.RecoverSendingActions(ctx); err != nil {
		return nil, errors.Join(err, db.Close())
	}

	return &testRepository{Repository: &r, DB: db}, nil
}

func setup(t *testing.T) (*testRepository, *Service) {
	t.Helper()
	db, err := openTestRepository(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	seedRule(t, db)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, NewService(db, resolver.NewService(db), fake.New(), "")
}
func seedRule(t *testing.T, db *testRepository) {
	t.Helper()
	if _, err := db.CreateWorkflow(context.Background(), entity.NewWorkflow("pricing", "reply", "question", "https://example.com/link")); err != nil {
		t.Fatal(err)
	}
}
func event(kind types.EventType, id, text string) dto.ReceiveEventIn {
	return dto.ReceiveEventIn{Platform: "fake", AccountID: "brand", ContactID: "person", Kind: kind, EventID: id, Text: text, OccurredAt: time.Now().Add(-time.Second)}
}
func receive(t *testing.T, s *Service, e dto.ReceiveEventIn) {
	t.Helper()
	if err := s.Receive(context.Background(), e); err != nil {
		t.Fatal(err)
	}
}
func work(t *testing.T, s *Service, sender contracts.InstagramClient) {
	t.Helper()
	s.instagram = sender
	ok, err := s.Work(context.Background())
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
	c := event(types.Comment, "c1", "pricing")
	receive(t, s, c)
	receive(t, s, c)
	receive(t, s, event(types.Comment, "c2", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	if got := state(t, db.DB); got != "waiting_email" {
		t.Fatal(got)
	}
	receive(t, s, event(types.Message, "bad", "invalid"))
	if got := state(t, db.DB); got != "waiting_email" {
		t.Fatal(got)
	}
	m := event(types.Message, "m1", "person@example.com")
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

func TestFirstMatchingRuleAndSnapshot(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	if _, err := db.CreateWorkflow(context.Background(), entity.NewWorkflow("price", "second", "other question", "https://example.com/second")); err != nil {
		t.Fatal(err)
	}
	receive(t, s, event(types.Comment, "no-match", "hello"))
	if count(t, db.DB, `SELECT count(*) FROM runs`) != 0 {
		t.Fatal("unmatched comment started a run")
	}
	receive(t, s, event(types.Comment, "match", "PRICING and price"))
	if _, err := db.DB.Exec(`UPDATE workflows SET keyword='changed', public_reply='changed', email_question='changed', link='https://example.com/changed' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	var id int64
	var keyword, reply, question, link string
	if err := db.DB.QueryRow(`SELECT workflow_id,keyword,reply,question,link FROM runs`).Scan(&id, &keyword, &reply, &question, &link); err != nil {
		t.Fatal(err)
	}
	if id != 1 || keyword != "pricing" || reply != "reply" || question != "question" || link != "https://example.com/link" {
		t.Fatalf("wrong snapshot: id=%d keyword=%q reply=%q question=%q link=%q", id, keyword, reply, question, link)
	}
}

type failingActionRepository struct {
	contracts.Repo
}

func (f failingActionRepository) CreateAction(context.Context, types.ID, types.ActionStep) error {
	return errors.New("action insert failed")
}

func TestReceiveRollsBackAndCanBeRedelivered(t *testing.T) {
	t.Parallel()

	db, service := setup(t)
	service.repo = failingActionRepository{Repo: db}
	comment := event(types.Comment, "c1", "pricing")
	if err := service.Receive(context.Background(), comment); err == nil {
		t.Fatal("expected action insert error")
	}
	if got := count(t, db.DB, `SELECT count(*) FROM inbox`); got != 0 {
		t.Fatalf("event committed after failed action: %d", got)
	}
	if got := count(t, db.DB, `SELECT count(*) FROM runs`); got != 0 {
		t.Fatalf("run committed after failed action: %d", got)
	}

	service.repo = db
	receive(t, service, comment)
	if got := count(t, db.DB, `SELECT count(*) FROM actions`); got != 1 {
		t.Fatalf("redelivery did not create an action: %d", got)
	}
}

type failingFinishRepository struct {
	contracts.Repo
}

func (f failingFinishRepository) FinishAction(context.Context, string, entity.SendResult) error {
	return errors.New("action update failed")
}

func TestInterruptedSendRecoversWithoutResending(t *testing.T) {
	t.Parallel()

	db, service := setup(t)
	service.repo = failingFinishRepository{Repo: db}
	receive(t, service, event(types.Comment, "c1", "pricing"))
	if _, err := service.Work(context.Background()); err == nil {
		t.Fatal("expected action update error")
	}

	service.repo = db
	if err := service.RecoverSending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := state(t, db.DB); got != "unknown" {
		t.Fatalf("state after interrupted send: %s", got)
	}
	if worked, err := service.Work(context.Background()); err != nil || worked {
		t.Fatalf("interrupted action was retried: worked=%t err=%v", worked, err)
	}
}

func TestConcurrentAnswers(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	sender := fake.New()
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"m1", "m2"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			errs <- s.Receive(context.Background(), event(types.Message, id, "person@example.com"))
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
	db, err := openTestRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	seedRule(t, db)
	sender := fake.New()
	s := NewService(db, resolver.NewService(db), sender, "")
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openTestRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s = NewService(db, resolver.NewService(db), sender, "")
	receive(t, s, event(types.Message, "m1", "person@example.com"))
	work(t, s, sender)
	if got := state(t, db.DB); got != "completed" {
		t.Fatal(got)
	}
}
func TestUnknownStops(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	sender := fake.New()
	sender.Set(types.AskEmail, types.Unknown)
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	if got := state(t, db.DB); got != "unknown" {
		t.Fatal(got)
	}
	worked, err := s.Work(context.Background())
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

func (b *blockingSender) PrivateReply(ctx context.Context, accountID, commentID, contactID, text string) (instagram.Message, error) {
	close(b.started)
	select {
	case <-b.release:
		return instagram.Message{ID: "question", RecipientID: contactID}, nil
	case <-ctx.Done():
		return instagram.Message{}, ctx.Err()
	}
}

func (b *blockingSender) ReplyComment(context.Context, string, string) (string, error) {
	return "reply", nil
}

func (b *blockingSender) SendMessage(_ context.Context, accountID, contactID, text string) (instagram.Message, error) {
	return instagram.Message{ID: "link", RecipientID: contactID}, nil
}
func TestEarlyAnswer(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, fake.New())
	b := &blockingSender{make(chan struct{}), make(chan struct{})}
	done := make(chan error, 1)
	s.instagram = b
	go func() { _, err := s.Work(context.Background()); done <- err }()
	<-b.started
	receive(t, s, event(types.Message, "m1", "person@example.com"))
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
	db, err := openTestRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	seedRule(t, db)
	s := NewService(db, resolver.NewService(db), fake.New(), "")
	receive(t, s, event(types.Comment, "c1", "pricing"))
	if _, _, err = s.repo.ClaimAction(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openTestRepository(path)
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
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, sender)
	work(t, s, sender)
	receive(t, s, event(types.Comment, "c2", "pricing"))
	now = now.Add(24 * time.Hour)
	receive(t, s, event(types.Message, "m1", "person@example.com"))
	if _, err := s.Work(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := state(t, db.DB); got != "expired" {
		t.Fatal(got)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n != 0 {
		t.Fatal(n)
	}
	receive(t, s, event(types.Comment, "c2", "pricing"))
	if n := count(t, db.DB, `SELECT count(*) FROM runs`); n != 1 {
		t.Fatal(n)
	}
}

type fixedResultSender struct{ result entity.SendResult }

func (f fixedResultSender) ReplyComment(context.Context, string, string) (string, error) {
	return "reply", f.err()
}

func (f fixedResultSender) PrivateReply(_ context.Context, accountID, commentID, contactID, text string) (instagram.Message, error) {
	return instagram.Message{ID: "question", RecipientID: contactID}, f.err()
}

func (f fixedResultSender) SendMessage(_ context.Context, accountID, contactID, text string) (instagram.Message, error) {
	return instagram.Message{ID: "link", RecipientID: contactID}, f.err()
}

func (f fixedResultSender) err() error {
	if f.result.Outcome() == types.Accepted {
		return nil
	}

	if f.result.Outcome() == types.Rejected {
		return errs.ErrRequestRejected
	}

	return errors.New(f.result.ErrorCode())
}

func TestMessageBeforeQuestionIsNotReused(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	receive(t, s, event(types.Comment, "c1", "pricing"))
	receive(t, s, event(types.Message, "early", "person@example.com"))
	work(t, s, fake.New())
	work(t, s, fake.New())
	if got := state(t, db.DB); got != "waiting_email" {
		t.Fatalf("state: %s", got)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n != 0 {
		t.Fatalf("link actions: %d", n)
	}
}

func TestReceiveUsesSingleTimeAtDeadline(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, fake.New())
	work(t, s, fake.New())
	now = now.Add(24*time.Hour - time.Millisecond)
	calls := 0
	s.now = func() time.Time { calls++; result := now; now = now.Add(2 * time.Millisecond); return result }
	receive(t, s, event(types.Message, "m1", "person@example.com"))
	if got := state(t, db.DB); got != "link_pending" {
		t.Fatalf("state: %s", got)
	}
	if calls != 1 {
		t.Fatalf("clock calls: %d", calls)
	}
}

func TestErrorCodeSeparateFromOutcome(t *testing.T) {
	t.Parallel()
	db, s := setup(t)
	receive(t, s, event(types.Comment, "c1", "pricing"))
	result := entity.NewSendResult()
	result.MarkUnknown("transport_error")
	work(t, s, fixedResultSender{result})
	var status, code string
	if err := db.DB.QueryRow(`SELECT status,error_code FROM actions`).Scan(&status, &code); err != nil {
		t.Fatal(err)
	}
	if status != "unknown" || code != "transport_error" {
		t.Fatalf("status=%s code=%s", status, code)
	}
}

func TestSendLinkRequiresOpenMessageWindow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		delta   time.Duration
		missing bool
		want    string
	}{
		{"expired", -24 * time.Hour, false, "message_window_expired"},
		{"unknown", 0, true, "message_time_unknown"},
		{"future", time.Second, false, "message_time_future"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, s := setup(t)
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			s.now = func() time.Time { return now }
			receive(t, s, event(types.Comment, "c1", "pricing"))
			work(t, s, fake.New())
			work(t, s, fake.New())
			message := event(types.Message, "m1", "person@example.com")
			if tc.missing {
				message.OccurredAt = time.Time{}
			} else {
				message.OccurredAt = now.Add(tc.delta)
			}
			receive(t, s, message)
			result := entity.NewSendResult()
			result.Accept("sent")
			work(t, s, fixedResultSender{result})
			var actionStatus, code string
			if err := db.DB.QueryRow(`SELECT status,error_code FROM actions WHERE step='send_link'`).Scan(&actionStatus, &code); err != nil {
				t.Fatal(err)
			}
			if actionStatus != "rejected" || code != tc.want {
				t.Fatalf("status=%s code=%s", actionStatus, code)
			}
		})
	}
}

func TestMessageWindowCheckedAfterRestart(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "db")
	db, err := openTestRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	seedRule(t, db)
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewService(db, resolver.NewService(db), fake.New(), "")
	s.now = func() time.Time { return now }
	receive(t, s, event(types.Comment, "c1", "pricing"))
	work(t, s, fake.New())
	work(t, s, fake.New())
	message := event(types.Message, "m1", "person@example.com")
	message.OccurredAt = now
	receive(t, s, message)
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openTestRepository(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now = now.Add(24 * time.Hour)
	result := entity.NewSendResult()
	result.Accept("sent")
	s = NewService(db, resolver.NewService(db), fixedResultSender{result}, "")
	s.now = func() time.Time { return now }
	work(t, s, s.instagram)
	var code string
	if err := db.DB.QueryRow(`SELECT error_code FROM actions WHERE step='send_link'`).Scan(&code); err != nil {
		t.Fatal(err)
	}
	if code != "message_window_expired" {
		t.Fatalf("code: %s", code)
	}
}
