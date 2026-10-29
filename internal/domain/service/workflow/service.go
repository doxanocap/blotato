package workflow

import (
	"context"
	"time"

	"github.com/doxanocap/blotato/internal/domain/service/workflow/contracts"
)

const (
	emailWaitDuration     = 24 * time.Hour
	messageWindowDuration = 24 * time.Hour
)

// Service executes workflows using consumer-owned dependency contracts.
type Service struct {
	repo      contracts.Repo
	resolver  contracts.Resolver
	instagram contracts.InstagramClient

	accountID  string
	now        func() time.Time
	classifier EmailClassifier
}

type EmailClassifier interface {
	Decide(ctx context.Context, text string) (decision, model string, confidence float64, err error)
}

func (s *Service) SetEmailClassifier(classifier EmailClassifier) { s.classifier = classifier }

func NewService(
	repo contracts.Repo,
	resolver contracts.Resolver,
	instagram contracts.InstagramClient,
	accountID string,
) *Service {
	return &Service{
		repo:      repo,
		resolver:  resolver,
		instagram: instagram,
		accountID: accountID,
		now:       time.Now,
	}
}
