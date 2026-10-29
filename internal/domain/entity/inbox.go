package entity

import (
	"time"

	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

// InboxMessage is a message deferred while its email question is being sent.
type InboxMessage struct {
	id         types.ID
	receivedAt time.Time
	text       string
	occurredAt time.Time
}

// ScanInboxMessage restores receipt time separately from the platform timestamp.
func ScanInboxMessage(id types.ID, receivedAt time.Time, text string, occurredAt time.Time) InboxMessage {
	return InboxMessage{
		id:         id,
		receivedAt: receivedAt,
		text:       text,
		occurredAt: occurredAt,
	}
}

func (m InboxMessage) ID() types.ID          { return m.id }
func (m InboxMessage) ReceivedAt() time.Time { return m.receivedAt }
func (m InboxMessage) Text() string          { return m.text }
func (m InboxMessage) OccurredAt() time.Time { return m.occurredAt }
