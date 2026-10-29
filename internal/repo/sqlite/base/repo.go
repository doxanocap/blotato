package base

import (
	"context"
	"database/sql"
	"errors"

	"braces.dev/errtrace"
	sq "github.com/Masterminds/squirrel"
)

// Repo is the common SQL and transaction boundary embedded by Repository.
type Repo interface {
	Query(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) *sql.Row
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Builder() sq.StatementBuilderType
	Tx(ctx context.Context, fn func(context.Context) error) error
}

type querier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

// BaseRepo selects the connection or transaction owned by this database.
type BaseRepo struct {
	db *sql.DB
}

type txKey struct{}

type transaction struct {
	owner *BaseRepo
	tx    *sql.Tx
}

func New(db *sql.DB) *BaseRepo {
	return &BaseRepo{db: db}
}

func (r *BaseRepo) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := r.conn(ctx).QueryContext(ctx, query, args...)
	return rows, errtrace.Wrap(err)
}

func (r *BaseRepo) QueryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.conn(ctx).QueryRowContext(ctx, query, args...)
}

func (r *BaseRepo) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	result, err := r.conn(ctx).ExecContext(ctx, query, args...)
	return result, errtrace.Wrap(err)
}

func (r *BaseRepo) Builder() sq.StatementBuilderType {
	return sq.StatementBuilder.PlaceholderFormat(sq.Question)
}

func (r *BaseRepo) conn(ctx context.Context) querier {
	if current, ok := ctx.Value(txKey{}).(transaction); ok && current.owner == r {
		return current.tx
	}

	return r.db
}

// Tx reuses a transaction owned by this repository and rolls back on errors or panic.
func (r *BaseRepo) Tx(ctx context.Context, fn func(context.Context) error) (err error) {
	if current, ok := ctx.Value(txKey{}).(transaction); ok && current.owner == r {
		return errtrace.Wrap(fn(ctx))
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return errtrace.Wrap(err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, errtrace.Wrap(rollbackErr))
		}
	}()

	txCtx := context.WithValue(ctx, txKey{}, transaction{owner: r, tx: tx})
	if err := fn(txCtx); err != nil {
		return errtrace.Wrap(err)
	}

	return errtrace.Wrap(tx.Commit())
}
