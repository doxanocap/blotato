package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"braces.dev/errtrace"

	"github.com/doxanocap/blotato/internal/clients/instagram"
	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
	"github.com/doxanocap/blotato/internal/domain/errs"
)

// Work processes one durable action. A single worker is used in this process.
func (s *Service) Work(ctx context.Context) (bool, error) {
	if err := s.repo.ExpireRuns(ctx, s.now()); err != nil {
		return false, errtrace.Wrap(err)
	}

	cmd, found, err := s.repo.ClaimAction(ctx)
	if err != nil {
		return false, errtrace.Wrap(err)
	}
	if !found {
		return false, nil
	}

	result := s.sendAction(ctx, cmd)

	if err := s.finish(ctx, cmd, result); err != nil {
		return true, errtrace.Wrap(err)
	}

	return true, nil
}

// sendAction validates the send and records its outcome without changing storage.
func (s *Service) sendAction(ctx context.Context, cmd dto.SendActionIn) entity.SendResult {
	result := entity.NewSendResult()

	if cmd.Step == types.SendLink {
		now := s.now()

		switch {
		case cmd.AnswerAt.IsZero():
			result.Reject("message_time_unknown")

			return result
		case cmd.AnswerAt.After(now):
			result.Reject("message_time_future")

			return result
		case !now.Before(cmd.AnswerAt.Add(messageWindowDuration)):
			result.Reject("message_window_expired")

			return result
		}
	}

	if cmd.AccountID == "" || cmd.Text == "" || !utf8.ValidString(cmd.Text) ||
		s.accountID != "" && cmd.AccountID != s.accountID {
		result.Reject("invalid_configuration_or_command")

		return result
	}

	var id string
	var err error
	var message instagram.Message

	switch cmd.Step {
	case types.ReplyComment:
		if cmd.CommentID == "" || cmd.CommentID == "." || cmd.CommentID == ".." ||
			strings.ContainsAny(cmd.CommentID, "/\\?#%") {
			result.Reject("missing_comment_id")

			return result
		}

		id, err = s.instagram.ReplyComment(ctx, cmd.CommentID, cmd.Text)
	case types.AskEmail:
		if len(cmd.Text) > 1000 {
			result.Reject("message_too_long")

			return result
		}

		if cmd.ContactID == "" {
			result.Reject("missing_contact_id")

			return result
		}

		if cmd.CommentID == "" {
			result.Reject("missing_comment_id")

			return result
		}

		message, err = s.instagram.PrivateReply(ctx, cmd.AccountID, cmd.CommentID, cmd.ContactID, cmd.Text)
		id = message.ID
	case types.SendLink:
		if len(cmd.Text) > 1000 {
			result.Reject("message_too_long")

			return result
		}

		if cmd.ContactID == "" {
			result.Reject("missing_contact_id")

			return result
		}

		message, err = s.instagram.SendMessage(ctx, cmd.AccountID, cmd.ContactID, cmd.Text)
		id = message.ID
	default:
		result.Reject("invalid_step")

		return result
	}

	if err != nil {
		result.MarkUnknown("transport_error")

		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError

		switch {
		case errors.Is(err, errs.ErrRequestRejected):
			result.Reject("rejected")
		case errors.Is(err, errs.ErrUnexpectedResponse):
			result.MarkUnknown("ambiguous_response")
		case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF),
			errors.As(err, &syntaxErr), errors.As(err, &typeErr):
			result.MarkUnknown("invalid_response")
		}

		return result
	}

	if id == "" || cmd.Step != types.ReplyComment && message.RecipientID == "" {
		result.MarkUnknown("unconfirmed_response")

		return result
	}

	if cmd.Step != types.ReplyComment && message.RecipientID != cmd.ContactID {
		result.MarkUnknown("recipient_mismatch")

		return result
	}

	result.Accept(id)

	return result
}

func (s *Service) finish(ctx context.Context, cmd dto.SendActionIn, result entity.SendResult) error {
	return s.repo.Tx(ctx, func(ctx context.Context) error {
		runID, err := s.repo.GetSendingActionRun(ctx, cmd.ID)
		if err != nil {
			return errtrace.Wrap(err)
		}

		if err := s.repo.FinishAction(ctx, cmd.ID, result); err != nil {
			return errtrace.Wrap(err)
		}

		if result.Outcome() != types.Accepted {
			state := types.RunUnknown
			if result.Outcome() == types.Rejected {
				state = types.RunFailed
			}

			return errtrace.Wrap(s.repo.FailRun(ctx, runID, state, result.ErrorCode()))
		}

		if err := s.advance(ctx, runID, cmd.Step); err != nil {
			return errtrace.Wrap(err)
		}

		return nil
	})
}

func (s *Service) advance(ctx context.Context, runID types.ID, step types.ActionStep) error {
	switch step {
	case types.ReplyComment:
		if err := s.repo.UpdateRunState(ctx, runID, types.RunQuestionPending); err != nil {
			return errtrace.Wrap(err)
		}

		return errtrace.Wrap(s.repo.CreateAction(ctx, runID, types.AskEmail))
	case types.AskEmail:
		if err := s.repo.SetRunWaitingEmail(ctx, runID, s.now()); err != nil {
			return errtrace.Wrap(err)
		}

		return s.deferred(ctx, runID)
	case types.SendLink:
		return errtrace.Wrap(s.repo.UpdateRunState(ctx, runID, types.RunCompleted))
	default:
		return errtrace.New("unknown step")
	}
}
