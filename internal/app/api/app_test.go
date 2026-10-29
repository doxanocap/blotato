package api

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kelseyhightower/envconfig"
	"github.com/stretchr/testify/require"

	"github.com/doxanocap/blotato/internal/io/api/controllers/models"
	"github.com/doxanocap/blotato/internal/pkg/config"
)

type releaseSend struct {
	method string
	path   string
	auth   string
	body   []byte
}

func TestOpenRouterKeyRequiredWhenEmailIntentDisabled(t *testing.T) {
	cfg := appConfig{
		App:    config.App{Mode: "fake", Listen: "127.0.0.1:8080"},
		SQLite: config.SQLite{Path: "test.db"},
	}
	require.ErrorContains(t, cfg.Validate(), "OPENROUTER_API_KEY is required")
	cfg.Jev.APIKey = "test-openrouter-key"
	require.NoError(t, cfg.Validate())
}

func TestUnifiedLoginDefaultHost(t *testing.T) {
	t.Setenv("INSTAGRAM_API_BASE", "")
	// envconfig treats an unset variable as the instruction to use the default.
	require.NoError(t, os.Unsetenv("INSTAGRAM_API_BASE"))

	var cfg config.Instagram
	require.NoError(t, envconfig.Process("", &cfg))
	require.Equal(t, "https://graph.facebook.com/v26.0", cfg.Host)

	t.Setenv("INSTAGRAM_API_BASE", "http://127.0.0.1:12345/v26.0")
	require.NoError(t, envconfig.Process("", &cfg))
	require.Equal(t, "http://127.0.0.1:12345/v26.0", cfg.Host)
}

// This test uses newApp and Serve, the assembly and lifecycle used by Run.
// Only Graph API is replaced; API requests and SQLite are real.
func TestReleaseInstagramHTTPFlow(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var sends []releaseSend
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()
		sends = append(sends, releaseSend{method: r.Method, path: r.URL.Path, auth: r.Header.Get("Authorization"), body: body})
		mu.Unlock()

		var payload struct {
			Recipient struct {
				CommentID string `json:"comment_id"`
				ID        string `json:"id"`
			} `json:"recipient"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		var response string
		switch {
		case r.URL.Path == "/v26.0/comment-1/replies":
			response = `{"id":"public-1"}`
		case r.URL.Path == "/v26.0/brand/messages" && payload.Recipient.CommentID == "comment-1":
			response = `{"message_id":"question-1","recipient_id":"person"}`
		case r.URL.Path == "/v26.0/brand/messages" && payload.Recipient.ID == "person":
			response = `{"message_id":"link-1","recipient_id":"person"}`
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, response); err != nil {
			t.Errorf("write mock Meta response: %v", err)
		}
	}))
	t.Cleanup(meta.Close)
	requestCount := func() int {
		mu.Lock()
		defer mu.Unlock()
		return len(sends)
	}

	cfg := appConfig{
		App: config.App{
			Mode:            "instagram",
			AdminToken:      "test-admin",
			SwaggerBasePath: "/",
		},
		SQLite: config.SQLite{Path: filepath.Join(t.TempDir(), "release.db")},
		Instagram: config.Instagram{
			Host:           meta.URL + "/v26.0",
			AccountID:      "brand",
			Token:          "test-instagram-token",
			RequestTimeout: 2 * time.Second,
		},
		Meta: config.Meta{AppSecret: "test-secret", VerifyToken: "test-verify"},
		Jev:  config.Jev{APIKey: "test-openrouter-key"},
	}
	app, baseURL, stop := startReleaseApp(t, cfg)
	client := &http.Client{Timeout: 2 * time.Second}
	admin := map[string]string{"Authorization": "Bearer test-admin"}

	status, _ := releaseRequest(t, client, http.MethodGet, baseURL+"/api/v1/workflows", nil, nil)
	require.Equal(t, http.StatusUnauthorized, status)
	for _, path := range []string{"/events", "/fake/outcome"} {
		status, _ := releaseRequest(t, client, http.MethodPost, baseURL+path, []byte(`{}`), nil)
		require.Equal(t, http.StatusNotFound, status)
	}

	status, ui := releaseRequest(t, client, http.MethodGet, baseURL+"/swagger/index.html", nil, nil)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, string(ui), "SwaggerUIBundle")
	status, spec := releaseRequest(t, client, http.MethodGet, baseURL+"/swagger/doc.json", nil, nil)
	require.Equal(t, http.StatusOK, status)
	var document struct {
		Swagger string                     `json:"swagger"`
		Paths   map[string]json.RawMessage `json:"paths"`
	}
	require.NoError(t, json.Unmarshal(spec, &document))
	require.Equal(t, "2.0", document.Swagger)
	require.Contains(t, document.Paths, "/api/v1/workflows")
	require.Contains(t, document.Paths, "/webhook/instagram")

	rule := []byte(`{"keyword":"pricing","public_reply":"Public reply","email_question":"Email question?","link":"https://example.com/pricing"}`)
	status, created := releaseRequest(t, client, http.MethodPost, baseURL+"/api/v1/workflows", rule, admin)
	require.Equal(t, http.StatusCreated, status)
	var saved models.Workflow
	require.NoError(t, json.Unmarshal(created, &saved))
	require.Positive(t, saved.ID)

	// Unified Login fixture: https://developers.facebook.com/documentation/instagram-platform/webhooks/examples/
	// Identifiers and text are synthetic; omitted media metadata is not consumed.
	comment := []byte(`{"object":"instagram","entry":[{"id":"brand","changes":[{"field":"comments","value":{"comment_id":"comment-1","from":{"id":"person"},"text":"pricing"}}]}]}`)
	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", comment, nil)
	require.Equal(t, http.StatusForbidden, status)
	signature := releaseSignature(comment, cfg.Meta.AppSecret)
	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", append(append([]byte(nil), comment...), '\n'), signature)
	require.Equal(t, http.StatusForbidden, status, "signature must cover the original bytes, including whitespace")
	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", comment, signature)
	require.Equal(t, http.StatusOK, status)
	var inboxCount int
	require.NoError(t, app.pool.QueryRow("SELECT count(*) FROM inbox").Scan(&inboxCount))
	require.Equal(t, 1, inboxCount, "webhook acknowledgement must follow durable receipt")
	waitReleaseRuns(t, client, baseURL, admin, "waiting_email", 2)
	require.Equal(t, 2, requestCount())

	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", comment, signature)
	require.Equal(t, http.StatusOK, status)
	// A notification from the previous login format has the same dedupe identity.
	legacyComment := []byte(`{"object":"instagram","entry":[{"id":"brand","field":"comments","value":{"id":"comment-1","from":{"id":"person"},"text":"pricing"}}]}`)
	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", legacyComment, releaseSignature(legacyComment, cfg.Meta.AppSecret))
	require.Equal(t, http.StatusOK, status)
	stop()

	app, baseURL, _ = startReleaseApp(t, cfg)
	waitReleaseRuns(t, client, baseURL, admin, "waiting_email", 2)
	status, listed := releaseRequest(t, client, http.MethodGet, baseURL+"/api/v1/workflows", nil, admin)
	require.Equal(t, http.StatusOK, status)
	var rules []models.Workflow
	require.NoError(t, json.Unmarshal(listed, &rules))
	require.Equal(t, []models.Workflow{saved}, rules)
	require.Equal(t, 2, requestCount(), "restart must not resend the question")

	// Messaging fixture: https://developers.facebook.com/documentation/business-messaging/instagram-messaging/webhooks
	messageTime := time.Now().UnixMilli()
	message, err := json.Marshal(map[string]any{
		"object": "instagram",
		"entry": []any{map[string]any{
			"id": "brand",
			"messaging": []any{map[string]any{
				"timestamp": messageTime,
				"sender":    map[string]string{"id": "person"},
				"recipient": map[string]string{"id": "brand"},
				"message":   map[string]string{"mid": "message-1", "text": "release@example.com"},
			}},
		}},
	})
	require.NoError(t, err)
	messageSignature := releaseSignature(message, cfg.Meta.AppSecret)
	for range 2 {
		status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", message, messageSignature)
		require.Equal(t, http.StatusOK, status)
	}
	waitReleaseRuns(t, client, baseURL, admin, "completed", 3)

	// Redelivery after completion must not create another run or another send.
	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", comment, signature)
	require.Equal(t, http.StatusOK, status)
	status, _ = releaseRequest(t, client, http.MethodPost, baseURL+"/webhook/instagram", message, messageSignature)
	require.Equal(t, http.StatusOK, status)
	var runs, actions int
	require.NoError(t, app.pool.QueryRow("SELECT count(*) FROM runs").Scan(&runs))
	require.NoError(t, app.pool.QueryRow("SELECT count(*) FROM actions").Scan(&actions))
	require.NoError(t, app.pool.QueryRow("SELECT count(*) FROM inbox").Scan(&inboxCount))
	require.Equal(t, 1, runs)
	require.Equal(t, 3, actions)
	require.Equal(t, 2, inboxCount)
	var occurredAt, answerAt int64
	require.NoError(t, app.pool.QueryRow("SELECT occurred_at FROM inbox WHERE event_id='message-1'").Scan(&occurredAt))
	require.NoError(t, app.pool.QueryRow("SELECT answer_at FROM runs").Scan(&answerAt))
	require.Equal(t, messageTime, occurredAt)
	require.Equal(t, messageTime, answerAt, "DM window must use the timestamp of the person's message")

	mu.Lock()
	requests := append([]releaseSend(nil), sends...)
	mu.Unlock()
	require.Len(t, requests, 3)
	for _, request := range requests {
		require.Equal(t, http.MethodPost, request.method)
		require.Equal(t, "Bearer test-instagram-token", request.auth)
	}
	require.Equal(t, "/v26.0/comment-1/replies", requests[0].path)
	require.JSONEq(t, `{"message":"Public reply"}`, string(requests[0].body))
	require.Equal(t, "/v26.0/brand/messages", requests[1].path)
	require.JSONEq(t, `{"recipient":{"comment_id":"comment-1"},"message":{"text":"Email question?"}}`, string(requests[1].body))
	require.Equal(t, "/v26.0/brand/messages", requests[2].path)
	require.JSONEq(t, `{"recipient":{"id":"person"},"message":{"text":"https://example.com/pricing"}}`, string(requests[2].body))
	t.Log("confirmed: signed HTTP webhooks -> SQLite -> public reply -> private reply -> restart waiting_email -> DM link -> completed; 3 sends, 1 run, 2 inbox events")
}

func startReleaseApp(t *testing.T, cfg appConfig) (*App, string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	cfg.App.Listen = listener.Addr().String()
	require.NoError(t, listener.Close())
	app, err := newApp(context.Background(), cfg)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Serve(ctx) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				require.NoError(t, errors.Join(err, app.pool.Close()))
			case <-time.After(7 * time.Second):
				t.Error("application did not stop")
			}
		})
	}
	t.Cleanup(stop)
	baseURL := "http://" + cfg.App.Listen
	client := &http.Client{Timeout: time.Second}
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get(baseURL + "/swagger/index.html")
		if err == nil {
			require.NoError(t, response.Body.Close())
			if response.StatusCode == http.StatusOK {
				return app, baseURL, stop
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("application did not become ready")
	return nil, "", stop
}

func releaseRequest(t *testing.T, client *http.Client, method, url string, body []byte, headers map[string]string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, url, bytes.NewReader(body))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := client.Do(request)
	require.NoError(t, err)
	defer func() { require.NoError(t, response.Body.Close()) }()
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, data
}

func releaseSignature(body []byte, secret string) map[string]string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return map[string]string{"X-Hub-Signature-256": "sha256=" + hex.EncodeToString(mac.Sum(nil))}
}

func waitReleaseRuns(t *testing.T, client *http.Client, baseURL string, headers map[string]string, state string, count int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); {
		status, body := releaseRequest(t, client, http.MethodGet, baseURL+"/api/v1/workflows/runs", nil, headers)
		require.Equal(t, http.StatusOK, status)
		require.NotContains(t, string(body), "release@example.com")
		var runs []models.Run
		require.NoError(t, json.Unmarshal(body, &runs))
		ready := len(runs) == count
		for _, run := range runs {
			ready = ready && run.State == state && run.Status == "accepted"
		}
		if ready {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("runs did not reach %s with %d accepted actions", state, count)
}
