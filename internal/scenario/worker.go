package scenario

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/doxanocap/blotato/internal/adapter"
)

// Work processes one durable action. A single worker is used in this process.
func (s *Service) Work(ctx context.Context, sender adapter.Sender) (bool, error) {
	cmd, err := s.claim(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, s.expire(ctx)
	}
	if err != nil {
		return false, err
	}
	result := sender.Send(ctx, cmd)
	if result.Outcome != adapter.Accepted && result.Outcome != adapter.Rejected && result.Outcome != adapter.Retryable && result.Outcome != adapter.Unknown {
		result = adapter.Result{Outcome: adapter.Unknown, ErrorCode: "invalid_adapter_result"}
	}
	if err = s.finish(ctx, cmd, result); err != nil {
		return true, err
	}
	return true, nil
}
func (s *Service) claim(ctx context.Context) (adapter.Command, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return adapter.Command{}, err
	}
	defer tx.Rollback()
	var id int64
	var c adapter.Command
	err = tx.QueryRowContext(ctx, `SELECT a.id,r.account_id,r.contact_id,r.comment_id,a.step,CASE a.step WHEN 'reply_comment' THEN r.reply WHEN 'ask_email' THEN r.question ELSE r.link END FROM actions a JOIN runs r ON r.id=a.run_id WHERE a.status='pending' ORDER BY a.id LIMIT 1`).Scan(&id, &c.AccountID, &c.ContactID, &c.CommentID, &c.Step, &c.Text)
	if err != nil {
		return c, err
	}
	c.ID = strconv.FormatInt(id, 10)
	if _, err = tx.ExecContext(ctx, `UPDATE actions SET status='sending' WHERE id=? AND status='pending'`, id); err != nil {
		return c, err
	}
	return c, tx.Commit()
}
func (s *Service) finish(ctx context.Context, c adapter.Command, result adapter.Result) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin finish: %w", err)
	}
	defer tx.Rollback()
	var rid int64
	if err = tx.QueryRowContext(ctx, `SELECT run_id FROM actions WHERE id=? AND status='sending'`, c.ID).Scan(&rid); err != nil {
		return fmt.Errorf("find action: %w", err)
	}
	// Persist only a controlled classification, not potentially sensitive provider text.
	code := string(result.Outcome)
	if result.Outcome == adapter.Accepted {
		code = ""
	}
	if _, err = tx.ExecContext(ctx, `UPDATE actions SET status=?,external_id=?,error_code=? WHERE id=?`, result.Outcome, result.ExternalID, code, c.ID); err != nil {
		return err
	}
	if result.Outcome != adapter.Accepted {
		state := "unknown"
		if result.Outcome == adapter.Rejected {
			state = "failed"
		}
		if result.Outcome == adapter.Retryable {
			state = "failed"
			code = "retryable_no_auto_retry"
		}
		_, err = tx.ExecContext(ctx, `UPDATE runs SET state=?,error_code=? WHERE id=?`, state, code, rid)
	} else {
		err = s.advance(ctx, tx, rid, c.Step)
	}
	if err != nil {
		return fmt.Errorf("finish action: %w", err)
	}
	return tx.Commit()
}
func (s *Service) advance(ctx context.Context, tx *sql.Tx, rid int64, step adapter.Step) error {
	switch step {
	case adapter.ReplyComment:
		if _, err := tx.ExecContext(ctx, `UPDATE runs SET state='question_pending' WHERE id=?`, rid); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO actions(run_id,step,status) VALUES(?,'ask_email','pending')`, rid)
		return err
	case adapter.AskEmail:
		if _, err := tx.ExecContext(ctx, `UPDATE runs SET state='waiting_email',deadline=?+wait_ms WHERE id=?`, s.now().UnixMilli(), rid); err != nil {
			return err
		}
		return s.deferred(ctx, tx, rid)
	case adapter.SendLink:
		_, err := tx.ExecContext(ctx, `UPDATE runs SET state='completed' WHERE id=?`, rid)
		return err
	default:
		return errors.New("unknown step")
	}
}
func (s *Service) deferred(ctx context.Context, tx *sql.Tx, rid int64) error {
	rows, err := tx.QueryContext(ctx, `SELECT id,text,received_at FROM inbox WHERE run_id=? AND kind='message' AND status='deferred' ORDER BY id`, rid)
	if err != nil {
		return err
	}
	type item struct {
		id, at int64
		text   string
	}
	var items []item
	for rows.Next() {
		var v item
		if err = rows.Scan(&v.id, &v.text, &v.at); err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	var deadline int64
	if err = tx.QueryRowContext(ctx, `SELECT deadline FROM runs WHERE id=?`, rid).Scan(&deadline); err != nil {
		return err
	}
	won := false
	for _, v := range items {
		if !won && v.at < deadline && validEmail(v.text) {
			if _, err = tx.ExecContext(ctx, `UPDATE runs SET state='link_pending',email=? WHERE id=?`, v.text, rid); err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO actions(run_id,step,status) VALUES(?,'send_link','pending')`, rid); err != nil {
				return err
			}
			won = true
		}
		if _, err = tx.ExecContext(ctx, `UPDATE inbox SET status='processed' WHERE id=?`, v.id); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) expire(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE runs SET state='expired' WHERE state='waiting_email' AND deadline<=?`, s.now().UnixMilli())
	return err
}

// Run polls persisted work until the process is stopped.
func (s *Service) Run(ctx context.Context, sender adapter.Sender) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		worked, err := s.Work(ctx, sender)
		if err != nil {
			return err
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
