package automation

import "github.com/doxanocap/blotato/internal/domain/service/automation/contracts"

// Service manages saved workflow rules.
type Service struct {
	repo contracts.Repo
}

func NewService(repo contracts.Repo) *Service {
	return &Service{repo: repo}
}
