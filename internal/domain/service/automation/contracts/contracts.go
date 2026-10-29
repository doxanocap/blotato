package contracts

import (
	"context"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

type Repo interface {
	CreateWorkflow(ctx context.Context, workflow entity.Workflow) (types.ID, error)
	GetWorkflows(ctx context.Context) ([]entity.Workflow, error)
}
