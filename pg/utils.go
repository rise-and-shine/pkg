package pg

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/code19m/errx"
	"github.com/jackc/pgx/v5/pgconn"
)

// PostgreSQL error code for unique constraint violations.
const pgConflictCode = "23505"

// IsConflict checks if the error is a PostgreSQL unique constraint violation.
func IsConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == pgConflictCode
	}
	return false
}

// IsNotFound checks if the error indicates that no rows were found.
func IsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

// GetPgErrorDetails extracts detailed information from a PostgreSQL error.
func GetPgErrorDetails(err error, query fmt.Stringer) errx.D {
	details := make(errx.D)
	queryStr := getSafeQueryString(query)
	if queryStr != "" {
		details["query"] = strings.ReplaceAll(queryStr, `"`, ``)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return details
	}

	details["pg.code"] = pgErr.Code
	details["pg.severity"] = pgErr.Severity
	details["pg.message"] = pgErr.Message
	details["pg.detail"] = pgErr.Detail
	details["pg.hint"] = pgErr.Hint
	details["pg.schema"] = pgErr.SchemaName
	details["pg.table"] = pgErr.TableName
	details["pg.column"] = pgErr.ColumnName
	details["pg.data_type"] = pgErr.DataTypeName
	details["pg.constraint"] = pgErr.ConstraintName

	return details
}

// getSafeQueryString safely converts a query to a string, preventing panics.
//
// Some query implementations, like bun.InsertQuery, can panic when String() is called
// in certain conditions. This function safely handles those cases by recovering
// from any panics that might occur.
//
// Returns the string representation of the query, or an empty string if
// query is nil or if String() panics.
func getSafeQueryString(query fmt.Stringer) string {
	defer func() {
		_ = recover()
	}()

	if query == nil {
		return ""
	}

	return query.String()
}
