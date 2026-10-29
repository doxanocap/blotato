package worker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testJob struct {
	name  string
	calls chan struct{}
}

func (j testJob) Name() string {
	return j.name
}

func (j testJob) Run(context.Context) error {
	j.calls <- struct{}{}
	return nil
}

type failingWorkflow struct {
	calls     int
	recovered int
}

type tenv struct {
	ctx       context.Context
	cancel    context.CancelFunc
	service   *failingWorkflow
	job       *WorkflowJob
	enabled   testJob
	disabled  testJob
	scheduler *Worker
}

func newTenv(t *testing.T) *tenv {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	service := &failingWorkflow{}
	enabled := testJob{name: "enabled", calls: make(chan struct{}, 1)}
	disabled := testJob{name: "disabled", calls: make(chan struct{}, 1)}

	return &tenv{
		ctx:       ctx,
		cancel:    cancel,
		service:   service,
		job:       NewWorkflowJob(service),
		enabled:   enabled,
		disabled:  disabled,
		scheduler: NewWorker(time.Hour, []string{"enabled"}, enabled, disabled),
	}
}

func (s *failingWorkflow) Work(context.Context) (bool, error) {
	s.calls++
	if s.calls == 1 {
		return false, errors.New("work failed")
	}
	return false, nil
}

func (s *failingWorkflow) RecoverSending(context.Context) error {
	s.recovered++
	return nil
}

func TestWorkflowJobRecoversBeforeNextRun(t *testing.T) {
	t.Parallel()

	env := newTenv(t)
	if err := env.job.Run(env.ctx); err == nil {
		t.Fatal("expected work failure")
	}
	if err := env.job.Run(env.ctx); err != nil {
		t.Fatal(err)
	}
	if env.service.recovered != 1 || env.service.calls != 2 {
		t.Fatalf("recovered=%d calls=%d", env.service.recovered, env.service.calls)
	}
}

func TestWorkerStartsEnabledJobImmediatelyAndStops(t *testing.T) {
	t.Parallel()

	env := newTenv(t)
	env.scheduler.Serve(env.ctx)

	select {
	case <-env.enabled.calls:
	case <-time.After(time.Second):
		t.Fatal("enabled job did not run immediately")
	}
	select {
	case <-env.disabled.calls:
		t.Fatal("disabled job ran")
	default:
	}

	env.cancel()
	env.scheduler.WaitShutdown()
}
