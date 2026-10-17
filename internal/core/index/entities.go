/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/index/entities.go
 * Description: Implements the entities.Registry contract on Index, providing
 *               SQLite persistence and lookup for repository entities and
 *               their directed relations.
 *
 * Exported Types / Functions:
 * - Index:
 *
 *   Registry (entities.Registry):
 *   - PutEntity():           Upserts an entity record into the entities table.
 *   - GetEntity():           Retrieves a single entity by its identifier.
 *   - ListEntities():        Lists all entities matching an optional kind filter.
 *   - PutRelation():         Upserts a directed relation between two entities.
 *   - GetRelations():        Returns outbound relations originating from a source entity.
 *   - GetInverseRelations(): Returns inbound relations targeting a destination entity.
 */

package index

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// Compile-time check ensuring Index implements entities.Registry.
var _ entities.Registry = (*Index)(nil)

// PutEntity inserts or replaces an entity record in the entities table within a write transaction.
func (idx *Index) PutEntity(ctx context.Context, ent entities.Entity) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO entities (id, kind, name) VALUES (?, ?, ?)",
			ent.ID, string(ent.Kind), ent.Name)
		return err
	})
}

// GetEntity retrieves an entity by its unique ID.
func (idx *Index) GetEntity(ctx context.Context, id string) (entities.Entity, error) {
	var e entities.Entity
	var kind string
	err := idx.db.QueryRowContext(ctx, "SELECT id, kind, name FROM entities WHERE id = ?", id).Scan(&e.ID, &kind, &e.Name)
	if err != nil {
		if err == sql.ErrNoRows {
			return e, fmt.Errorf("entity not found")
		}
		return e, err
	}
	e.Kind = entities.Kind(kind)
	return e, nil
}

// ListEntities returns all entities stored in the index, optionally filtered by kind when non-empty.
func (idx *Index) ListEntities(ctx context.Context, kind entities.Kind) ([]entities.Entity, error) {
	// --- Step 1: Build and execute the entity selection query ---
	query := "SELECT id, kind, name FROM entities"
	var args []interface{}
	if kind != "" {
		query += " WHERE kind = ?"
		args = append(args, string(kind))
	}
	rows, err := idx.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// --- Step 2: Scan matching rows into Entity domain objects ---
	var result []entities.Entity
	for rows.Next() {
		var e entities.Entity
		var k string
		if err := rows.Scan(&e.ID, &k, &e.Name); err != nil {
			return nil, err
		}
		e.Kind = entities.Kind(k)
		result = append(result, e)
	}
	return result, rows.Err()
}

// PutRelation inserts or replaces a directed relation between two entities in the index.
func (idx *Index) PutRelation(ctx context.Context, rel entities.Relation) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO entity_relations (source_id, target_id, kind) VALUES (?, ?, ?)",
			rel.SourceID, rel.TargetID, string(rel.Kind))
		return err
	})
}

// GetRelations returns all outbound relations originating from sourceID.
func (idx *Index) GetRelations(ctx context.Context, sourceID string) ([]entities.Relation, error) {
	// --- Step 1: Query outbound relations by source entity ID ---
	rows, err := idx.db.QueryContext(ctx, "SELECT source_id, target_id, kind FROM entity_relations WHERE source_id = ?", sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// --- Step 2: Decode relation rows ---
	var result []entities.Relation
	for rows.Next() {
		var r entities.Relation
		var k string
		if err := rows.Scan(&r.SourceID, &r.TargetID, &k); err != nil {
			return nil, err
		}
		r.Kind = entities.RelationKind(k)
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetInverseRelations returns all inbound relations targeting targetID.
func (idx *Index) GetInverseRelations(ctx context.Context, targetID string) ([]entities.Relation, error) {
	// --- Step 1: Query inbound relations by target entity ID ---
	rows, err := idx.db.QueryContext(ctx, "SELECT source_id, target_id, kind FROM entity_relations WHERE target_id = ?", targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// --- Step 2: Decode relation rows ---
	var result []entities.Relation
	for rows.Next() {
		var r entities.Relation
		var k string
		if err := rows.Scan(&r.SourceID, &r.TargetID, &k); err != nil {
			return nil, err
		}
		r.Kind = entities.RelationKind(k)
		result = append(result, r)
	}
	return result, rows.Err()
}
