/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/gitops/worktree.go
 * Description: Manages the dedicated Two-Lane `memory/pending` git worktree for
 *              publishing shared op batches and projected dossiers without
 *              touching the user's primary working tree.
 *
 * Exported Types / Functions:
 * - MemoryPendingBranch:      Dedicated git branch name ("memory/pending") for Two-Lane publishing.
 * - WorktreePublishResult:    Outcome of publishing op batches and dossiers into the memory worktree.
 * - CloneID():                Computes a deterministic 12-hex-char SHA-256 prefix for a repoRoot path.
 * - EnsureMemoryWorktree():   Ensures the dedicated `memory/pending` git worktree exists and is valid.
 * - PublishToMemoryWorktree(): Writes op batches, re-projects dossiers, and commits onto `memory/pending`.
 */

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
	WorktreePath string `json:"worktree_path"`        // Absolute filesystem path to the managed memory worktree.
	Branch       string `json:"branch"`               // Git branch checked out in the worktree (MemoryPendingBranch).
	CommitSHA    string `json:"commit_sha,omitempty"` // HEAD commit SHA in the worktree after publishing.
	Committed    bool   `json:"committed"`            // True if a new commit was created during this publish pass.
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
	// --- Step 1: Validate inputs and verify that repoRoot has a valid git HEAD ---
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

	// --- Step 2: Reuse existing worktree if already initialized and valid ---
	worktreePath := filepath.Join(dataDir, repoID, "worktrees", CloneID(repoRoot))
	gitMarker := filepath.Join(worktreePath, ".git")
	if _, err := os.Stat(gitMarker); err == nil {
		checkCmd := exec.CommandContext(ctx, "git", "-C", worktreePath, "rev-parse", "--is-inside-work-tree")
		if out, cErr := checkCmd.Output(); cErr == nil && strings.TrimSpace(string(out)) == "true" {
			return worktreePath, nil
		}
	}

	// --- Step 3: Clean any stale directory, prune worktrees, and add the memory/pending worktree ---
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
	// --- Step 1: Handle empty batch list and ensure the memory/pending worktree exists ---
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

	// --- Step 2: Sync existing shared ops from repoRoot and write new op batches ---
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

	// --- Step 3: Project and render all dossiers and ATLAS.md inside the worktree ---
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

	// --- Step 4: Stage .memory changes and commit onto memory/pending if modified ---
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

// syncExistingOps copies any missing JSON op batch files from srcOpsDir into dstOpsDir.
func syncExistingOps(srcOpsDir, dstOpsDir string) error {
	// --- Step 1: Return early if the source ops directory does not exist ---
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

	// --- Step 2: Walk source JSON batch files and copy any missing files into dstOpsDir ---
	return filepath.WalkDir(srcOpsDir, func(filePath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		rel, err := filepath.Rel(srcOpsDir, filePath)
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
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0o644)
	})
}
