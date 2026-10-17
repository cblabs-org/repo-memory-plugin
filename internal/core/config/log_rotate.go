/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/log_rotate.go
 * Description: Size-bounded log file rotation and diagnostic probe log cleanup
 *              for repo-memory data directories.
 *
 * Exported Types / Functions:
 * - DefaultMaxLogBytes:     Default byte threshold (10 MB) before rotating a log file.
 * - DefaultMaxRotatedFiles: Default number of rotated log backups retained (3).
 * - RotateLogFile():        Rotates a log file when its size reaches or exceeds maxBytes.
 * - CleanProbeLogs():       Deletes diagnostic probe log and transcript files older than a threshold.
 */

package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultMaxLogBytes is 10 MB.
	DefaultMaxLogBytes int64 = 10 * 1024 * 1024
	// DefaultMaxRotatedFiles is 3 (keeps .1, .2, .3, deletes .3).
	DefaultMaxRotatedFiles = 3
)

// RotateLogFile checks if the file at path exceeds maxBytes.
// If it does, it rotates the file:
// - If path.maxFiles exists, it is deleted.
// - Existing rotated files path.N are shifted to path.(N+1).
// - path is renamed to path.1.
// If maxBytes <= 0, DefaultMaxLogBytes is used.
// If maxFiles <= 0, DefaultMaxRotatedFiles is used.
func RotateLogFile(path string, maxBytes int64, maxFiles int) error {
	// --- Step 1: Normalize rotation thresholds and check active log file size ---
	if path == "" {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxLogBytes
	}
	if maxFiles <= 0 {
		maxFiles = DefaultMaxRotatedFiles
	}

	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat log file %s: %w", path, err)
	}

	if fi.Size() < maxBytes {
		return nil
	}

	// --- Step 2: Remove the oldest rotated log file if present ---
	oldest := fmt.Sprintf("%s.%d", path, maxFiles)
	if err := os.Remove(oldest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing oldest rotated log %s: %w", oldest, err)
	}

	// --- Step 3: Shift existing rotated backups downwards (.N -> .(N+1)) ---
	for i := maxFiles - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", path, i)
		dst := fmt.Sprintf("%s.%d", path, i+1)
		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return fmt.Errorf("rotating log %s to %s: %w", src, dst, err)
			}
		}
	}

	// --- Step 4: Rename the active log file to .1 ---
	dst := fmt.Sprintf("%s.1", path)
	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("renaming active log %s to %s: %w", path, dst, err)
	}

	return nil
}

// CleanProbeLogs searches baseDir for probe logs and deletes matching files.
// Matches: probe*.log, *.probe, probe_*.json, probe_*.jsonl.
// If olderThan > 0, only files with ModTime older than olderThan are removed.
// If olderThan <= 0, all matching files are removed.
func CleanProbeLogs(baseDir string, olderThan time.Duration) (int, error) {
	// --- Step 1: Resolve base directory and verify existence ---
	if baseDir == "" {
		baseDir = DataDir()
	}

	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return 0, nil
	}

	deleted := 0
	now := time.Now()

	// --- Step 2: Walk directory tree and remove matching probe artifacts ---
	err := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip unreadable paths without failing walk
		}
		if d.IsDir() {
			return nil
		}

		name := d.Name()
		isProbe := strings.HasPrefix(name, "probe") ||
			strings.HasSuffix(name, ".probe") ||
			strings.HasSuffix(name, "_transcript.jsonl")

		if !isProbe {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		if olderThan > 0 && now.Sub(info.ModTime()) < olderThan {
			return nil
		}

		if err := os.Remove(path); err == nil {
			deleted++
		}
		return nil
	})

	return deleted, err
}
