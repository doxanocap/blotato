package contracts

import (
	"context"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

type Service interface {
	CreateWorkflow(ctx context.Context, in dto.PostWorkflowIn) (entity.Workflow, error)
	GetWorkflows(ctx context.Context) ([]entity.Workflow, error)
}

type ExecutionService interface {
	Receive(ctx context.Context, in dto.ReceiveEventIn) error
	GetRuns(ctx context.Context) ([]dto.RunStatus, error)
}

type FakeSender interface {
	Set(types.ActionStep, types.SendOutcome)
}
