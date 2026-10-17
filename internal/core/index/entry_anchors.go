/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/index/entry_anchors.go
 * Description: Provides anchor association and prefix/exact anchor lookup
 *               operations for indexed dossier entries.
 *
 * Exported Types / Functions:
 * - Index:
 *   - UpsertEntryAnchors():  Replaces all anchor strings associated with an entry ID.
 *   - LookupByAnchor():      Finds distinct entry IDs matching an anchor prefix.
 *   - LookupByAnchorExact(): Finds distinct entry IDs matching an anchor string exactly.
 */

package index

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// UpsertEntryAnchors replaces all anchors for an entry.
func (idx *Index) UpsertEntryAnchors(ctx context.Context, entryID string, anchors []string) error {
	return idx.WriteTx(ctx, func(tx *sql.Tx) error {
		// --- Step 1: Remove existing anchors for the entry ---
		if _, err := tx.ExecContext(ctx, "DELETE FROM entry_anchors WHERE entry_id = ?", entryID); err != nil {
			return fmt.Errorf("deleting old anchors for %s: %w", entryID, err)
		}

		if len(anchors) == 0 {
			return nil
		}

		// --- Step 2: Prepare insertion statement and deduplicate non-empty anchors ---
		stmt, err := tx.PrepareContext(ctx, "INSERT INTO entry_anchors (entry_id, anchor) VALUES (?, ?)")
		if err != nil {
			return fmt.Errorf("preparing anchor insert: %w", err)
		}
		defer stmt.Close()

		seen := make(map[string]bool)
		for _, a := range anchors {
			trimmed := strings.TrimSpace(a)
			if trimmed == "" || seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			if _, err := stmt.ExecContext(ctx, entryID, trimmed); err != nil {
				return fmt.Errorf("inserting anchor for %s: %w", entryID, err)
			}
		}
		return nil
	})
}

// LookupByAnchor returns entry IDs matching the anchor (prefix match using LIKE).
func (idx *Index) LookupByAnchor(ctx context.Context, anchor string) ([]string, error) {
	// --- Step 1: Normalize the anchor prefix and construct the LIKE pattern ---
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return []string{}, nil
	}

	pattern := anchor
	if !strings.HasSuffix(pattern, "%") {
		pattern += "%"
	}

	// --- Step 2: Query distinct matching entry IDs ---
	query := `SELECT DISTINCT entry_id FROM entry_anchors WHERE anchor LIKE ? ORDER BY entry_id`
	rows, err := idx.db.QueryContext(ctx, query, pattern)
	if err != nil {
		return nil, fmt.Errorf("lookup by anchor %q: %w", anchor, err)
	}
	defer rows.Close()

	// --- Step 3: Collect and return matched entry IDs ---
	results := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning entry id: %w", err)
		}
		results = append(results, id)
	}
	return results, rows.Err()
}

// LookupByAnchorExact returns entry IDs matching the anchor exactly.
func (idx *Index) LookupByAnchorExact(ctx context.Context, anchor string) ([]string, error) {
	// --- Step 1: Validate and trim the target anchor ---
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return []string{}, nil
	}

	// --- Step 2: Query distinct entry IDs with an exact anchor match ---
	query := `SELECT DISTINCT entry_id FROM entry_anchors WHERE anchor = ? ORDER BY entry_id`
	rows, err := idx.db.QueryContext(ctx, query, anchor)
	if err != nil {
		return nil, fmt.Errorf("lookup by exact anchor %q: %w", anchor, err)
	}
	defer rows.Close()

	// --- Step 3: Scan and return matched entry IDs ---
	results := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scanning entry id: %w", err)
		}
		results = append(results, id)
	}
	return results, rows.Err()
}

// getEntryAnchors fetches all anchors for entryID in insertion order.
func (idx *Index) getEntryAnchors(ctx context.Context, entryID string) ([]string, error) {
	// --- Step 1: Query anchor rows ordered by SQLite rowid ---
	query := `SELECT anchor FROM entry_anchors WHERE entry_id = ? ORDER BY rowid`
	rows, err := idx.db.QueryContext(ctx, query, entryID)
	if err != nil {
		return nil, fmt.Errorf("getting anchors for %s: %w", entryID, err)
	}
	defer rows.Close()

	// --- Step 2: Scan anchor strings into a slice ---
	var anchors []string
	for rows.Next() {
		var a string
		if err := rows.Scan(&a); err != nil {
			return nil, fmt.Errorf("scanning anchor: %w", err)
		}
		anchors = append(anchors, a)
	}
	return anchors, rows.Err()
}
