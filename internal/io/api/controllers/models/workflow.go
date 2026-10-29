package models

import "github.com/doxanocap/blotato/internal/domain/entity"

// Workflow is the public representation of a saved rule.
type Workflow struct {
	ID            int64  `json:"id"`
	Keyword       string `json:"keyword"`
	PublicReply   string `json:"public_reply"`
	EmailQuestion string `json:"email_question"`
	Link          string `json:"link"`
}

func (m *Workflow) FromEntity(workflow entity.Workflow) Workflow {
	return Workflow{
		ID:            workflow.ID().Int64(),
		Keyword:       workflow.Keyword(),
		PublicReply:   workflow.PublicReply(),
		EmailQuestion: workflow.EmailQuestion(),
		Link:          workflow.Link(),
	}
}
