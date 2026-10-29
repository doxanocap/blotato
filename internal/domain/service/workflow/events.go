package workflow

import (
	"context"
	"net/mail"
	"strings"
	"time"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// Receive commits an incoming event and its immediate transition together.
func (s *Service) Receive(ctx context.Context, in dto.ReceiveEventIn) error {
	if in.Platform == "" || in.AccountID == "" || in.ContactID == "" || in.EventID == "" || !in.Kind.IsValid() {
		return errs.ErrBadRequest("invalid event").WithPrivateErrs(errs.ErrInvalidEvent)
	}

	return s.repo.Tx(ctx, func(ctx context.Context) error {
		receivedAt := s.now()
		id, inserted, err := s.repo.CreateInboxEvent(ctx, in, receivedAt)
		if err != nil {
			return errtrace.Wrap(err)
		}
		if !inserted {
			return nil
		}

		if in.Kind == types.Comment {
			return s.comment(ctx, in, id)
		}

		return s.message(ctx, in, id, receivedAt)
	})
}

func (s *Service) comment(ctx context.Context, in dto.ReceiveEventIn, id types.ID) error {
	rule, found, err := s.resolver.GetWorkflowForComment(ctx, in.Text)
	if err != nil {
		return errtrace.Wrap(err)
	}

	if !found {
		return errtrace.Wrap(s.repo.ProcessInboxEvent(ctx, id))
	}

	run := entity.NewRun(rule, in.AccountID, in.ContactID, in.EventID, emailWaitDuration)
	runID, created, err := s.repo.CreateRun(ctx, run)
	if err != nil {
		return errtrace.Wrap(err)
	}
	if created {
		if err := s.repo.CreateAction(ctx, runID, types.ReplyComment); err != nil {
			return errtrace.Wrap(err)
		}
	}

	return errtrace.Wrap(s.repo.ProcessInboxEvent(ctx, id))
}

func (s *Service) message(ctx context.Context, in dto.ReceiveEventIn, id types.ID, receivedAt time.Time) error {
	run, found, err := s.repo.GetActiveRun(ctx, in.AccountID, in.ContactID)
	if err != nil {
		return errtrace.Wrap(err)
	}
	if !found {
		return errtrace.Wrap(s.repo.ProcessInboxEvent(ctx, id))
	}

	if run.State() == types.RunQuestionPending || run.State() == types.RunUnknown {
		sendingQuestion, err := s.repo.IsQuestionActionSending(ctx, run.ID())
		if err != nil {
			return errtrace.Wrap(err)
		}
		if sendingQuestion {
			return errtrace.Wrap(s.repo.DeferInboxEvent(ctx, id, run.ID()))
		}
	}

	deadline := run.Deadline()
	if s.classifier != nil && run.State() == types.RunWaitingEmail && deadline != nil && receivedAt.Before(*deadline) && !validEmail(in.Text) {
		if err := s.repo.QueueEmailDecision(ctx, id, run.ID()); err != nil {
			return errtrace.Wrap(err)
		}
	}
	if run.State() == types.RunWaitingEmail && deadline != nil && receivedAt.Before(*deadline) && validEmail(in.Text) {
		if err := s.repo.UpdateRunEmail(ctx, run.ID(), strings.TrimSpace(in.Text), in.OccurredAt); err != nil {
			return errtrace.Wrap(err)
		}
		if err := s.repo.CreateAction(ctx, run.ID(), types.SendLink); err != nil {
			return errtrace.Wrap(err)
		}
	}

	return errtrace.Wrap(s.repo.ProcessInboxEvent(ctx, id))
}

func validEmail(text string) bool {
	text = strings.TrimSpace(text)
	address, err := mail.ParseAddress(text)
	return err == nil && address.Address == text && strings.Contains(text, "@")
}

func (s *Service) deferred(ctx context.Context, runID types.ID) error {
	messages, err := s.repo.GetDeferredInboxMessages(ctx, runID)
	if err != nil {
		return errtrace.Wrap(err)
	}

	deadline, err := s.repo.GetRunDeadline(ctx, runID)
	if err != nil {
		return errtrace.Wrap(err)
	}

	won := false
	for _, message := range messages {
		if !won && message.ReceivedAt().Before(deadline) && validEmail(message.Text()) {
			if err := s.repo.UpdateRunEmail(ctx, runID, strings.TrimSpace(message.Text()), message.OccurredAt()); err != nil {
				return errtrace.Wrap(err)
			}
			if err := s.repo.CreateAction(ctx, runID, types.SendLink); err != nil {
				return errtrace.Wrap(err)
			}
			won = true
		}
		if !won && s.classifier != nil && message.ReceivedAt().Before(deadline) && !validEmail(message.Text()) {
			if err := s.repo.QueueEmailDecision(ctx, message.ID(), runID); err != nil {
				return errtrace.Wrap(err)
			}
		}
		if err := s.repo.ProcessInboxEvent(ctx, message.ID()); err != nil {
			return errtrace.Wrap(err)
		}
	}
	return nil
}
