package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/doxanocap/blotato/internal/app/worker/contracts"
)

const defaultTimeout = 60 * time.Second

// Worker runs enabled jobs on a common schedule.
type Worker struct {
	wg          sync.WaitGroup
	schedule    time.Duration
	enabledJobs []string
	jobs        []contracts.Job
}

// NewWorker creates a scheduler for the supplied jobs.
func NewWorker(
	schedule time.Duration,
	enabledJobs []string,
	jobs ...contracts.Job,
) *Worker {
	return &Worker{
		schedule:    schedule,
		enabledJobs: enabledJobs,
		jobs:        jobs,
	}
}

// Serve starts a loop for each enabled job.
func (w *Worker) Serve(ctx context.Context) {
	if w.schedule <= 0 || len(w.enabledJobs) == 0 {
		return
	}

	for _, job := range w.jobs {
		if !w.isEnabled(job.Name()) {
			continue
		}
		w.wg.Go(func() {
			w.runLoop(ctx, job)
		})
	}
}

// WaitShutdown waits until all job loops have stopped.
func (w *Worker) WaitShutdown() {
	w.wg.Wait()
}

func (w *Worker) runLoop(ctx context.Context, job contracts.Job) {
	ticker := time.NewTicker(w.schedule)
	defer ticker.Stop()

	for {
		if ctx.Err() != nil {
			return
		}
		w.runPipeline(ctx, job)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) isEnabled(name string) bool {
	for _, enabledJob := range w.enabledJobs {
		if strings.TrimSpace(enabledJob) == name {
			return true
		}
	}
	return false
}

func (w *Worker) runPipeline(ctx context.Context, job contracts.Job) {
	jobCtx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	if err := w.runJob(jobCtx, job); err != nil && ctx.Err() == nil {
		slog.ErrorContext(ctx, "worker job failed", "job", job.Name(), "error", err)
	}
}

func (w *Worker) runJob(ctx context.Context, job contracts.Job) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("worker job panicked: %T", recovered)
		}
	}()

	return job.Run(ctx)
}
