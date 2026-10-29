package instagram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/pkg/config"
	"github.com/doxanocap/blotato/internal/pkg/httpc"
)

type receivedRequest struct {
	method string
	path   string
	auth   string
	body   []byte
}

type tenv struct {
	ctx           context.Context
	client        *Client
	mu            sync.Mutex
	request       receivedRequest
	calls         atomic.Int32
	status        int
	response      string
	disconnect    bool
	waitForCancel bool
}

func newTenv(t *testing.T, opts ...httpc.Option) *tenv {
	t.Helper()

	env := &tenv{
		ctx:      context.Background(),
		status:   http.StatusOK,
		response: `{"recipient_id":"person","message_id":"out1"}`,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		env.calls.Add(1)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}

		env.mu.Lock()
		env.request = receivedRequest{
			method: r.Method,
			path:   r.URL.Path,
			auth:   r.Header.Get("Authorization"),
			body:   body,
		}
		status := env.status
		response := env.response
		disconnect := env.disconnect
		waitForCancel := env.waitForCancel
		env.mu.Unlock()

		if waitForCancel {
			<-r.Context().Done()
			return
		}

		if disconnect {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}

			_ = conn.Close()
			return
		}

		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(server.Close)

	opts = append([]httpc.Option{httpc.WithHTTPClient(server.Client())}, opts...)

	client, err := New(config.Instagram{
		Host:           server.URL + "/v25.0",
		AccountID:      "brand",
		Token:          "test-token",
		RequestTimeout: time.Second,
	}, opts...)
	require.NoError(t, err)

	env.client = client

	return env
}

func TestTimeoutReturnsErrorWithoutRetry(t *testing.T) {
	t.Parallel()

	env := newTenv(t, httpc.WithTimeout(50*time.Millisecond))
	env.mu.Lock()
	env.waitForCancel = true
	env.mu.Unlock()

	_, err := env.client.PrivateReply(env.ctx, "brand", "c", "person", "question")

	require.Error(t, err)
	require.EqualValues(t, 1, env.calls.Load())
}

func TestDirectMessagesPreserveRecipient(t *testing.T) {
	t.Parallel()

	for _, step := range []types.ActionStep{types.AskEmail, types.SendLink} {
		t.Run(string(step), func(t *testing.T) {
			t.Parallel()

			env := newTenv(t)
			env.mu.Lock()
			env.response = `{"recipient_id":"other","message_id":"out1"}`
			env.mu.Unlock()

			var res Message
			var err error
			if step == types.AskEmail {
				res, err = env.client.PrivateReply(env.ctx, "brand", "comment1", "person", "text")
			} else {
				res, err = env.client.SendMessage(env.ctx, "brand", "person", "text")
			}

			require.NoError(t, err)
			require.Equal(t, "out1", res.ID)
			require.Equal(t, "other", res.RecipientID)
			require.EqualValues(t, 1, env.calls.Load())
		})
	}
}

func TestSenderRequests(t *testing.T) {
	t.Parallel()

	cases := []struct {
		step types.ActionStep
		path string
		want string
	}{
		{
			step: types.ReplyComment,
			path: "/v25.0/comment1/replies",
			want: `{"message":"text"}`,
		},
		{
			step: types.AskEmail,
			path: "/v25.0/brand/messages",
			want: `{"message":{"text":"text"},"recipient":{"comment_id":"comment1"}}`,
		},
		{
			step: types.SendLink,
			path: "/v25.0/brand/messages",
			want: `{"message":{"text":"text"},"recipient":{"id":"person"}}`,
		},
	}

	for _, tc := range cases {
		t.Run(string(tc.step), func(t *testing.T) {
			t.Parallel()

			env := newTenv(t)
			if tc.step == types.ReplyComment {
				env.mu.Lock()
				env.response = `{"id":"out1"}`
				env.mu.Unlock()
			}

			id, err := sendForTest(env.ctx, env.client, tc.step, "comment1", "person", "text")

			require.NoError(t, err)
			require.Equal(t, "out1", id)

			env.mu.Lock()
			request := env.request
			env.mu.Unlock()

			if request.method != "POST" || request.path != tc.path || request.auth != "Bearer test-token" {
				t.Fatalf("unexpected request %s %s", request.method, request.path)
			}

			var actual, expected any
			if json.Unmarshal(request.body, &actual) != nil || json.Unmarshal([]byte(tc.want), &expected) != nil {
				t.Fatal("invalid json")
			}

			actualJSON, _ := json.Marshal(actual)
			expectedJSON, _ := json.Marshal(expected)
			if string(actualJSON) != string(expectedJSON) {
				t.Fatalf("unexpected body %s", request.body)
			}
		})
	}
}

func TestSenderResponses(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		code    int
		body    string
		close   bool
		wantErr bool
	}{
		{
			name:    "unconfirmed 400 code 100",
			code:    400,
			body:    `{"error":{"code":100}}`,
			close:   false,
			wantErr: true,
		},
		{
			name:    "rate limited",
			code:    429,
			body:    `{"error":{"code":4}}`,
			close:   false,
			wantErr: true,
		},
		{
			name:    "server error",
			code:    500,
			body:    `{"error":{"code":2}}`,
			close:   false,
			wantErr: true,
		},
		{
			name:    "empty confirmation",
			code:    200,
			body:    `{}`,
			close:   false,
			wantErr: false,
		},
		{
			name:    "different recipient",
			code:    200,
			body:    `{"recipient_id":"other","message_id":"out1"}`,
			close:   false,
			wantErr: false,
		},
		{
			name:    "lost response",
			code:    0,
			body:    "",
			close:   true,
			wantErr: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			env := newTenv(t)
			env.mu.Lock()
			env.status = tc.code
			env.response = tc.body
			env.disconnect = tc.close
			env.mu.Unlock()

			_, err := env.client.PrivateReply(env.ctx, "brand", "c", "person", "question")

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.EqualValues(t, 1, env.calls.Load())
		})
	}
}

func TestRedirectDoesNotResendOrExposeToken(t *testing.T) {
	t.Parallel()

	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(target.Close)

	var sourceCalls atomic.Int32
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceCalls.Add(1)
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	t.Cleanup(source.Close)

	client, err := New(config.Instagram{
		Host:           source.URL,
		AccountID:      "brand",
		Token:          "test-token",
		RequestTimeout: time.Second,
	}, httpc.WithHTTPClient(source.Client()))
	require.NoError(t, err)

	_, err = client.PrivateReply(context.Background(), "brand", "c1", "person", "email?")

	require.Error(t, err)
	require.EqualValues(t, 1, sourceCalls.Load())
	require.Zero(t, targetCalls.Load())
}

func TestUntrustedTLSIsNotAccepted(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client, err := New(config.Instagram{
		Host:           server.URL,
		AccountID:      "brand",
		Token:          "test-token",
		RequestTimeout: time.Second,
	}, httpc.WithHTTPClient(&http.Client{}))
	require.NoError(t, err)

	_, err = client.PrivateReply(context.Background(), "brand", "c1", "person", "email?")

	require.Error(t, err)
	require.Zero(t, calls.Load())
}

func sendForTest(
	ctx context.Context,
	client *Client,
	step types.ActionStep,
	commentID string,
	contactID string,
	text string,
) (string, error) {
	switch step {
	case types.ReplyComment:
		return client.ReplyComment(ctx, commentID, text)
	case types.AskEmail:
		message, err := client.PrivateReply(ctx, "brand", commentID, contactID, text)

		return message.ID, err
	default:
		message, err := client.SendMessage(ctx, "brand", contactID, text)

		return message.ID, err
	}
}
