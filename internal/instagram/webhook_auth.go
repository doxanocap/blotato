// Package instagram implements the Instagram integration.
package instagram

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

// VerifyWebhook authenticates the subscription handshake and POST body before
// passing a delivery to next. An empty secret or verify token fails closed.
func VerifyWebhook(verifyToken, secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if verifyToken == "" || secret == "" || next == nil {
			http.Error(w, "webhook not configured", http.StatusServiceUnavailable)
			return
		}
		switch r.Method {
		case http.MethodGet:
			q := r.URL.Query()
			if q.Get("hub.mode") != "subscribe" || q.Get("hub.challenge") == "" || subtle.ConstantTimeCompare([]byte(q.Get("hub.verify_token")), []byte(verifyToken)) != 1 {
				http.Error(w, "verification failed", http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = io.WriteString(w, q.Get("hub.challenge"))
		case http.MethodPost:
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
			if err != nil {
				http.Error(w, "invalid body", http.StatusBadRequest)
				return
			}
			sig := r.Header.Get("X-Hub-Signature-256")
			if !strings.HasPrefix(sig, "sha256=") {
				http.Error(w, "invalid signature", http.StatusForbidden)
				return
			}
			provided, err := hex.DecodeString(strings.TrimPrefix(sig, "sha256="))
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write(body)
			if err != nil || !hmac.Equal(provided, mac.Sum(nil)) {
				http.Error(w, "invalid signature", http.StatusForbidden)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		default:
			w.Header().Set("Allow", "GET, POST")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
