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
const MaxArtifactSize = 1024 * 1024 // 1 MB

// ArtifactDir returns the destination directory path for conversation artifacts:
// <data-dir>/<repo-id>/evidence/artifacts/<conversation-id>/
func ArtifactDir(dataDir, repoID, conversationID string) string {
	if dataDir == "" {
		dataDir = config.DataDir()
	}
	return filepath.Join(dataDir, repoID, "evidence", "artifacts", conversationID)
}

// CopyArtifacts copies .md files and .metadata.json from srcDir to dstDir,
// skipping .system_generated/ and files > 1 MB.
// Returns the count of files copied.
// Idempotent: if destination file exists with the same size, it is skipped.
func CopyArtifacts(srcDir, dstDir string) (int, error) {
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
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".metadata.json") {
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

		if dstInfo, err := os.Stat(targetPath); err == nil {
			if dstInfo.Size() == info.Size() {
				return nil
			}
		}

		if err := copyFile(path, targetPath); err != nil {
			return fmt.Errorf("copying artifact %s to %s: %w", path, targetPath, err)
		}

		copiedCount++
		return nil
	})

	if err != nil {
		return copiedCount, err
	}

	return copiedCount, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	dir := filepath.Dir(dst)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

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

	if runtime.GOOS == "windows" {
		_ = os.Remove(dst)
	}

	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName)
		return err
	}

	return nil
}
