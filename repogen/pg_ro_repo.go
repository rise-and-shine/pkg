package repogen

import (
	"context"
	"fmt"
	"reflect"

	"github.com/code19m/errx"
	"github.com/rise-and-shine/pkg/pg"
	"github.com/uptrace/bun"
)

// PgReadOnlyRepo provides read-only access to a PostgreSQL database using bun ORM.
type PgReadOnlyRepo[E any, F any] struct {
	idb          bun.IDB
	schemaName   string
	notFoundCode string

	filterFunc func(q *bun.SelectQuery, filters F) *bun.SelectQuery
}

// PgReadOnlyRepoBuilder is a builder for PgReadOnlyRepo with sensible defaults.
type PgReadOnlyRepoBuilder[E any, F any] struct {
	idb          bun.IDB
	schemaName   string
	notFoundCode string
	filterFunc   func(q *bun.SelectQuery, filters F) *bun.SelectQuery
}

// NewPgReadOnlyRepoBuilder creates a new builder with sensible defaults.
func NewPgReadOnlyRepoBuilder[E any, F any](idb bun.IDB) *PgReadOnlyRepoBuilder[E, F] {
	return &PgReadOnlyRepoBuilder[E, F]{
		idb:          idb,
		schemaName:   "public",
		notFoundCode: "OBJECT_NOT_FOUND",
		filterFunc:   func(q *bun.SelectQuery, _ F) *bun.SelectQuery { return q },
	}
}

// WithSchemaName sets the schema name.
func (b *PgReadOnlyRepoBuilder[E, F]) WithSchemaName(name string) *PgReadOnlyRepoBuilder[E, F] {
	b.schemaName = name
	return b
}

// WithNotFoundCode sets the error code for not found errors.
func (b *PgReadOnlyRepoBuilder[E, F]) WithNotFoundCode(code string) *PgReadOnlyRepoBuilder[E, F] {
	b.notFoundCode = code
	return b
}

// WithFilterFunc sets the filter function.
func (b *PgReadOnlyRepoBuilder[E, F]) WithFilterFunc(
	fn func(q *bun.SelectQuery, filters F) *bun.SelectQuery,
) *PgReadOnlyRepoBuilder[E, F] {
	b.filterFunc = fn
	return b
}

// Build creates the PgReadOnlyRepo.
func (b *PgReadOnlyRepoBuilder[E, F]) Build() *PgReadOnlyRepo[E, F] {
	return &PgReadOnlyRepo[E, F]{
		idb:          b.idb,
		schemaName:   b.schemaName,
		notFoundCode: b.notFoundCode,
		filterFunc:   b.filterFunc,
	}
}

func (r *PgReadOnlyRepo[E, F]) Get(ctx context.Context, filters F) (*E, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities).Limit(2) //nolint:mnd // limit 2 to check for multiple rows
	q = r.applyModelTableExpr(q)
	q = r.filterFunc(q, filters)

	err := q.Scan(ctx)
	if err != nil {
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	if len(entities) == 0 {
		return nil, errx.New(
			fmt.Sprintf("no %s found", nameOf(new(E))),
			errx.WithCode(r.notFoundCode),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	if len(entities) > 1 {
		return nil, errx.New(
			fmt.Sprintf("multiple %s found", nameOf(new(E))),
			errx.WithCode(codeMultipleRowsFound),
			errx.WithDetails(pg.GetPgErrorDetails(err, q)),
		)
	}

	return &entities[0], nil
}

func (r *PgReadOnlyRepo[E, F]) List(ctx context.Context, filters F) ([]E, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities)
	q = r.applyModelTableExpr(q)
	q = r.filterFunc(q, filters)

	err := q.Scan(ctx)
	if err != nil {
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return entities, nil
}

func (r *PgReadOnlyRepo[E, F]) Count(ctx context.Context, filters F) (int, error) {
	q := r.idb.NewSelect().Model((*E)(nil))
	q = r.applyModelTableExpr(q)
	q = r.filterFunc(q, filters)
	q = q.Offset(0).Limit(0)

	count, err := q.Count(ctx)
	if err != nil {
		return 0, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return count, nil
}

func (r *PgReadOnlyRepo[E, F]) ListWithCount(ctx context.Context, filters F) ([]E, int, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities)
	q = r.applyModelTableExpr(q)
	q = r.filterFunc(q, filters)

	err := q.Scan(ctx)
	if err != nil {
		return nil, 0, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	q = q.Offset(0).Limit(0)
	count, err := q.Count(ctx)
	if err != nil {
		return nil, 0, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return entities, count, nil
}

func (r *PgReadOnlyRepo[E, F]) FirstOrNil(ctx context.Context, filters F) (*E, error) {
	var entities = make([]E, 0)
	q := r.idb.NewSelect().Model(&entities).Limit(1)
	q = r.applyModelTableExpr(q)
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
	q = r.applyModelTableExpr(q)
	q = r.filterFunc(q, filters)

	exists, err := q.Exists(ctx)
	if err != nil {
		return false, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return exists, nil
}

func (r *PgReadOnlyRepo[E, F]) applyModelTableExpr(q *bun.SelectQuery) *bun.SelectQuery {
	table := q.GetModel().(bun.TableModel).Table() //nolint:errcheck // table name is always available
	return q.ModelTableExpr("?.? AS ?", bun.Ident(r.schemaName), bun.Ident(table.Name), bun.Ident(table.Alias))
}

// nameOf returns the name of the type of the given value.
// If the value is a pointer, it returns the name of the pointed-to type.
func nameOf(v any) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Pointer {
		return t.Elem().Name()
	}
	return t.Name()
}
