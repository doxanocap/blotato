package scenario

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/doxanocap/blotato/internal/adapter"
)

type Settings struct {
	Reply, Question, Link string
	Wait                  time.Duration
}
type Service struct {
	db       *sql.DB
	settings Settings
	now      func() time.Time
}

var ErrInvalidEvent = errors.New("invalid event")

func New(db *sql.DB, settings Settings) *Service {
	return &Service{db: db, settings: settings, now: time.Now}
}

// Receive commits an incoming event and its immediate state transition together.
func (s *Service) Receive(ctx context.Context, e adapter.Event) error {
	if e.Platform == "" || e.AccountID == "" || e.ContactID == "" || e.EventID == "" || (e.Kind != adapter.Comment && e.Kind != adapter.Message) {
		return ErrInvalidEvent
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin receive: %w", err)
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO inbox(platform,account_id,contact_id,kind,event_id,text,received_at,status) VALUES(?,?,?,?,?,?,?,'new') ON CONFLICT DO NOTHING`, e.Platform, e.AccountID, e.ContactID, e.Kind, e.EventID, e.Text, s.now().UnixMilli())
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return tx.Commit()
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	if e.Kind == adapter.Comment {
		err = s.comment(ctx, tx, e, id)
	} else {
		err = s.message(ctx, tx, e, id)
	}
	if err != nil {
		return fmt.Errorf("process event: %w", err)
	}
	return tx.Commit()
}
func (s *Service) comment(ctx context.Context, tx *sql.Tx, e adapter.Event, id int64) error {
	if strings.Contains(strings.ToLower(e.Text), "pricing") {
		res, err := tx.ExecContext(ctx, `INSERT INTO runs(account_id,contact_id,comment_id,state,reply,question,link,wait_ms) VALUES(?,?,?,'reply_pending',?,?,?,?) ON CONFLICT DO NOTHING`, e.AccountID, e.ContactID, e.EventID, s.settings.Reply, s.settings.Question, s.settings.Link, s.settings.Wait.Milliseconds())
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n > 0 {
			rid, err := res.LastInsertId()
			if err != nil {
				return err
			}
			if _, err = tx.ExecContext(ctx, `INSERT INTO actions(run_id,step,status) VALUES(?,'reply_comment','pending')`, rid); err != nil {
				return err
			}
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE inbox SET status='processed' WHERE id=?`, id)
	return err
}
func (s *Service) message(ctx context.Context, tx *sql.Tx, e adapter.Event, id int64) error {
	var runID int64
	var state string
	var deadline sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT id,state,deadline FROM runs WHERE account_id=? AND contact_id=? AND state IN ('reply_pending','question_pending','waiting_email','link_pending','unknown')`, e.AccountID, e.ContactID).Scan(&runID, &state, &deadline)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE inbox SET status='processed' WHERE id=?`, id)
		return err
	}
	if err != nil {
		return err
	}
	if state == "reply_pending" || state == "question_pending" || state == "unknown" {
		_, err = tx.ExecContext(ctx, `UPDATE inbox SET status='deferred',run_id=? WHERE id=?`, runID, id)
		return err
	}
	if state == "waiting_email" && deadline.Valid && s.now().UnixMilli() < deadline.Int64 && validEmail(e.Text) {
		if _, err = tx.ExecContext(ctx, `UPDATE runs SET state='link_pending',email=? WHERE id=?`, strings.TrimSpace(e.Text), runID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO actions(run_id,step,status) VALUES(?,'send_link','pending')`, runID); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE inbox SET status='processed' WHERE id=?`, id)
	return err
}
func validEmail(text string) bool {
	text = strings.TrimSpace(text)
	a, err := mail.ParseAddress(text)
	return err == nil && a.Address == text && strings.Contains(text, "@")
}
