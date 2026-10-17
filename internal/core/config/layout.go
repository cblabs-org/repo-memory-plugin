/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/config/layout.go
 * Description: Directory layout provisioning and gitignore management for
 *              machine-level repository storage (<data-dir>/<repo-id>/) and
 *              in-repository memory directories (<repoRoot>/.memory/).
 *
 * Exported Types / Functions:
 * - DataDirSubdirectories:    Subdirectories provisioned under <data-dir>/<repo-id>/.
 * - RepoMemorySubdirectories: Subdirectories provisioned under <repoRoot>/.memory/.
 * - EnsureDataDirLayout():    Creates the <data-dir>/<repo-id> directory hierarchy.
 * - EnsureRepoLayout():       Creates the <repoRoot>/.memory directory hierarchy and .gitignore.
 * - EnsureMemoryGitignore():  Ensures <repoRoot>/.memory/.gitignore ignores local transient paths.
 * - EnsureStorageLayout():    Provisions data-dir and optional repo-level storage hierarchies.
 * - IndexPath():              Returns the canonical SQLite index file path for a repository.
 */

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DataDirSubdirectories lists the subdirectories under <data-dir>/<repo-id>/.
var DataDirSubdirectories = []string{
	"spool",
	"queue",
	"runs",
	"episodes",
	"evidence",
	filepath.Join("evidence", "artifacts"),
	filepath.Join("evidence", "notes"),
	"pending",
	"alerts",
	"metrics",
	"logs",
	"state",
	filepath.Join("state", "marks"),
	"quarantine",
	"user-dossiers",
	filepath.Join("user-dossiers", "ops"),
	filepath.Join("user-dossiers", "dossiers"),
	"worktrees",
}

// RepoMemorySubdirectories lists the subdirectories under <repoRoot>/.memory/.
var RepoMemorySubdirectories = []string{
	"claims",
	filepath.Join("events", "reviews"),
	filepath.Join("events", "answers"),
	filepath.Join("views", "index"),
	filepath.Join("views", "subsystems"),
	".cache",
}

// EnsureDataDirLayout creates the <data-dir>/<repo-id> directory structure and its subdirectories:
// spool, queue, runs, episodes, pending, alerts, metrics, logs, state.
// It is idempotent.
func EnsureDataDirLayout(dataDir, repoID string) error {
	// --- Step 1: Validate inputs and resolve default data directory ---
	if dataDir == "" {
		dataDir = DataDir()
	}
	if repoID == "" {
		return errors.New("repoID cannot be empty")
	}

	// --- Step 2: Create each required subdirectory under <data-dir>/<repo-id> ---
	base := filepath.Join(dataDir, repoID)
	for _, sub := range DataDirSubdirectories {
		dir := filepath.Join(base, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}

// EnsureRepoLayout creates the <repoRoot>/.memory directory structure and its subdirectories:
// claims, events/reviews, events/answers, views/index, views/subsystems, .cache.
// It also ensures a .gitignore exists ignoring .cache/, .enrolled, and local/ without overwriting custom rules.
func EnsureRepoLayout(repoRoot string) error {
	if repoRoot == "" {
		repoRoot = "."
	}

	memoryDir := filepath.Join(repoRoot, ".memory")
	for _, sub := range RepoMemorySubdirectories {
		dir := filepath.Join(memoryDir, sub)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return EnsureMemoryGitignore(repoRoot)
}

// EnsureMemoryGitignore ensures <repoRoot>/.memory/.gitignore exists and contains
// .cache/, .enrolled, and local/ without clobbering existing lines.
func EnsureMemoryGitignore(repoRoot string) error {
	// --- Step 1: Ensure the .memory directory exists ---
	if repoRoot == "" {
		repoRoot = "."
	}

	memoryDir := filepath.Join(repoRoot, ".memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", memoryDir, err)
	}

	// --- Step 2: Read existing .gitignore or write a fresh file if absent ---
	gitignorePath := filepath.Join(memoryDir, ".gitignore")
	required := []string{".cache/", ".enrolled", "local/"}

	bGit, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			content := strings.Join(required, "\n") + "\n"
			if wErr := os.WriteFile(gitignorePath, []byte(content), 0644); wErr != nil {
				return fmt.Errorf("creating gitignore in %s: %w", memoryDir, wErr)
			}
			return nil
		}
		return fmt.Errorf("reading gitignore in %s: %w", memoryDir, err)
	}

	// --- Step 3: Check which required entries are missing ---
	existingContent := string(bGit)
	lines := strings.Split(strings.ReplaceAll(existingContent, "\r\n", "\n"), "\n")
	hasEntry := func(entry string) bool {
		cleanEntry := strings.TrimSuffix(entry, "/")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == entry || strings.TrimSuffix(trimmed, "/") == cleanEntry {
				return true
			}
		}
		return false
	}

	var missing []string
	for _, req := range required {
		if !hasEntry(req) {
			missing = append(missing, req)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	// --- Step 4: Append missing entries while preserving existing content ---
	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("updating gitignore in %s: %w", memoryDir, err)
	}
	defer f.Close()

	if len(bGit) > 0 && bGit[len(bGit)-1] != '\n' {
		if _, err := f.WriteString("\n"); err != nil {
			return fmt.Errorf("writing newline to gitignore in %s: %w", memoryDir, err)
		}
	}
	for _, entry := range missing {
		if _, err := f.WriteString(entry + "\n"); err != nil {
			return fmt.Errorf("writing %s to gitignore in %s: %w", entry, memoryDir, err)
		}
	}

	return nil
}

// EnsureStorageLayout creates the directory structures required for repo-memory storage.
// It creates the data directory layout for repoID in dataDir.
// If repoRoot is optionally provided as a 3rd parameter, it also initializes the .memory layout in repoRoot.
func EnsureStorageLayout(dataDir, repoID string, repoRoot ...string) error {
	if err := EnsureDataDirLayout(dataDir, repoID); err != nil {
		return err
	}

	if len(repoRoot) > 0 && repoRoot[0] != "" {
		if err := EnsureRepoLayout(repoRoot[0]); err != nil {
			return err
		}
	}

	return nil
}

// IndexPath returns the canonical SQLite index file path: <dataDir>/<repoID>/index.db.
// If dataDir is empty, DataDir() is used.
func IndexPath(dataDir, repoID string) string {
	if dataDir == "" {
		dataDir = DataDir()
	}
	return filepath.Join(dataDir, repoID, "index.db")
}
