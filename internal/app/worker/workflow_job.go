package worker

import (
	"context"

	"github.com/doxanocap/blotato/internal/app/worker/contracts"
)

const WorkflowJobName = "workflow"

// WorkflowJob runs pending actions until the queue is empty.
type WorkflowJob struct {
	service        contracts.WorkflowService
	recoverSending bool
}

// NewWorkflowJob creates a job for workflow processing.
func NewWorkflowJob(service contracts.WorkflowService) *WorkflowJob {
	return &WorkflowJob{service: service}
}

// Name identifies the job in the enabled job list.
func (j *WorkflowJob) Name() string {
	return WorkflowJobName
}

// Run processes pending actions until none remain or the context ends.
func (j *WorkflowJob) Run(ctx context.Context) error {
	if j.recoverSending {
		if err := j.service.RecoverSending(ctx); err != nil {
			return err
		}
		j.recoverSending = false
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		j.recoverSending = true
		worked, err := j.service.Work(ctx)
		if err == nil {
			j.recoverSending = false
		}
		if err != nil || !worked {
			return err
		}
	}
}
