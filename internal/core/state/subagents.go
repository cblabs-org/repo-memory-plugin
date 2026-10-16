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
	db *sql.DB
}

// SubagentDBPath returns the path to the subagent registry SQLite database.
func SubagentDBPath(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "state", "subagents.sqlite")
}

// OpenSubagentStore opens or creates the subagent registry SQLite store.
func OpenSubagentStore(dataDir, repoID string) (*SubagentStore, error) {
	dbPath := SubagentDBPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("creating subagent state directory: %w", err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening subagents database: %w", err)
	}

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
	store, err := OpenSubagentStore(dataDir, repoID)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.ListChildren(ctx, parentConvID)
}
