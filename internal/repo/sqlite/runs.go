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

const runsTable = "runs"

func (r *Repository) CreateRun(ctx context.Context, run entity.Run) (types.ID, bool, error) {
	query, args, err := r.Builder().Insert(runsTable).
		Columns(
			"account_id", "contact_id", "comment_id", "workflow_id", "keyword",
			"state", "reply", "question", "link", "wait_ms",
		).
		Values(
			run.AccountID(), run.ContactID(), run.CommentID(), run.WorkflowID(), run.Keyword(),
			run.State(), run.Reply(), run.Question(), run.Link(), run.Wait().Milliseconds(),
		).
		Suffix("ON CONFLICT DO NOTHING").ToSql()
	if err != nil {
		return 0, false, errtrace.Wrap(err)
	}

	result, err := r.Exec(ctx, query, args...)
	if err != nil {
		return 0, false, errtrace.Wrap(err)
	}

	count, err := result.RowsAffected()
	if err != nil || count == 0 {
		return 0, false, errtrace.Wrap(err)
	}

	id, err := result.LastInsertId()
	return types.ID(id), err == nil, errtrace.Wrap(err)
}

func (r *Repository) GetActiveRun(ctx context.Context, accountID, contactID string) (entity.Run, bool, error) {
	query, args, err := r.selectRun().Where(sq.Eq{
		"account_id": accountID,
		"contact_id": contactID,
		"state":      types.ActiveRunStates(),
	}).ToSql()
	if err != nil {
		return entity.Run{}, false, errtrace.Wrap(err)
	}

	run, err := r.scanRun(r.QueryRow(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Run{}, false, nil
	}

	return run, err == nil, errtrace.Wrap(err)
}

func (r *Repository) UpdateRunEmail(ctx context.Context, runID types.ID, email string, occurredAt time.Time) error {
	query, args, err := r.Builder().Update(runsTable).
		Set("state", types.RunLinkPending).
		Set("email", email).
		Set("answer_at", nullableMillis(occurredAt)).
		Where(sq.Eq{"id": runID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) UpdateRunState(ctx context.Context, runID types.ID, state types.RunState) error {
	query, args, err := r.Builder().Update(runsTable).
		Set("state", state).Where(sq.Eq{"id": runID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) FailRun(ctx context.Context, runID types.ID, state types.RunState, code string) error {
	query, args, err := r.Builder().Update(runsTable).
		Set("state", state).Set("error_code", code).
		Where(sq.Eq{"id": runID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) SetRunWaitingEmail(ctx context.Context, runID types.ID, now time.Time) error {
	query, args, err := r.Builder().Update(runsTable).
		Set("state", types.RunWaitingEmail).
		Set("deadline", sq.Expr("? + wait_ms", now.UnixMilli())).
		Where(sq.Eq{"id": runID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) GetRunDeadline(ctx context.Context, runID types.ID) (time.Time, error) {
	query, args, err := r.Builder().Select("deadline").From(runsTable).
		Where(sq.Eq{"id": runID}).ToSql()
	if err != nil {
		return time.Time{}, errtrace.Wrap(err)
	}

	var deadline int64
	if err := r.QueryRow(ctx, query, args...).Scan(&deadline); err != nil {
		return time.Time{}, errtrace.Wrap(err)
	}

	return time.UnixMilli(deadline), nil
}

func (r *Repository) ExpireRuns(ctx context.Context, now time.Time) error {
	query, args, err := r.Builder().Update(runsTable).
		Set("state", types.RunExpired).
		Where(sq.Eq{"state": types.RunWaitingEmail}).
		Where(sq.LtOrEq{"deadline": now.UnixMilli()}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) GetRuns(ctx context.Context) (_ []dto.RunStatus, err error) {
	query, args, err := r.Builder().Select(
		"r.id", "r.state", "r.error_code", "COALESCE(a.step,'')",
		"COALESCE(a.status,'')", "COALESCE(a.error_code,'')",
	).From(runsTable+" r").
		LeftJoin(actionsTable+" a ON a.run_id=r.id").
		OrderBy("r.id", "a.id").ToSql()
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	rows, err := r.Query(ctx, query, args...)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer func() { err = errors.Join(err, errtrace.Wrap(rows.Close())) }()

	runs := []dto.RunStatus{}
	for rows.Next() {
		var run dto.RunStatus
		if err := rows.Scan(&run.RunID, &run.State, &run.Error, &run.Step, &run.Status, &run.ActionError); err != nil {
			return nil, errtrace.Wrap(err)
		}

		runs = append(runs, run)
	}

	return runs, errtrace.Wrap(rows.Err())
}

func (r *Repository) selectRun() sq.SelectBuilder {
	return r.Builder().Select(
		"id", "workflow_id", "keyword", "account_id", "contact_id", "comment_id",
		"state", "reply", "question", "link", "wait_ms", "deadline",
	).From(runsTable)
}

func (r *Repository) scanRun(row interface{ Scan(...any) error }) (entity.Run, error) {
	var id types.ID
	var workflowID, deadline sql.NullInt64
	var keyword sql.NullString
	var accountID, contactID, commentID, reply, question, link string
	var state types.RunState
	var waitMS int64
	if err := row.Scan(
		&id, &workflowID, &keyword, &accountID, &contactID, &commentID,
		&state, &reply, &question, &link, &waitMS, &deadline,
	); err != nil {
		return entity.Run{}, errtrace.Wrap(err)
	}

	var deadlineAt *time.Time
	if deadline.Valid {
		deadlineAt = new(time.UnixMilli(deadline.Int64))
	}

	return entity.ScanRun(
		id, types.ID(workflowID.Int64), keyword.String, accountID, contactID, commentID,
		state, reply, question, link, time.Duration(waitMS)*time.Millisecond, deadlineAt,
	), nil
}
