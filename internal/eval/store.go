/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/eval/store.go
 * Description: Provides append-only JSONL persistence for benchmark evaluation results.
 *
 * Exported Types / Functions:
 * - Result: Single benchmark task evaluation record serialized to JSONL.
 * - Store:  Append-only JSONL writer for evaluation results.
 *
 *   Lifecycle:
 *   - NewStore(): Opens or creates results.jsonl inside the target directory.
 *   - Close():    Closes the underlying results.jsonl file handle. [io.Closer]
 *
 *   Own Public API:
 *   - Record():   Timestamps and appends a Result entry as a JSON line.
 */

package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Result represents a single benchmark task evaluation record written to results.jsonl.
type Result struct {
	Timestamp time.Time   `json:"timestamp"`         // Wall-clock time when the result was recorded.
	Benchmark string      `json:"benchmark"`         // Evaluation suite name or tier code (e.g. "B2_fts_retrieval" or "X6_replay").
	DatasetID string      `json:"dataset_id"`        // Identifier of the evaluated Dataset.
	TaskID    string      `json:"task_id"`           // Identifier of the evaluated Task.
	Query     string      `json:"query,omitempty"`   // Search query issued for the task.
	Matches   []string    `json:"matches,omitempty"` // Retrieved episode IDs or snippets.
	Metrics   interface{} `json:"metrics,omitempty"` // Optional benchmark-specific score payload.
}

// Store manages an append-only JSONL file for recording benchmark evaluation results.
type Store struct {
	path string        // Filesystem path to results.jsonl.
	file *os.File      // Open append-only file handle.
	enc  *json.Encoder // Streaming JSON encoder bound to file.
}

// NewStore creates or opens results.jsonl inside dir for appending evaluation results.
// The caller must call Close on the returned Store when finished.
func NewStore(dir string) (*Store, error) {
	// --- Step 1: Ensure the output directory exists ---
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating eval result dir: %w", err)
	}

	// --- Step 2: Open results.jsonl in append mode and bind a JSON encoder ---
	resultPath := filepath.Join(dir, "results.jsonl")
	f, err := os.OpenFile(resultPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening eval result file: %w", err)
	}

	return &Store{
		path: resultPath,
		file: f,
		enc:  json.NewEncoder(f),
	}, nil
}

// Record stamps r with the current timestamp and encodes it as a JSON line.
func (s *Store) Record(r Result) error {
	r.Timestamp = time.Now()
	if err := s.enc.Encode(r); err != nil {
		return fmt.Errorf("encoding result: %w", err)
	}
	return nil
}

// Close closes the underlying results.jsonl file handle.
func (s *Store) Close() error {
	return s.file.Close()
}
