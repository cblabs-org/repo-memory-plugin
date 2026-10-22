/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package tick

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runGitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test Author",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test Author",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func setupBackfillTestRepo(t *testing.T) (repoRoot string, dataDir string, repoID string) {
	t.Helper()
	repoRoot = t.TempDir()
	dataDir = t.TempDir()
	repoID = "repo-backfill-test"

	runGitForTest(t, repoRoot, "init")

	// Create a doc file under docs/ for default doc seeding globs (docs/**/*.md)
	docsDir := filepath.Join(repoRoot, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	docContent := "# Architecture\n\n## Auth Invariants\n- **Token Verification:** Always call `Verify()` in `internal/auth/token.go` before granting session access.\n"
	if err := os.WriteFile(filepath.Join(docsDir, "architecture.md"), []byte(docContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a PA workspace artifact
	paDir := filepath.Join(repoRoot, ".agents", "skills", "principal-architect-workspace", "pr-1-core", "wo1-init")
	if err := os.MkdirAll(paDir, 0o755); err != nil {
		t.Fatal(err)
	}
	woContent := "# WORKORDER: WO1\n\n## Summary\nImplemented initial storage layer.\n"
	if err := os.WriteFile(filepath.Join(paDir, "WORKORDER.md"), []byte(woContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a source file, commit it, and then commit a revert of that change
	pkgDir := filepath.Join(repoRoot, "internal", "auth")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	authFile := filepath.Join(pkgDir, "token.go")
	if err := os.WriteFile(authFile, []byte("package auth\n\nfunc Verify() bool { return false }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	runGitForTest(t, repoRoot, "add", ".")
	runGitForTest(t, repoRoot, "commit", "-m", "feat(auth): disable token check")

	if err := os.WriteFile(authFile, []byte("package auth\n\nfunc Verify() bool { return true }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitForTest(t, repoRoot, "add", ".")
	runGitForTest(t, repoRoot, "commit", "-m", `Revert "feat(auth): disable token check"`)

	// Also place an uncovered flat episode in dataDir
	epDir := filepath.Join(dataDir, repoID, "evidence", "episodes")
	if err := os.MkdirAll(epDir, 0o755); err != nil {
		t.Fatal(err)
	}
	epJSON := `{"session_id":"ep-hist-1","note_ids":[],"processed":false}`
	if err := os.WriteFile(filepath.Join(epDir, "ep-hist-1.json"), []byte(epJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	return repoRoot, dataDir, repoID
}

func TestRunBackfill_DryRunAndExecutionAndSpendGuards(t *testing.T) {
	ctx := context.Background()
	repoRoot, dataDir, repoID := setupBackfillTestRepo(t)

	// 1. Dry-run preview across PA files, doc seeds, and git revert commit without mutating disk
	dryRes, err := RunBackfill(ctx, BackfillOptions{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
		RepoID:   repoID,
		DryRun:   true,
	})
	if err != nil {
		t.Fatalf("DryRun RunBackfill failed: %v", err)
	}
	if !dryRes.DryRun {
		t.Errorf("expected DryRun=true")
	}
	if !dryRes.SpendGuardAllowed {
		t.Errorf("expected SpendGuardAllowed=true on initial dry-run, got reason=%q", dryRes.SpendGuardReason)
	}
	if dryRes.PAFilesImported != 1 {
		t.Errorf("expected PAFilesImported=1 in dry-run, got %d", dryRes.PAFilesImported)
	}
	if dryRes.EpisodesQueued != 1 {
		t.Errorf("expected EpisodesQueued=1 in dry-run, got %d", dryRes.EpisodesQueued)
	}
	if dryRes.DocsSeeded < 1 {
		t.Errorf("expected DocsSeeded >= 1 in dry-run, got %d", dryRes.DocsSeeded)
	}
	if dryRes.GitRevertsScanned != 1 || dryRes.GitNotesCreated != 1 {
		t.Errorf("expected 1 git revert scanned & created in dry-run, got scanned=%d created=%d",
			dryRes.GitRevertsScanned, dryRes.GitNotesCreated)
	}

	// Verify dry-run did not write notes, ops, or spend_guards.json
	if _, err := os.Stat(filepath.Join(dataDir, repoID, "state", "spend_guards.json")); err == nil {
		t.Errorf("dry-run should not write spend_guards.json")
	}
	if _, err := os.Stat(filepath.Join(dataDir, repoID, "evidence", "notes")); err == nil {
		t.Errorf("dry-run should not create evidence/notes directory")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".memory", "ops")); err == nil {
		t.Errorf("dry-run should not create .memory/ops directory")
	}

	// 2. Real execution
	res1, err := RunBackfill(ctx, BackfillOptions{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
		RepoID:   repoID,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("Real RunBackfill failed: %v", err)
	}
	if !res1.SpendGuardAllowed {
		t.Fatalf("expected SpendGuardAllowed=true on first real run")
	}
	if res1.PAFilesImported != 1 {
		t.Errorf("expected PAFilesImported=1, got %d", res1.PAFilesImported)
	}
	if res1.DocsSeeded < 1 {
		t.Errorf("expected DocsSeeded >= 1, got %d", res1.DocsSeeded)
	}
	if res1.GitNotesCreated != 1 || len(res1.CreatedNoteIDs) != 1 {
		t.Fatalf("expected 1 git revert note created, got %d (%v)", res1.GitNotesCreated, res1.CreatedNoteIDs)
	}

	notePath := filepath.Join(dataDir, repoID, "evidence", "notes", res1.CreatedNoteIDs[0]+".json")
	noteBytes, err := os.ReadFile(notePath)
	if err != nil {
		t.Fatalf("expected git revert note on disk at %s: %v", notePath, err)
	}
	noteContent := string(noteBytes)
	if !strings.Contains(noteContent, `"kind": "gotcha"`) || !strings.Contains(noteContent, `"source": "backfill-git"`) {
		t.Errorf("unexpected git revert note content:\n%s", noteContent)
	}
	if !strings.Contains(noteContent, `"scope": "internal/auth"`) {
		t.Errorf("expected inferred scope internal/auth in note content:\n%s", noteContent)
	}

	if _, err := os.Stat(filepath.Join(dataDir, repoID, "state", "spend_guards.json")); err != nil {
		t.Errorf("expected spend_guards.json to be written after real backfill: %v", err)
	}

	// 3. Immediate second run without --force should be throttled by A5 spend guard
	res2, err := RunBackfill(ctx, BackfillOptions{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
		RepoID:   repoID,
		DryRun:   false,
		Force:    false,
	})
	if err != nil {
		t.Fatalf("Second RunBackfill failed: %v", err)
	}
	if res2.SpendGuardAllowed {
		t.Errorf("expected second RunBackfill to be throttled by spend guard")
	}
	if res2.SpendGuardReason != "min_interval not elapsed" {
		t.Errorf("expected SpendGuardReason='min_interval not elapsed', got %q", res2.SpendGuardReason)
	}
	if res2.ItemsProcessed != 0 {
		t.Errorf("expected 0 items processed when throttled, got %d", res2.ItemsProcessed)
	}

	// 4. Third run with Force=true should bypass spend guard and prove idempotency (0 duplicate notes/imports)
	res3, err := RunBackfill(ctx, BackfillOptions{
		RepoRoot: repoRoot,
		DataDir:  dataDir,
		RepoID:   repoID,
		DryRun:   false,
		Force:    true,
	})
	if err != nil {
		t.Fatalf("Forced RunBackfill failed: %v", err)
	}
	if !res3.SpendGuardAllowed {
		t.Errorf("expected SpendGuardAllowed=true with Force=true")
	}
	if res3.PAFilesImported != 0 {
		t.Errorf("expected 0 duplicate PA files imported on forced re-run, got %d", res3.PAFilesImported)
	}
	if res3.EpisodesQueued != 0 {
		t.Errorf("expected 0 duplicate episodes queued on forced re-run, got %d", res3.EpisodesQueued)
	}
	if res3.DocsSeeded != 0 {
		t.Errorf("expected 0 unchanged docs re-seeded on forced re-run, got %d", res3.DocsSeeded)
	}
	if res3.GitNotesCreated != 0 {
		t.Errorf("expected 0 duplicate git revert notes created on forced re-run, got %d", res3.GitNotesCreated)
	}
}
