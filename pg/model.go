package pg

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// BaseModel provides common timestamp fields that can be embedded in other models.
type BaseModel struct {
	// CreatedAt stores the timestamp when the record was created.
	CreatedAt time.Time `bun:",nullzero" json:"created_at"`
	// UpdatedAt stores the timestamp when the record was last updated.
	UpdatedAt time.Time `bun:",nullzero" json:"updated_at"`
}

// Verify that BaseModel implements bun.BeforeAppendModelHook.
var _ bun.BeforeAppendModelHook = (*BaseModel)(nil)

// BeforeAppendModel implements bun.BeforeAppendModelHook interface to automatically
// update timestamp fields before database operations.
func (m *BaseModel) BeforeAppendModel(_ context.Context, query bun.Query) error {
	switch query.(type) {
	case *bun.InsertQuery:
		m.CreatedAt = time.Now()
		m.UpdatedAt = time.Now()
	case *bun.UpdateQuery:
		m.UpdatedAt = time.Now()
	}
	return nil
}
