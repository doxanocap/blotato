package entity

import (
	"net/url"
	"strings"

	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// Workflow is a rule shared by the posts of the connected account.
type Workflow struct {
	id            types.ID
	keyword       string
	publicReply   string
	emailQuestion string
	link          string
}

// NewWorkflow normalizes a new rule before it is validated and saved.
func NewWorkflow(keyword, publicReply, emailQuestion, link string) Workflow {
	return Workflow{
		keyword:       strings.TrimSpace(keyword),
		publicReply:   strings.TrimSpace(publicReply),
		emailQuestion: strings.TrimSpace(emailQuestion),
		link:          strings.TrimSpace(link),
	}
}

// ScanWorkflow restores a saved rule without changing its persisted values.
func ScanWorkflow(id types.ID, keyword, publicReply, emailQuestion, link string) Workflow {
	return Workflow{
		id:            id,
		keyword:       keyword,
		publicReply:   publicReply,
		emailQuestion: emailQuestion,
		link:          link,
	}
}

// Validate checks the rule required by the comment-to-DM workflow.
func (a Workflow) Validate() error {
	u, err := url.Parse(a.link)
	if a.keyword == "" || a.publicReply == "" || a.emailQuestion == "" || len(a.emailQuestion) > 1000 || len(a.link) > 1000 || err != nil || u.Host == "" || u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return errs.ErrInvalidWorkflow
	}

	return nil
}

func (a Workflow) ID() types.ID          { return a.id }
func (a Workflow) Keyword() string       { return a.keyword }
func (a Workflow) PublicReply() string   { return a.publicReply }
func (a Workflow) EmailQuestion() string { return a.emailQuestion }
func (a Workflow) Link() string          { return a.link }

func (a *Workflow) SetID(id types.ID) {
	a.id = id
}
