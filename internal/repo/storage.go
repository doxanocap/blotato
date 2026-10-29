package repo

import (
	"database/sql"

	"github.com/doxanocap/blotato/internal/repo/sqlite"
)

// Repo assembles the persistence used by the application.
type Repo struct {
	sqliteRepo sqlite.Repository
}

func New(db *sql.DB) Repo {
	return Repo{sqliteRepo: sqlite.New(db)}
}

func (r *Repo) SQLite() *sqlite.Repository {
	return &r.sqliteRepo
}
