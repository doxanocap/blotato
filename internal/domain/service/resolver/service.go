package resolver

import "github.com/doxanocap/blotato/internal/domain/service/resolver/contracts"

// Service resolves business resources needed by workflow operations.
type Service struct {
	repo contracts.Repo
}

func NewService(repo contracts.Repo) *Service {
	return &Service{repo: repo}
}
