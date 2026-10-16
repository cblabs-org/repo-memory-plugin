package gitops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func TestEnsureAndPublishToMemoryWorktree(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "test-repo-worktree"

	runGitCmd(t, repoRoot, "init", "-b", "main")
	runGitCmd(t, repoRoot, "config", "user.name", "Test User")
	runGitCmd(t, repoRoot, "config", "user.email", "test@example.com")

	mainFile := filepath.Join(repoRoot, "main.go")
	if err := os.WriteFile(mainFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}
	runGitCmd(t, repoRoot, "add", "main.go")
	runGitCmd(t, repoRoot, "commit", "-m", "initial commit")

	cloneID := CloneID(repoRoot)
	if len(cloneID) != 12 {
		t.Fatalf("expected 12-char CloneID, got %q (len %d)", cloneID, len(cloneID))
	}
	if CloneID(repoRoot) != cloneID {
		t.Fatalf("CloneID must be deterministic")
	}

	// Empty batch publish should be a no-op without creating a commit.
	emptyRes, err := PublishToMemoryWorktree(ctx, dataDir, repoID, repoRoot, nil)
	if err != nil {
		t.Fatalf("PublishToMemoryWorktree(nil) failed: %v", err)
	}
	if emptyRes.Committed || emptyRes.Branch != MemoryPendingBranch {
		t.Fatalf("unexpected empty publish result: %+v", emptyRes)
	}

	// Pre-populate an existing shared op batch in repoRoot/.memory/ops to verify sync into worktree.
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)
	existingBatch := ops.NewBatchWithTime([]ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-01J00000000000000000000001",
			DossierScope: "core/main",
			Timestamp:    now,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "invariant",
				"entry_type": "invariant",
				"purpose":    "Core main invariants.",
				"text":       "**Main package must compile**\nWhy: entry point.\nDo: run `go build ./...`.",
				"anchors":    []string{"main.go"},
				"evidence":   []string{"note-01"},
			},
		},
	}, now)
	repoOpsDir := filepath.Join(repoRoot, ".memory", "ops")
	if err := ops.NewWriter(repoOpsDir).Write(existingBatch); err != nil {
		t.Fatalf("write existing batch in repoRoot: %v", err)
	}

	// Publish a new batch to the memory worktree.
	newNow := now.Add(time.Minute)
	newBatch := ops.NewBatchWithTime([]ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "got-01J00000000000000000000002",
			DossierScope: "core/gitops",
			Timestamp:    newNow,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "gotcha",
				"entry_type": "gotcha",
				"text":       "**Worktree commits stay off main**\nWhy: avoid dirtying checkout.\nDo: publish to memory/pending.",
				"anchors":    []string{"main.go"},
				"evidence":   []string{"note-02"},
			},
		},
	}, newNow)

	pubRes, err := PublishToMemoryWorktree(ctx, dataDir, repoID, repoRoot, []ops.OpBatch{newBatch})
	if err != nil {
		t.Fatalf("PublishToMemoryWorktree failed: %v", err)
	}
	if !pubRes.Committed {
		t.Fatalf("expected Committed=true on first publish, got %+v", pubRes)
	}
	if pubRes.Branch != MemoryPendingBranch {
		t.Fatalf("expected Branch=%q, got %q", MemoryPendingBranch, pubRes.Branch)
	}
	if len(pubRes.CommitSHA) != 40 {
		t.Fatalf("expected 40-char CommitSHA, got %q", pubRes.CommitSHA)
	}

	wantWorktreePath := filepath.Join(dataDir, repoID, "worktrees", cloneID)
	if pubRes.WorktreePath != wantWorktreePath {
		t.Fatalf("WorktreePath = %q, want %q", pubRes.WorktreePath, wantWorktreePath)
	}

	// Verify worktree branch is memory/pending.
	wtBranch, err := GetCurrentBranch(pubRes.WorktreePath)
	if err != nil {
		t.Fatalf("GetCurrentBranch(worktree): %v", err)
	}
	if wtBranch != MemoryPendingBranch {
		t.Fatalf("worktree branch = %q, want %q", wtBranch, MemoryPendingBranch)
	}

	// Verify both synced and new dossiers + ATLAS.md exist in the worktree.
	for _, rel := range []string{
		filepath.Join(".memory", "ATLAS.md"),
		filepath.Join(".memory", "dossiers", "core", "main.md"),
		filepath.Join(".memory", "dossiers", "core", "gitops.md"),
	} {
		p := filepath.Join(pubRes.WorktreePath, rel)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist in worktree: %v", p, err)
		}
	}

	// Verify idempotency: publishing the same batch again produces no new commit.
	pubRes2, err := PublishToMemoryWorktree(ctx, dataDir, repoID, repoRoot, []ops.OpBatch{newBatch})
	if err != nil {
		t.Fatalf("second PublishToMemoryWorktree failed: %v", err)
	}
	if pubRes2.Committed {
		t.Fatalf("expected Committed=false on idempotent re-publish, got true")
	}
	if pubRes2.CommitSHA != pubRes.CommitSHA {
		t.Fatalf("expected CommitSHA to remain %q, got %q", pubRes.CommitSHA, pubRes2.CommitSHA)
	}

	// Verify primary repoRoot active branch and staged index were untouched.
	primaryBranch, err := GetCurrentBranch(repoRoot)
	if err != nil {
		t.Fatalf("GetCurrentBranch(repoRoot): %v", err)
	}
	if primaryBranch != "main" {
		t.Fatalf("primary repoRoot branch changed to %q, want 'main'", primaryBranch)
	}

	stagedOut, err := exec.Command("git", "-C", repoRoot, "diff", "--cached", "--name-only").Output()
	if err != nil {
		t.Fatalf("git diff --cached in repoRoot: %v", err)
	}
	if strings.TrimSpace(string(stagedOut)) != "" {
		t.Fatalf("expected primary repoRoot staged index to be empty, got: %s", string(stagedOut))
	}
}
