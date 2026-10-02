package claudecode

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

func init() {
	tick.RegisterCopier(CopyTranscripts)
}

// CopyTranscripts scans ~/.claude/projects/ for enrolled repositories and copies their transcripts
// raw (without parsing) into <data-dir>/<repo-id>/raw/claude-code/, respecting read-only source permissions.
func CopyTranscripts(ctx context.Context, memoryDataDir string, enrolledRepos map[string]config.RegistryEntry) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting user home dir: %w", err)
	}

	claudeProjectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(claudeProjectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No Claude projects dir yet, not an error
		}
		return fmt.Errorf("reading claude projects dir %s: %w", claudeProjectsDir, err)
	}

	// Map sanitized project directory names to enrolled repo IDs and roots
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projName := entry.Name()
		projPath := filepath.Join(claudeProjectsDir, projName)

		// Check which enrolled repo matches this project directory
		var matchedRepoID string
		for _, repo := range enrolledRepos {
			expectedProjName := projectDirNameForRoot(repo.RootPath)
			if projName == expectedProjName {
				matchedRepoID = repo.RepoID
				break
			}
		}

		if matchedRepoID == "" {
			continue
		}

		destRawDir := filepath.Join(memoryDataDir, matchedRepoID, "raw", "claude-code")

		// Walk project directory to copy all .jsonl transcript files (including subagents/*.jsonl),
		// skipping memory/ and respecting read-only source permissions.
		err = filepath.Walk(projPath, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil // skip inaccessible files
			}
			if info.IsDir() {
				if filepath.Base(path) == "memory" && filepath.Dir(path) == projPath {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".jsonl" {
				return nil
			}

			rel, err := filepath.Rel(projPath, path)
			if err != nil {
				return nil
			}

			dstFilePath := filepath.Join(destRawDir, rel)

			// Copy file if source changed or destination doesn't exist
			if shouldCopy(path, dstFilePath) {
				if err := copyRawFile(path, dstFilePath); err != nil {
					fmt.Fprintf(os.Stderr, "error copying claude transcript %s: %v\n", path, err)
				}
			}

			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error walking claude project dir %s: %v\n", projPath, err)
		}
	}

	return nil
}

func projectDirNameForRoot(rootPath string) string {
	cleaned := filepath.Clean(rootPath)
	sanitized := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' {
			return '-'
		}
		return r
	}, cleaned)
	if !strings.HasPrefix(sanitized, "-") {
		sanitized = "-" + sanitized
	}
	return sanitized
}

func shouldCopy(src, dst string) bool {
	srcFi, err := os.Stat(src)
	if err != nil {
		return false
	}
	dstFi, err := os.Stat(dst)
	if err != nil {
		return true // dst doesn't exist
	}
	// Copy if source is larger or modified after destination
	return srcFi.Size() != dstFi.Size() || srcFi.ModTime().After(dstFi.ModTime())
}

func copyRawFile(src, dst string) error {
	srcFile, err := os.Open(src) // Read only
	if err != nil {
		return fmt.Errorf("opening source file: %w", err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("opening destination file: %w", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying file content: %w", err)
	}

	return dstFile.Sync()
}
