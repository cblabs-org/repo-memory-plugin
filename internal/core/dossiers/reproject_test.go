package dossiers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
)

func seedOpsBatchForReprojectTest(t *testing.T, opsDir string) {
	t.Helper()
	w := ops.NewWriter(opsDir)

	ts := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	batch := ops.NewBatchWithTime([]ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "dec-01J00000000000000000000001",
			DossierScope: "internal/core/jobs",
			Timestamp:    ts,
			Source:       ops.SourceHuman,
			Fields: map[string]any{
				"entry_type": string(EntryTypeDecision),
				"text":       "Use SQLite WAL mode for local job queue\n- **Why:** Prevent writer contention between capture hook and tick worker\n- **Do:** Enable PRAGMA journal_mode=WAL on open",
				"verified":   "abc1234",
				"anchors":    []string{"internal/core/jobs/store.go"},
			},
		},
		{
			Type:         ops.OpAdd,
			EntryID:      "got-01J00000000000000000000002",
			DossierScope: "internal/core/dossiers",
			Timestamp:    ts.Add(time.Second),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"entry_type": string(EntryTypeGotcha),
				"text":       "Avoid mutating dossiers without op-log entry\n- **Why:** Reprojection replays ops from scratch\n- **Do:** Always emit an ops batch or reconcile hand-edits first",
				"verified":   "def5678",
				"anchors":    []string{"internal/core/dossiers/projector.go"},
			},
		},
	}, ts)
	if err := w.Write(batch); err != nil {
		t.Fatalf("Write batch failed: %v", err)
	}
}

func TestReproject_DryRunAndDeterministicIdempotency(t *testing.T) {
	ctx := context.Background()
	memDir := filepath.Join(t.TempDir(), ".memory")
	opsDir := filepath.Join(memDir, "ops")
	seedOpsBatchForReprojectTest(t, opsDir)

	// 1. Dry-run when no dossiers exist on disk yet
	dryRes, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("DryRun Reproject failed: %v", err)
	}
	if !dryRes.DryRun {
		t.Errorf("expected DryRun=true")
	}
	if dryRes.DossiersProjected != 2 || dryRes.ActiveEntries != 2 {
		t.Errorf("expected 2 dossiers and 2 active entries, got dossiers=%d entries=%d",
			dryRes.DossiersProjected, dryRes.ActiveEntries)
	}
	if len(dryRes.FilesAdded) != 3 {
		t.Errorf("expected 3 FilesAdded (2 dossiers + ATLAS.md) in dry-run, got %v", dryRes.FilesAdded)
	}
	if _, err := os.Stat(filepath.Join(memDir, "ATLAS.md")); err == nil {
		t.Errorf("dry-run should not write ATLAS.md to disk")
	}

	// 2. Real execution writes dossiers and ATLAS.md
	res1, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Real Reproject failed: %v", err)
	}
	if len(res1.FilesAdded) != 3 || len(res1.FilesUpdated) != 0 {
		t.Errorf("expected 3 FilesAdded and 0 FilesUpdated on first write, got added=%v updated=%v",
			res1.FilesAdded, res1.FilesUpdated)
	}

	// 3. Second real run is 100% deterministic and reports all files in FilesUnchanged
	res2, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("Second Reproject failed: %v", err)
	}
	if len(res2.FilesUnchanged) != 3 {
		t.Errorf("expected 3 FilesUnchanged on second run, got %v", res2.FilesUnchanged)
	}
	if len(res2.FilesAdded) != 0 || len(res2.FilesUpdated) != 0 || len(res2.FilesRemoved) != 0 {
		t.Errorf("expected 0 added/updated/removed on second run, got added=%v updated=%v removed=%v",
			res2.FilesAdded, res2.FilesUpdated, res2.FilesRemoved)
	}
	if res2.PreExistingEntries != 2 {
		t.Errorf("expected PreExistingEntries=2, got %d", res2.PreExistingEntries)
	}
	if res2.VerifiedPreserved != 2 {
		t.Errorf("expected VerifiedPreserved=2, got %d", res2.VerifiedPreserved)
	}
	if res2.HumanDecisionsPreserved != 1 {
		t.Errorf("expected HumanDecisionsPreserved=1, got %d", res2.HumanDecisionsPreserved)
	}
}

func TestReproject_HumanDecisionPreservationGuard(t *testing.T) {
	ctx := context.Background()
	memDir := filepath.Join(t.TempDir(), ".memory")
	opsDir := filepath.Join(memDir, "ops")
	seedOpsBatchForReprojectTest(t, opsDir)

	if _, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
	}); err != nil {
		t.Fatalf("initial Reproject failed: %v", err)
	}

	// Simulate an unrecorded hand-added decision inside .memory/dossiers/internal/core/jobs.md
	jobsDossierPath := filepath.Join(memDir, "dossiers", "internal", "core", "jobs.md")
	origBytes, err := os.ReadFile(jobsDossierPath)
	if err != nil {
		t.Fatal(err)
	}
	handAdded := string(origBytes) + "\n### [dec-01J99999999999999999999999] Never drop dead-letter jobs automatically\n`decision · kind: semantic · verified 999aaaa · source: human`\n- **Why:** Forensics requires inspecting failed payloads\n- **Do:** Keep dead jobs until explicit prune\n"
	if err := os.WriteFile(jobsDossierPath, []byte(handAdded), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reproject without AllowLoss must fail with ErrHumanDecisionLost and NOT overwrite the file
	resLoss, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
		AllowLoss: false,
	})
	if !errors.Is(err, ErrHumanDecisionLost) {
		t.Fatalf("expected ErrHumanDecisionLost, got: %v", err)
	}
	if resLoss == nil || len(resLoss.LostEntries) != 1 {
		t.Fatalf("expected 1 LostEntryViolation, got: %+v", resLoss)
	}
	if resLoss.LostEntries[0].EntryID != "dec-01J99999999999999999999999" {
		t.Errorf("unexpected lost entry ID: %+v", resLoss.LostEntries[0])
	}
	if !strings.Contains(resLoss.LostEntries[0].Reason, "human decision") {
		t.Errorf("expected reason to mention human decision, got %q", resLoss.LostEntries[0].Reason)
	}

	// Verify disk file was not overwritten
	afterBytes, err := os.ReadFile(jobsDossierPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(afterBytes), "dec-01J99999999999999999999999") {
		t.Errorf("expected hand-added entry to remain untouched on disk when Reproject fails")
	}

	// Reproject with AllowLoss: true succeeds and updates the file
	resAllowed, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
		AllowLoss: true,
	})
	if err != nil {
		t.Fatalf("expected Reproject with AllowLoss=true to succeed, got: %v", err)
	}
	if len(resAllowed.FilesUpdated) != 1 || resAllowed.FilesUpdated[0] != "dossiers/internal/core/jobs.md" {
		t.Errorf("expected dossiers/internal/core/jobs.md in FilesUpdated, got %v", resAllowed.FilesUpdated)
	}
}

func TestReproject_CapturesAndPreservesHumanProseHandEdits(t *testing.T) {
	ctx := context.Background()
	memDir := filepath.Join(t.TempDir(), ".memory")
	opsDir := filepath.Join(memDir, "ops")
	seedOpsBatchForReprojectTest(t, opsDir)

	if _, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
	}); err != nil {
		t.Fatalf("initial Reproject failed: %v", err)
	}

	// Simulate a human prose edit to an existing entry in .memory/dossiers/internal/core/jobs.md
	jobsDossierPath := filepath.Join(memDir, "dossiers", "internal", "core", "jobs.md")
	origBytes, err := os.ReadFile(jobsDossierPath)
	if err != nil {
		t.Fatal(err)
	}
	editedContent := strings.Replace(
		string(origBytes),
		"Enable PRAGMA journal_mode=WAL on open",
		"Enable PRAGMA journal_mode=WAL and busy_timeout=5000 on open",
		1,
	)
	if editedContent == string(origBytes) {
		t.Fatalf("failed to apply simulated prose edit to jobs.md:\n%s", string(origBytes))
	}
	if err := os.WriteFile(jobsDossierPath, []byte(editedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Reproject captures the hand edit as an OpAmend batch in .memory/ops and preserves the human prose
	res1, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
	})
	if err != nil {
		t.Fatalf("Reproject after hand edit failed: %v", err)
	}
	if res1.HandEditsCaptured != 1 {
		t.Errorf("HandEditsCaptured = %d, want 1", res1.HandEditsCaptured)
	}

	afterBytes, err := os.ReadFile(jobsDossierPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(afterBytes), "busy_timeout=5000") {
		t.Errorf("expected human prose edit to be preserved in jobs.md after Reproject, got:\n%s", string(afterBytes))
	}

	// Subsequent Reproject replays the captured OpAmend from .memory/ops with 0 new hand edits and 0 file churn
	res2, _, err := Reproject(ctx, ReprojectOptions{
		MemoryDir: memDir,
		OpsDirs:   []string{opsDir},
	})
	if err != nil {
		t.Fatalf("second Reproject failed: %v", err)
	}
	if res2.HandEditsCaptured != 0 {
		t.Errorf("second Reproject HandEditsCaptured = %d, want 0", res2.HandEditsCaptured)
	}
	if len(res2.FilesUpdated) != 0 {
		t.Errorf("second Reproject FilesUpdated = %v, want empty", res2.FilesUpdated)
	}
}
