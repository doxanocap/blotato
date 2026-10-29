package contracts

import (
	"context"

	"github.com/doxanocap/blotato/internal/domain/entity"
)

type Repo interface {
	GetWorkflows(ctx context.Context) ([]entity.Workflow, error)
}
