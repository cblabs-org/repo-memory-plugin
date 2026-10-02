package index

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
)

// WriteTx executes fn inside a database transaction.
func (idx *Index) WriteTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Rebuild rebuilds the index from the provided claims and entities into a new versioned file,
// then atomically switches the pointer.
func Rebuild(ctx context.Context, cacheDir string, allClaims []*claims.Claim, allEntities []entities.Entity) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}

	version := time.Now().UnixNano()
	newDbName := fmt.Sprintf("index-%d.db", version)
	newDbPath := filepath.Join(cacheDir, newDbName)
	
	idx, err := NewIndex(newDbPath)
	if err != nil {
		return err
	}
	
	err = idx.WriteTx(ctx, func(tx *sql.Tx) error {
		// Insert Entities
		stmtEnt, err := tx.PrepareContext(ctx, "INSERT INTO entities (id, kind, name) VALUES (?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmtEnt.Close()
		
		for _, e := range allEntities {
			if _, err := stmtEnt.ExecContext(ctx, e.ID, string(e.Kind), e.Name); err != nil {
				return err
			}
		}
		
		// Insert Claims
		stmtClaim, err := tx.PrepareContext(ctx, "INSERT INTO claims (id, type, title, status, relevant_when, apply, verified_at, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmtClaim.Close()
		
		stmtAnchor, err := tx.PrepareContext(ctx, "INSERT INTO anchors (claim_id, kind, ref, fp) VALUES (?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmtAnchor.Close()
		
		stmtRelation, err := tx.PrepareContext(ctx, "INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmtRelation.Close()
		
		stmtCondition, err := tx.PrepareContext(ctx, "INSERT INTO conditions (claim_id, context, key, ref, state, target, result) VALUES (?, ?, ?, ?, ?, ?, ?)")
		if err != nil {
			return err
		}
		defer stmtCondition.Close()
		
		for _, c := range allClaims {
			if _, err := stmtClaim.ExecContext(ctx, c.ID, string(c.Type), c.Title, string(c.Status), c.RelevantWhen, c.Apply, c.VerifiedAt, c.Body); err != nil {
				return err
			}
			
			for _, a := range c.Anchors {
				if _, err := stmtAnchor.ExecContext(ctx, c.ID, a.Kind, a.Ref, a.FP); err != nil {
					return err
				}
			}
			
			if c.Relations != nil {
				insertRels := func(relType string, targets []string) error {
					for _, target := range targets {
						if _, err := stmtRelation.ExecContext(ctx, c.ID, relType, target); err != nil {
							return err
						}
					}
					return nil
				}
				if err := insertRels("supports", c.Relations.Supports); err != nil { return err }
				if err := insertRels("contradicts", c.Relations.Contradicts); err != nil { return err }
				if err := insertRels("supersedes", c.Relations.Supersedes); err != nil { return err }
				if err := insertRels("explains", c.Relations.Explains); err != nil { return err }
				if err := insertRels("refines", c.Relations.Refines); err != nil { return err }
				if err := insertRels("depends_on", c.Relations.DependsOn); err != nil { return err }
			}
			
			for k, v := range c.Conditions {
				if _, err := stmtCondition.ExecContext(ctx, c.ID, "map", k, "", "", "", v); err != nil {
					return err
				}
			}
			if c.ClosesWhen != nil {
				for _, cw := range c.ClosesWhen.Any {
					if _, err := stmtCondition.ExecContext(ctx, c.ID, "closes_when", cw.Kind, cw.Ref, cw.State, cw.Target, cw.Result); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	
	idx.Close()
	
	if err != nil {
		os.Remove(newDbPath)
		return err
	}
	
	// Atomically switch the pointer
	symlinkPath := filepath.Join(cacheDir, "index.db")
	tmpSymlink := filepath.Join(cacheDir, "index.db.tmp")
	
	_ = os.Remove(tmpSymlink)
	
	// Create symlink
	if err := os.Symlink(newDbName, tmpSymlink); err != nil {
		// Fallback to direct rename if symlink fails
		if err := os.Rename(newDbPath, symlinkPath); err != nil {
			return fmt.Errorf("switching pointer (rename fallback): %w", err)
		}
		return nil
	}
	
	if err := os.Rename(tmpSymlink, symlinkPath); err != nil {
		return fmt.Errorf("switching pointer (symlink rename): %w", err)
	}
	
	return nil
}
