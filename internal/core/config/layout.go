package config

import (
	"strings"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	if dataDir == "" {
		dataDir = DataDir()
	}
	if repoID == "" {
		return errors.New("repoID cannot be empty")
	}

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
// It also ensures a .gitignore exists ignoring .cache/.
// It is idempotent.
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
	if repoRoot == "" {
		repoRoot = "."
	}

	memoryDir := filepath.Join(repoRoot, ".memory")
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", memoryDir, err)
	}

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
// If repoRoot is optionally provided as a 3rd parameter, it also creates the .memory layout in repoRoot.
// It is idempotent.
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
