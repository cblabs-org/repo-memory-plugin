package index

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// RebuildV1Index is an alias for the legacy v1 Rebuild function.
var RebuildV1Index = Rebuild

// RebuildFromOpLog rebuilds all v2 index tables from the op log reader.
func (idx *Index) RebuildFromOpLog(ctx context.Context, opsReader *ops.Reader) error {
	if opsReader == nil {
		return fmt.Errorf("ops reader cannot be nil")
	}

	// 1. Clear all v2 tables
	err := idx.WriteTx(ctx, func(tx *sql.Tx) error {
		tables := []string{"entry_anchors", "error_patterns", "aliases", "entry_history", "entries"}
		for _, t := range tables {
			if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", t)); err != nil {
				return fmt.Errorf("clearing table %s: %w", t, err)
			}
		}
		_, _ = tx.ExecContext(ctx, "INSERT INTO entries_fts(entries_fts) VALUES('rebuild')")
		return nil
	})
	if err != nil {
		return fmt.Errorf("clearing v2 index tables: %w", err)
	}

	// 2. Project dossiers from op log
	projector := dossiers.NewProjector(opsReader)
	dossierMap, err := projector.Project(ctx)
	if err != nil {
		return fmt.Errorf("projecting dossiers: %w", err)
	}

	// 3. For each dossier and its entries: UpsertEntry, UpsertEntryAnchors
	// For failure-signature entries: InsertErrorPattern
	// For retired/superseded entries in History: InsertHistory
	for _, d := range dossierMap {
		for _, entry := range d.Entries {
			if err := idx.UpsertEntry(ctx, entry); err != nil {
				return fmt.Errorf("upserting entry %s: %w", entry.ID, err)
			}
			if len(entry.Anchors) > 0 {
				if err := idx.UpsertEntryAnchors(ctx, entry.ID, entry.Anchors); err != nil {
					return fmt.Errorf("upserting anchors for %s: %w", entry.ID, err)
				}
			}
			if entry.Type == dossiers.EntryTypeFailureSignature {
				cmdPrefix, pattern := parseErrorPatternFromEntry(entry)
				if err := idx.InsertErrorPattern(ctx, entry.ID, cmdPrefix, pattern, entry.DossierScope); err != nil {
					return fmt.Errorf("inserting error pattern for %s: %w", entry.ID, err)
				}
			}
		}

		for _, hist := range d.History {
			reason := hist.RetiredReason
			if reason == "" {
				reason = "retired"
			}
			if err := idx.InsertHistory(ctx, hist, reason); err != nil {
				return fmt.Errorf("inserting history for %s: %w", hist.ID, err)
			}
		}
	}

	// 4. Build aliases from config.BuildAliasMap
	aliasMap, err := config.BuildAliasMap(opsReader, ctx)
	if err != nil {
		return fmt.Errorf("building alias map: %w", err)
	}
	if aliasMap != nil {
		for _, oldScope := range aliasMap.AllAliases() {
			if aliasMap.HasAlias(oldScope) {
				resolved := aliasMap.Resolve(oldScope)
				if resolved != "" && resolved != oldScope {
					if err := idx.UpsertAlias(ctx, oldScope, resolved); err != nil {
						return fmt.Errorf("upserting alias %s -> %s: %w", oldScope, resolved, err)
					}
				}
			}
		}
	}

	return nil
}

func parseErrorPatternFromEntry(entry dossiers.Entry) (cmdPrefix, pattern string) {
	lines := strings.Split(entry.Text, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.HasPrefix(lower, "command:") || strings.HasPrefix(lower, "command_prefix:") || strings.HasPrefix(lower, "cmd:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				cmdPrefix = strings.TrimSpace(parts[1])
			}
		} else if strings.HasPrefix(lower, "pattern:") || strings.HasPrefix(lower, "match:") || strings.HasPrefix(lower, "symptom:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				pattern = strings.TrimSpace(parts[1])
			}
		}
	}
	if pattern == "" && len(lines) > 0 {
		pattern = strings.TrimSpace(lines[0])
	}
	if pattern == "" {
		pattern = entry.ID
	}
	return cmdPrefix, pattern
}

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
