package contracts

import "context"

// Job is a scheduled unit of work.
type Job interface {
	Name() string
	Run(ctx context.Context) error
}

// WorkflowService processes and recovers persisted workflow actions.
type WorkflowService interface {
	Work(ctx context.Context) (bool, error)
	RecoverSending(ctx context.Context) error
}
