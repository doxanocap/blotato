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

const inboxTable = "inbox"

func (r *Repository) CreateInboxEvent(ctx context.Context, event dto.ReceiveEventIn, receivedAt time.Time) (types.ID, bool, error) {
	query, args, err := r.Builder().Insert(inboxTable).
		Columns(
			"platform", "account_id", "contact_id", "kind", "event_id",
			"text", "received_at", "occurred_at", "status",
		).
		Values(
			event.Platform, event.AccountID, event.ContactID, event.Kind, event.EventID,
			event.Text, receivedAt.UnixMilli(), nullableMillis(event.OccurredAt), "new",
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

func (r *Repository) DeferInboxEvent(ctx context.Context, eventID, runID types.ID) error {
	query, args, err := r.Builder().Update(inboxTable).
		Set("status", "deferred").Set("run_id", runID).
		Where(sq.Eq{"id": eventID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) ProcessInboxEvent(ctx context.Context, eventID types.ID) error {
	query, args, err := r.Builder().Update(inboxTable).
		Set("status", "processed").Where(sq.Eq{"id": eventID}).ToSql()
	if err != nil {
		return errtrace.Wrap(err)
	}

	_, err = r.Exec(ctx, query, args...)
	return errtrace.Wrap(err)
}

func (r *Repository) GetDeferredInboxMessages(ctx context.Context, runID types.ID) (_ []entity.InboxMessage, err error) {
	query, args, err := r.Builder().Select("id", "text", "received_at", "occurred_at").
		From(inboxTable).
		Where(sq.Eq{"run_id": runID, "kind": types.Message, "status": "deferred"}).
		OrderBy("id").ToSql()
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	rows, err := r.Query(ctx, query, args...)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer func() { err = errors.Join(err, errtrace.Wrap(rows.Close())) }()

	var messages []entity.InboxMessage
	for rows.Next() {
		message, err := r.scanInboxMessage(rows)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}

		messages = append(messages, message)
	}

	return messages, errtrace.Wrap(rows.Err())
}

func (r *Repository) scanInboxMessage(row interface{ Scan(...any) error }) (entity.InboxMessage, error) {
	var id types.ID
	var text string
	var receivedAt int64
	var occurred sql.NullInt64
	if err := row.Scan(&id, &text, &receivedAt, &occurred); err != nil {
		return entity.InboxMessage{}, errtrace.Wrap(err)
	}

	var occurredAt time.Time
	if occurred.Valid {
		occurredAt = time.UnixMilli(occurred.Int64)
	}

	return entity.ScanInboxMessage(id, time.UnixMilli(receivedAt), text, occurredAt), nil
}
