package resolver

import (
	"context"
	"strings"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/domain/entity"
)

// GetWorkflowForComment returns the first match in repository ID order.
// It uses the caller's context so resolution participates in its transaction.
func (s *Service) GetWorkflowForComment(
	ctx context.Context,
	text string,
) (entity.Workflow, bool, error) {
	rules, err := s.repo.GetWorkflows(ctx)
	if err != nil {
		return entity.Workflow{}, false, errtrace.Wrap(err)
	}

	text = strings.ToLower(text)
	for _, rule := range rules {
		if strings.Contains(text, strings.ToLower(rule.Keyword())) {
			return rule, true, nil
		}
	}

	return entity.Workflow{}, false, nil
}
