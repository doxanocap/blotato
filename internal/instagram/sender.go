package instagram

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/doxanocap/blotato/internal/adapter"
)

// Sender uses Instagram Login and an Instagram user access token. APIBase is
// the versioned Graph URL, for example https://graph.instagram.com/v25.0.
type Sender struct {
	Client      *http.Client
	APIBase     string
	AccountID   string
	AccessToken string
	Timeout     time.Duration
}

func (s *Sender) Send(ctx context.Context, c adapter.Command) adapter.Result {
	reject := func(code string) adapter.Result { return adapter.Result{Outcome: adapter.Rejected, ErrorCode: code} }
	unknown := func(code string) adapter.Result { return adapter.Result{Outcome: adapter.Unknown, ErrorCode: code} }
	if s.Client == nil || !pathID(s.AccountID) || s.AccessToken == "" || s.Timeout <= 0 || c.AccountID != s.AccountID || c.Text == "" || !utf8.ValidString(c.Text) {
		return reject("invalid_configuration_or_command")
	}
	base, err := url.Parse(s.APIBase)
	if err != nil || (base.Scheme != "https" && base.Scheme != "http") || base.Host == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return reject("invalid_api_base")
	}
	if base.Scheme == "http" {
		ip := net.ParseIP(base.Hostname())
		if base.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return reject("invalid_api_base")
		}
	}
	var endpoint string
	var payload any
	switch c.Step {
	case adapter.ReplyComment:
		if !pathID(c.CommentID) {
			return reject("missing_comment_id")
		}
		endpoint = url.PathEscape(c.CommentID) + "/replies"
		payload = map[string]any{"message": c.Text}
	case adapter.AskEmail, adapter.SendLink:
		if len(c.Text) > 1000 {
			return reject("message_too_long")
		}
		recipient := map[string]string{}
		if c.Step == adapter.AskEmail {
			if c.CommentID == "" {
				return reject("missing_comment_id")
			}
			recipient["comment_id"] = c.CommentID
		} else {
			if c.ContactID == "" {
				return reject("missing_contact_id")
			}
			recipient["id"] = c.ContactID
		}
		endpoint = url.PathEscape(s.AccountID) + "/messages"
		payload = map[string]any{"recipient": recipient, "message": map[string]string{"text": c.Text}}
	default:
		return reject("invalid_step")
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/" + endpoint
	raw, err := json.Marshal(payload)
	if err != nil {
		return reject("invalid_command")
	}
	ctx, cancel := context.WithTimeout(ctx, s.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base.String(), bytes.NewReader(raw))
	if err != nil {
		return reject("invalid_request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.AccessToken)
	// Do not follow redirects: a redirected request could leak the token or be resent.
	client := *s.Client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		return unknown("transport_error")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 65537))
	if err != nil || len(body) > 65536 {
		return unknown("invalid_response")
	}
	var data struct {
		ID          string `json:"id"`
		MessageID   string `json:"message_id"`
		RecipientID string `json:"recipient_id"`
		Error       *struct {
			Code json.Number `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &data) != nil {
		return unknown("invalid_response")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 && data.Error == nil {
		id := data.MessageID
		if c.Step == adapter.ReplyComment {
			id = data.ID
		}
		if id == "" || (c.Step != adapter.ReplyComment && data.RecipientID == "") {
			return unknown("unconfirmed_response")
		}
		if c.Step != adapter.ReplyComment && c.ContactID != "" && data.RecipientID != c.ContactID {
			return unknown("recipient_mismatch")
		}
		return adapter.Result{Outcome: adapter.Accepted, ExternalID: id}
	}
	if resp.StatusCode == 400 && data.Error != nil && data.Error.Code == "100" {
		return reject("provider_rejected")
	}
	return unknown("ambiguous_response")
}

func pathID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.ContainsAny(id, "/\\?#%")
}

var _ adapter.Sender = (*Sender)(nil)
