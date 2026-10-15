package index

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func (idx *Index) RebuildFromOpLog(ctx context.Context, opsReader *ops.Reader) error {
	if opsReader == nil {
		return fmt.Errorf("ops reader cannot be nil")
	}

	projector := dossiers.NewProjector(opsReader)
	dossierMap, err := projector.Project(ctx)
	if err != nil {
		return fmt.Errorf("projecting dossiers: %w", err)
	}



	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin rebuild tx: %w", err)
	}
	defer tx.Rollback()

	tables := []string{"entry_anchors", "error_patterns", "aliases", "entry_history", "entries"}
	for _, t := range tables {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", t)); err != nil {
			return fmt.Errorf("clearing table %s: %w", t, err)
		}
	}
	_, _ = tx.ExecContext(ctx, "INSERT INTO entries_fts(entries_fts) VALUES('rebuild')")

	insertEntryQuery := `
		INSERT INTO entries (
			id, type, kind, dossier_scope, text, source,
			source_doc_path, source_doc_blob_sha, success_count, failure_count,
			last_succeeded_at, stale_suspect, render_as_skill,
			verified, closes_when, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			kind = excluded.kind,
			dossier_scope = excluded.dossier_scope,
			text = excluded.text,
			source = excluded.source,
			source_doc_path = excluded.source_doc_path,
			source_doc_blob_sha = excluded.source_doc_blob_sha,
			success_count = excluded.success_count,
			failure_count = excluded.failure_count,
			last_succeeded_at = excluded.last_succeeded_at,
			stale_suspect = excluded.stale_suspect,
			render_as_skill = excluded.render_as_skill,
			verified = excluded.verified,
			closes_when = excluded.closes_when,
			created_at = CASE WHEN excluded.created_at != '' THEN excluded.created_at ELSE entries.created_at END,
			updated_at = excluded.updated_at
	`
	insertAnchorQuery := `INSERT INTO entry_anchors (entry_id, anchor) VALUES (?, ?)`
	insertErrPatternQuery := `INSERT INTO error_patterns (entry_id, command_prefix, pattern, dossier_scope) VALUES (?, ?, ?, ?)`
	insertHistoryQuery := `
		INSERT INTO entry_history (id, entry_id, type, dossier_scope, text, reason, superseded_by, retired_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`

	for _, d := range dossierMap {
		for _, entry := range d.Entries {
			createdAt := formatTime(entry.CreatedAt)
			updatedAt := formatTime(entry.UpdatedAt)
			if updatedAt == "" {
				updatedAt = formatTime(time.Now().UTC())
			}

			_, err := tx.ExecContext(ctx, insertEntryQuery,
				entry.ID,
				string(entry.Type),
				string(entry.EffectiveKind()),
				entry.DossierScope,
				entry.Text,
				entry.Source,
				entry.SourceDocPath,
				entry.SourceDocBlobSHA,
				entry.SuccessCount,
				entry.FailureCount,
				entry.LastSucceededAt,
				boolToInt(entry.StaleSuspect),
				entry.RenderAsSkill,
				entry.Verified,
				entry.ClosesWhen,
				createdAt,
				updatedAt,
			)
			if err != nil {
				return fmt.Errorf("inserting entry %s: %w", entry.ID, err)
			}

			if _, err := tx.ExecContext(ctx, "DELETE FROM entry_anchors WHERE entry_id = ?", entry.ID); err != nil {
				return fmt.Errorf("clearing anchors for %s: %w", entry.ID, err)
			}
			if _, err := tx.ExecContext(ctx, "DELETE FROM error_patterns WHERE entry_id = ?", entry.ID); err != nil {
				return fmt.Errorf("clearing error patterns for %s: %w", entry.ID, err)
			}

			if len(entry.Anchors) > 0 {
				for _, a := range entry.Anchors {
					trimmed := strings.TrimSpace(a)
					if trimmed == "" {
						continue
					}
					if _, err := tx.ExecContext(ctx, insertAnchorQuery, entry.ID, trimmed); err != nil {
						return err
					}
				}
			}

			if entry.Type == dossiers.EntryTypeFailureSignature {
				cmdPrefix, pattern := parseErrorPatternFromEntry(entry)
				if _, err := tx.ExecContext(ctx, insertErrPatternQuery, entry.ID, cmdPrefix, pattern, entry.DossierScope); err != nil {
					return fmt.Errorf("inserting error pattern for %s: %w", entry.ID, err)
				}
			}
		}

		for _, hist := range d.History {
			reason := hist.RetiredReason
			if reason == "" {
				reason = "retired"
			}
			histID := fmt.Sprintf("%s-hist-%d", hist.ID, time.Now().UnixNano())
			retiredAt := formatTime(hist.UpdatedAt)
			if retiredAt == "" {
				retiredAt = formatTime(time.Now().UTC())
			}

			_, err := tx.ExecContext(ctx, insertHistoryQuery,
				histID, hist.ID, string(hist.Type), hist.DossierScope,
				hist.Text, reason, hist.SupersededBy, retiredAt)
			if err != nil {
				return fmt.Errorf("inserting history for %s: %w", hist.ID, err)
			}
		}
	}

	aliasMap, err := config.BuildAliasMap(opsReader, ctx)
	if err != nil {
		return fmt.Errorf("building alias map: %w", err)
	}
	if aliasMap != nil {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		for _, oldScope := range aliasMap.AllAliases() {
			if aliasMap.HasAlias(oldScope) {
				resolved := aliasMap.Resolve(oldScope)
				if resolved != "" && resolved != oldScope {
					_, err := tx.ExecContext(ctx, "INSERT INTO aliases (old_scope, new_scope, created_at) VALUES (?, ?, ?)",
						oldScope, resolved, now)
					if err != nil {
						return fmt.Errorf("upserting alias %s -> %s: %w", oldScope, resolved, err)
					}
				}
			}
		}
	}

	return tx.Commit()
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
