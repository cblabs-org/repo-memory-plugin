package consolidate

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func runGitCmdForPublishTest(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed (%v): %s", args, err, string(out))
	}
}

func TestEnsureMemoryGitignore(t *testing.T) {
	repoRoot := t.TempDir()

	// 1. Fresh creation when .memory/.gitignore does not exist.
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		t.Fatalf("EnsureMemoryGitignore on fresh dir failed: %v", err)
	}
	gitignorePath := filepath.Join(repoRoot, ".memory", ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	content := string(data)
	for _, want := range []string{".cache/", ".enrolled", "local/"} {
		if !strings.Contains(content, want) {
			t.Errorf("expected .gitignore to contain %q, got %q", want, content)
		}
	}

	// 2. Idempotency: calling again does not duplicate lines.
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		t.Fatalf("second EnsureMemoryGitignore failed: %v", err)
	}
	data2, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore second time: %v", err)
	}
	if string(data2) != content {
		t.Errorf("expected idempotent .gitignore content %q, got %q", content, string(data2))
	}

	// 3. Preserves custom existing lines while appending missing required entries.
	customContent := "# custom comment\n.cache/\ncustom-ignore/\n"
	if err := os.WriteFile(gitignorePath, []byte(customContent), 0o644); err != nil {
		t.Fatalf("write custom .gitignore: %v", err)
	}
	if err := EnsureMemoryGitignore(repoRoot); err != nil {
		t.Fatalf("EnsureMemoryGitignore on custom .gitignore failed: %v", err)
	}
	data3, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("read .gitignore third time: %v", err)
	}
	content3 := string(data3)
	for _, want := range []string{"# custom comment", "custom-ignore/", ".cache/", ".enrolled", "local/"} {
		if !strings.Contains(content3, want) {
			t.Errorf("expected updated .gitignore to contain %q, got %q", want, content3)
		}
	}
	if strings.Count(content3, ".cache") != 1 {
		t.Errorf("expected .cache to appear once, got %d in %q", strings.Count(content3, ".cache"), content3)
	}
}

func TestFiveTierIsolation_UserBranchRepo(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-five-tier"

	// 1. Record a Tier 3 (user:) op batch.
	userOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "got-01J00000000000000000000011",
			DossierScope: "preferences/serial-port",
			Timestamp:    now,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "gotcha",
				"entry_type": "gotcha",
				"purpose":    "Developer workstation serial preferences.",
				"text":       "**Local USB serial port mapping**\nWhy: workstation-specific.\nDo: use /dev/ttyUSB0 locally.",
				"evidence":   []string{"note-user-1"},
			},
		},
	}
	userRes, err := RecordUserTier(ctx, dataDir, repoID, userOps, nil, nil, now)
	if err != nil {
		t.Fatalf("RecordUserTier failed: %v", err)
	}
	if userRes.Batch == nil {
		t.Fatalf("expected non-nil user batch")
	}

	// Verify user tier wrote to <dataDir>/<repoID>/user-dossiers and NOT to repoRoot/.memory.
	userDossierFile := filepath.Join(UserTierBaseDir(dataDir, repoID), "dossiers", "preferences", "serial-port.md")
	if _, err := os.Stat(userDossierFile); err != nil {
		t.Fatalf("expected user dossier at %s: %v", userDossierFile, err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".memory", "ops")); !os.IsNotExist(err) {
		t.Fatalf("expected repoRoot/.memory/ops not to exist after user tier write")
	}

	// 2. Record a Tier 4 (branch:) op batch for branch "feat/new-thing".
	branchOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-01J00000000000000000000022",
			DossierScope: "core/newthing",
			Timestamp:    now,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "invariant",
				"entry_type": "invariant",
				"purpose":    "NewThing feature branch invariants.",
				"text":       "**NewThing requires non-empty ID**\nWhy: prevent empty keys.\nDo: validate ID before save.",
				"anchors":    []string{"internal/newthing/service.go"},
				"evidence":   []string{"note-branch-1"},
			},
		},
	}
	branchRes, err := RecordBranchTier(ctx, repoRoot, "feat/new-thing", branchOps, nil, nil, now)
	if err != nil {
		t.Fatalf("RecordBranchTier failed: %v", err)
	}
	if branchRes.Batch == nil {
		t.Fatalf("expected non-nil branch batch")
	}

	// Verify branch tier wrote to .memory/local/{ops,dossiers}/feat-new-thing and NOT to .memory/ops.
	branchBatchPath := filepath.Join(BranchTierOpsDir(repoRoot, "feat/new-thing"), now.Format("2006-01"), branchRes.Batch.BatchID+".json")
	if _, err := os.Stat(branchBatchPath); err != nil {
		t.Fatalf("expected branch batch file at %s: %v", branchBatchPath, err)
	}
	branchDirectDossier := filepath.Join(BranchTierDossiersDir(repoRoot, "feat/new-thing"), "core", "newthing.md")
	if _, err := os.Stat(branchDirectDossier); err != nil {
		t.Fatalf("expected branch dossier at %s: %v", branchDirectDossier, err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".memory", "ops")); !os.IsNotExist(err) {
		t.Fatalf("expected shared .memory/ops not to exist after branch tier write")
	}

	// 3. Record a Tier 5 (repo:) op batch.
	repoOps := []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "con-01J00000000000000000000033",
			DossierScope: "core/shared",
			Timestamp:    now,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "contract",
				"entry_type": "contract",
				"purpose":    "Shared repository contracts.",
				"text":       "**All batches use ULID filenames**\nWhy: conflict-free git merges.\nDo: write to .memory/ops/<yyyy-mm>/<ulid>.json.",
				"evidence":   []string{"note-repo-1"},
			},
		},
	}
	repoRes, err := RecordRepoTier(ctx, repoRoot, repoOps, nil, nil, now)
	if err != nil {
		t.Fatalf("RecordRepoTier failed: %v", err)
	}
	if repoRes.Batch == nil {
		t.Fatalf("expected non-nil repo batch")
	}
	sharedDossier := filepath.Join(repoRoot, ".memory", "dossiers", "core", "shared.md")
	if _, err := os.Stat(sharedDossier); err != nil {
		t.Fatalf("expected shared dossier at %s: %v", sharedDossier, err)
	}
}

func TestPromoteMergedBranchOps_AndMultiTierIndex(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 9, 30, 0, 0, time.UTC)

	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-promote-test"

	// Initialize git repository on main.
	runGitCmdForPublishTest(t, repoRoot, "init", "-b", "main")
	runGitCmdForPublishTest(t, repoRoot, "config", "user.name", "Test User")
	runGitCmdForPublishTest(t, repoRoot, "config", "user.email", "test@example.com")

	readmePath := filepath.Join(repoRoot, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Repo\n"), 0o644); err != nil {
		t.Fatalf("write README.md: %v", err)
	}
	runGitCmdForPublishTest(t, repoRoot, "add", "README.md")
	runGitCmdForPublishTest(t, repoRoot, "commit", "-m", "initial main commit")

	// Record a Tier 3 (user:) entry so we can verify multi-tier indexing across user, branch, and repo.
	userEntryID := "got-01J00000000000000000000101"
	_, err := RecordUserTier(ctx, dataDir, repoID, []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      userEntryID,
			DossierScope: "preferences/editor",
			Timestamp:    now,
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "gotcha",
				"entry_type": "gotcha",
				"purpose":    "Personal editor settings.",
				"text":       "**Personal debugger port**\nWhy: local setup.\nDo: bind to localhost:4321.",
				"evidence":   []string{"note-user-101"},
			},
		},
	}, nil, nil, now)
	if err != nil {
		t.Fatalf("RecordUserTier: %v", err)
	}

	// Switch to feature branch feat/new-thing and commit a branch-only file.
	runGitCmdForPublishTest(t, repoRoot, "checkout", "-b", "feat/new-thing")
	featFileRel := filepath.Join("internal", "newthing", "handler.go")
	featFileAbs := filepath.Join(repoRoot, featFileRel)
	if err := os.MkdirAll(filepath.Dir(featFileAbs), 0o755); err != nil {
		t.Fatalf("mkdir feature dir: %v", err)
	}
	if err := os.WriteFile(featFileAbs, []byte("package newthing\n\nfunc Handle() {}\n"), 0o644); err != nil {
		t.Fatalf("write feature file: %v", err)
	}
	runGitCmdForPublishTest(t, repoRoot, "add", featFileRel)
	runGitCmdForPublishTest(t, repoRoot, "commit", "-m", "feat: add newthing handler")

	// Record a Tier 4 (branch:) op batch anchored to internal/newthing/handler.go.
	branchEntryID := "inv-01J00000000000000000000202"
	branchRes, err := RecordBranchTier(ctx, repoRoot, "feat/new-thing", []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      branchEntryID,
			DossierScope: "newthing",
			Timestamp:    now.Add(time.Minute),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "invariant",
				"entry_type": "invariant",
				"purpose":    "Newthing module invariants.",
				"text":       "**Handle must be idempotent**\nWhy: retried on timeout.\nDo: check request ID before mutating state.",
				"anchors":    []string{"internal/newthing/handler.go#Handle"},
				"evidence":   []string{"note-branch-202"},
			},
		},
	}, nil, nil, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("RecordBranchTier: %v", err)
	}

	// Open SQLite index and verify RebuildMultiTierIndex on feat/new-thing includes both branch: and user: entries.
	idx, err := index.NewIndex(index.IndexPath(dataDir, repoID))
	if err != nil {
		t.Fatalf("NewIndex: %v", err)
	}
	defer idx.Close()

	if err := RebuildMultiTierIndex(ctx, idx, dataDir, repoID, repoRoot, "feat/new-thing"); err != nil {
		t.Fatalf("RebuildMultiTierIndex on feat/new-thing failed: %v", err)
	}
	if _, err := idx.GetEntry(ctx, branchEntryID); err != nil {
		t.Fatalf("expected branch entry %s in index while on feat/new-thing: %v", branchEntryID, err)
	}
	if _, err := idx.GetEntry(ctx, userEntryID); err != nil {
		t.Fatalf("expected user entry %s in index while on feat/new-thing: %v", userEntryID, err)
	}

	// While still on feat/new-thing (not on default branch), PromoteMergedBranchOps should not promote.
	prePromote, err := PromoteMergedBranchOps(ctx, repoRoot)
	if err != nil {
		t.Fatalf("PromoteMergedBranchOps on feature branch failed: %v", err)
	}
	if len(prePromote.PromotedBranches) != 0 {
		t.Fatalf("expected 0 promoted branches while checked out on feature branch, got %v", prePromote.PromotedBranches)
	}

	// Switch back to main and merge feat/new-thing.
	runGitCmdForPublishTest(t, repoRoot, "checkout", "main")
	runGitCmdForPublishTest(t, repoRoot, "merge", "--no-ff", "feat/new-thing", "-m", "Merge branch 'feat/new-thing'")

	// Run PromoteMergedBranchOps on main.
	promoteRes, err := PromoteMergedBranchOps(ctx, repoRoot)
	if err != nil {
		t.Fatalf("PromoteMergedBranchOps on main failed: %v", err)
	}
	if len(promoteRes.PromotedBranches) != 1 || promoteRes.PromotedBranches[0] != "feat-new-thing" {
		t.Fatalf("expected PromotedBranches=['feat-new-thing'], got %v", promoteRes.PromotedBranches)
	}
	if len(promoteRes.PromotedBatches) != 1 || promoteRes.PromotedBatches[0].BatchID != branchRes.Batch.BatchID {
		t.Fatalf("expected promoted batch %s, got %+v", branchRes.Batch.BatchID, promoteRes.PromotedBatches)
	}

	// 1. Verify the batch file moved to .memory/ops/<yyyy-mm>/<batch>.json.
	promotedBatchPath := filepath.Join(repoRoot, ".memory", "ops", now.Add(time.Minute).Format("2006-01"), branchRes.Batch.BatchID+".json")
	if _, err := os.Stat(promotedBatchPath); err != nil {
		t.Fatalf("expected promoted batch file at %s: %v", promotedBatchPath, err)
	}

	// 2. Verify .memory/local/ops/feat-new-thing and .memory/local/dossiers/feat-new-thing were removed.
	if _, err := os.Stat(BranchTierOpsDir(repoRoot, "feat/new-thing")); !os.IsNotExist(err) {
		t.Fatalf("expected .memory/local/ops/feat-new-thing to be removed, stat err=%v", err)
	}
	if _, err := os.Stat(BranchTierDossiersDir(repoRoot, "feat/new-thing")); !os.IsNotExist(err) {
		t.Fatalf("expected .memory/local/dossiers/feat-new-thing to be removed, stat err=%v", err)
	}

	// 3. Verify .memory/dossiers/ and .memory/ATLAS.md were updated with the promoted dossier.
	promotedDossierPath := filepath.Join(repoRoot, ".memory", "dossiers", "newthing.md")
	if _, err := os.Stat(promotedDossierPath); err != nil {
		t.Fatalf("expected promoted dossier at %s: %v", promotedDossierPath, err)
	}
	atlasBytes, err := os.ReadFile(filepath.Join(repoRoot, ".memory", "ATLAS.md"))
	if err != nil {
		t.Fatalf("read .memory/ATLAS.md: %v", err)
	}
	if !strings.Contains(string(atlasBytes), "newthing") {
		t.Fatalf("expected .memory/ATLAS.md to list promoted 'newthing' dossier, got:\n%s", string(atlasBytes))
	}

	// 4. Verify RebuildMultiTierIndex on main indexes both the promoted repo: entry and user: entries.
	if err := RebuildMultiTierIndex(ctx, idx, dataDir, repoID, repoRoot, "main"); err != nil {
		t.Fatalf("RebuildMultiTierIndex on main failed: %v", err)
	}
	if _, err := idx.GetEntry(ctx, branchEntryID); err != nil {
		t.Fatalf("expected promoted entry %s in index on main: %v", branchEntryID, err)
	}
	if _, err := idx.GetEntry(ctx, userEntryID); err != nil {
		t.Fatalf("expected user entry %s in index on main: %v", userEntryID, err)
	}
}
