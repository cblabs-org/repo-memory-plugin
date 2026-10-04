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
	"pending",
	"alerts",
	"metrics",
	"logs",
	"state",
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

	gitignorePath := filepath.Join(memoryDir, ".gitignore")
	if bGit, err := os.ReadFile(gitignorePath); err == nil {
		if !strings.Contains(string(bGit), ".enrolled") {
			f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0644)
			if err == nil {
				if len(bGit) > 0 && bGit[len(bGit)-1] != '\n' {
					f.WriteString("\n")
				}
				f.WriteString(".enrolled\n")
				f.Close()
			}
		}
	} else if os.IsNotExist(err) {
		content := []byte(".cache/\n.enrolled\n")
		if err := os.WriteFile(gitignorePath, content, 0644); err != nil {
			return fmt.Errorf("creating gitignore in %s: %w", memoryDir, err)
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
