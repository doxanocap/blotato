package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

func (r *Repository) QueueEmailDecision(ctx context.Context, eventID, runID types.ID) error {
	_, err := r.Exec(ctx, `INSERT INTO email_decisions(event_id,run_id,status) VALUES(?,?,'pending') ON CONFLICT DO NOTHING`, eventID, runID)
	return err
}

func (r *Repository) NextEmailDecision(ctx context.Context) (entity.InboxMessage, types.ID, bool, error) {
	var id, runID types.ID
	var text string
	var received int64
	var occurred sql.NullInt64
	err := r.QueryRow(ctx, `SELECT i.id,i.text,i.received_at,i.occurred_at,d.run_id FROM email_decisions d JOIN inbox i ON i.id=d.event_id WHERE d.status='pending' ORDER BY d.event_id LIMIT 1`).Scan(&id, &text, &received, &occurred, &runID)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.InboxMessage{}, 0, false, nil
	}
	if err != nil {
		return entity.InboxMessage{}, 0, false, err
	}
	var at time.Time
	if occurred.Valid {
		at = time.UnixMilli(occurred.Int64)
	}
	return entity.ScanInboxMessage(id, time.UnixMilli(received), text, at), runID, true, nil
}

func (r *Repository) FinishEmailDecision(ctx context.Context, id types.ID, decision, model, version string, confidence float64) (bool, error) {
	result, err := r.Exec(ctx, `UPDATE email_decisions SET status='done',decision=?,model=?,instruction_version=?,confidence=? WHERE event_id=? AND status='pending'`, decision, model, version, confidence, id)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	return n == 1, err
}

func (r *Repository) ApplyEmailDecision(ctx context.Context, eventID, runID types.ID, email string, now time.Time) error {
	result, err := r.Exec(ctx, `UPDATE runs SET state='link_pending',email=?,answer_at=(SELECT occurred_at FROM inbox WHERE id=?) WHERE id=? AND state='waiting_email' AND email IS NULL AND deadline>? AND EXISTS (SELECT 1 FROM inbox WHERE id=? AND received_at<runs.deadline)`, email, eventID, runID, now.UnixMilli(), eventID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil || n == 0 {
		return err
	}
	return r.CreateAction(ctx, runID, types.SendLink)
}
