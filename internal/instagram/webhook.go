package instagram

import (
	"encoding/json"
	"net/http"

	"github.com/doxanocap/blotato/internal/adapter"
)

// Webhook receives Business Login for Instagram notifications. Receiver must
// persist and deduplicate events before returning nil.
func Webhook(accountID string, receiver adapter.Receiver) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if accountID == "" || receiver == nil {
			http.Error(w, "not configured", http.StatusServiceUnavailable)
			return
		}
		var data struct {
			Object string `json:"object"`
			Entry  []struct {
				ID    string `json:"id"`
				Field string `json:"field"`
				Value struct {
					ID   string `json:"id"`
					From struct {
						ID string `json:"id"`
					} `json:"from"`
					Text *string `json:"text"`
				} `json:"value"`
				Messaging []struct {
					Sender struct {
						ID string `json:"id"`
					} `json:"sender"`
					Recipient struct {
						ID string `json:"id"`
					} `json:"recipient"`
					IsSelf  bool `json:"is_self"`
					Message *struct {
						ID          string  `json:"mid"`
						Text        *string `json:"text"`
						Echo        bool    `json:"is_echo"`
						Self        bool    `json:"is_self"`
						Deleted     bool    `json:"is_deleted"`
						Unsupported bool    `json:"is_unsupported"`
					} `json:"message"`
				} `json:"messaging"`
			} `json:"entry"`
		}
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&data); err != nil || data.Object != "instagram" {
			http.Error(w, "invalid notification", http.StatusBadRequest)
			return
		}
		for _, entry := range data.Entry {
			if entry.ID != accountID {
				continue
			}
			if entry.Field == "comments" {
				v := entry.Value
				if v.From.ID != accountID {
					if v.ID == "" || v.From.ID == "" || v.Text == nil {
						http.Error(w, "incomplete comment", http.StatusBadRequest)
						return
					}
					event := adapter.Event{Platform: "instagram", AccountID: accountID, ContactID: v.From.ID, Kind: adapter.Comment, EventID: v.ID, Text: *v.Text}
					if err := receiver.Receive(r.Context(), event); err != nil {
						http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
						return
					}
				}
			}
			for _, m := range entry.Messaging {
				if m.Message == nil || m.IsSelf || m.Message.Self || m.Message.Echo || m.Message.Deleted || m.Message.Unsupported || m.Sender.ID == accountID {
					continue
				}
				if m.Message.Text == nil {
					continue
				}
				if m.Sender.ID == "" || m.Recipient.ID != accountID || m.Message.ID == "" {
					http.Error(w, "incomplete message", http.StatusBadRequest)
					return
				}
				event := adapter.Event{Platform: "instagram", AccountID: accountID, ContactID: m.Sender.ID, Kind: adapter.Message, EventID: m.Message.ID, Text: *m.Message.Text}
				if err := receiver.Receive(r.Context(), event); err != nil {
					http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
					return
				}
			}
		}
		w.WriteHeader(http.StatusOK)
	})
}
