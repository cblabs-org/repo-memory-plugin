/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/spool/writer.go
 * Description: Atomically writes durable JSON spool entries to the spool
 *              directory with Windows-safe handle closure and retry semantics.
 *
 * Exported Types / Functions:
 * - Writer: Writes durable spool entries into a configured spool directory.
 *   - NewWriter(): Constructs a Writer targeting the given spool directory.
 *   - SpoolDir():  Returns the configured spool directory path.
 *   - Write():     Atomically writes a spool Entry and returns its destination path.
 * - Write(): Package-level helper that writes an Entry to spoolDir.
 */

package spool

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Writer writes durable spool entries into the spool directory.
type Writer struct {
	spoolDir string // Target directory path where spool JSON files are written.
}

// NewWriter returns a new Writer targeting the given spool directory.
func NewWriter(spoolDir string) *Writer {
	return &Writer{spoolDir: spoolDir}
}

// SpoolDir returns the configured spool directory path.
func (w *Writer) SpoolDir() string {
	return w.spoolDir
}

// Write writes the given spool entry to a unique temporary file in the spool directory,
// flushes and closes it, and then atomically renames it to its final destination name.
// It respects Windows file-locking semantics by closing all handles prior to rename.
func (w *Writer) Write(entry Entry) (string, error) {
	// --- Step 1: Validate spool directory and populate default entry fields ---
	if w.spoolDir == "" {
		return "", fmt.Errorf("spool directory cannot be empty")
	}

	if err := os.MkdirAll(w.spoolDir, 0755); err != nil {
		return "", fmt.Errorf("creating spool directory: %w", err)
	}

	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	} else {
		entry.Timestamp = entry.Timestamp.UTC()
	}

	if entry.Kind == "" {
		entry.Kind = KindCapture
	}

	if entry.ID == "" {
		suffix, err := randomHex(6)
		if err != nil {
			suffix = fmt.Sprintf("%06x", time.Now().UnixNano()%0xffffff)
		}
		entry.ID = fmt.Sprintf("spool-%d-%s", entry.Timestamp.UnixNano(), suffix)
	}

	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling spool entry: %w", err)
	}

	// --- Step 2: Create a hidden temporary file in spoolDir ---
	// The leading dot ensures readers filter it out while in-progress.
	tmpFile, err := os.CreateTemp(w.spoolDir, ".spool_tmp_*")
	if err != nil {
		return "", fmt.Errorf("creating temp spool file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// --- Step 3: Write payload, flush to disk, and close handle before rename ---
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("writing spool entry: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("syncing spool entry: %w", err)
	}

	// On Windows, attempting to rename an open file fails with ERROR_ACCESS_DENIED.
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("closing temp spool file: %w", err)
	}

	// --- Step 4: Construct sortable destination filename and atomically rename ---
	// Lexicographically sortable timestamp without colons (safe for Windows filenames).
	tsFormatted := entry.Timestamp.Format("20060102T150405.000000000Z")
	finalFilename := fmt.Sprintf("%s_%s_%s.json", tsFormatted, entry.Kind, entry.ID)
	destPath := filepath.Join(w.spoolDir, finalFilename)

	if err := atomicRename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("renaming temp spool file: %w", err)
	}

	return destPath, nil
}

// Write writes an entry using a new Writer for the given spool directory.
func Write(spoolDir string, entry Entry) (string, error) {
	return NewWriter(spoolDir).Write(entry)
}

// randomHex returns a cryptographically random hex string encoded from bytes random bytes.
func randomHex(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// atomicRename renames src to dst, retrying briefly on Windows if a transient sharing lock occurs.
func atomicRename(src, dst string) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		err := os.Rename(src, dst)
		if err == nil {
			return nil
		}
		lastErr = err
		time.Sleep(time.Duration(5*(i+1)) * time.Millisecond)
	}
	return lastErr
}
