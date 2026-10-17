/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/spool/reader.go
 * Description: Discovers, chronologically orders, deserializes, and acknowledges
 *              durable spool entry files for background tick ingestion.
 *
 * Exported Types / Functions:
 * - Item:        Bundles a deserialized Entry with its on-disk file path.
 * - Reader:      Scans, reads, and acknowledges spool files in a spool directory.
 *   - NewReader():      Constructs a Reader for the specified spool directory.
 *   - SpoolDir():       Returns the configured spool directory path.
 *   - ReadEntries():    Returns all valid spool items ordered chronologically.
 *   - ReadEntry():      Reads and unmarshals a single spool JSON file.
 *   - Acknowledge():    Removes a processed spool file from disk.
 *   - AcknowledgeAll(): Removes a slice of processed spool items from disk.
 *   - Ingest():         Iterates over spool items, invoking a handler and acknowledging on success.
 * - ReadEntries(): Package-level helper that reads all entries from a spool directory.
 */

package spool

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Item bundles a spool Entry with the on-disk file path it was read from.
type Item struct {
	Path  string // Full filesystem path to the spool JSON file.
	Entry Entry  // Deserialized spool entry payload.
}

// Reader discovers and reads durable spool entries for tick ingestion.
type Reader struct {
	spoolDir string // Directory path containing queued spool JSON files.
}

// NewReader returns a new Reader for the specified spool directory.
func NewReader(spoolDir string) *Reader {
	return &Reader{spoolDir: spoolDir}
}

// SpoolDir returns the spool directory being read.
func (r *Reader) SpoolDir() string {
	return r.spoolDir
}

// ReadEntries scans the spool directory, filters out non-spool and in-progress temporary files,
// sorts entries chronologically by filename, and deserializes them.
// If the spool directory does not exist, an empty slice and nil error are returned.
func (r *Reader) ReadEntries() ([]Item, error) {
	// --- Step 1: Read spool directory listing ---
	if r.spoolDir == "" {
		return nil, fmt.Errorf("spool directory cannot be empty")
	}

	entries, err := os.ReadDir(r.spoolDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Item{}, nil
		}
		return nil, fmt.Errorf("reading spool directory: %w", err)
	}

	// --- Step 2: Filter out directories, temporary files, and non-JSON entries ---
	var validFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Filter out hidden / temporary files (e.g. .spool_tmp_*).
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		validFiles = append(validFiles, name)
	}

	// --- Step 3: Sort filenames chronologically and unmarshal each entry ---
	// Lexicographical sorting corresponds to chronological order because filenames start with ISO8601 UTC timestamp.
	sort.Strings(validFiles)

	items := make([]Item, 0, len(validFiles))
	for _, name := range validFiles {
		fullPath := filepath.Join(r.spoolDir, name)
		entry, err := r.ReadEntry(fullPath)
		if err != nil {
			return nil, fmt.Errorf("reading spool entry %s: %w", fullPath, err)
		}
		items = append(items, Item{
			Path:  fullPath,
			Entry: entry,
		})
	}

	return items, nil
}

// ReadEntry reads and parses a single spool file from disk.
func (r *Reader) ReadEntry(path string) (Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, fmt.Errorf("reading spool file: %w", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return Entry{}, fmt.Errorf("unmarshaling spool JSON: %w", err)
	}

	return entry, nil
}

// Acknowledge removes the spool entry file from disk after it has been safely ingested.
// It returns nil if the file has already been removed.
func (r *Reader) Acknowledge(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing spool entry %s: %w", path, err)
	}
	return nil
}

// AcknowledgeAll removes all given items from disk.
func (r *Reader) AcknowledgeAll(items []Item) error {
	for _, item := range items {
		if err := r.Acknowledge(item.Path); err != nil {
			return err
		}
	}
	return nil
}

// Ingest iterates through available spool entries, executing fn for each item.
// If fn returns nil, the spool entry is automatically acknowledged (removed).
// If fn returns an error, ingestion halts immediately, leaving the failing item
// and subsequent items in the spool for the next tick.
// It returns the count of successfully ingested items.
func (r *Reader) Ingest(fn func(item Item) error) (int, error) {
	// --- Step 1: Load ordered spool entries from disk ---
	items, err := r.ReadEntries()
	if err != nil {
		return 0, err
	}

	// --- Step 2: Execute callback per item and acknowledge on success ---
	processed := 0
	for _, item := range items {
		if err := fn(item); err != nil {
			return processed, err
		}
		if err := r.Acknowledge(item.Path); err != nil {
			return processed, err
		}
		processed++
	}

	return processed, nil
}

// ReadEntries reads entries from spoolDir using a new Reader.
func ReadEntries(spoolDir string) ([]Item, error) {
	return NewReader(spoolDir).ReadEntries()
}
