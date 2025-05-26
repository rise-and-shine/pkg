package repogen

import (
	"context"
	"fmt"

	"github.com/code19m/errx"
	"github.com/code19m/pkg/pg"
	"github.com/uptrace/bun"
)

const (
	codeMultipleRowsFound      = "MULTIPLE_ROWS_FOUND"
	codeIncorrectRowsAffection = "INCORRECT_ROWS_AFFECTTION"

	largeBulkSize = 10
)

// PgRepo provides CRUD + Bulk operations for a PostgreSQL database using bun ORM.
type PgRepo[E any, F any] struct {
	*PgReadOnlyRepo[E, F]
	conflictCode string
}

func NewPgRepo[E any, F any](
	idb bun.IDB,
	entityName string,
	notFoundCode string,
	conflictCode string,
	filterFunc func(q *bun.SelectQuery, filters F) *bun.SelectQuery,
) *PgRepo[E, F] {
	roRepo := NewPgReadOnlyRepo[E, F](idb, entityName, notFoundCode, filterFunc)

	return &PgRepo[E, F]{
		PgReadOnlyRepo: roRepo,
		conflictCode:   conflictCode,
	}
}

func (r *PgRepo[E, F]) Create(ctx context.Context, entity *E) (*E, error) {
	q := r.idb.NewInsert().Model(entity).Returning("*")
	_, err := q.Exec(ctx)
	if err != nil {
		if pg.IsConflict(err) {
			return nil, errx.New(
				fmt.Sprintf("conflict while creating %s", r.entityName),
				errx.WithCode(r.conflictCode),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return entity, nil
}

func (r *PgRepo[E, F]) Update(ctx context.Context, entity *E) (*E, error) {
	q := r.idb.NewUpdate().Model(entity).WherePK().Returning("*")
	result, err := q.Exec(ctx)
	if err != nil {
		if pg.IsConflict(err) {
			return nil, errx.New(
				fmt.Sprintf("conflict while updating %s", r.entityName),
				errx.WithCode(r.conflictCode),
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
	_, err := q.Exec(ctx)
	if err != nil {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large inserts
		}
		if pg.IsConflict(err) {
			return errx.New(
				fmt.Sprintf("conflict while bulk creating %s", r.entityName),
				errx.WithCode(r.conflictCode),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return nil
}

func (r *PgRepo[E, F]) BulkUpdate(ctx context.Context, entities []E) error {
	q := r.idb.NewUpdate().Model(&entities).Bulk()
	result, err := q.Exec(ctx)
	if err != nil {
		if len(entities) > largeBulkSize {
			q = nil // Set q to nil to avoid huge log size in large updates
		}
		if pg.IsConflict(err) {
			return errx.New(
				fmt.Sprintf("conflict while bulk updating %s", r.entityName),
				errx.WithCode(r.conflictCode),
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
