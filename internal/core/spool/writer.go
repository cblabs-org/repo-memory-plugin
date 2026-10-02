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
	spoolDir string
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
// Respects Windows file-locking semantics by closing all handles prior to rename.
func (w *Writer) Write(entry Entry) (string, error) {
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

	// 1. Create temporary file directly in spoolDir (same filesystem for atomic rename).
	// The leading dot ensures readers filter it out while in-progress.
	tmpFile, err := os.CreateTemp(w.spoolDir, ".spool_tmp_*")
	if err != nil {
		return "", fmt.Errorf("creating temp spool file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// 2. Write data and flush to disk.
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

	// 3. Close the file handle BEFORE renaming.
	// On Windows, attempting to rename an open file fails with ERROR_ACCESS_DENIED.
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("closing temp spool file: %w", err)
	}

	// 4. Construct final destination filename.
	// Lexicographically sortable timestamp without colons (safe for Windows filenames).
	tsFormatted := entry.Timestamp.Format("20060102T150405.000000000Z")
	finalFilename := fmt.Sprintf("%s_%s_%s.json", tsFormatted, entry.Kind, entry.ID)
	destPath := filepath.Join(w.spoolDir, finalFilename)

	// 5. Atomically rename with retry loop for Windows file-system edge cases.
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
