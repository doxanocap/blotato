package types

// ActionStep identifies one outgoing action in a workflow.
type ActionStep string

const (
	ReplyComment ActionStep = "reply_comment"
	AskEmail     ActionStep = "ask_email"
	SendLink     ActionStep = "send_link"
)

func (step ActionStep) IsValid() bool {
	return step == ReplyComment || step == AskEmail || step == SendLink
}
