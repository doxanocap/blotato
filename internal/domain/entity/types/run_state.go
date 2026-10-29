package types

// RunState is the persisted state of a workflow execution.
type RunState string

const (
	RunReplyPending    RunState = "reply_pending"
	RunQuestionPending RunState = "question_pending"
	RunWaitingEmail    RunState = "waiting_email"
	RunLinkPending     RunState = "link_pending"
	RunUnknown         RunState = "unknown"
	RunFailed          RunState = "failed"
	RunCompleted       RunState = "completed"
	RunExpired         RunState = "expired"
)

// ActiveRunStates matches the active_run unique index in SQLite.
func ActiveRunStates() []RunState {
	return []RunState{
		RunReplyPending,
		RunQuestionPending,
		RunWaitingEmail,
		RunLinkPending,
		RunUnknown,
	}
}
