package index

import (
	"context"

	"database/sql"
	"fmt"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	_ "modernc.org/sqlite"

)

// Index provides SQLite FTS indexing over episodes.
type Index struct {
	db *sql.DB
}

// NewIndex opens or creates a new Index backed by SQLite at dbPath.
// It configures WAL mode for better concurrency.
func NewIndex(dbPath string) (*Index, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", dbPath)
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
func NewReadOnlyIndex(dbPath string) (*Index, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro", dbPath)
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

func (idx *Index) initSchema() error {
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
	
	CREATE TABLE IF NOT EXISTS claims (
		id TEXT PRIMARY KEY,
		type TEXT,
		title TEXT,
		status TEXT,
		relevant_when TEXT,
		apply TEXT,
		verified_at TEXT,
		body TEXT
	);
	
	CREATE TABLE IF NOT EXISTS anchors (
		claim_id TEXT,
		kind TEXT,
		ref TEXT,
		fp TEXT,
		FOREIGN KEY(claim_id) REFERENCES claims(id) ON DELETE CASCADE
	);
	
	CREATE TABLE IF NOT EXISTS relations (
		source_id TEXT,
		relation_type TEXT,
		target_id TEXT,
		FOREIGN KEY(source_id) REFERENCES claims(id) ON DELETE CASCADE
	);
	
	CREATE TABLE IF NOT EXISTS conditions (
		claim_id TEXT,
		context TEXT,
		key TEXT,
		ref TEXT,
		state TEXT,
		target TEXT,
		result TEXT,
		FOREIGN KEY(claim_id) REFERENCES claims(id) ON DELETE CASCADE
	);
	
	CREATE TABLE IF NOT EXISTS entity_relations (
		source_id TEXT,
		target_id TEXT,
		kind TEXT,
		PRIMARY KEY(source_id, target_id, kind)
	);
	`
	_, err := idx.db.Exec(query)
	if err != nil {
		return fmt.Errorf("initializing schema: %w", err)
	}
	return nil
}

// EpisodeRecord represents a row in the episodes FTS table.
type EpisodeRecord struct {
	EpisodeID    string
	SessionID    string
	Content      string
	FilesTouched string
	Outcomes     string
	Date         time.Time
	Snippet      string
}

// InsertEpisode inserts a new episode record into the FTS index.
func (idx *Index) InsertEpisode(ctx context.Context, record EpisodeRecord) error {
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

// InsertClaim inserts or updates a claim and its related tables in the index.
func (idx *Index) IndexClaim(ctx context.Context, claim *claims.Claim) error {
	tx, err := idx.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	query := `INSERT OR REPLACE INTO claims (id, type, title, status, relevant_when, apply, verified_at, body) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err = tx.ExecContext(ctx, query, claim.ID, claim.Type, claim.Title, claim.Status, claim.RelevantWhen, claim.Apply, claim.VerifiedAt, claim.Body)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM anchors WHERE claim_id = ?`, claim.ID)
	if err != nil {
		return err
	}
	for _, a := range claim.Anchors {
		_, err = tx.ExecContext(ctx, `INSERT INTO anchors (claim_id, kind, ref, fp) VALUES (?, ?, ?, ?)`, claim.ID, a.Kind, a.Ref, a.FP)
		if err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM relations WHERE source_id = ?`, claim.ID)
	if err != nil {
		return err
	}
	if claim.Relations != nil {
		for _, target := range claim.Relations.Supports {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "supports", target)
		}
		for _, target := range claim.Relations.Contradicts {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "contradicts", target)
		}
		for _, target := range claim.Relations.Supersedes {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "supersedes", target)
		}
		for _, target := range claim.Relations.Explains {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "explains", target)
		}
		for _, target := range claim.Relations.Refines {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "refines", target)
		}
		for _, target := range claim.Relations.DependsOn {
			tx.ExecContext(ctx, `INSERT INTO relations (source_id, relation_type, target_id) VALUES (?, ?, ?)`, claim.ID, "depends_on", target)
		}
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM conditions WHERE claim_id = ?`, claim.ID)
	if err != nil {
		return err
	}
	for k, v := range claim.Conditions {
		_, err = tx.ExecContext(ctx, `INSERT INTO conditions (claim_id, key, result) VALUES (?, ?, ?)`, claim.ID, k, v)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
