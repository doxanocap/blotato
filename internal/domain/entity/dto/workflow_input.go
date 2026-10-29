package dto

import (
	"time"

	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

// ReceiveEventIn is a normalized platform event identified within an account.
type ReceiveEventIn struct {
	Platform   string
	AccountID  string
	ContactID  string
	Kind       types.EventType
	EventID    string
	Text       string
	OccurredAt time.Time
}

// SendActionIn describes one send; ID is not a platform idempotency key.
type SendActionIn struct {
	ID        string
	AccountID string
	ContactID string
	CommentID string
	Step      types.ActionStep
	Text      string
	AnswerAt  time.Time
}
