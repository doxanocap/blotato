package types

// ActionStatus includes queue states and the final platform outcome.
type ActionStatus string

const (
	ActionPending   ActionStatus = "pending"
	ActionSending   ActionStatus = "sending"
	ActionAccepted  ActionStatus = "accepted"
	ActionRetryable ActionStatus = "retryable"
	ActionRejected  ActionStatus = "rejected"
	ActionUnknown   ActionStatus = "unknown"
)
