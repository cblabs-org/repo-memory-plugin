package index

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// Ensure Index implements entities.Registry
var _ entities.Registry = (*Index)(nil)

func (idx *Index) PutEntity(ctx context.Context, ent entities.Entity) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO entities (id, kind, name) VALUES (?, ?, ?)",
			ent.ID, string(ent.Kind), ent.Name)
		return err
	})
}

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

func (idx *Index) ListEntities(ctx context.Context, kind entities.Kind) ([]entities.Entity, error) {
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

func (idx *Index) PutRelation(ctx context.Context, rel entities.Relation) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT OR REPLACE INTO entity_relations (source_id, target_id, kind) VALUES (?, ?, ?)",
			rel.SourceID, rel.TargetID, string(rel.Kind))
		return err
	})
}

func (idx *Index) GetRelations(ctx context.Context, sourceID string) ([]entities.Relation, error) {
	rows, err := idx.db.QueryContext(ctx, "SELECT source_id, target_id, kind FROM entity_relations WHERE source_id = ?", sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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

func (idx *Index) GetInverseRelations(ctx context.Context, targetID string) ([]entities.Relation, error) {
	rows, err := idx.db.QueryContext(ctx, "SELECT source_id, target_id, kind FROM entity_relations WHERE target_id = ?", targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

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
