// Package sorter_test contains tests for the sorter package.
package sorter_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/code19m/pkg/sorter"
)

func TestMakeFromStr(t *testing.T) {
	tests := []struct {
		name          string
		sortString    string
		allowedFields []string
		expected      sorter.SortOpts
	}{
		{
			name:          "empty string",
			sortString:    "",
			allowedFields: []string{"name", "created_at"},
			expected:      nil,
		},
		{
			name:          "valid single sort option",
			sortString:    "name:asc",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "name", D: "asc"},
			),
		},
		{
			name:          "valid multiple sort options",
			sortString:    "name:asc,created_at:desc",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "name", D: "asc"},
				sorter.Opt{F: "created_at", D: "desc"},
			),
		},
		{
			name:          "invalid field not in allowed list",
			sortString:    "name:asc,age:desc",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "name", D: "asc"},
			),
		},
		{
			name:          "invalid direction",
			sortString:    "name:ascending,created_at:desc",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "created_at", D: "desc"},
			),
		},
		{
			name:          "invalid format missing colon",
			sortString:    "name_asc,created_at:desc",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "created_at", D: "desc"},
			),
		},
		{
			name:          "with spaces to trim",
			sortString:    " name : asc , created_at : desc ",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "name", D: "asc"},
				sorter.Opt{F: "created_at", D: "desc"},
			),
		},
		{
			name:          "mixed case direction",
			sortString:    "name:ASC,created_at:DESC",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "name", D: "asc"},
				sorter.Opt{F: "created_at", D: "desc"},
			),
		},
		{
			name:          "empty parts after splitting",
			sortString:    ",,name:asc,,created_at:desc,,",
			allowedFields: []string{"name", "created_at"},
			expected: sorter.Make(
				sorter.Opt{F: "name", D: "asc"},
				sorter.Opt{F: "created_at", D: "desc"},
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := sorter.MakeFromStr(tc.sortString, tc.allowedFields...)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestMake(t *testing.T) {
	tests := []struct {
		name     string
		options  []sorter.Opt
		expected sorter.SortOpts
	}{
		{
			name:     "empty options",
			options:  []sorter.Opt{},
			expected: sorter.SortOpts{},
		},
		{
			name: "single option",
			options: []sorter.Opt{
				{F: "name", D: "asc"},
			},
			expected: sorter.SortOpts{
				{F: "name", D: "asc"},
			},
		},
		{
			name: "multiple options",
			options: []sorter.Opt{
				{F: "name", D: "asc"},
				{F: "created_at", D: "desc"},
			},
			expected: sorter.SortOpts{
				{F: "name", D: "asc"},
				{F: "created_at", D: "desc"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := sorter.Make(tc.options...)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestOptToSQL(t *testing.T) {
	tests := []struct {
		name     string
		opt      sorter.Opt
		expected string
	}{
		{
			name:     "ascending order",
			opt:      sorter.Opt{F: "name", D: "asc"},
			expected: "name asc",
		},
		{
			name:     "descending order",
			opt:      sorter.Opt{F: "created_at", D: "desc"},
			expected: "created_at desc",
		},
		{
			name:     "with special characters",
			opt:      sorter.Opt{F: "user.name", D: "asc"},
			expected: "user.name asc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.opt.ToSQL()
			assert.Equal(t, tc.expected, actual)
		})
	}
}

// TestSortOptsUsage demonstrates how the SortOpts type might be used in a real application.
func TestSortOptsUsage(t *testing.T) {
	t.Run("converting multiple sort options to SQL", func(t *testing.T) {
		// Arrange
		sortStr := "name:asc,created_at:desc,status:asc"
		allowedFields := []string{"name", "created_at", "status", "updated_at"}

		// Act
		opts := sorter.MakeFromStr(sortStr, allowedFields...)

		// Assert
		assert.Len(t, opts, 3)

		// This would typically be used to build SQL ORDER BY clauses
		var orderClauses []string
		for _, opt := range opts {
			orderClauses = append(orderClauses, opt.ToSQL())
		}

		// Check the individual clauses
		assert.Equal(t, "name asc", orderClauses[0])
		assert.Equal(t, "created_at desc", orderClauses[1])
		assert.Equal(t, "status asc", orderClauses[2])
	})
}
