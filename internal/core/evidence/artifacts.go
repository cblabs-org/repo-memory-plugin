/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/evidence/artifacts.go
 * Description: Copies conversation Markdown artifacts and metadata JSON files
 *              from harness brain directories into the repository's evidence
 *              artifact store with size bounds and atomic file writes.
 *
 * Exported Types / Functions:
 * - MaxArtifactSize: Maximum byte size (1 MB) for an individual artifact copy.
 * - ArtifactDir():   Returns the canonical destination directory for a conversation's artifacts.
 * - CopyArtifacts(): Recursively copies eligible artifact files from srcDir to dstDir.
 */

package evidence

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

// MaxArtifactSize is the maximum size (1 MB) for artifacts to be copied.
const MaxArtifactSize = 1024 * 1024 // 1 MB upper bound per artifact file.

// ArtifactDir returns the destination directory path for conversation artifacts:
// <data-dir>/<repo-id>/evidence/artifacts/<conversation-id>/
func ArtifactDir(dataDir, repoID, conversationID string) string {
	if dataDir == "" {
		dataDir = config.DataDir()
	}
	return filepath.Join(dataDir, repoID, "evidence", "artifacts", conversationID)
}

// CopyArtifacts copies .md, .md.resolved, and .metadata.json files from srcDir to dstDir,
// skipping .system_generated/ and files larger than MaxArtifactSize.
// It returns the count of files copied and is idempotent when size and modtime match.
func CopyArtifacts(srcDir, dstDir string) (int, error) {
	// --- Step 1: Validate source and destination directory paths ---
	if srcDir == "" {
		return 0, errors.New("srcDir cannot be empty")
	}
	if dstDir == "" {
		return 0, errors.New("dstDir cannot be empty")
	}

	srcInfo, err := os.Stat(srcDir)
	if err != nil {
		return 0, err
	}
	if !srcInfo.IsDir() {
		return 0, fmt.Errorf("srcDir %s is not a directory", srcDir)
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return 0, fmt.Errorf("creating destination directory %s: %w", dstDir, err)
	}

	// --- Step 2: Walk source tree, filter eligible artifacts, and copy changed files ---
	copiedCount := 0
	err = filepath.WalkDir(srcDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() {
			if d.Name() == ".system_generated" {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".metadata.json") && !strings.HasSuffix(name, ".md.resolved") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		if info.Size() > MaxArtifactSize {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dstDir, rel)

		// Skip copy if target already exists with identical size and modification time.
		if dstInfo, err := os.Stat(targetPath); err == nil {
			if dstInfo.Size() == info.Size() && dstInfo.ModTime().Equal(info.ModTime()) {
				return nil
			}
		}

		if err := copyFile(path, targetPath); err != nil {
			return fmt.Errorf("copying artifact %s to %s: %w", path, targetPath, err)
		}
		_ = os.Chtimes(targetPath, info.ModTime(), info.ModTime())

		copiedCount++
		return nil
	})

	if err != nil {
		return copiedCount, err
	}

	return copiedCount, nil
}

// copyFile atomically copies src to dst via a temporary file in dst's parent directory.
func copyFile(src, dst string) error {
	// --- Step 1: Open source file and ensure destination parent directory exists ---
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// --- Step 2: Stream contents into a sibling temporary file and flush to disk ---
	tmpFile, err := os.CreateTemp(dir, ".artifact_tmp_*")
	if err != nil {
		return err
	}
	tmpName := tmpFile.Name()

	if _, err := io.Copy(tmpFile, in); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return err
	}

	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return err
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	// --- Step 3: Commit temporary file to destination path ---
	if runtime.GOOS == "windows" {
		_ = os.Remove(dst)
	}

	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return nil
}
