package index

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

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
