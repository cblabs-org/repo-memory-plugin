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
	Path  string
	Entry Entry
}

// Reader discovers and reads durable spool entries for tick ingestion.
type Reader struct {
	spoolDir string
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

	var validFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Filter out hidden / temporary files (e.g. .spool_tmp_*)
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".tmp") {
			continue
		}
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		validFiles = append(validFiles, name)
	}

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
// Returns nil if the file has already been removed.
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
// Returns the count of successfully ingested items.
func (r *Reader) Ingest(fn func(item Item) error) (int, error) {
	items, err := r.ReadEntries()
	if err != nil {
		return 0, err
	}

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
