// Package fake provides a local, controllable sender.
package fake

import (
	"context"
	"github.com/doxanocap/blotato/internal/adapter"
	"sync"
)

type Sender struct {
	mu       sync.Mutex
	outcomes map[adapter.Step]adapter.Outcome
}

func New() *Sender { return &Sender{outcomes: make(map[adapter.Step]adapter.Outcome)} }
func (s *Sender) Set(step adapter.Step, outcome adapter.Outcome) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.outcomes[step] = outcome
}
func (s *Sender) Send(_ context.Context, c adapter.Command) adapter.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	outcome := s.outcomes[c.Step]
	if outcome == "" {
		outcome = adapter.Accepted
	}
	return adapter.Result{Outcome: outcome, ExternalID: "fake-" + c.ID}
}
