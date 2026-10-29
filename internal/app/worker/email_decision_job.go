package worker

import "context"

const EmailDecisionJobName = "email_decision"

type EmailDecisionService interface {
	WorkEmailDecision(context.Context) (bool, error)
}

type EmailDecisionJob struct{ service EmailDecisionService }

func NewEmailDecisionJob(service EmailDecisionService) *EmailDecisionJob {
	return &EmailDecisionJob{service: service}
}

func (j *EmailDecisionJob) Name() string { return EmailDecisionJobName }

func (j *EmailDecisionJob) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		worked, err := j.service.WorkEmailDecision(ctx)
		if err != nil || !worked {
			return err
		}
	}
	return ctx.Err()
}
