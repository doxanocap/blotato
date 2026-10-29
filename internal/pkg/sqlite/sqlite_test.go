package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestUpgradeExistingDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE inbox (id INTEGER PRIMARY KEY, platform TEXT NOT NULL, account_id TEXT NOT NULL, contact_id TEXT NOT NULL, kind TEXT NOT NULL, event_id TEXT NOT NULL, text TEXT NOT NULL, received_at INTEGER NOT NULL, status TEXT NOT NULL, run_id INTEGER, UNIQUE(platform,account_id,kind,event_id));
CREATE TABLE runs (id INTEGER PRIMARY KEY, account_id TEXT NOT NULL, contact_id TEXT NOT NULL, comment_id TEXT NOT NULL, state TEXT NOT NULL, reply TEXT NOT NULL, question TEXT NOT NULL, link TEXT NOT NULL, wait_ms INTEGER NOT NULL, deadline INTEGER, email TEXT, error_code TEXT NOT NULL DEFAULT '');
INSERT INTO runs(account_id,contact_id,comment_id,state,reply,question,link,wait_ms) VALUES('brand','person','c1','waiting_email','r','q','l',1000);`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	var answer sql.NullInt64
	var workflowID sql.NullInt64
	var state string
	if err := store.QueryRow(`SELECT state,answer_at,workflow_id FROM runs`).Scan(&state, &answer, &workflowID); err != nil {
		t.Fatal(err)
	}
	if state != "waiting_email" || answer.Valid || workflowID.Valid {
		t.Fatalf("state=%s answer valid=%v", state, answer.Valid)
	}
	var tableCount int
	if err := store.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='email_decisions'`).Scan(&tableCount); err != nil || tableCount != 1 {
		t.Fatalf("email_decisions table count=%d err=%v", tableCount, err)
	}
	if _, err := store.Exec(`INSERT INTO inbox(platform,account_id,contact_id,kind,event_id,text,received_at,occurred_at,status) VALUES('p','a','u','message','m1','x',1,NULL,'processed')`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
}
