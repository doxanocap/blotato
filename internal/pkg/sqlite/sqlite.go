package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"braces.dev/errtrace"
	_ "modernc.org/sqlite"

	migrations "github.com/doxanocap/blotato/migrations/sqlite"
)

// Open prepares the persistent database without starting workflow processing.
func Open(ctx context.Context, filename string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	db.SetMaxOpenConns(1)

	if err := initializeSchema(ctx, db); err != nil {
		return nil, errtrace.Wrap(errors.Join(err, db.Close()))
	}

	return db, nil
}

func initializeSchema(ctx context.Context, db *sql.DB) (err error) {
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		return errtrace.Wrap(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, errtrace.Wrap(rollbackErr))
		}
	}()
	if err := migrateWorkflowNames(ctx, tx); err != nil {
		return errtrace.Wrap(err)
	}
	if _, err := tx.ExecContext(ctx, migrations.Schema); err != nil {
		return errtrace.Wrap(err)
	}

	// Preserve legacy runs and snapshots while adding the previously shipped columns.
	for _, column := range []struct{ table, name, definition string }{
		{"inbox", "occurred_at", "INTEGER"},
		{"runs", "answer_at", "INTEGER"},
		{"runs", "workflow_id", "INTEGER REFERENCES workflows(id)"},
		{"runs", "keyword", "TEXT"},
	} {
		found, err := hasColumn(ctx, tx, column.table, column.name)
		if err != nil {
			return errtrace.Wrap(err)
		}
		if found {
			continue
		}

		query := "ALTER TABLE " + column.table + " ADD COLUMN " + column.name + " " + column.definition
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return errtrace.Wrap(err)
		}
	}

	return errtrace.Wrap(tx.Commit())
}

type schemaQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func hasColumn(ctx context.Context, db schemaQuerier, table, column string) (_ bool, err error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, errtrace.Wrap(err)
	}
	defer func() { err = errors.Join(err, errtrace.Wrap(rows.Close())) }()

	found := false
	for rows.Next() {
		var cid, notNull, pk int
		var name, kind string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			return false, errtrace.Wrap(err)
		}

		found = found || name == column
	}

	return found, errtrace.Wrap(rows.Err())
}

// migrateWorkflowNames upgrades shipped databases without copying or deleting
// records. SQLite updates foreign-key references when the table is renamed.
// All renames and subsequent additions share the startup schema transaction.
func migrateWorkflowNames(ctx context.Context, tx *sql.Tx) error {
	var oldTable, newTable bool
	err := tx.QueryRowContext(ctx, `SELECT
EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='automations'),
EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name='workflows')`).Scan(&oldTable, &newTable)
	if err != nil {
		return errtrace.Wrap(err)
	}
	if oldTable && newTable {
		return errtrace.New("both legacy and current workflow tables exist; explicit reconciliation is required")
	}
	if oldTable {
		if _, err := tx.ExecContext(ctx, "ALTER TABLE automations RENAME TO workflows"); err != nil {
			return errtrace.Wrap(err)
		}
	}
	oldColumn, err := hasColumn(ctx, tx, "runs", "automation_id")
	if err != nil {
		return errtrace.Wrap(err)
	}
	if !oldColumn {
		return nil
	}
	newColumn, err := hasColumn(ctx, tx, "runs", "workflow_id")
	if err != nil {
		return errtrace.Wrap(err)
	}
	if newColumn {
		return errtrace.New("runs contains both legacy and current workflow identifiers")
	}
	_, err = tx.ExecContext(ctx, "ALTER TABLE runs RENAME COLUMN automation_id TO workflow_id")
	return errtrace.Wrap(err)
}
