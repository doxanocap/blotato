package instagram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifyWebhook(t *testing.T) {
	t.Parallel()
	secret := "test-secret"
	body := "{ \"text\": \"hello\" }"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(body))
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	cases := []struct {
		name, method, target, body, signature string
		status                                int
		forward                               bool
	}{
		{"handshake", "GET", "/?hub.mode=subscribe&hub.verify_token=verify&hub.challenge=123", "", "", 200, false},
		{"wrong token", "GET", "/?hub.mode=subscribe&hub.verify_token=bad&hub.challenge=123", "", "", 403, false},
		{"valid signature", "POST", "/", body, signature, 204, true},
		{"changed raw body", "POST", "/", strings.ReplaceAll(body, " ", ""), signature, 403, false},
		{"missing signature", "POST", "/", body, "", 403, false},
		{"bad signature", "POST", "/", body, "sha256=aa", 403, false},
		{"method", "PUT", "/", "", "", 405, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called := false
			h := VerifyWebhook("verify", secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				got, err := io.ReadAll(r.Body)
				if err != nil || string(got) != body {
					t.Fatalf("body changed or unreadable")
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			req.Header.Set("X-Hub-Signature-256", tc.signature)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != tc.status || called != tc.forward {
				t.Fatalf("status=%d forwarded=%t", w.Code, called)
			}
			if tc.name == "handshake" && w.Body.String() != "123" {
				t.Fatal("challenge mismatch")
			}
		})
	}
}
