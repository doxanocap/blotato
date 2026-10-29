package automation

import (
	"context"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// CreateWorkflow validates and saves a new rule.
func (s *Service) CreateWorkflow(ctx context.Context, in dto.PostWorkflowIn) (entity.Workflow, error) {
	workflow := entity.NewWorkflow(in.Keyword, in.PublicReply, in.EmailQuestion, in.Link)
	if err := workflow.Validate(); err != nil {
		return entity.Workflow{}, errs.ErrBadRequest("invalid workflow").WithPrivateErrs(errtrace.Wrap(err))
	}

	id, err := s.repo.CreateWorkflow(ctx, workflow)
	if err != nil {
		return entity.Workflow{}, errtrace.Wrap(err)
	}

	workflow.SetID(id)
	return workflow, nil
}

// GetWorkflows lists rules in matching order.
func (s *Service) GetWorkflows(ctx context.Context) ([]entity.Workflow, error) {
	workflows, err := s.repo.GetWorkflows(ctx)
	return workflows, errtrace.Wrap(err)
}
