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

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// UpsertEntry inserts or updates an entry in the entries table.
func (idx *Index) UpsertEntry(ctx context.Context, entry dossiers.Entry) error {
	createdAt := formatTime(entry.CreatedAt)
	updatedAt := formatTime(entry.UpdatedAt)
	if updatedAt == "" {
		updatedAt = formatTime(time.Now().UTC())
	}

	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		query := `
		INSERT INTO entries (id, type, dossier_scope, text, source, verified, closes_when, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			dossier_scope = excluded.dossier_scope,
			text = excluded.text,
			source = excluded.source,
			verified = excluded.verified,
			closes_when = excluded.closes_when,
			created_at = CASE WHEN excluded.created_at != '' THEN excluded.created_at ELSE entries.created_at END,
			updated_at = excluded.updated_at
		`
		_, err := tx.ExecContext(ctx, query,
			entry.ID,
			string(entry.Type),
			entry.DossierScope,
			entry.Text,
			entry.Source,
			entry.Verified,
			entry.ClosesWhen,
			createdAt,
			updatedAt,
		)
		if err != nil {
			return fmt.Errorf("upserting entry %s: %w", entry.ID, err)
		}

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
	createdAt := formatTime(entry.CreatedAt)
	updatedAt := formatTime(entry.UpdatedAt)
	if updatedAt == "" {
		updatedAt = formatTime(time.Now().UTC())
	}

	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM entry_anchors WHERE entry_id = ?", entry.ID); err != nil {
			return fmt.Errorf("clearing anchors for %s: %w", entry.ID, err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM error_patterns WHERE entry_id = ?", entry.ID); err != nil {
			return fmt.Errorf("clearing error patterns for %s: %w", entry.ID, err)
		}

		query := `
		INSERT INTO entries (id, type, dossier_scope, text, source, verified, closes_when, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			dossier_scope = excluded.dossier_scope,
			text = excluded.text,
			source = excluded.source,
			verified = excluded.verified,
			closes_when = excluded.closes_when,
			created_at = CASE WHEN excluded.created_at != '' THEN excluded.created_at ELSE entries.created_at END,
			updated_at = excluded.updated_at
		`
		_, err := tx.ExecContext(ctx, query,
			entry.ID,
			string(entry.Type),
			entry.DossierScope,
			entry.Text,
			entry.Source,
			entry.Verified,
			entry.ClosesWhen,
			createdAt,
			updatedAt,
		)
		if err != nil {
			return fmt.Errorf("inserting entry %s: %w", entry.ID, err)
		}

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

// GetEntry retrieves an entry by its ID.
func (idx *Index) GetEntry(ctx context.Context, id string) (*dossiers.Entry, error) {
	query := `
	SELECT id, type, dossier_scope, text, source, verified, closes_when, created_at, updated_at
	FROM entries
	WHERE id = ?
	`
	var e dossiers.Entry
	var typeStr string
	var source, verified, closesWhen, createdAtStr, updatedAtStr sql.NullString

	err := idx.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID,
		&typeStr,
		&e.DossierScope,
		&e.Text,
		&source,
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

	e.Type = dossiers.EntryType(typeStr)
	e.Source = source.String
	e.Verified = verified.String
	e.ClosesWhen = closesWhen.String
	e.CreatedAt = parseTime(createdAtStr.String)
	e.UpdatedAt = parseTime(updatedAtStr.String)

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

// ListEntriesByScope returns all entries belonging to the specified dossier scope.
func (idx *Index) ListEntriesByScope(ctx context.Context, scope string) ([]dossiers.Entry, error) {
	query := `
	SELECT id, type, dossier_scope, text, source, verified, closes_when, created_at, updated_at
	FROM entries
	WHERE dossier_scope = ?
	ORDER BY id
	`
	rows, err := idx.db.QueryContext(ctx, query, scope)
	if err != nil {
		return nil, fmt.Errorf("listing entries for scope %s: %w", scope, err)
	}
	defer rows.Close()

	var results []dossiers.Entry
	for rows.Next() {
		var e dossiers.Entry
		var typeStr string
		var source, verified, closesWhen, createdAtStr, updatedAtStr sql.NullString
		if err := rows.Scan(
			&e.ID,
			&typeStr,
			&e.DossierScope,
			&e.Text,
			&source,
			&verified,
			&closesWhen,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("scanning entry row: %w", err)
		}
		e.Type = dossiers.EntryType(typeStr)
		e.Source = source.String
		e.Verified = verified.String
		e.ClosesWhen = closesWhen.String
		e.CreatedAt = parseTime(createdAtStr.String)
		e.UpdatedAt = parseTime(updatedAtStr.String)
		results = append(results, e)
	}
	if err := rows.Err(); err != nil {
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

type ScoredEntry struct {
	Entry dossiers.Entry
	Score float64
}

// SearchEntriesWithScore performs an FTS5 search over the entries_fts table,
// returning matching entries with their BM25 relevance score.
// In SQLite FTS5, bm25() returns negative numbers where more negative is more relevant.
// This function negates the value (-bm25) so higher score represents higher relevance.
func (idx *Index) SearchEntriesWithScore(ctx context.Context, query string) ([]ScoredEntry, error) {
	matchQuery := strings.TrimSpace(query)
	if matchQuery == "" {
		return []ScoredEntry{}, nil
	}

	sqlQuery := `
	SELECT e.id, e.type, e.dossier_scope, e.text, e.source, e.verified, e.closes_when, e.created_at, e.updated_at, bm25(entries_fts)
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

	var results []ScoredEntry
	for rows.Next() {
		var e dossiers.Entry
		var typeStr string
		var source, verified, closesWhen, createdAtStr, updatedAtStr sql.NullString
		var rawBM25 float64
		if err := rows.Scan(
			&e.ID,
			&typeStr,
			&e.DossierScope,
			&e.Text,
			&source,
			&verified,
			&closesWhen,
			&createdAtStr,
			&updatedAtStr,
			&rawBM25,
		); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		e.Type = dossiers.EntryType(typeStr)
		e.Source = source.String
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
	EntryID       string `json:"entry_id"`
	CommandPrefix string `json:"command_prefix"`
	Pattern       string `json:"pattern"`
	DossierScope  string `json:"dossier_scope"`
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
	current := strings.TrimSpace(scope)
	if current == "" {
		return "", nil
	}

	maxDepth := 10
	visited := make(map[string]bool)

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
	histReason := reason
	if histReason == "" {
		histReason = entry.RetiredReason
	}
	histID := fmt.Sprintf("%s-hist-%d", entry.ID, time.Now().UnixNano())
	retiredAt := formatTime(entry.UpdatedAt)
	if retiredAt == "" {
		retiredAt = formatTime(time.Now().UTC())
	}

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
	rows, err := idx.db.QueryContext(ctx,
		`SELECT entry_id, type, dossier_scope, text, reason, superseded_by, retired_at
		 FROM entry_history
		 WHERE dossier_scope = ?
		 ORDER BY retired_at DESC`, scope)
	if err != nil {
		return nil, fmt.Errorf("querying entry_history for scope %s: %w", scope, err)
	}
	defer rows.Close()

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
