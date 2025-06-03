package repogen

import (
	"context"
	"fmt"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/uptrace/bun"
)

// PgReadOnlyRepo provides read-only access to a PostgreSQL database using bun ORM.
type PgReadOnlyRepo[E any, F any] struct {
	idb          bun.IDB
	entityName   string
	notFoundCode string

	filterFunc func(q *bun.SelectQuery, filters F) *bun.SelectQuery
}

func NewPgReadOnlyRepo[E any, F any](
	idb bun.IDB,
	entityName string,
	notFoundCode string,
	filterFunc func(q *bun.SelectQuery, filters F) *bun.SelectQuery,
) *PgReadOnlyRepo[E, F] {
	return &PgReadOnlyRepo[E, F]{
		idb:          idb,
		entityName:   entityName,
		notFoundCode: notFoundCode,
		filterFunc:   filterFunc,
	}
}

func (r *PgReadOnlyRepo[E, F]) Get(ctx context.Context, filters F) (*E, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities).Limit(2) //nolint:mnd // limit 2 to check for multiple rows
	q = r.filterFunc(q, filters)

	err := q.Scan(ctx)
	if err != nil {
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if len(entities) == 0 {
		return nil, errx.New(
			fmt.Sprintf("no %s found", r.entityName),
			errx.WithCode(r.notFoundCode),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	if len(entities) > 1 {
		return nil, errx.New(
			fmt.Sprintf("multiple %s found", r.entityName),
			errx.WithCode(codeMultipleRowsFound),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	return &entities[0], nil
}

func (r *PgReadOnlyRepo[E, F]) List(ctx context.Context, filters F) ([]E, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities)
	q = r.filterFunc(q, filters)

	err := q.Scan(ctx)
	if err != nil {
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return entities, nil
}

func (r *PgReadOnlyRepo[E, F]) Count(ctx context.Context, filters F) (int, error) {
	q := r.idb.NewSelect().Model((*E)(nil))
	q = r.filterFunc(q, filters)
	q = q.Offset(0).Limit(0)

	count, err := q.Count(ctx)
	if err != nil {
		return 0, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return count, nil
}

func (r *PgReadOnlyRepo[E, F]) FirstOrNil(ctx context.Context, filters F) (*E, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities).Limit(1)
	q = r.filterFunc(q, filters)

	err := q.Scan(ctx)
	if err != nil {
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if len(entities) == 0 {
		return nil, nil //nolint:nilnil // Intentionally returning nil,nil as function name indicates
	}

	return &entities[0], nil
}

func (r *PgReadOnlyRepo[E, F]) Exists(ctx context.Context, filters F) (bool, error) {
	q := r.idb.NewSelect().Model((*E)(nil))
	q = r.filterFunc(q, filters)

	exists, err := q.Exists(ctx)
	if err != nil {
		return false, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return exists, nil
}
