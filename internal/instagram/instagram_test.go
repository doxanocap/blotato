package instagram

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/doxanocap/blotato/internal/adapter"
)

type receiverFunc func(context.Context, adapter.Event) error

func (f receiverFunc) Receive(ctx context.Context, e adapter.Event) error { return f(ctx, e) }

// Shapes: https://developers.facebook.com/documentation/instagram-platform/webhooks/examples
const batch = `{"object":"instagram","entry":[{"id":"brand","field":"comments","value":{"id":"comment1","from":{"id":"person","username":"user"},"text":"pricing","media":{"id":"media","media_product_type":"FEED"}}},{"id":"brand","messaging":[{"sender":{"id":"person"},"recipient":{"id":"brand"},"message":{"mid":"msg1","text":"person@example.com"}},{"sender":{"id":"brand"},"recipient":{"id":"person"},"message":{"mid":"echo","text":"sent","is_echo":true}},{"sender":{"id":"person"},"recipient":{"id":"brand"},"postback":{"mid":"service"}}]}]}`

func TestWebhookBatchAndRedelivery(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	stored := map[string]adapter.Event{}
	fail := true
	receiver := receiverFunc(func(_ context.Context, e adapter.Event) error {
		mu.Lock()
		defer mu.Unlock()
		if e.EventID == "msg1" && fail {
			fail = false
			return errors.New("failure")
		}
		stored[e.EventID] = e
		return nil
	})
	h := VerifyWebhook("verify", "secret", Webhook("brand", receiver))
	deliver := func(body string) int {
		m := hmac.New(sha256.New, []byte("secret"))
		_, _ = m.Write([]byte(body))
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(m.Sum(nil)))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		return w.Code
	}
	if got := deliver(batch); got != 503 {
		t.Fatalf("first status=%d", got)
	}
	if got := deliver(batch); got != 200 {
		t.Fatalf("second status=%d", got)
	}
	if len(stored) != 2 || stored["comment1"].ContactID != "person" || stored["comment1"].Kind != adapter.Comment || stored["msg1"].Kind != adapter.Message {
		t.Fatalf("wrong stored events: %#v", stored)
	}
	if got := deliver(`{"object":"instagram","entry":[{"id":"brand","field":"other"}]}`); got != 200 {
		t.Fatalf("unsupported status=%d", got)
	}
	if got := deliver(`{"object":"instagram","entry":[{"id":"brand","field":"comments","value":{"from":{"id":"person"},"text":"pricing"}}]}`); got != 400 {
		t.Fatalf("missing id status=%d", got)
	}
}

func TestSenderRequests(t *testing.T) {
	t.Parallel()
	cases := []struct {
		step adapter.Step
		path string
		want string
	}{
		{adapter.ReplyComment, "/v25.0/comment1/replies", `{"message":"text"}`},
		{adapter.AskEmail, "/v25.0/brand/messages", `{"message":{"text":"text"},"recipient":{"comment_id":"comment1"}}`},
		{adapter.SendLink, "/v25.0/brand/messages", `{"message":{"text":"text"},"recipient":{"id":"person"}}`},
	}
	for _, tc := range cases {
		t.Run(string(tc.step), func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != tc.path || r.Header.Get("Authorization") != "Bearer test-token" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				raw, _ := io.ReadAll(r.Body)
				var a, b any
				if json.Unmarshal(raw, &a) != nil || json.Unmarshal([]byte(tc.want), &b) != nil {
					t.Error("invalid json")
				}
				aa, _ := json.Marshal(a)
				bb, _ := json.Marshal(b)
				if string(aa) != string(bb) {
					t.Errorf("unexpected body %s", raw)
				}
				if tc.step == adapter.ReplyComment {
					_, _ = io.WriteString(w, `{"id":"out1"}`)
				} else {
					_, _ = io.WriteString(w, `{"recipient_id":"person","message_id":"out1"}`)
				}
			}))
			defer srv.Close()
			s := Sender{Client: srv.Client(), APIBase: srv.URL + "/v25.0", AccountID: "brand", AccessToken: "test-token", Timeout: time.Second}
			got := s.Send(context.Background(), adapter.Command{AccountID: "brand", CommentID: "comment1", ContactID: "person", Step: tc.step, Text: "text"})
			if got.Outcome != adapter.Accepted || got.ExternalID != "out1" {
				t.Fatalf("result: %#v", got)
			}
		})
	}
}

func TestSenderOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		code    int
		body    string
		close   bool
		outcome adapter.Outcome
	}{
		{"rejected", 400, `{"error":{"code":100}}`, false, adapter.Rejected},
		{"rate limited", 429, `{"error":{"code":4}}`, false, adapter.Unknown},
		{"server error", 500, `{"error":{"code":2}}`, false, adapter.Unknown},
		{"malformed success", 200, `{}`, false, adapter.Unknown},
		{"different recipient", 200, `{"recipient_id":"other","message_id":"out1"}`, false, adapter.Unknown},
		{"lost response", 0, "", true, adapter.Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.close {
					c, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					_ = c.Close()
					return
				}
				w.WriteHeader(tc.code)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer srv.Close()
			s := Sender{Client: srv.Client(), APIBase: srv.URL + "/v25.0", AccountID: "brand", AccessToken: "token", Timeout: time.Second}
			result := s.Send(context.Background(), adapter.Command{AccountID: "brand", CommentID: "c", ContactID: "person", Step: adapter.AskEmail, Text: "question"})
			if result.Outcome != tc.outcome || calls.Load() != 1 {
				t.Fatalf("result=%#v calls=%d", result, calls.Load())
			}
		})
	}
}
