package gitops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

// MemoryPendingBranch is the dedicated git branch used for Two-Lane worktree publishing.
const MemoryPendingBranch = "memory/pending"

// WorktreePublishResult holds the outcome of publishing op batches and rendered dossiers
// into the dedicated memory/pending git worktree.
type WorktreePublishResult struct {
	WorktreePath string `json:"worktree_path"`
	Branch       string `json:"branch"`
	CommitSHA    string `json:"commit_sha,omitempty"`
	Committed    bool   `json:"committed"`
}

// CloneID computes a deterministic 12-hex-character SHA-256 prefix of filepath.Clean(repoRoot).
func CloneID(repoRoot string) string {
	clean := filepath.Clean(strings.TrimSpace(repoRoot))
	sum := sha256.Sum256([]byte(clean))
	return hex.EncodeToString(sum[:])[:12]
}

// EnsureMemoryWorktree ensures a dedicated git worktree checked out on MemoryPendingBranch ("memory/pending")
// exists at <dataDir>/<repoID>/worktrees/<CloneID(repoRoot)> without touching repoRoot's active checkout.
func EnsureMemoryWorktree(ctx context.Context, dataDir, repoID, repoRoot string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(dataDir) == "" {
		return "", errors.New("dataDir cannot be empty")
	}
	if strings.TrimSpace(repoID) == "" {
		return "", errors.New("repoID cannot be empty")
	}
	if strings.TrimSpace(repoRoot) == "" {
		return "", errors.New("repoRoot cannot be empty")
	}

	verifyCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "HEAD")
	if out, err := verifyCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("verifying git repository HEAD in %s: %w (%s)", repoRoot, err, strings.TrimSpace(string(out)))
	}

	worktreePath := filepath.Join(dataDir, repoID, "worktrees", CloneID(repoRoot))
	gitMarker := filepath.Join(worktreePath, ".git")
	if _, err := os.Stat(gitMarker); err == nil {
		checkCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-parse", "--is-inside-work-tree")
		if out, cErr := checkCmd.Output(); cErr == nil && strings.TrimSpace(string(out)) == "true" {
			return worktreePath, nil
		}
	}

	if err := os.RemoveAll(worktreePath); err != nil {
		return "", fmt.Errorf("cleaning stale worktree directory %s: %w", worktreePath, err)
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return "", fmt.Errorf("creating worktrees parent directory: %w", err)
	}

	pruneCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "prune")
	_ = pruneCmd.Run()

	addCmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "worktree", "add", "-B", MemoryPendingBranch, worktreePath, "HEAD")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("creating memory worktree at %s: %w (%s)", worktreePath, err, strings.TrimSpace(string(out)))
	}

	return worktreePath, nil
}

// PublishToMemoryWorktree writes shared repo: op batches into the dedicated memory/pending worktree,
// syncs any existing shared ops from repoRoot, re-projects dossiers and ATLAS.md inside the worktree,
// and commits any changes onto memory/pending without touching the user's primary working tree.
func PublishToMemoryWorktree(ctx context.Context, dataDir, repoID, repoRoot string, batches []ops.OpBatch) (*WorktreePublishResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(batches) == 0 {
		return &WorktreePublishResult{
			Branch:    MemoryPendingBranch,
			Committed: false,
		}, nil
	}

	worktreePath, err := EnsureMemoryWorktree(ctx, dataDir, repoID, repoRoot)
	if err != nil {
		return nil, err
	}

	wtMemoryDir := filepath.Join(worktreePath, ".memory")
	wtOpsDir := filepath.Join(wtMemoryDir, "ops")
	if err := os.MkdirAll(wtOpsDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating worktree ops directory %s: %w", wtOpsDir, err)
	}

	// Sync any existing shared ops from repoRoot/.memory/ops into worktree/.memory/ops if not already present.
	repoOpsDir := filepath.Join(repoRoot, ".memory", "ops")
	if err := syncExistingOps(repoOpsDir, wtOpsDir); err != nil {
		return nil, fmt.Errorf("syncing existing shared ops to worktree: %w", err)
	}

	writer := ops.NewWriter(wtOpsDir)
	for _, batch := range batches {
		if err := writer.Write(batch); err != nil {
			return nil, fmt.Errorf("writing op batch %s to worktree: %w", batch.BatchID, err)
		}
	}

	projected, err := dossiers.NewProjector(ops.NewReader(wtOpsDir)).Project(ctx)
	if err != nil {
		return nil, fmt.Errorf("projecting dossiers in worktree %s: %w", wtOpsDir, err)
	}
	for scope, d := range projected {
		if d == nil {
			continue
		}
		if d.Scope == "" {
			d.Scope = scope
		}
		if strings.TrimSpace(d.Purpose) == "" && len(d.Entries) > 0 {
			d.Purpose = fmt.Sprintf("Curated repository memory for %s.", d.Scope)
		}
	}

	if err := dossiers.NewWriter(wtMemoryDir).WriteAll(projected); err != nil {
		return nil, fmt.Errorf("writing projected dossiers in worktree %s: %w", wtMemoryDir, err)
	}

	addCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "add", "--", ".memory")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("staging .memory in worktree %s: %w (%s)", worktreePath, err, strings.TrimSpace(string(out)))
	}

	statusCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "status", "--porcelain", "--", ".memory")
	statusOut, err := statusCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("checking worktree git status in %s: %w", worktreePath, err)
	}

	res := &WorktreePublishResult{
		WorktreePath: worktreePath,
		Branch:       MemoryPendingBranch,
	}

	if strings.TrimSpace(string(statusOut)) != "" {
		commitCmd := exec.CommandContext(
			ctx,
			"git", "-C", worktreePath,
			"-c", "user.name=repo-memory",
			"-c", "user.email=repo-memory@local",
			"commit", "-m", "chore(memory): consolidate op batch",
		)
		if out, err := commitCmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("committing memory batch in worktree %s: %w (%s)", worktreePath, err, strings.TrimSpace(string(out)))
		}
		res.Committed = true
	}

	revCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-parse", "HEAD")
	if revOut, err := revCmd.Output(); err == nil {
		res.CommitSHA = strings.TrimSpace(string(revOut))
	}

	return res, nil
}

func syncExistingOps(srcOpsDir, dstOpsDir string) error {
	info, err := os.Stat(srcOpsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	return filepath.WalkDir(srcOpsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		rel, err := filepath.Rel(srcOpsDir, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dstOpsDir, rel)
		if _, err := os.Stat(dstPath); err == nil {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0o644)
	})
}
