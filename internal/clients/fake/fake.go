// Package fake provides a local, controllable sender.
package fake

import (
	"context"
	"sync"

	"braces.dev/errtrace"
	"github.com/google/uuid"

	"github.com/doxanocap/blotato/internal/clients/instagram"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

type Client struct {
	mu       sync.Mutex
	outcomes map[types.ActionStep]types.SendOutcome
}

func New() *Client {
	return &Client{outcomes: make(map[types.ActionStep]types.SendOutcome)}
}

func (s *Client) Set(step types.ActionStep, outcome types.SendOutcome) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.outcomes[step] = outcome
}

func (s *Client) ReplyComment(
	_ context.Context,
	commentID string,
	text string,
) (string, error) {
	if err := s.result(types.ReplyComment); err != nil {
		return "", errtrace.Wrap(err)
	}

	return "fake-" + uuid.NewString(), nil
}

func (s *Client) PrivateReply(
	_ context.Context,
	accountID string,
	commentID string,
	contactID string,
	text string,
) (instagram.Message, error) {
	if err := s.result(types.AskEmail); err != nil {
		return instagram.Message{}, errtrace.Wrap(err)
	}

	return instagram.Message{
		ID:          "fake-" + uuid.NewString(),
		RecipientID: contactID,
	}, nil
}

func (s *Client) SendMessage(
	_ context.Context,
	accountID string,
	contactID string,
	text string,
) (instagram.Message, error) {
	if err := s.result(types.SendLink); err != nil {
		return instagram.Message{}, errtrace.Wrap(err)
	}

	return instagram.Message{
		ID:          "fake-" + uuid.NewString(),
		RecipientID: contactID,
	}, nil
}

func (s *Client) result(step types.ActionStep) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	outcome := s.outcomes[step]
	if outcome == "" || outcome == types.Accepted {
		return nil
	}

	if outcome == types.Rejected || outcome == types.Retryable {
		return errs.ErrRequestRejected
	}

	return errs.ErrUnexpectedResponse
}
