package sqlite

import (
	"context"
	"errors"

	"braces.dev/errtrace"
	sq "github.com/Masterminds/squirrel"

	"github.com/doxanocap/blotato/internal/domain/entity"
	"github.com/doxanocap/blotato/internal/domain/entity/types"
)

const workflowsTable = "workflows"

func (r *Repository) CreateWorkflow(ctx context.Context, a entity.Workflow) (types.ID, error) {
	query, args, err := r.Builder().
		Insert(workflowsTable).
		Columns("keyword", "public_reply", "email_question", "link").
		Values(a.Keyword(), a.PublicReply(), a.EmailQuestion(), a.Link()).
		ToSql()
	if err != nil {
		return 0, errtrace.Wrap(err)
	}

	result, err := r.Exec(ctx, query, args...)
	if err != nil {
		return 0, errtrace.Wrap(err)
	}

	id, err := result.LastInsertId()
	return types.ID(id), errtrace.Wrap(err)
}

func (r *Repository) GetWorkflows(ctx context.Context) (_ []entity.Workflow, err error) {
	query, args, err := r.selectWorkflow().OrderBy("id").ToSql()
	if err != nil {
		return nil, errtrace.Wrap(err)
	}

	rows, err := r.Query(ctx, query, args...)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer func() { err = errors.Join(err, errtrace.Wrap(rows.Close())) }()

	workflows := []entity.Workflow{}
	for rows.Next() {
		workflow, err := r.scanWorkflow(rows)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}

		workflows = append(workflows, workflow)
	}

	return workflows, errtrace.Wrap(rows.Err())
}

func (r *Repository) selectWorkflow() sq.SelectBuilder {
	return r.Builder().
		Select("id", "keyword", "public_reply", "email_question", "link").
		From(workflowsTable)
}

func (r *Repository) scanWorkflow(row interface{ Scan(...any) error }) (entity.Workflow, error) {
	var id types.ID
	var keyword, publicReply, emailQuestion, link string
	if err := row.Scan(&id, &keyword, &publicReply, &emailQuestion, &link); err != nil {
		return entity.Workflow{}, errtrace.Wrap(err)
	}

	return entity.ScanWorkflow(id, keyword, publicReply, emailQuestion, link), nil
}
