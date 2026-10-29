package entity

import (
	"time"

	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

// Run holds the rule snapshot and the state needed to resume an execution.
type Run struct {
	id           types.ID
	workflowID types.ID
	keyword      string
	accountID    string
	contactID    string
	commentID    string
	state        types.RunState
	reply        string
	question     string
	link         string
	wait         time.Duration
	deadline     *time.Time
}

// NewRun snapshots the matching rule so later rule edits do not change a run.
func NewRun(rule Workflow, accountID, contactID, commentID string, wait time.Duration) Run {
	return Run{
		workflowID: rule.ID(),
		keyword:      rule.Keyword(),
		accountID:    accountID,
		contactID:    contactID,
		commentID:    commentID,
		state:        types.RunReplyPending,
		reply:        rule.PublicReply(),
		question:     rule.EmailQuestion(),
		link:         rule.Link(),
		wait:         wait,
	}
}

// ScanRun restores a persisted run, including nullable legacy rule identifiers.
func ScanRun(
	id, workflowID types.ID,
	keyword, accountID, contactID, commentID string,
	state types.RunState,
	reply, question, link string,
	wait time.Duration,
	deadline *time.Time,
) Run {
	return Run{
		id:           id,
		workflowID: workflowID,
		keyword:      keyword,
		accountID:    accountID,
		contactID:    contactID,
		commentID:    commentID,
		state:        state,
		reply:        reply,
		question:     question,
		link:         link,
		wait:         wait,
		deadline:     deadline,
	}
}

func (r Run) ID() types.ID           { return r.id }
func (r Run) WorkflowID() types.ID { return r.workflowID }
func (r Run) Keyword() string        { return r.keyword }
func (r Run) AccountID() string      { return r.accountID }
func (r Run) ContactID() string      { return r.contactID }
func (r Run) CommentID() string      { return r.commentID }
func (r Run) State() types.RunState  { return r.state }
func (r Run) Reply() string          { return r.reply }
func (r Run) Question() string       { return r.question }
func (r Run) Link() string           { return r.link }
func (r Run) Wait() time.Duration    { return r.wait }

func (r Run) Deadline() *time.Time {
	if r.deadline == nil {
		return nil
	}

	return new(*r.deadline)
}
