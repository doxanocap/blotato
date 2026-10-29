package sqlite

import (
	"database/sql"
	"time"

	"github.com/doxanocap/blotato/internal/repo/sqlite/base"
)

// Repository groups queries for workflows, inbox, runs and actions.
type Repository struct {
	base.Repo
}

func New(db *sql.DB) Repository {
	return Repository{Repo: base.New(db)}
}

func nullableMillis(t time.Time) any {
	if t.IsZero() {
		return nil
	}

	return t.UnixMilli()
}
