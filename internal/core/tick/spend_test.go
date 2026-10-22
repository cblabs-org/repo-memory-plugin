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
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestSpendGuard_MinIntervalMaxItemsAndCalibration(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-spend-test"

	now := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, nil)
	exec.Now = func() time.Time { return now }

	// 1. Initial check uses DefaultSpendConfig (fallback: min_interval=1h, max_items=10)
	allowed, reason, maxItems := exec.CheckSpendGuard("fallback", false)
	if !allowed || reason != "" || maxItems != 10 {
		t.Fatalf("initial CheckSpendGuard(fallback): got allowed=%v reason=%q maxItems=%d, want true, '', 10", allowed, reason, maxItems)
	}

	// 2. Record a spend run and verify min_interval blocks within 1h when !force
	exec.RecordSpendRun("fallback", 100, 120)

	now = now.Add(30 * time.Minute)
	allowed, reason, _ = exec.CheckSpendGuard("fallback", false)
	if allowed || reason != "min_interval not elapsed" {
		t.Fatalf("after 30m CheckSpendGuard(fallback, false): got allowed=%v reason=%q, want false 'min_interval not elapsed'", allowed, reason)
	}

	// Force bypasses min_interval
	allowed, _, _ = exec.CheckSpendGuard("fallback", true)
	if !allowed {
		t.Fatal("expected force=true to bypass min_interval")
	}

	// After 61 minutes total, min_interval has elapsed
	now = now.Add(31 * time.Minute)
	allowed, reason, _ = exec.CheckSpendGuard("fallback", false)
	if !allowed || reason != "" {
		t.Fatalf("after 61m CheckSpendGuard(fallback, false): got allowed=%v reason=%q, want true ''", allowed, reason)
	}

	// 3. Test rolling 10-sample calibration factor
	// Record 11 more samples (total 12 -> window keeps last 10)
	for i := 0; i < 11; i++ {
		exec.RecordSpendRun("consolidation", 100, 200) // ratio 2.0
	}
	st, err := LoadSpendGuardState(dataDir, repoID)
	if err != nil {
		t.Fatalf("LoadSpendGuardState failed: %v", err)
	}
	if len(st.CalibrationRatios) != MaxCalibrationSamples {
		t.Fatalf("expected %d calibration samples, got %d", MaxCalibrationSamples, len(st.CalibrationRatios))
	}
	if mult := st.CalibrationMultiplier(); mult != 2.0 {
		t.Errorf("expected median calibration multiplier 2.0, got %v", mult)
	}
	// 40 bytes -> base 10 tokens * 2.0 = 20 tokens
	if est := EstimateTokensCalibrated(strings.Repeat("x", 40), st); est != 20 {
		t.Errorf("expected calibrated token estimate 20, got %d", est)
	}
}

func TestRunConsolidation_MaxItemsAndSpendGuard(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-cons-spend"

	// Create anchored file so anchors pass
	anchorFile := filepath.Join(repoRoot, "internal", "svc", "handler.go")
	if err := os.MkdirAll(filepath.Dir(anchorFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(anchorFile, []byte("package svc\nfunc Handle() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 4, 15, 14, 0, 0, 0, time.UTC)
	inboxStore := inbox.NewStore(dataDir, repoID)
	for i := 0; i < 5; i++ {
		_, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-spend",
			CreatedAt:      now.Add(-48 * time.Hour),
			Items: []inbox.NoteItem{
				{
					Type:    dossiers.EntryTypeGotcha,
					Title:   "Handler note",
					Why:     "Avoid nil context",
					Do:      "Pass non-nil context",
					Anchors: []string{"internal/svc/handler.go"},
					Scope:   "svc",
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, episode.NewStore(dataDir, repoID))
	exec.Now = func() time.Time { return now }
	cfg := config.DefaultRepoConfig()
	cfg.Spend.Jobs["consolidation"] = config.JobSpendConfig{
		MinInterval: "24h",
		MaxItems:    2,
	}
	exec.SetConfig(cfg)

	// Run 1: should cap batch at max_items = 2 notes
	res1, err := exec.RunConsolidation(ctx, false)
	if err != nil {
		t.Fatalf("RunConsolidation 1 failed: %v", err)
	}
	if res1.Skipped {
		t.Fatalf("expected RunConsolidation 1 to run, got skipped (%s)", res1.SkipReason)
	}
	if res1.ProcessedNotes != 2 {
		t.Errorf("expected ProcessedNotes=2 (max_items cap), got %d", res1.ProcessedNotes)
	}

	remaining, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 3 {
		t.Errorf("expected 3 unconsolidated notes remaining, got %d", len(remaining))
	}

	// Run 2 (1 hour later, < 24h min_interval, and < 10 notes): should be skipped due to cadence/min_interval
	now = now.Add(1 * time.Hour)
	res2, err := exec.RunConsolidation(ctx, false)
	if err != nil {
		t.Fatalf("RunConsolidation 2 failed: %v", err)
	}
	if !res2.Skipped {
		t.Errorf("expected RunConsolidation 2 to be skipped before min_interval elapsed")
	}
}

type stubDistillHousekeeper struct {
	calls int
}

func (s *stubDistillHousekeeper) Run(_ context.Context, _ harness.Stage, _ any) (harness.Result, error) {
	s.calls++
	return harness.Result{
		Payload: `{"items":[]}`,
		Usage:   harness.Usage{InputTokens: 40, OutputTokens: 10},
	}, nil
}

func TestRunSidecarFallback_SpendGuardMaxItemsAndMinInterval(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-fb-spend"

	epStore := episode.NewStore(dataDir, repoID)
	now := time.Date(2026, 4, 15, 16, 0, 0, 0, time.UTC)

	parentConv := "parent-conv"
	for i, id := range []string{"ep-sub-1", "ep-sub-2", "ep-sub-3"} {
		ep := &episode.Episode{
			Header: episode.Header{
				EpisodeID:     id,
				SessionID:     id,
				ParentEpisode: &parentConv,
				Ended:         now.Add(-time.Duration(i+1) * time.Minute),
				Commits:       []string{"abc1234"},
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindCommand, Command: "git commit -m test", Commit: "abc1234"},
			},
		}
		if _, err := epStore.WriteEpisode(ep); err != nil {
			t.Fatal(err)
		}
	}

	hk := &stubDistillHousekeeper{}
	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)
	exec.SetHousekeeper(hk)
	exec.Now = func() time.Time { return now }
	cfg := config.DefaultRepoConfig()
	cfg.Spend.Jobs["fallback"] = config.JobSpendConfig{
		MinInterval: "1h",
		MaxItems:    2,
	}
	exec.SetConfig(cfg)

	// Run 1: should cap distilled episodes at max_items = 2
	if _, err := exec.RunSidecarFallback(ctx); err != nil {
		t.Fatalf("RunSidecarFallback 1 failed: %v", err)
	}
	if hk.calls != 2 {
		t.Fatalf("expected 2 distiller calls (capped by max_items=2), got %d", hk.calls)
	}

	// Run 2 (10m later, < 1h min_interval): should skip without calling housekeeper
	now = now.Add(10 * time.Minute)
	if _, err := exec.RunSidecarFallback(ctx); err != nil {
		t.Fatalf("RunSidecarFallback 2 failed: %v", err)
	}
	if hk.calls != 2 {
		t.Fatalf("expected min_interval guard to block second fallback run within 1h, got %d calls", hk.calls)
	}
}

func TestFormatFreshnessLine_States(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-freshness-line"

	// Initialize a real git repo in repoRoot
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repoRoot}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@users.noreply.github.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@users.noreply.github.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}
	runGit("init", "-b", "main")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "initial")

	idx, err := index.NewIndex(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer idx.Close()

	inboxStore := inbox.NewStore(dataDir, repoID)
	exec := NewPipelineExecutor(dataDir, repoRoot, repoID, nil, nil)
	exec.idx = idx

	// Refresh freshness state when current
	exec.refreshFreshnessState(ctx, true)

	line1 := FormatFreshnessLine(ctx, dataDir, repoID, repoRoot, idx, inboxStore)
	want1 := "memory: index current to op-log · HEAD matches last build · 0 notes not yet consolidated · 0 entries suspect"
	if line1 != want1 {
		t.Errorf("freshness line (clean):\n got: %q\nwant: %q", line1, want1)
	}

	// Now advance git HEAD, write 2 unconsolidated notes, 1 new op-log batch on disk, and 1 suspect entry in index
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("# updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "README.md")
	runGit("commit", "-m", "second")

	for i := 0; i < 2; i++ {
		if _, err := inboxStore.Write(&inbox.Note{
			Source:         inbox.SourceTurnNote,
			ConversationID: "conv-1",
			Items:          []inbox.NoteItem{{Type: dossiers.EntryTypeGotcha, Title: "note"}},
		}); err != nil {
			t.Fatal(err)
		}
	}

	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	batch := ops.NewBatch([]ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      "inv-0001",
			DossierScope: "core",
			Source:       ops.SourceConsolidation,
			Fields:       map[string]any{"type": "invariant", "entry_type": "invariant", "text": "test"},
		},
	})
	if err := ops.NewWriter(opsDir).Write(batch); err != nil {
		t.Fatal(err)
	}

	if err := idx.UpsertEntry(ctx, dossiers.Entry{
		ID:           "inv-0001",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "Suspect entry",
		StaleSuspect: true,
	}); err != nil {
		t.Fatal(err)
	}

	line2 := FormatFreshnessLine(ctx, dataDir, repoID, repoRoot, idx, inboxStore)
	if !strings.Contains(line2, "index behind op-log (last built ") {
		t.Errorf("expected 'index behind op-log', got: %q", line2)
	}
	if !strings.Contains(line2, "HEAD moved since last build · 2 notes not yet consolidated · 1 entries suspect") {
		t.Errorf("unexpected freshness line (dirty): %q", line2)
	}
}
