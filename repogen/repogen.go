// Package repogen provides generic repository interfaces for data access patterns.
//
// It defines generic interfaces for read-only and read-write repositories, supporting
// CRUD operations, bulk operations, and flexible filtering. These interfaces are intended
// to be implemented for various storage backends, enabling consistent and testable data access layers.
package repogen

import (
	"context"
)

// ReadOnlyRepo defines a generic read-only repository interface for entities of type E
// with filter type F. It provides methods for retrieving, listing, counting, and checking
// the existence of entities.
type ReadOnlyRepo[E any, F any] interface {
	// Get retrieves a single entity matching the provided filters.
	// Returns the entity or an error if not found or on failure.
	Get(ctx context.Context, filters F) (*E, error)
	// List returns all entities matching the provided filters.
	List(ctx context.Context, filters F) ([]E, error)
	// Count returns the number of entities matching the filters.
	Count(ctx context.Context, filters F) (int, error)
	// FirstOrNil returns the first entity matching the filters, or nil if none found.
	FirstOrNil(ctx context.Context, filters F) (*E, error)
	// Exists checks if any entity matches the filters.
	Exists(ctx context.Context, filters F) (bool, error)
}

// Repo defines a generic read-write repository interface for entities of type E
// with filter type F. It embeds ReadOnlyRepo and adds methods for creating, updating,
// deleting, and performing bulk operations on entities.
type Repo[E any, F any] interface {
	ReadOnlyRepo[E, F]
	// Create inserts a new entity and returns the created entity or an error.
	Create(ctx context.Context, entity *E) (*E, error)
	// Update modifies an existing entity and returns the updated entity or an error.
	Update(ctx context.Context, entity *E) (*E, error)
	// Delete removes an entity from the repository.
	Delete(ctx context.Context, entity *E) error

	// BulkCreate inserts multiple entities in a single operation.
	BulkCreate(ctx context.Context, entities []E) error
	// BulkUpdate updates multiple entities in a single operation.
	BulkUpdate(ctx context.Context, entities []E) error
	// BulkDelete deletes multiple entities in a single operation.
	BulkDelete(ctx context.Context, entities []E) error
}
