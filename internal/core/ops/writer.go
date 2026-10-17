/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/ops/writer.go
 * Description: Validates and writes OpBatch JSON files into monthly
 *              subdirectories under the operation log base directory.
 *
 * Exported Types / Functions:
 * - Writer: Persists validated OpBatch files into YYYY-MM subdirectories.
 *   - NewWriter(): Constructs a Writer rooted at baseDir.
 *   - Write():     Validates and writes an OpBatch as indented JSON to disk.
 */

package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Writer writes OpBatch files into the ops log directory.
type Writer struct {
	baseDir string // Root directory where YYYY-MM batch subdirectories are created.
}

// NewWriter creates a new Writer for the specified base directory.
func NewWriter(baseDir string) *Writer {
	return &Writer{
		baseDir: baseDir,
	}
}

// Write writes the batch as indented JSON to .memory/ops/<yyyy-mm>/<batch-id>.json,
// creating the month directory if needed.
func (w *Writer) Write(batch OpBatch) error {
	// --- Step 1: Validate batch invariants and ensure the target YYYY-MM directory exists ---
	if err := ValidateBatch(batch); err != nil {
		return fmt.Errorf("validate batch: %w", err)
	}

	month := batch.CreatedAt.UTC().Format("2006-01")
	monthDir := filepath.Join(w.baseDir, month)
	if err := os.MkdirAll(monthDir, 0o755); err != nil {
		return fmt.Errorf("create month directory %s: %w", monthDir, err)
	}

	// --- Step 2: Marshal indented JSON with trailing newline and write batch file ---
	filePath := filepath.Join(monthDir, batch.BatchID+".json")
	data, err := json.MarshalIndent(batch, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal batch %s: %w", batch.BatchID, err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("write batch file %s: %w", filePath, err)
	}

	return nil
}
