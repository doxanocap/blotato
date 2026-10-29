package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"braces.dev/errtrace"
	sq "github.com/Masterminds/squirrel"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/dto"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

const actionsTable = "actions"

func (r *Repository) CreateAction(ctx context.Context, runID types.ID, step types.ActionStep) error {
	query, args, err := r.Builder().Insert(actionsTable).
		Columns("run_id", "step", "status").
		Values(runID, step, types.ActionPending).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) IsQuestionActionSending(ctx context.Context, runID types.ID) (bool, error) {
	query, args, err := r.Builder().Select("COUNT(*) > 0").From(actionsTable).
		Where(sq.Eq{
			"run_id": runID,
			"step":   types.AskEmail,
			"status": []types.ActionStatus{types.ActionSending, types.ActionUnknown},
		}).ToSql()
	if err != nil {
		return false, errtrace.Wrap(err)
	}

	var sending bool
	err = r.QueryRow(ctx, query, args...).Scan(&sending)
	return sending, errtrace.Wrap(err)
}

// RecoverSendingActions atomically marks interrupted actions and their runs unknown.
func (r *Repository) RecoverSendingActions(ctx context.Context) error {
	return r.Tx(ctx, func(ctx context.Context) error {
		query, args, err := r.Builder().Update(runsTable).
			Set("state", types.RunUnknown).
			Set("error_code", "interrupted_send").
			Where("id IN (SELECT run_id FROM actions WHERE status=?)", types.ActionSending).
			ToSql()
		if err != nil {
			return errtrace.Wrap(err)
		}

		if _, err := r.Exec(ctx, query, args...); err != nil {
			return errtrace.Wrap(err)
		}

		query, args, err = r.Builder().Update(actionsTable).
			Set("status", types.ActionUnknown).
			Set("error_code", "interrupted_send").
			Where(sq.Eq{"status": types.ActionSending}).ToSql()
		if err != nil {
			return errtrace.Wrap(err)
		}

		_, err = r.Exec(ctx, query, args...)
		return errtrace.Wrap(err)
	})
}

// ClaimAction reserves a pending action in a transaction before the HTTP call.
func (r *Repository) ClaimAction(ctx context.Context) (dto.SendActionIn, bool, error) {
	var command dto.SendActionIn
	var found bool
	err := r.Tx(ctx, func(ctx context.Context) error {
		query, args, err := r.selectActionCommand().
			Where(sq.Eq{"a.status": types.ActionPending}).OrderBy("a.id").Limit(1).ToSql()
		if err != nil {
			return errtrace.Wrap(err)
		}

		command, err = r.scanActionCommand(r.QueryRow(ctx, query, args...))
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return errtrace.Wrap(err)
		}

		query, args, err = r.Builder().Update(actionsTable).
			Set("status", types.ActionSending).
			Where(sq.Eq{"id": command.ID, "status": types.ActionPending}).ToSql()
		if err != nil {
			return errtrace.Wrap(err)
		}

		result, err := r.Exec(ctx, query, args...)
		if err != nil {
			return errtrace.Wrap(err)
		}

		count, err := result.RowsAffected()
		found = count == 1
		return errtrace.Wrap(err)
	})

	return command, found, errtrace.Wrap(err)
}

func (r *Repository) GetSendingActionRun(ctx context.Context, actionID string) (types.ID, error) {
	query, args, err := r.Builder().Select("run_id").From(actionsTable).
		Where(sq.Eq{"id": actionID, "status": types.ActionSending}).ToSql()
	if err != nil {
		return 0, errtrace.Wrap(err)
	}

	var runID types.ID
	err = r.QueryRow(ctx, query, args...).Scan(&runID)
	return runID, errtrace.Wrap(err)
}

func (r *Repository) FinishAction(
	ctx context.Context,
	actionID string,
	result entity.SendResult,
) error {
	query, args, err := r.Builder().Update(actionsTable).
		Set("status", types.ActionStatus(result.Outcome())).
		Set("external_id", result.ExternalID()).
		Set("error_code", result.ErrorCode()).
		Where(sq.Eq{"id": actionID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) selectActionCommand() sq.SelectBuilder {
	return r.Builder().Select(
		"a.id", "r.account_id", "r.contact_id", "r.comment_id", "a.step",
		"CASE a.step WHEN 'reply_comment' THEN r.reply WHEN 'ask_email' THEN r.question ELSE r.link END",
		"r.answer_at",
	).From(actionsTable + " a").Join(runsTable + " r ON r.id=a.run_id")
}

func (r *Repository) scanActionCommand(row interface{ Scan(...any) error }) (dto.SendActionIn, error) {
	var id types.ID
	var answerAt sql.NullInt64
	var command dto.SendActionIn
	if err := row.Scan(
		&id, &command.AccountID, &command.ContactID, &command.CommentID,
		&command.Step, &command.Text, &answerAt,
	); err != nil {
		return dto.SendActionIn{}, errtrace.Wrap(err)
	}

	command.ID = id.String()
	if answerAt.Valid {
		command.AnswerAt = time.UnixMilli(answerAt.Int64)
	}

	return command, nil
}
