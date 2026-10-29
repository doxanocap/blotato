package workflow

import (
	"context"
	"regexp"
	"strings"
)

const emailInstructionVersion = "email-intent-v1"

var emailCandidate = regexp.MustCompile(`[A-Za-z0-9.!#$%&'*+/=?^_\x60{|}~-]+@[A-Za-z0-9.-]+`)

func (s *Service) WorkEmailDecision(ctx context.Context) (bool, error) {
	if s.classifier == nil {
		return false, nil
	}
	message, runID, found, err := s.repo.NextEmailDecision(ctx)
	if err != nil || !found {
		return false, err
	}
	matches := emailCandidate.FindAllString(message.Text(), -1)
	decision, model, confidence := "invalid", "", 0.0
	var email string
	if len(matches) == 1 && strings.Count(message.Text(), "@") == 1 {
		candidate := matches[0]
		if strings.HasSuffix(candidate, ".") && !strings.HasSuffix(candidate, "..") {
			candidate = strings.TrimSuffix(candidate, ".")
		}
		if validEmail(candidate) && !strings.HasSuffix(candidate, ".") {
			email = candidate
		}
	}
	if email != "" {
		masked := strings.Replace(message.Text(), email, "EMAIL_0", 1)
		decision, model, confidence, err = s.classifier.Decide(ctx, masked)
		if err != nil {
			decision = "error"
		}
	}
	err = s.repo.Tx(ctx, func(ctx context.Context) error {
		finished, err := s.repo.FinishEmailDecision(ctx, message.ID(), decision, model, emailInstructionVersion, confidence)
		if err != nil {
			return err
		}
		if finished && decision == "EMAIL_0" && confidence >= 0.95 && email != "" {
			return s.repo.ApplyEmailDecision(ctx, message.ID(), runID, email, s.now())
		}
		return nil
	})
	return true, err
}
