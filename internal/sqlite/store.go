package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	if _, err = db.Exec(`PRAGMA foreign_keys=ON;
 CREATE TABLE IF NOT EXISTS inbox (id INTEGER PRIMARY KEY, platform TEXT NOT NULL, account_id TEXT NOT NULL, contact_id TEXT NOT NULL, kind TEXT NOT NULL, event_id TEXT NOT NULL, text TEXT NOT NULL, received_at INTEGER NOT NULL, status TEXT NOT NULL, run_id INTEGER, UNIQUE(platform,account_id,kind,event_id));
 CREATE TABLE IF NOT EXISTS runs (id INTEGER PRIMARY KEY, account_id TEXT NOT NULL, contact_id TEXT NOT NULL, comment_id TEXT NOT NULL, state TEXT NOT NULL, reply TEXT NOT NULL, question TEXT NOT NULL, link TEXT NOT NULL, wait_ms INTEGER NOT NULL, deadline INTEGER, email TEXT, error_code TEXT NOT NULL DEFAULT '');
 CREATE UNIQUE INDEX IF NOT EXISTS active_run ON runs(account_id,contact_id) WHERE state IN ('reply_pending','question_pending','waiting_email','link_pending','unknown');
 CREATE TABLE IF NOT EXISTS actions (id INTEGER PRIMARY KEY, run_id INTEGER NOT NULL REFERENCES runs(id), step TEXT NOT NULL, status TEXT NOT NULL, external_id TEXT NOT NULL DEFAULT '', error_code TEXT NOT NULL DEFAULT '', UNIQUE(run_id,step));`); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize sqlite: %w", err)
	}
	// An interrupted call may have reached the platform. Never retry it blindly.
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		db.Close()
		return nil, err
	}
	if _, err = tx.Exec(`UPDATE runs SET state='unknown', error_code='interrupted_send' WHERE id IN (SELECT run_id FROM actions WHERE status='sending')`); err == nil {
		_, err = tx.Exec(`UPDATE actions SET status='unknown', error_code='interrupted_send' WHERE status='sending'`)
	}
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("recover sends: %w", err)
	}
	if err = tx.Commit(); err != nil {
		db.Close()
		return nil, fmt.Errorf("commit recovery: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }
