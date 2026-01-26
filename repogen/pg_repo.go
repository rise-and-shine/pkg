package repogen

import (
	"context"
	"fmt"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/uptrace/bun"
)

const (
	codeMultipleRowsFound      = "MULTIPLE_ROWS_FOUND"
	codeIncorrectRowsAffection = "INCORRECT_ROWS_AFFECTION"

	largeBulkSize = 10
)

// PgRepo provides CRUD + Bulk operations for a PostgreSQL database using bun ORM.
type PgRepo[E any, F any] struct {
	*PgReadOnlyRepo[E, F]

	// conflictCodesMap is a map of PostgreSQL constraint names to error codes. E.g. map["users_email_key"] = "EMAIL_ALREADY_EXISTS"
	conflictCodesMap map[string]string
}

func NewPgRepo[E any, F any](
	idb bun.IDB,
	entityName string,
	schemaName string,
	notFoundCode string,
	conflictCodesMap map[string]string,
	filterFunc func(q *bun.SelectQuery, filters F) *bun.SelectQuery,
) *PgRepo[E, F] {
	roRepo := NewPgReadOnlyRepo[E](idb, entityName, schemaName, notFoundCode, filterFunc)

	return &PgRepo[E, F]{
		PgReadOnlyRepo:   roRepo,
		conflictCodesMap: conflictCodesMap,
	}
}

func (r *PgRepo[E, F]) Create(ctx context.Context, entity *E) (*E, error) {
	q := r.idb.NewInsert().Model(entity).Returning("*")
	q = r.applyInsertModelTableExpr(q)
	_, err := q.Exec(ctx)
	if err != nil {
		if code, exists := r.conflictCodesMap[pg.ConstraintName(err)]; exists {
			return nil, errx.New(
				fmt.Sprintf("conflict while creating %s", r.entityName),
				errx.WithCode(code),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return entity, nil
}

func (r *PgRepo[E, F]) Update(ctx context.Context, entity *E) (*E, error) {
	q := r.idb.NewUpdate().Model(entity).WherePK().Returning("*")
	q = r.applyUpdateModelTableExpr(q)
	result, err := q.Exec(ctx)
	if err != nil {
		if code, exists := r.conflictCodesMap[pg.ConstraintName(err)]; exists {
			return nil, errx.New(
				fmt.Sprintf("conflict while updating %s", r.entityName),
				errx.WithCode(code),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if rowsAffected == 0 {
		return nil, errx.New(
			fmt.Sprintf("no %s found to update", r.entityName),
			errx.WithCode(codeIncorrectRowsAffection),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	return entity, nil
}

func (r *PgRepo[E, F]) Delete(ctx context.Context, entity *E) error {
	q := r.idb.NewDelete().Model(entity).WherePK()
	q = r.applyDeleteModelTableExpr(q)
	result, err := q.Exec(ctx)
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if rowsAffected == 0 {
		return errx.New(
			fmt.Sprintf("no %s found to delete", r.entityName),
			errx.WithCode(codeIncorrectRowsAffection),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	return nil
}

func (r *PgRepo[E, F]) BulkCreate(ctx context.Context, entities []E) error {
	q := r.idb.NewInsert().Model(&entities)
	q = r.applyInsertModelTableExpr(q)
	_, err := q.Exec(ctx)
	if err != nil {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large inserts
		}
		if code, exists := r.conflictCodesMap[pg.ConstraintName(err)]; exists {
			return errx.New(
				fmt.Sprintf("conflict while bulk creating %s", r.entityName),
				errx.WithCode(code),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return nil
}

func (r *PgRepo[E, F]) BulkUpdate(ctx context.Context, entities []E) error {
	q := r.idb.NewUpdate().Model(&entities).Bulk()
	q = r.applyUpdateModelTableExpr(q)
	result, err := q.Exec(ctx)
	if err != nil {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large updates
		}
		if code, exists := r.conflictCodesMap[pg.ConstraintName(err)]; exists {
			return errx.New(
				fmt.Sprintf("conflict while bulk updating %s", r.entityName),
				errx.WithCode(code),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if rowsAffected != int64(len(entities)) {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large updates
		}
		return errx.New(
			fmt.Sprintf("not all %s were updated", r.entityName),
			errx.WithCode(codeIncorrectRowsAffection),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	return nil
}

func (r *PgRepo[E, F]) BulkDelete(ctx context.Context, entities []E) error {
	q := r.idb.NewDelete().Model(&entities).WherePK()
	q = r.applyDeleteModelTableExpr(q)
	result, err := q.Exec(ctx)
	if err != nil {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large deletes
		}
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if rowsAffected != int64(len(entities)) {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large deletes
		}
		return errx.New(
			fmt.Sprintf("not all %s were deleted", r.entityName),
			errx.WithCode(codeIncorrectRowsAffection),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	return nil
}

func (r *PgRepo[E, F]) applyInsertModelTableExpr(q *bun.InsertQuery) *bun.InsertQuery {
	table := q.GetModel().(bun.TableModel).Table() //nolint:errcheck // table name is always available
	return q.ModelTableExpr("?.? AS ?", bun.Ident(r.schemaName), bun.Ident(table.Name), bun.Ident(table.Alias))
}

func (r *PgRepo[E, F]) applyUpdateModelTableExpr(q *bun.UpdateQuery) *bun.UpdateQuery {
	table := q.GetModel().(bun.TableModel).Table() //nolint:errcheck // table name is always available
	return q.ModelTableExpr("?.? AS ?", bun.Ident(r.schemaName), bun.Ident(table.Name), bun.Ident(table.Alias))
}

func (r *PgRepo[E, F]) applyDeleteModelTableExpr(q *bun.DeleteQuery) *bun.DeleteQuery {
	table := q.GetModel().(bun.TableModel).Table() //nolint:errcheck // table name is always available
	return q.ModelTableExpr("?.? AS ?", bun.Ident(r.schemaName), bun.Ident(table.Name), bun.Ident(table.Alias))
}
