/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/index/entries.go
 * Description: Provides CRUD, FTS5 BM25 full-text search, error-pattern lookup,
 *               scope alias resolution, and historical entry queries on Index.
 *
 * Exported Types / Functions:
 * - ErrNotFound:                Sentinel error returned when an entry or record is missing.
 * - ScoredEntry:                Pairs a dossiers.Entry with its positive BM25 relevance score.
 * - ErrorPatternResult:         Represents a matched failure-signature error pattern row.
 * - Index:
 *   - UpsertEntry():            Inserts or updates an entry and synchronizes its anchors and error patterns.
 *   - InsertEntry():            Inserts or replaces an entry and resets its child rows.
 *   - GetEntry():               Retrieves a single entry and its anchors by ID.
 *   - DeleteEntry():            Deletes an entry and its associated anchors and error patterns.
 *   - ListEntriesByScope():     Returns all entries belonging to a dossier scope.
 *   - ListEntriesBySource():    Returns all entries matching a provenance source.
 *   - ListProceduralEntries():  Returns all procedural/recipe entries ordered by ID.
 *   - ListAllEntries():         Returns all active entries ordered by ID.
 *   - SearchEntriesWithScore(): Performs an FTS5 BM25 search returning scored entries.
 *   - SearchEntries():          Performs an FTS5 search returning matching entries.
 *   - InsertErrorPattern():     Registers an error pattern for a failure-signature entry.
 *   - LookupErrorPatterns():    Returns matching error patterns for a command prefix.
 *   - UpsertAlias():            Records or updates a dossier scope alias mapping.
 *   - ResolveAlias():           Follows the scope alias chain to its target scope.
 *   - InsertHistory():          Appends a retired or superseded entry record to entry_history.
 *   - GetHistory():             Retrieves historical records for an entry ID.
 *   - QueryHistoryByScope():    Returns historical records for a dossier scope.
 */

package index

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
)

// ErrNotFound is returned when an entry or record is not found in the index.
var ErrNotFound = errors.New("not found")

// parseTime parses an RFC3339Nano or RFC3339 timestamp string, returning the zero Time on failure.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Time{}
}

// formatTime formats a non-zero time as UTC RFC3339Nano, or returns an empty string for the zero Time.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// boolToInt converts a boolean into a SQLite integer flag (1 or 0).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// UpsertEntry inserts or updates an entry in the entries table.
func (idx *Index) UpsertEntry(ctx context.Context, entry dossiers.Entry) error {
	// --- Step 1: Normalize timestamps before opening the write transaction ---
	createdAt := formatTime(entry.CreatedAt)
	updatedAt := formatTime(entry.UpdatedAt)
	if updatedAt == "" {
		updatedAt = formatTime(time.Now().UTC())
	}

	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		// --- Step 2: Upsert the primary row in the entries table ---
		query := `
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
		_, err := tx.ExecContext(ctx, query,
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
			return fmt.Errorf("upserting entry %s: %w", entry.ID, err)
		}

		// --- Step 3: Replace entry anchors if provided ---
		if len(entry.Anchors) > 0 {
			if _, err := tx.ExecContext(ctx, "DELETE FROM entry_anchors WHERE entry_id = ?", entry.ID); err != nil {
				return err
			}
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO entry_anchors (entry_id, anchor) VALUES (?, ?)")
			if err != nil {
				return err
			}
			defer stmt.Close()
			seen := make(map[string]bool, len(entry.Anchors))
			for _, a := range entry.Anchors {
				trimmed := strings.TrimSpace(a)
				if trimmed == "" || seen[trimmed] {
					continue
				}
				seen[trimmed] = true
				if _, err := stmt.ExecContext(ctx, entry.ID, trimmed); err != nil {
					return err
				}
			}
		}

		// --- Step 4: Synchronize error_patterns with the entry type and scope ---
		if entry.Type != dossiers.EntryTypeFailureSignature {
			if _, err := tx.ExecContext(ctx, "DELETE FROM error_patterns WHERE entry_id = ?", entry.ID); err != nil {
				return err
			}
		} else if entry.DossierScope != "" {
			if _, err := tx.ExecContext(ctx, "UPDATE error_patterns SET dossier_scope = ? WHERE entry_id = ?", entry.DossierScope, entry.ID); err != nil {
				return err
			}
		}

		return nil
	})
}

// InsertEntry inserts or replaces an entry and all of its child rows (entry_anchors, error_patterns)
// while keeping entries_fts synchronized without orphan FTS rows.
func (idx *Index) InsertEntry(ctx context.Context, entry dossiers.Entry) error {
	// --- Step 1: Format entry timestamps ---
	createdAt := formatTime(entry.CreatedAt)
	updatedAt := formatTime(entry.UpdatedAt)
	if updatedAt == "" {
		updatedAt = formatTime(time.Now().UTC())
	}

	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		// --- Step 2: Clear existing child anchor and error-pattern rows ---
		if _, err := tx.ExecContext(ctx, "DELETE FROM entry_anchors WHERE entry_id = ?", entry.ID); err != nil {
			return fmt.Errorf("clearing anchors for %s: %w", entry.ID, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM error_patterns WHERE entry_id = ?", entry.ID); err != nil {
			return fmt.Errorf("clearing error patterns for %s: %w", entry.ID, err)
		}

		// --- Step 3: Upsert the entry row so FTS triggers maintain entries_fts ---
		query := `
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
		_, err := tx.ExecContext(ctx, query,
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

		// --- Step 4: Insert deduplicated entry anchors ---
		if len(entry.Anchors) > 0 {
			stmt, err := tx.PrepareContext(ctx, "INSERT INTO entry_anchors (entry_id, anchor) VALUES (?, ?)")
			if err != nil {
				return err
			}
			defer stmt.Close()
			seen := make(map[string]bool, len(entry.Anchors))
			for _, a := range entry.Anchors {
				trimmed := strings.TrimSpace(a)
				if trimmed == "" || seen[trimmed] {
					continue
				}
				seen[trimmed] = true
				if _, err := stmt.ExecContext(ctx, entry.ID, trimmed); err != nil {
					return err
				}
			}
		}

		// --- Step 5: Extract and insert error patterns for failure-signature entries ---
		if entry.Type == dossiers.EntryTypeFailureSignature {
			cmdPrefix, pattern := parseErrorPatternFromEntry(entry)
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO error_patterns (entry_id, command_prefix, pattern, dossier_scope) VALUES (?, ?, ?, ?)",
				entry.ID, cmdPrefix, pattern, entry.DossierScope,
			); err != nil {
				return fmt.Errorf("inserting error pattern for %s: %w", entry.ID, err)
			}
		}

		return nil
	})
}

// GetEntry retrieves an entry by its ID. It returns ErrNotFound (wrapped) if no entry exists with id.
func (idx *Index) GetEntry(ctx context.Context, id string) (*dossiers.Entry, error) {
	// --- Step 1: Query and scan the entry row by primary key ---
	query := `
	SELECT id, type, COALESCE(kind, ''), dossier_scope, text, source,
	       COALESCE(source_doc_path, ''), COALESCE(source_doc_blob_sha, ''),
	       COALESCE(success_count, 0), COALESCE(failure_count, 0),
	       COALESCE(last_succeeded_at, ''), COALESCE(stale_suspect, 0),
	       COALESCE(render_as_skill, ''),
	       verified, closes_when, created_at, updated_at
	FROM entries
	WHERE id = ?
	`
	var e dossiers.Entry
	var typeStr, kindStr string
	var source, verified, closesWhen, createdAtStr, updatedAtStr sql.NullString
	var staleSuspectInt int

	err := idx.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID,
		&typeStr,
		&kindStr,
		&e.DossierScope,
		&e.Text,
		&source,
		&e.SourceDocPath,
		&e.SourceDocBlobSHA,
		&e.SuccessCount,
		&e.FailureCount,
		&e.LastSucceededAt,
		&staleSuspectInt,
		&e.RenderAsSkill,
		&verified,
		&closesWhen,
		&createdAtStr,
		&updatedAtStr,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: entry %s", ErrNotFound, id)
		}
		return nil, fmt.Errorf("getting entry %s: %w", id, err)
	}

	// --- Step 2: Populate typed fields and derive fallback cognitive kind ---
	e.Type = dossiers.EntryType(typeStr)
	e.Kind = dossiers.CognitiveKind(kindStr)
	if !e.Kind.Valid() {
		e.Kind = dossiers.DeriveCognitiveKind(e.Type)
	}
	e.Source = source.String
	e.StaleSuspect = staleSuspectInt != 0
	e.Verified = verified.String
	e.ClosesWhen = closesWhen.String
	e.CreatedAt = parseTime(createdAtStr.String)
	e.UpdatedAt = parseTime(updatedAtStr.String)

	// --- Step 3: Load associated anchors for the entry ---
	anchors, err := idx.getEntryAnchors(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	e.Anchors = anchors

	return &e, nil
}

// DeleteEntry deletes an entry and its associated anchors and error patterns.
func (idx *Index) DeleteEntry(ctx context.Context, id string) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM entry_anchors WHERE entry_id = ?", id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM error_patterns WHERE entry_id = ?", id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM entries WHERE id = ?", id); err != nil {
			return err
		}
		return nil
	})
}

// scanEntryRows scans a sql.Rows cursor over the standard 17 entry columns into a slice of dossiers.Entry.
func scanEntryRows(rows *sql.Rows) ([]dossiers.Entry, error) {
	// --- Step 1: Iterate over SQL rows and scan each column ---
	var results []dossiers.Entry
	for rows.Next() {
		var e dossiers.Entry
		var typeStr, kindStr string
		var source, verified, closesWhen, createdAtStr, updatedAtStr sql.NullString
		var staleSuspectInt int
		if err := rows.Scan(
			&e.ID,
			&typeStr,
			&kindStr,
			&e.DossierScope,
			&e.Text,
			&source,
			&e.SourceDocPath,
			&e.SourceDocBlobSHA,
			&e.SuccessCount,
			&e.FailureCount,
			&e.LastSucceededAt,
			&staleSuspectInt,
			&e.RenderAsSkill,
			&verified,
			&closesWhen,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scanning entry row: %w", err)
		}

		// --- Step 2: Normalize domain enums, nullable strings, and timestamps ---
		e.Type = dossiers.EntryType(typeStr)
		e.Kind = dossiers.CognitiveKind(kindStr)
		if !e.Kind.Valid() {
			e.Kind = dossiers.DeriveCognitiveKind(e.Type)
		}
		e.Source = source.String
		e.StaleSuspect = staleSuspectInt != 0
		e.Verified = verified.String
		e.ClosesWhen = closesWhen.String
		e.CreatedAt = parseTime(createdAtStr.String)
		e.UpdatedAt = parseTime(updatedAtStr.String)
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// ListEntriesByScope returns all entries belonging to the specified dossier scope.
func (idx *Index) ListEntriesByScope(ctx context.Context, scope string) ([]dossiers.Entry, error) {
	// --- Step 1: Query entries filtered by dossier_scope ---
	query := `
	SELECT id, type, COALESCE(kind, ''), dossier_scope, text, source,
	       COALESCE(source_doc_path, ''), COALESCE(source_doc_blob_sha, ''),
	       COALESCE(success_count, 0), COALESCE(failure_count, 0),
	       COALESCE(last_succeeded_at, ''), COALESCE(stale_suspect, 0),
	       COALESCE(render_as_skill, ''),
	       verified, closes_when, created_at, updated_at
	FROM entries
	WHERE dossier_scope = ?
	ORDER BY id
	`
	rows, err := idx.db.QueryContext(ctx, query, scope)
	if err != nil {
		return nil, fmt.Errorf("listing entries for scope %s: %w", scope, err)
	}
	defer rows.Close()

	// --- Step 2: Decode entry rows and hydrate their anchors ---
	results, err := scanEntryRows(rows)
	if err != nil {
		return nil, err
	}

	for i := range results {
		anchors, err := idx.getEntryAnchors(ctx, results[i].ID)
		if err != nil {
			return nil, err
		}
		results[i].Anchors = anchors
	}

	if results == nil {
		results = []dossiers.Entry{}
	}
	return results, nil
}

// ListEntriesBySource returns all entries matching the specified provenance source.
func (idx *Index) ListEntriesBySource(ctx context.Context, source string) ([]dossiers.Entry, error) {
	// --- Step 1: Query entries filtered by provenance source ---
	query := `
	SELECT id, type, COALESCE(kind, ''), dossier_scope, text, source,
	       COALESCE(source_doc_path, ''), COALESCE(source_doc_blob_sha, ''),
	       COALESCE(success_count, 0), COALESCE(failure_count, 0),
	       COALESCE(last_succeeded_at, ''), COALESCE(stale_suspect, 0),
	       COALESCE(render_as_skill, ''),
	       verified, closes_when, created_at, updated_at
	FROM entries
	WHERE source = ?
	ORDER BY id
	`
	rows, err := idx.db.QueryContext(ctx, query, source)
	if err != nil {
		return nil, fmt.Errorf("listing entries for source %s: %w", source, err)
	}
	defer rows.Close()

	// --- Step 2: Decode entry rows and hydrate their anchors ---
	results, err := scanEntryRows(rows)
	if err != nil {
		return nil, err
	}

	for i := range results {
		anchors, err := idx.getEntryAnchors(ctx, results[i].ID)
		if err != nil {
			return nil, err
		}
		results[i].Anchors = anchors
	}

	if results == nil {
		results = []dossiers.Entry{}
	}
	return results, nil
}

// ListProceduralEntries returns all active entries where type IN ('recipe', 'procedure') or kind = 'procedural', ordered by id.
func (idx *Index) ListProceduralEntries(ctx context.Context) ([]dossiers.Entry, error) {
	// --- Step 1: Query procedural and recipe entries ordered by ID ---
	query := `
	SELECT id, type, COALESCE(kind, ''), dossier_scope, text, source,
	       COALESCE(source_doc_path, ''), COALESCE(source_doc_blob_sha, ''),
	       COALESCE(success_count, 0), COALESCE(failure_count, 0),
	       COALESCE(last_succeeded_at, ''), COALESCE(stale_suspect, 0),
	       COALESCE(render_as_skill, ''),
	       verified, closes_when, created_at, updated_at
	FROM entries
	WHERE type IN ('recipe', 'procedure') OR kind = 'procedural'
	ORDER BY id
	`
	rows, err := idx.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing procedural entries: %w", err)
	}
	defer rows.Close()

	// --- Step 2: Decode entry rows and hydrate their anchors ---
	results, err := scanEntryRows(rows)
	if err != nil {
		return nil, err
	}

	for i := range results {
		anchors, err := idx.getEntryAnchors(ctx, results[i].ID)
		if err != nil {
			return nil, err
		}
		results[i].Anchors = anchors
	}

	if results == nil {
		results = []dossiers.Entry{}
	}
	return results, nil
}

// ListAllEntries returns all active entries ordered by id.
func (idx *Index) ListAllEntries(ctx context.Context) ([]dossiers.Entry, error) {
	// --- Step 1: Query all rows from the entries table ordered by ID ---
	query := `
	SELECT id, type, COALESCE(kind, ''), dossier_scope, text, source,
	       COALESCE(source_doc_path, ''), COALESCE(source_doc_blob_sha, ''),
	       COALESCE(success_count, 0), COALESCE(failure_count, 0),
	       COALESCE(last_succeeded_at, ''), COALESCE(stale_suspect, 0),
	       COALESCE(render_as_skill, ''),
	       verified, closes_when, created_at, updated_at
	FROM entries
	ORDER BY id
	`
	rows, err := idx.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing all entries: %w", err)
	}
	defer rows.Close()

	// --- Step 2: Decode entry rows and hydrate their anchors ---
	results, err := scanEntryRows(rows)
	if err != nil {
		return nil, err
	}

	for i := range results {
		anchors, err := idx.getEntryAnchors(ctx, results[i].ID)
		if err != nil {
			return nil, err
		}
		results[i].Anchors = anchors
	}

	if results == nil {
		results = []dossiers.Entry{}
	}
	return results, nil
}

// ScoredEntry pairs a matched dossier entry with its positive BM25 relevance score.
type ScoredEntry struct {
	Entry dossiers.Entry // Hydrated active dossier entry returned by the FTS5 join.
	Score float64        // Non-negative relevance score (-bm25) where higher values indicate stronger matches.
}

// SearchEntriesWithScore performs an FTS5 search over the entries_fts table,
// returning matching entries with their BM25 relevance score.
// In SQLite FTS5, bm25() returns negative numbers where more negative is more relevant.
// This function negates the value (-bm25) so higher score represents higher relevance.
func (idx *Index) SearchEntriesWithScore(ctx context.Context, query string) ([]ScoredEntry, error) {
	// --- Step 1: Validate and execute the FTS5 match query ---
	matchQuery := strings.TrimSpace(query)
	if matchQuery == "" {
		return []ScoredEntry{}, nil
	}

	sqlQuery := `
	SELECT e.id, e.type, COALESCE(e.kind, ''), e.dossier_scope, e.text, e.source,
	       COALESCE(e.source_doc_path, ''), COALESCE(e.source_doc_blob_sha, ''),
	       COALESCE(e.success_count, 0), COALESCE(e.failure_count, 0),
	       COALESCE(e.last_succeeded_at, ''), COALESCE(e.stale_suspect, 0),
	       COALESCE(e.render_as_skill, ''),
	       e.verified, e.closes_when, e.created_at, e.updated_at, bm25(entries_fts)
	FROM entries_fts
	JOIN entries e ON entries_fts.rowid = e.rowid
	WHERE entries_fts MATCH ?
	ORDER BY bm25(entries_fts)
	`
	rows, err := idx.db.QueryContext(ctx, sqlQuery, matchQuery)
	if err != nil {
		return nil, fmt.Errorf("searching entries with %q: %w", matchQuery, err)
	}
	defer rows.Close()

	// --- Step 2: Scan matched entries and convert negative SQLite BM25 scores into positive relevance ---
	var results []ScoredEntry
	for rows.Next() {
		var e dossiers.Entry
		var typeStr, kindStr string
		var source, verified, closesWhen, createdAtStr, updatedAtStr sql.NullString
		var staleSuspectInt int
		var rawBM25 float64
		if err := rows.Scan(
			&e.ID,
			&typeStr,
			&kindStr,
			&e.DossierScope,
			&e.Text,
			&source,
			&e.SourceDocPath,
			&e.SourceDocBlobSHA,
			&e.SuccessCount,
			&e.FailureCount,
			&e.LastSucceededAt,
			&staleSuspectInt,
			&e.RenderAsSkill,
			&verified,
			&closesWhen,
			&createdAtStr,
			&updatedAtStr,
			&rawBM25,
		); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		e.Type = dossiers.EntryType(typeStr)
		e.Kind = dossiers.CognitiveKind(kindStr)
		if !e.Kind.Valid() {
			e.Kind = dossiers.DeriveCognitiveKind(e.Type)
		}
		e.Source = source.String
		e.StaleSuspect = staleSuspectInt != 0
		e.Verified = verified.String
		e.ClosesWhen = closesWhen.String
		e.CreatedAt = parseTime(createdAtStr.String)
		e.UpdatedAt = parseTime(updatedAtStr.String)

		relevance := -rawBM25
		if relevance < 0 {
			relevance = 0
		}
		results = append(results, ScoredEntry{
			Entry: e,
			Score: relevance,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// --- Step 3: Populate anchors for each scored entry ---
	for i := range results {
		anchors, err := idx.getEntryAnchors(ctx, results[i].Entry.ID)
		if err != nil {
			return nil, err
		}
		results[i].Entry.Anchors = anchors
	}

	if results == nil {
		results = []ScoredEntry{}
	}
	return results, nil
}

// SearchEntries performs an FTS5 search over the entries_fts table.
func (idx *Index) SearchEntries(ctx context.Context, query string) ([]dossiers.Entry, error) {
	scored, err := idx.SearchEntriesWithScore(ctx, query)
	if err != nil {
		return nil, err
	}
	results := make([]dossiers.Entry, len(scored))
	for i, s := range scored {
		results[i] = s.Entry
	}
	return results, nil
}

// ErrorPatternResult represents a matched error pattern.
type ErrorPatternResult struct {
	EntryID       string `json:"entry_id"`       // ID of the associated failure-signature entry.
	CommandPrefix string `json:"command_prefix"` // Command prefix constraint (empty matches any command).
	Pattern       string `json:"pattern"`        // Error substring or pattern to match against command output.
	DossierScope  string `json:"dossier_scope"`  // Dossier scope owning the failure-signature entry.
}

// InsertErrorPattern registers an error pattern for a failure-signature entry.
func (idx *Index) InsertErrorPattern(ctx context.Context, entryID, commandPrefix, pattern, scope string) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		delQuery := `DELETE FROM error_patterns WHERE entry_id = ? AND COALESCE(command_prefix, '') = ? AND pattern = ?`
		if _, err := tx.ExecContext(ctx, delQuery, entryID, commandPrefix, pattern); err != nil {
			return fmt.Errorf("clearing existing error pattern for %s: %w", entryID, err)
		}
		insQuery := `INSERT INTO error_patterns (entry_id, command_prefix, pattern, dossier_scope) VALUES (?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, insQuery, entryID, commandPrefix, pattern, scope); err != nil {
			return fmt.Errorf("inserting error pattern for %s: %w", entryID, err)
		}
		return nil
	})
}

// LookupErrorPatterns returns matching failure-signature entries for a command.
func (idx *Index) LookupErrorPatterns(ctx context.Context, commandPrefix string) ([]ErrorPatternResult, error) {
	// --- Step 1: Build the SQL query matching wildcard or prefix-overlapping command patterns ---
	commandPrefix = strings.TrimSpace(commandPrefix)
	var query string
	var args []any

	if commandPrefix == "" {
		query = `SELECT entry_id, COALESCE(command_prefix, ''), pattern, dossier_scope FROM error_patterns ORDER BY entry_id`
	} else {
		query = `
		SELECT entry_id, COALESCE(command_prefix, ''), pattern, dossier_scope
		FROM error_patterns
		WHERE command_prefix = ?
		   OR command_prefix = ''
		   OR command_prefix IS NULL
		   OR ? LIKE (command_prefix || '%')
		   OR command_prefix LIKE (? || '%')
		ORDER BY length(COALESCE(command_prefix, '')) DESC, entry_id
		`
		args = []any{commandPrefix, commandPrefix, commandPrefix}
	}

	// --- Step 2: Execute the query and scan matching ErrorPatternResult rows ---
	rows, err := idx.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("looking up error patterns for %q: %w", commandPrefix, err)
	}
	defer rows.Close()

	results := make([]ErrorPatternResult, 0)
	for rows.Next() {
		var r ErrorPatternResult
		if err := rows.Scan(&r.EntryID, &r.CommandPrefix, &r.Pattern, &r.DossierScope); err != nil {
			return nil, fmt.Errorf("scanning error pattern result: %w", err)
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// UpsertAlias records or updates a scope alias.
func (idx *Index) UpsertAlias(ctx context.Context, oldScope, newScope string) error {
	oldScope = strings.TrimSpace(oldScope)
	newScope = strings.TrimSpace(newScope)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM aliases WHERE old_scope = ?", oldScope); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO aliases (old_scope, new_scope, created_at) VALUES (?, ?, ?)",
			oldScope, newScope, now)
		return err
	})
}

// ResolveAlias follows the alias chain to return the final target scope.
func (idx *Index) ResolveAlias(ctx context.Context, scope string) (string, error) {
	// --- Step 1: Initialize alias traversal state with cycle protection ---
	current := strings.TrimSpace(scope)
	if current == "" {
		return "", nil
	}

	maxDepth := 10
	visited := make(map[string]bool)

	// --- Step 2: Follow alias hops up to maxDepth or until no further redirect exists ---
	for depth := 0; depth < maxDepth; depth++ {
		var next string
		err := idx.db.QueryRowContext(ctx,
			"SELECT new_scope FROM aliases WHERE old_scope = ? ORDER BY created_at DESC LIMIT 1",
			current).Scan(&next)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return "", fmt.Errorf("resolving alias %s: %w", current, err)
		}
		next = strings.TrimSpace(next)
		if next == "" || next == current || visited[next] {
			break
		}
		visited[current] = true
		current = next
	}
	return current, nil
}

// InsertHistory inserts a retired or superseded entry record into history.
func (idx *Index) InsertHistory(ctx context.Context, entry dossiers.Entry, reason string) error {
	// --- Step 1: Derive history record metadata and retirement timestamp ---
	histReason := reason
	if histReason == "" {
		histReason = entry.RetiredReason
	}
	histID := fmt.Sprintf("%s-hist-%d", entry.ID, time.Now().UnixNano())
	retiredAt := formatTime(entry.UpdatedAt)
	if retiredAt == "" {
		retiredAt = formatTime(time.Now().UTC())
	}

	// --- Step 2: Insert the historical entry snapshot into entry_history ---
	query := `
	INSERT INTO entry_history (id, entry_id, type, dossier_scope, text, reason, superseded_by, retired_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := idx.db.ExecContext(ctx, query,
		histID,
		entry.ID,
		string(entry.Type),
		entry.DossierScope,
		entry.Text,
		histReason,
		entry.SupersededBy,
		retiredAt,
	)
	if err != nil {
		return fmt.Errorf("inserting entry history for %s: %w", entry.ID, err)
	}
	return nil
}

// GetHistory retrieves historical entries for a given entry ID.
func (idx *Index) GetHistory(ctx context.Context, entryID string) ([]dossiers.Entry, error) {
	// --- Step 1: Query entry_history rows matching entry_id or history id ---
	query := `
	SELECT id, entry_id, type, dossier_scope, text, reason, superseded_by, retired_at
	FROM entry_history
	WHERE entry_id = ? OR id = ?
	ORDER BY retired_at DESC
	`
	rows, err := idx.db.QueryContext(ctx, query, entryID, entryID)
	if err != nil {
		return nil, fmt.Errorf("getting history for %s: %w", entryID, err)
	}
	defer rows.Close()

	// --- Step 2: Scan rows into retired dossiers.Entry values ---
	results := make([]dossiers.Entry, 0)
	for rows.Next() {
		var histID, eID, typeStr, scope, text, reason, supersededBy, retiredAt sql.NullString
		if err := rows.Scan(&histID, &eID, &typeStr, &scope, &text, &reason, &supersededBy, &retiredAt); err != nil {
			return nil, fmt.Errorf("scanning entry history: %w", err)
		}
		e := dossiers.Entry{
			ID:            eID.String,
			Type:          dossiers.EntryType(typeStr.String),
			DossierScope:  scope.String,
			Text:          text.String,
			Retired:       true,
			RetiredReason: reason.String,
			SupersededBy:  supersededBy.String,
			UpdatedAt:     parseTime(retiredAt.String),
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// QueryHistoryByScope returns retired/superseded entries from entry_history
// for the given scope. This uses a plain read — no write transaction.
func (idx *Index) QueryHistoryByScope(ctx context.Context, scope string) ([]dossiers.Entry, error) {
	// --- Step 1: Query entry_history filtered by dossier_scope ---
	rows, err := idx.db.QueryContext(ctx,
		`SELECT entry_id, type, dossier_scope, text, reason, superseded_by, retired_at
		 FROM entry_history
		 WHERE dossier_scope = ?
		 ORDER BY retired_at DESC`, scope)
	if err != nil {
		return nil, fmt.Errorf("querying entry_history for scope %s: %w", scope, err)
	}
	defer rows.Close()

	// --- Step 2: Decode historical rows into retired dossiers.Entry records ---
	var results []dossiers.Entry
	for rows.Next() {
		var eID, typeStr, dScope, text, reason, supersededBy, retiredAt sql.NullString
		if err := rows.Scan(&eID, &typeStr, &dScope, &text, &reason, &supersededBy, &retiredAt); err != nil {
			return nil, fmt.Errorf("scanning entry_history row: %w", err)
		}
		results = append(results, dossiers.Entry{
			ID:            eID.String,
			Type:          dossiers.EntryType(typeStr.String),
			DossierScope:  dScope.String,
			Text:          text.String,
			Retired:       true,
			RetiredReason: reason.String,
			SupersededBy:  supersededBy.String,
			UpdatedAt:     parseTime(retiredAt.String),
		})
	}
	return results, rows.Err()
}
