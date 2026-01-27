package repogen

import (
	"context"
	"fmt"
	"strings"

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

// PgRepoBuilder is a builder for PgRepo with sensible defaults.
type PgRepoBuilder[E any, F any] struct {
	idb              bun.IDB
	schemaName       string
	notFoundCode     string
	conflictCodesMap map[string]string
	filterFunc       func(q *bun.SelectQuery, filters F) *bun.SelectQuery
}

// NewPgRepoBuilder creates a new builder with sensible defaults.
func NewPgRepoBuilder[E any, F any](idb bun.IDB) *PgRepoBuilder[E, F] {
	return &PgRepoBuilder[E, F]{
		idb:              idb,
		schemaName:       "public",
		notFoundCode:     "OBJECT_NOT_FOUND",
		conflictCodesMap: make(map[string]string),
		filterFunc:       func(q *bun.SelectQuery, _ F) *bun.SelectQuery { return q },
	}
}

// WithSchemaName sets the schema name.
func (b *PgRepoBuilder[E, F]) WithSchemaName(name string) *PgRepoBuilder[E, F] {
	b.schemaName = name
	return b
}

// WithNotFoundCode sets the error code for not found errors.
func (b *PgRepoBuilder[E, F]) WithNotFoundCode(code string) *PgRepoBuilder[E, F] {
	b.notFoundCode = code
	return b
}

// WithConflictCodesMap sets the map of constraint names to error codes.
func (b *PgRepoBuilder[E, F]) WithConflictCodesMap(m map[string]string) *PgRepoBuilder[E, F] {
	b.conflictCodesMap = m
	return b
}

// WithFilterFunc sets the filter function.
func (b *PgRepoBuilder[E, F]) WithFilterFunc(
	fn func(q *bun.SelectQuery, filters F) *bun.SelectQuery,
) *PgRepoBuilder[E, F] {
	b.filterFunc = fn
	return b
}

// Build creates the PgRepo.
func (b *PgRepoBuilder[E, F]) Build() *PgRepo[E, F] {
	return &PgRepo[E, F]{
		PgReadOnlyRepo: &PgReadOnlyRepo[E, F]{
			idb:          b.idb,
			schemaName:   b.schemaName,
			notFoundCode: b.notFoundCode,
			filterFunc:   b.filterFunc,
		},
		conflictCodesMap: b.conflictCodesMap,
	}
}

func (r *PgRepo[E, F]) Create(ctx context.Context, entity *E) (*E, error) {
	q := r.idb.NewInsert().Model(entity)
	q = r.applyInsertModelTableExpr(q)
	q = q.Returning(returningColumns(q))
	_, err := q.Exec(ctx)
	if err != nil {
		if code, exists := r.conflictCodesMap[pg.ConstraintName(err)]; exists {
			return nil, errx.New(
				fmt.Sprintf("conflict while creating %s", nameOf(new(E))),
				errx.WithCode(code),
				errx.WithDetails(pg.GetPgErrorDetails(err, q)),
			)
		}
		return nil, errx.Wrap(err, errx.WithDetails(pg.GetPgErrorDetails(err, q)))
	}

	return entity, nil
}

func (r *PgRepo[E, F]) Update(ctx context.Context, entity *E) (*E, error) {
	q := r.idb.NewUpdate().Model(entity).WherePK()
	q = r.applyUpdateModelTableExpr(q)
	q = q.Returning(returningColumns(q))
	result, err := q.Exec(ctx)
	if err != nil {
		if code, exists := r.conflictCodesMap[pg.ConstraintName(err)]; exists {
			return nil, errx.New(
				fmt.Sprintf("conflict while updating %s", nameOf(new(E))),
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
			fmt.Sprintf("no %s found to update", nameOf(new(E))),
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
			fmt.Sprintf("no %s found to delete", nameOf(new(E))),
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
				fmt.Sprintf("conflict while bulk creating %s", nameOf(new(E))),
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
				fmt.Sprintf("conflict while bulk updating %s", nameOf(new(E))),
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
			fmt.Sprintf("not all %s were updated", nameOf(new(E))),
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
			fmt.Sprintf("not all %s were deleted", nameOf(new(E))),
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

// returningColumns builds a comma-separated list of column names from the table schema.
func returningColumns(q bun.Query) string {
	table := q.GetModel().(bun.TableModel).Table() //nolint:errcheck // table name is always available
	if len(table.Fields) == 0 {
		return "*"
	}
	names := make([]string, 0, len(table.Fields))
	for _, field := range table.Fields {
		names = append(names, string(field.SQLName))
	}
	return strings.Join(names, ", ")
}
