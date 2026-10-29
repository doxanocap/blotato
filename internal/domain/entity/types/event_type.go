package types

// EventType distinguishes normalized comments from incoming messages.
type EventType string

const (
	Comment EventType = "comment"
	Message EventType = "message"
)

func (kind EventType) IsValid() bool {
	return kind == Comment || kind == Message
}
