package ops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Reader reads and validates OpBatch files from the ops log directory.
type Reader struct {
	baseDir string
	gitDir  string
}

// NewReader creates a new Reader for the specified base directory.
func NewReader(baseDir string) *Reader {
	return &Reader{
		baseDir: baseDir,
	}
}

// WithGitDir configures the git directory used for resolving commit timestamps.
func (r *Reader) WithGitDir(gitDir string) *Reader {
	r.gitDir = gitDir
	return r
}

// ReadBatch reads and validates a single batch file from path.
// If path is not absolute and does not exist relative to the working directory,
// it is checked relative to r.baseDir.
func (r *Reader) ReadBatch(path string) (*OpBatch, error) {
	targetPath := path
	if !filepath.IsAbs(targetPath) {
		if _, err := os.Stat(targetPath); errors.Is(err, os.ErrNotExist) && r.baseDir != "" {
			targetPath = filepath.Join(r.baseDir, path)
		}
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("read batch file: %w", err)
	}

	var batch OpBatch
	if err := json.Unmarshal(data, &batch); err != nil {
		return nil, fmt.Errorf("unmarshal batch json: %w", err)
	}

	if err := ValidateBatch(batch); err != nil {
		return nil, fmt.Errorf("invalid batch in %s: %w", targetPath, err)
	}

	return &batch, nil
}
