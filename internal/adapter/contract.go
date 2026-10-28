// Package adapter defines the boundary between a platform and the scenario.
package adapter

import "context"

// Event is an incoming platform event, identified within an account and kind.
type Event struct {
	Platform  string `json:"platform"`
	AccountID string `json:"account_id"`
	ContactID string `json:"contact_id"`
	Kind      Kind   `json:"kind"`
	EventID   string `json:"event_id"`
	Text      string `json:"text"`
}

type Kind string

const (
	Comment Kind = "comment"
	Message Kind = "message"
)

// Receiver persists an event before returning success, including duplicates.
type Receiver interface {
	Receive(ctx context.Context, event Event) error
}

// Command is a single logical outgoing action. ID is internal, not a provider
// idempotency key unless the platform explicitly supports it.
type Command struct {
	ID        string
	AccountID string
	ContactID string
	CommentID string
	Step      Step
	Text      string
}

type Step string

const (
	ReplyComment Step = "reply_comment"
	AskEmail     Step = "ask_email"
	SendLink     Step = "send_link"
)

type Outcome string

const (
	Accepted  Outcome = "accepted"
	Retryable Outcome = "retryable"
	Rejected  Outcome = "rejected"
	Unknown   Outcome = "unknown"
)

// Result reports acceptance by the platform, not delivery or reading.
// Retryable means the adapter knows the action did not happen. ErrorCode must
// be a safe classification, never a token, response body, or message text.
type Result struct {
	Outcome    Outcome
	ExternalID string
	ErrorCode  string
}

// Sender performs an external action. Ambiguous results must be Unknown.
type Sender interface {
	Send(ctx context.Context, command Command) Result
}
