package entities

import "context"

// Registry defines the schema/interface for the Entity registry that will live in the index.
type Registry interface {
	// PutEntity stores or updates an entity.
	PutEntity(ctx context.Context, ent Entity) error

	// GetEntity retrieves an entity by its ID.
	GetEntity(ctx context.Context, id string) (Entity, error)

	// ListEntities lists entities of a specific kind, or all if kind is empty.
	ListEntities(ctx context.Context, kind Kind) ([]Entity, error)

	// PutRelation stores a new relation.
	PutRelation(ctx context.Context, rel Relation) error

	// GetRelations retrieves relations originating from the specified source ID.
	GetRelations(ctx context.Context, sourceID string) ([]Relation, error)

	// GetInverseRelations retrieves relations pointing to the specified target ID.
	GetInverseRelations(ctx context.Context, targetID string) ([]Relation, error)
}
