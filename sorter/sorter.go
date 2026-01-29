// Package sorter provides utilities for parsing and working with sorting options.
// It supports parsing sorting strings (e.g., "name:asc,created_at:desc") into structured
// sorting options and converting them into SQL-compatible order clauses.
package sorter

import (
	"slices"
	"strings"
)

type (
	SortOpts []Opt

	SortDirection string
)

const (
	Asc  SortDirection = "asc"
	Desc SortDirection = "desc"

	// expectedPartsCount is the expected number of parts in a sort option (field:direction).
	expectedPartsCount = 2
)

// MakeFromStr parses a sorting string (e.g., "name:asc,created_at:desc") into a slice of Opt.
// It filters out invalid or disallowed fields and directions, ensuring only valid options are returned.
// The allowedFields parameter specifies the list of fields that are permitted for sorting.
func MakeFromStr(sortString string, allowedFields ...string) SortOpts {
	if sortString == "" {
		return nil
	}

	var options []Opt
	pairs := strings.SplitSeq(sortString, ",")
	for pair := range pairs {
		parts := strings.Split(pair, ":")
		if len(parts) != expectedPartsCount {
			continue
		}

		key := strings.TrimSpace(parts[0])
		if !slices.Contains(allowedFields, key) {
			continue
		}

		direction := strings.ToLower(strings.TrimSpace(parts[1]))
		if direction != string(Asc) && direction != string(Desc) {
			continue
		}

		options = append(options, Opt{
			F: key,
			D: SortDirection(direction),
		})
	}

	return options
}

// Make creates a slice of Opt from a variadic list of Opt.
// It is a convenience function for creating a slice of sorting options
// without manually initializing a slice.
func Make(sortOptions ...Opt) SortOpts {
	return sortOptions
}

// Opt represents a single sorting option, consisting of a field and a direction.
type Opt struct {
	F string        // F is the field to sort by.
	D SortDirection // D is the sorting direction (asc or desc).
}

// ToSQL converts an Opt into an SQL-compatible clause (e.g., "name ASC").
func (o Opt) ToSQL() string {
	return o.F + " " + string(o.D)
}
