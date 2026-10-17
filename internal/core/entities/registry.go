/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/registry.go
 * Description: Defines the storage and query contract for persisting and
 *              traversing repository entities and their directed relations.
 *
 * Exported Types / Functions:
 * - Registry: Interface for storing and querying entities and relations in the index.
 *   - PutEntity():           Stores or updates an entity record.
 *   - GetEntity():           Retrieves a single entity by canonical ID.
 *   - ListEntities():        Lists entities filtered by Kind (or all when Kind is empty).
 *   - PutRelation():         Stores a directed relation between two entities.
 *   - GetRelations():        Returns outbound relations originating from a source entity ID.
 *   - GetInverseRelations(): Returns inbound relations pointing to a target entity ID.
 */

package entities

import "context"

// Registry defines the schema and storage interface for the entity registry in the index.
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
