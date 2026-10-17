/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/index/index.go
 * Description: Opens and manages the SQLite and FTS5 index database over
 *               episodes, entities, and repository memory entries.
 *
 * Exported Types / Functions:
 * - Index: SQLite and FTS5 index over episodes, entities, and dossier entries.
 *
 *   Lifecycle:
 *   - NewIndex():         Opens or creates a read-write Index backed by SQLite in WAL mode.
 *   - NewReadOnlyIndex(): Opens an existing SQLite Index in read-only mode.
 *   - Close():            Closes the underlying database connection. [io.Closer]
 *
 *   Own Public API:
 *   - InsertEpisode():    Inserts an episode record into the episodes_fts virtual table.
 *   - SearchEpisodes():   Performs a full-text search against the episodes_fts table.
 *
 * - EpisodeRecord:        Row representation for the episodes FTS5 table.
 * - IndexPath():          Returns the canonical SQLite index file path for a repository.
 */

package index

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	_ "modernc.org/sqlite"
)

// IndexPath returns the canonical SQLite index file path for a repository:
// <data-dir>/<repo-id>/index.db.
// If dataDir is empty, config.DataDir() is used.
func IndexPath(dataDir, repoID string) string {
	return config.IndexPath(dataDir, repoID)
}

// Index provides SQLite FTS indexing over episodes and v2 entries.
type Index struct {
	db *sql.DB // Underlying SQLite database connection handle.
}

// NewIndex opens or creates a new Index backed by SQLite at dbPath.
// It configures WAL mode for better concurrency. The caller must call Close when finished.
func NewIndex(dbPath string) (*Index, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=recursive_triggers(1)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening index database: %w", err)
	}

	idx := &Index{db: db}
	if err := idx.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return idx, nil
}

// NewReadOnlyIndex opens an Index backed by SQLite at dbPath in read-only mode.
// The caller must call Close when finished.
func NewReadOnlyIndex(dbPath string) (*Index, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening read-only index database: %w", err)
	}
	return &Index{db: db}, nil
}

// Close closes the underlying database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// initSchema creates all index tables, FTS5 virtual tables, and synchronization triggers if they do not exist.
func (idx *Index) initSchema() error {
	// --- Step 1: Execute base DDL for tables, FTS5 virtual tables, and triggers ---
	query := `
	CREATE VIRTUAL TABLE IF NOT EXISTS episodes_fts USING fts5(
		episode_id,
		session_id,
		content,
		files_touched,
		outcomes,
		date UNINDEXED
	);
	
	CREATE TABLE IF NOT EXISTS entities (
		id TEXT PRIMARY KEY,
		kind TEXT,
		name TEXT
	);
	
	CREATE TABLE IF NOT EXISTS entity_relations (
		source_id TEXT,
		target_id TEXT,
		kind TEXT,
		PRIMARY KEY(source_id, target_id, kind)
	);

	CREATE TABLE IF NOT EXISTS entries (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		kind TEXT DEFAULT '',
		dossier_scope TEXT NOT NULL,
		text TEXT NOT NULL,
		source TEXT,
		source_doc_path TEXT DEFAULT '',
		source_doc_blob_sha TEXT DEFAULT '',
		success_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		last_succeeded_at TEXT DEFAULT '',
		stale_suspect INTEGER DEFAULT 0,
		render_as_skill TEXT DEFAULT '',
		verified TEXT,
		closes_when TEXT,
		created_at TEXT,
		updated_at TEXT
	);

	CREATE VIRTUAL TABLE IF NOT EXISTS entries_fts USING fts5(
		id,
		dossier_scope,
		text,
		content=entries,
		content_rowid=rowid
	);

	CREATE TRIGGER IF NOT EXISTS entries_ai AFTER INSERT ON entries BEGIN
		INSERT INTO entries_fts(rowid, id, dossier_scope, text) VALUES (new.rowid, new.id, new.dossier_scope, new.text);
	END;

	CREATE TRIGGER IF NOT EXISTS entries_ad AFTER DELETE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, id, dossier_scope, text) VALUES('delete', old.rowid, old.id, old.dossier_scope, old.text);
	END;

	CREATE TRIGGER IF NOT EXISTS entries_au AFTER UPDATE ON entries BEGIN
		INSERT INTO entries_fts(entries_fts, rowid, id, dossier_scope, text) VALUES('delete', old.rowid, old.id, old.dossier_scope, old.text);
		INSERT INTO entries_fts(rowid, id, dossier_scope, text) VALUES (new.rowid, new.id, new.dossier_scope, new.text);
	END;

	CREATE TABLE IF NOT EXISTS entry_anchors (
		entry_id TEXT NOT NULL,
		anchor TEXT NOT NULL,
		kind TEXT,
		fingerprint TEXT,
		FOREIGN KEY(entry_id) REFERENCES entries(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_entry_anchors_anchor ON entry_anchors(anchor);

	CREATE TABLE IF NOT EXISTS error_patterns (
		entry_id TEXT NOT NULL,
		command_prefix TEXT,
		pattern TEXT NOT NULL,
		dossier_scope TEXT NOT NULL,
		FOREIGN KEY(entry_id) REFERENCES entries(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS aliases (
		old_scope TEXT NOT NULL,
		new_scope TEXT NOT NULL,
		created_at TEXT
	);

	CREATE TABLE IF NOT EXISTS entry_history (
		id TEXT NOT NULL,
		entry_id TEXT NOT NULL,
		type TEXT NOT NULL,
		dossier_scope TEXT NOT NULL,
		text TEXT,
		reason TEXT,
		superseded_by TEXT,
		retired_at TEXT
	);
	`
	_, err := idx.db.Exec(query)
	if err != nil {
		return fmt.Errorf("initializing schema: %w", err)
	}

	// --- Step 2: Run incremental column migrations on the entries table ---
	if err := idx.migrateEntriesColumns(); err != nil {
		return fmt.Errorf("migrating entries columns: %w", err)
	}
	return nil
}

// migrateEntriesColumns inspects existing columns on the entries table and adds any missing v2 columns.
func (idx *Index) migrateEntriesColumns() error {
	// --- Step 1: Query existing column definitions from PRAGMA table_info(entries) ---
	rows, err := idx.db.Query("PRAGMA table_info(entries)")
	if err != nil {
		return err
	}
	existing := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	// --- Step 2: Define required v2 columns and add any that are absent ---
	type colSpec struct {
		name string // SQLite column identifier.
		ddl  string // Column type and default constraint.
	}
	required := []colSpec{
		{"kind", "TEXT DEFAULT ''"},
		{"source_doc_path", "TEXT DEFAULT ''"},
		{"source_doc_blob_sha", "TEXT DEFAULT ''"},
		{"success_count", "INTEGER DEFAULT 0"},
		{"failure_count", "INTEGER DEFAULT 0"},
		{"last_succeeded_at", "TEXT DEFAULT ''"},
		{"stale_suspect", "INTEGER DEFAULT 0"},
		{"render_as_skill", "TEXT DEFAULT ''"},
	}

	for _, c := range required {
		if !existing[c.name] {
			alter := fmt.Sprintf("ALTER TABLE entries ADD COLUMN %s %s", c.name, c.ddl)
			if _, err := idx.db.Exec(alter); err != nil {
				return fmt.Errorf("adding column %s: %w", c.name, err)
			}
		}
	}
	return nil
}

// EpisodeRecord represents a row in the episodes FTS table.
type EpisodeRecord struct {
	EpisodeID    string    // Primary key matching the compacted Episode.Header.EpisodeID in episodes/*.jsonl.zst.
	SessionID    string    // Originating conversation or session identifier.
	Content      string    // Concatenated step text and commands for full-text search.
	FilesTouched string    // Space-separated list of unique file paths touched during the episode.
	Outcomes     string    // Newline-separated step outcomes recorded in the episode.
	Date         time.Time // Timestamp when the episode started (or ended).
	Snippet      string    // Highlighted FTS5 match snippet populated during search.
}

// InsertEpisode inserts a new episode record into the FTS index.
func (idx *Index) InsertEpisode(ctx context.Context, record EpisodeRecord) error {
	// --- Step 1: Execute FTS5 insert statement for the episode record ---
	query := `
	INSERT INTO episodes_fts (episode_id, session_id, content, files_touched, outcomes, date)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := idx.db.ExecContext(ctx, query,
		record.EpisodeID,
		record.SessionID,
		record.Content,
		record.FilesTouched,
		record.Outcomes,
		record.Date.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("inserting episode to index: %w", err)
	}
	return nil
}

// SearchEpisodes performs a full-text search against the episodes FTS table.
func (idx *Index) SearchEpisodes(ctx context.Context, matchQuery string) ([]EpisodeRecord, error) {
	// --- Step 1: Query episodes_fts ordered by BM25 relevance ---
	query := `
	SELECT episode_id, session_id, content, snippet(episodes_fts, 2, '>', '', '...', 64), files_touched, outcomes, date 
	FROM episodes_fts 
	WHERE episodes_fts MATCH ?
	ORDER BY bm25(episodes_fts)
	`
	rows, err := idx.db.QueryContext(ctx, query, matchQuery)
	if err != nil {
		return nil, fmt.Errorf("searching episodes: %w", err)
	}
	defer rows.Close()

	// --- Step 2: Scan matching rows and parse RFC3339 dates ---
	var results []EpisodeRecord
	for rows.Next() {
		var r EpisodeRecord
		var dateStr string
		if err := rows.Scan(&r.EpisodeID, &r.SessionID, &r.Content, &r.Snippet, &r.FilesTouched, &r.Outcomes, &dateStr); err != nil {
			return nil, fmt.Errorf("scanning search result: %w", err)
		}
		if d, err := time.Parse(time.RFC3339, dateStr); err == nil {
			r.Date = d
		}
		results = append(results, r)
	}
	return results, rows.Err()
}
