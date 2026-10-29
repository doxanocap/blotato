package workflow

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doxanocap/blotato/internal/clients/fake"
	"github.com/doxanocap/blotato/internal/clients/jev"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

func TestEmailDecisionHTTP(t *testing.T) {
	for _, tc := range []struct {
		name, message, response, want string
		calls                         int
	}{
		{"accept", "Write to anna@example.com", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":0.99}}}`, "link_pending", 1},
		{"period", "Write to anna@example.com.", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":0.99}}}`, "link_pending", 1},
		{"comma", "Write to anna@example.com, please", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":0.99}}}`, "link_pending", 1},
		{"parentheses", "Write to (anna@example.com)", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":0.99}}}`, "link_pending", 1},
		{"malformed dots", "Write to anna@example.com..", ``, "waiting_email", 0},
		{"deny", "Do not use anna@example.com", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"reject","confidence":0.99}}}`, "waiting_email", 1},
		{"uncertain", "Maybe anna@example.com", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"uncertain","confidence":0.99}}}`, "waiting_email", 1},
		{"low confidence", "Use anna@example.com", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":0.7}}}`, "waiting_email", 1},
		{"bad json", "Use anna@example.com", `{`, "waiting_email", 1},
		{"unknown", "Use anna@example.com", `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"other","confidence":1}}}`, "waiting_email", 1},
		{"multiple", "old@example.com or new@example.com", ``, "waiting_email", 0},
		{"invalid address", "Use anna@@example.com", ``, "waiting_email", 0},
		{"clean", "anna@example.com", ``, "link_pending", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				buf := make([]byte, 8192)
				n, _ := r.Body.Read(buf)
				if strings.Contains(string(buf[:n]), "anna@example.com") {
					t.Error("address sent to API")
				}
				fmt.Fprint(w, tc.response)
			}))
			defer server.Close()
			db, s := setup(t)
			s.SetEmailClassifier(jev.New("test", server.URL))
			receive(t, s, event(types.Comment, "comment", "pricing"))
			work(t, s, fake.New())
			work(t, s, fake.New())
			m := event(types.Message, "message", tc.message)
			receive(t, s, m)
			receive(t, s, m)
			if tc.name != "clean" {
				if _, err := s.WorkEmailDecision(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if got := state(t, db.DB); got != tc.want {
				t.Fatalf("state %s", got)
			}
			if int(calls.Load()) != tc.calls {
				t.Fatalf("calls %d", calls.Load())
			}
			if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n > 1 {
				t.Fatal(n)
			}
			if tc.want == "link_pending" {
				var email string
				if err := db.DB.QueryRow(`SELECT email FROM runs`).Scan(&email); err != nil || email != "anna@example.com" {
					t.Fatalf("email %q: %v", email, err)
				}
			}
		})
	}
}

func TestSlowEmailDecisionDoesNotBlockActionsOrExpiry(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	gate := release
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-gate
		fmt.Fprint(w, `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":1}}}`)
	}))
	defer server.Close()
	db, s := setup(t)
	s.SetEmailClassifier(jev.New("test", server.URL))
	receive(t, s, event(types.Comment, "c", "pricing"))
	work(t, s, fake.New())
	work(t, s, fake.New())
	receive(t, s, event(types.Message, "m", "Use anna@example.com"))
	done := make(chan error, 1)
	go func() { _, err := s.WorkEmailDecision(context.Background()); done <- err }()
	<-started
	defer func() {
		if release != nil {
			close(release)
			<-done
		}
	}()
	if _, err := db.DB.Exec(`UPDATE runs SET deadline=? WHERE contact_id='person'`, time.Now().Add(-time.Second).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	other := event(types.Comment, "other", "pricing")
	other.ContactID = "other"
	receive(t, s, other)
	if worked, err := s.Work(context.Background()); err != nil || !worked {
		t.Fatalf("work: %v %v", worked, err)
	}
	var expired, next string
	if err := db.DB.QueryRow(`SELECT state FROM runs WHERE contact_id='person'`).Scan(&expired); err != nil {
		t.Fatal(err)
	}
	if err := db.DB.QueryRow(`SELECT state FROM runs WHERE contact_id='other'`).Scan(&next); err != nil {
		t.Fatal(err)
	}
	if expired != "expired" || next != "question_pending" {
		t.Fatalf("expired=%s other=%s", expired, next)
	}
	close(release)
	release = nil
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n != 0 {
		t.Fatal(n)
	}
}

func TestEmailDecisionConcurrentCleanAnswer(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":1}}}`)
	}))
	defer server.Close()
	db, s := setup(t)
	s.SetEmailClassifier(jev.New("test", server.URL))
	receive(t, s, event(types.Comment, "c", "pricing"))
	work(t, s, fake.New())
	work(t, s, fake.New())
	receive(t, s, event(types.Message, "m", "Use anna@example.com"))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _, _ = s.WorkEmailDecision(context.Background()) }()
	<-started
	receive(t, s, event(types.Message, "m2", "other@example.com"))
	close(release)
	wg.Wait()
	if n := count(t, db.DB, `SELECT count(*) FROM actions WHERE step='send_link'`); n != 1 {
		t.Fatal(n)
	}
	var email string
	if err := db.DB.QueryRow(`SELECT email FROM runs`).Scan(&email); err != nil || email != "other@example.com" {
		t.Fatalf("email %q: %v", email, err)
	}
}

func TestEmailDecisionTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(5100 * time.Millisecond) }))
	defer server.Close()
	db, s := setup(t)
	s.SetEmailClassifier(jev.New("test", server.URL))
	receive(t, s, event(types.Comment, "c", "pricing"))
	work(t, s, fake.New())
	work(t, s, fake.New())
	receive(t, s, event(types.Message, "m", "Use anna@example.com"))
	if _, err := s.WorkEmailDecision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := state(t, db.DB); got != "waiting_email" {
		t.Fatal(got)
	}
	if n := count(t, db.DB, `SELECT count(*) FROM email_decisions WHERE status='done' AND decision='error'`); n != 1 {
		t.Fatal(n)
	}
}

func TestEarlyEmailDecision(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"model":"typesafe/jev-1.14-20261014","answers":{"intent":{"type":"choice","choice":"EMAIL_0","confidence":1}}}`)
	}))
	defer server.Close()
	db, s := setup(t)
	s.SetEmailClassifier(jev.New("test", server.URL))
	receive(t, s, event(types.Comment, "c", "pricing"))
	work(t, s, fake.New())
	b := &blockingSender{make(chan struct{}), make(chan struct{})}
	s.instagram = b
	done := make(chan error, 1)
	go func() { _, err := s.Work(context.Background()); done <- err }()
	<-b.started
	receive(t, s, event(types.Message, "m", "Use anna@example.com"))
	close(b.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if _, err := s.WorkEmailDecision(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := state(t, db.DB); got != "link_pending" {
		t.Fatal(got)
	}
}
