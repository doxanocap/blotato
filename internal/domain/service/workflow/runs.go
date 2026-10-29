package workflow

import (
	"context"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/domain/entity/dto"
)

// GetRuns returns the observable states of workflow actions.
func (s *Service) GetRuns(ctx context.Context) ([]dto.RunStatus, error) {
	runs, err := s.repo.GetRuns(ctx)

	return runs, errtrace.Wrap(err)
}

// RecoverSending marks calls interrupted before their result was saved unknown.
func (s *Service) RecoverSending(ctx context.Context) error {
	return errtrace.Wrap(s.repo.RecoverSendingActions(ctx))
}
