/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/ops/migrate.go
 * Description: Migrates legacy v1 event log directories to an archive directory
 *              with cross-device copy-and-remove fallback.
 *
 * Exported Types / Functions:
 * - MigrateV1Events(): Moves a legacy v1 events directory to an archive path if present.
 */

package ops

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// MigrateV1Events moves the v1 events directory to the archive directory if it exists.
// If eventsDir does not exist, it is a no-op and returns nil.
func MigrateV1Events(eventsDir, archiveDir string) error {
	// --- Step 1: Verify source directory exists and normalize paths ---
	info, err := os.Stat(eventsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat events dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("events path %s is not a directory", eventsDir)
	}

	cleanEvents := filepath.Clean(eventsDir)
	cleanArchive := filepath.Clean(archiveDir)
	if cleanEvents == cleanArchive {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(cleanArchive), 0o755); err != nil {
		return fmt.Errorf("create archive parent dir: %w", err)
	}

	// --- Step 2: Attempt fast atomic rename, falling back to recursive move ---
	// Try direct atomic rename first.
	if err := os.Rename(cleanEvents, cleanArchive); err == nil {
		return nil
	}

	// If destination already exists or rename fails across devices, move contents recursively.
	if err := moveDirContents(cleanEvents, cleanArchive); err != nil {
		return fmt.Errorf("migrate events from %s to %s: %w", cleanEvents, cleanArchive, err)
	}

	return os.RemoveAll(cleanEvents)
}

// moveDirContents recursively moves all files and subdirectories from srcDir into dstDir.
func moveDirContents(srcDir, dstDir string) error {
	// --- Step 1: Ensure destination directory exists and list source entries ---
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return err
	}

	// --- Step 2: Move each directory or file entry with copy-and-remove fallback ---
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		dstPath := filepath.Join(dstDir, entry.Name())
		if entry.IsDir() {
			if err := moveDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := os.Rename(srcPath, dstPath); err != nil {
				if err := copyFile(srcPath, dstPath); err != nil {
					return err
				}
				if err := os.Remove(srcPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// copyFile copies a single regular file from src to dst and syncs the destination file.
func copyFile(src, dst string) error {
	// --- Step 1: Open source file and create destination file ---
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// --- Step 2: Stream file contents and flush to disk ---
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
