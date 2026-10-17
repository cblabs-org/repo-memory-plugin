/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/state/subagents.go
 * Description: Manages the SQLite-backed parent-child subagent conversation
 *              registry at <dataDir>/<repoID>/state/subagents.sqlite.
 *
 * Exported Types / Functions:
 * - SubagentStore:          Manages the SQLite subagent parent-child conversation registry.
 *
 *   Lifecycle:
 *   - OpenSubagentStore():  Opens or creates the subagent registry SQLite store.
 *   - Close():              Closes the underlying SQLite connection. [io.Closer]
 *
 *   Own Public API:
 *   - Record():             Inserts or updates a child -> parent conversation mapping.
 *   - IsSubagent():         Reports whether a conversation ID is registered as a child subagent.
 *   - ListChildren():       Returns all child conversation IDs registered under a parent.
 *
 * - SubagentDBPath():       Returns the filesystem path to subagents.sqlite.
 * - RecordSubagent():       Opens the store, records a child->parent mapping, and closes it.
 * - IsSubagent():           Checks whether a conversation ID is a registered subagent without creating the DB.
 * - ListSubagentChildren(): Returns all child conversation IDs for a parent without creating the DB.
 */

package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// SubagentStore manages the SQLite subagent parent-child conversation registry
// at <dataDir>/<repoID>/state/subagents.sqlite.
type SubagentStore struct {
	db *sql.DB // Underlying SQLite database connection handle.
}

// SubagentDBPath returns the path to the subagent registry SQLite database.
func SubagentDBPath(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "state", "subagents.sqlite")
}

// OpenSubagentStore opens or creates the subagent registry SQLite store.
// The caller must call Close on the returned SubagentStore when finished.
func OpenSubagentStore(dataDir, repoID string) (*SubagentStore, error) {
	// --- Step 1: Ensure state directory exists and open SQLite connection with WAL mode ---
	dbPath := SubagentDBPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating subagent state directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening subagents database: %w", err)
	}

	// --- Step 2: Initialize the subagents table schema ---
	s := &SubagentStore{db: db}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the underlying SQLite connection.
func (s *SubagentStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// initSchema creates the subagents table and parent conversation index if they do not exist.
func (s *SubagentStore) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS subagents (
		child_conversation_id TEXT PRIMARY KEY,
		parent_conversation_id TEXT NOT NULL,
		role TEXT DEFAULT '',
		recorded_at TEXT NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_subagents_parent ON subagents(parent_conversation_id);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("initializing subagents schema: %w", err)
	}
	return nil
}

// Record inserts or updates a child -> parent conversation mapping.
func (s *SubagentStore) Record(ctx context.Context, childConvID, parentConvID, role string) error {
	// --- Step 1: Validate conversation IDs and upsert relationship row ---
	childConvID = strings.TrimSpace(childConvID)
	parentConvID = strings.TrimSpace(parentConvID)
	if childConvID == "" || parentConvID == "" {
		return errors.New("child and parent conversation IDs are required")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	query := `
	INSERT INTO subagents (child_conversation_id, parent_conversation_id, role, recorded_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(child_conversation_id) DO UPDATE SET
		parent_conversation_id = excluded.parent_conversation_id,
		role = CASE WHEN excluded.role != '' THEN excluded.role ELSE subagents.role END,
		recorded_at = excluded.recorded_at
	`
	_, err := s.db.ExecContext(ctx, query, childConvID, parentConvID, strings.TrimSpace(role), now)
	return err
}

// IsSubagent returns whether convID is registered as a child subagent and its parent conversation ID.
func (s *SubagentStore) IsSubagent(ctx context.Context, convID string) (bool, string, error) {
	// --- Step 1: Query parent_conversation_id for the candidate child conversation ---
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return false, "", nil
	}
	var parentID string
	err := s.db.QueryRowContext(ctx,
		`SELECT parent_conversation_id FROM subagents WHERE child_conversation_id = ?`,
		convID,
	).Scan(&parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, "", nil
		}
		return false, "", err
	}
	return true, parentID, nil
}

// ListChildren returns all child conversation IDs registered under parentConvID.
func (s *SubagentStore) ListChildren(ctx context.Context, parentConvID string) ([]string, error) {
	// --- Step 1: Query child conversation IDs ordered by registration time ---
	parentConvID = strings.TrimSpace(parentConvID)
	if parentConvID == "" {
		return []string{}, nil
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT child_conversation_id FROM subagents WHERE parent_conversation_id = ? ORDER BY recorded_at ASC, child_conversation_id ASC`,
		parentConvID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// --- Step 2: Scan result rows into a slice ---
	children := make([]string, 0)
	for rows.Next() {
		var childID string
		if err := rows.Scan(&childID); err != nil {
			return nil, err
		}
		children = append(children, childID)
	}
	return children, rows.Err()
}

// RecordSubagent opens the subagent store, records the child->parent relationship, and closes the store.
func RecordSubagent(ctx context.Context, dataDir, repoID, childConvID, parentConvID, role string) error {
	childConvID = strings.TrimSpace(childConvID)
	parentConvID = strings.TrimSpace(parentConvID)
	if childConvID == "" || parentConvID == "" || dataDir == "" || repoID == "" {
		return nil
	}
	store, err := OpenSubagentStore(dataDir, repoID)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.Record(ctx, childConvID, parentConvID, role)
}

// IsSubagent checks whether convID is a registered subagent conversation.
// If the subagents.sqlite file does not exist yet, it returns false, "", nil without creating the file.
func IsSubagent(ctx context.Context, dataDir, repoID, convID string) (bool, string, error) {
	// --- Step 1: Check for an existing subagents.sqlite file on disk ---
	convID = strings.TrimSpace(convID)
	if convID == "" || dataDir == "" || repoID == "" {
		return false, "", nil
	}
	dbPath := SubagentDBPath(dataDir, repoID)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return false, "", nil
		}
		return false, "", err
	}

	// --- Step 2: Open the store and look up the child conversation ---
	store, err := OpenSubagentStore(dataDir, repoID)
	if err != nil {
		return false, "", err
	}
	defer store.Close()
	return store.IsSubagent(ctx, convID)
}

// ListSubagentChildren returns all child conversation IDs registered for parentConvID.
// If the subagents.sqlite file does not exist yet, it returns an empty slice without creating the file.
func ListSubagentChildren(ctx context.Context, dataDir, repoID, parentConvID string) ([]string, error) {
	// --- Step 1: Check for an existing subagents.sqlite file on disk ---
	parentConvID = strings.TrimSpace(parentConvID)
	if parentConvID == "" || dataDir == "" || repoID == "" {
		return []string{}, nil
	}
	dbPath := SubagentDBPath(dataDir, repoID)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	// --- Step 2: Open the store and list registered child conversations ---
	store, err := OpenSubagentStore(dataDir, repoID)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.ListChildren(ctx, parentConvID)
}
