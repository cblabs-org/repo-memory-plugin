package tick_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func runGitForConsolidateTest(t *testing.T, dir string, args ...string) {
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

func TestRunConsolidation_CadenceAndForce(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC)

	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-cadence-test"

	epStore := episode.NewStore(dataDir, repoID)
	execPipe := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)
	execPipe.Now = func() time.Time { return now }

	// 0. No unconsolidated notes -> skips with "no_unconsolidated_notes".
	res0, err := execPipe.RunConsolidation(ctx, false)
	if err != nil {
		t.Fatalf("RunConsolidation with 0 notes: %v", err)
	}
	if !res0.Skipped || res0.SkipReason != "no_unconsolidated_notes" {
		t.Fatalf("expected Skipped=true SkipReason='no_unconsolidated_notes', got %+v", res0)
	}

	inboxStore := inbox.NewStore(dataDir, repoID)
	writeNote := func(id string, created time.Time) {
		_, wErr := inboxStore.Write(&inbox.Note{
			ID:        id,
			Source:    inbox.SourceTurnNote,
			Branch:    "main",
			CreatedAt: created,
			Items: []inbox.NoteItem{
				{
					Type:  dossiers.EntryTypeGotcha,
					Title: "Gotcha for " + id,
					Why:   "Test cadence handling",
					Do:    "Verify note consolidation cadence",
				},
			},
		})
		if wErr != nil {
			t.Fatalf("Write note %s: %v", id, wErr)
		}
	}

	// 1. Write 2 recent notes (< 10) and set LastConsolidationAt to 2 hours ago (< 24h).
	writeNote("note-cad-1", now.Add(-1*time.Hour))
	writeNote("note-cad-2", now.Add(-30*time.Minute))
	if err := tick.SaveFreshnessState(dataDir, repoID, &tick.FreshnessState{
		LastConsolidationAt: now.Add(-2 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveFreshnessState: %v", err)
	}

	resSkip, err := execPipe.RunConsolidation(ctx, false)
	if err != nil {
		t.Fatalf("RunConsolidation(false) failed: %v", err)
	}
	if !resSkip.Skipped || resSkip.SkipReason != "cadence_not_due" {
		t.Fatalf("expected Skipped=true SkipReason='cadence_not_due', got %+v", resSkip)
	}
	if resSkip.UnconsolidatedNotes != 2 {
		t.Errorf("UnconsolidatedNotes = %d, want 2", resSkip.UnconsolidatedNotes)
	}

	// 2. When LastConsolidationAt is > 24h ago, RunConsolidation(ctx, false) executes.
	if err := tick.SaveFreshnessState(dataDir, repoID, &tick.FreshnessState{
		LastConsolidationAt: now.Add(-25 * time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("SaveFreshnessState >24h: %v", err)
	}
	resAgeDue, err := execPipe.RunConsolidation(ctx, false)
	if err != nil {
		t.Fatalf("RunConsolidation(false) with >24h age failed: %v", err)
	}
	if resAgeDue.Skipped || resAgeDue.ProcessedNotes != 2 {
		t.Fatalf("expected 2 processed notes when >24h old, got %+v", resAgeDue)
	}

	// 3. With 1 recent note and recent LastConsolidationAt, Force=true executes immediately.
	writeNote("note-cad-force", now.Add(-5*time.Minute))
	resForce, err := execPipe.RunConsolidation(ctx, true)
	if err != nil {
		t.Fatalf("RunConsolidation(true) failed: %v", err)
	}
	if resForce.Skipped || resForce.ProcessedNotes != 1 {
		t.Fatalf("expected Force=true to process 1 note, got %+v", resForce)
	}

	// 4. With >= 10 notes and recent LastConsolidationAt, RunConsolidation(ctx, false) executes.
	for i := 0; i < tick.DefaultConsolidationMinNotes; i++ {
		writeNote("note-batch-"+string(rune('a'+i)), now.Add(-time.Minute))
	}
	resBatchDue, err := execPipe.RunConsolidation(ctx, false)
	if err != nil {
		t.Fatalf("RunConsolidation(false) with >=10 notes failed: %v", err)
	}
	if resBatchDue.Skipped || resBatchDue.ProcessedNotes != tick.DefaultConsolidationMinNotes {
		t.Fatalf("expected >=10 notes to trigger consolidation, got %+v", resBatchDue)
	}
}

func TestRunConsolidation_EndToEndPipeline(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 11, 0, 0, 0, time.UTC)

	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-e2e-consolidate"

	// Initialize git repository on main with an existing source file.
	runGitForConsolidateTest(t, repoRoot, "init", "-b", "main")
	runGitForConsolidateTest(t, repoRoot, "config", "user.name", "Test User")
	runGitForConsolidateTest(t, repoRoot, "config", "user.email", "test@example.com")

	storeFileRel := filepath.Join("internal", "db", "store.go")
	storeFileAbs := filepath.Join(repoRoot, storeFileRel)
	if err := os.MkdirAll(filepath.Dir(storeFileAbs), 0o755); err != nil {
		t.Fatalf("mkdir store dir: %v", err)
	}
	if err := os.WriteFile(storeFileAbs, []byte("package db\n\nfunc QueryRows() {}\n"), 0o644); err != nil {
		t.Fatalf("write store.go: %v", err)
	}
	runGitForConsolidateTest(t, repoRoot, "add", storeFileRel)
	runGitForConsolidateTest(t, repoRoot, "commit", "-m", "initial commit")

	// Seed an existing repo: dossier entry got-existing1 in scope "db".
	existingEntryID := "got-01J00000000000000000000001"
	_, err := consolidate.RecordRepoTier(ctx, repoRoot, []ops.Op{
		{
			Type:         ops.OpAdd,
			EntryID:      existingEntryID,
			DossierScope: "db",
			Timestamp:    now.Add(-2 * time.Hour),
			Source:       ops.SourceConsolidation,
			Fields: map[string]any{
				"type":       "gotcha",
				"entry_type": "gotcha",
				"purpose":    "Database layer memory.",
				"text":       "**Close SQLite rows before commit**\nWhy: prevents SQLITE_BUSY under WAL.\nDo: call rows.Close() before tx.Commit().",
				"anchors":    []string{"internal/db/store.go"},
				"evidence":   []string{"note-seed-0"},
			},
		},
	}, nil, nil, now.Add(-2*time.Hour))
	if err != nil {
		t.Fatalf("RecordRepoTier initial seed failed: %v", err)
	}

	epStore := episode.NewStore(dataDir, repoID)
	// Episode for Note 1 (read-only, no edit+test -> echo is NOT reconfirmed).
	_, err = epStore.WriteEpisode(&episode.Episode{
		Header: episode.Header{
			EpisodeID: "conv-echo-1",
			SessionID: "conv-echo-1",
			RepoID:    repoID,
			Branch:    "main",
		},
		Steps: []harness.Step{
			{Index: 0, Kind: harness.StepKindRead, File: "internal/db/store.go"},
		},
	})
	if err != nil {
		t.Fatalf("WriteEpisode conv-echo-1: %v", err)
	}

	inboxStore := inbox.NewStore(dataDir, repoID)

	// Note 1: Echo of got-existing1 without anchored edit+test -> dropped by P13 Echo Filter.
	_, err = inboxStore.Write(&inbox.Note{
		ID:             "note-e2e-1",
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-echo-1",
		Branch:         "main",
		CreatedAt:      now.Add(-40 * time.Minute),
		ExposureList: []delivery.ExposureRecord{
			{EntryID: existingEntryID, Channel: delivery.ChannelSystem, Moment: delivery.MomentTaskStart},
		},
		Items: []inbox.NoteItem{
			{
				Type:    dossiers.EntryTypeGotcha,
				Title:   "Close SQLite rows before commit",
				Why:     "prevents SQLITE_BUSY under WAL",
				Do:      "call rows.Close() before tx.Commit()",
				Anchors: []string{"internal/db/store.go"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Write note-e2e-1: %v", err)
	}

	// Note 2: Uncorroborated invariant without static anchor or passing test -> downgraded by VD18 to open-issue.
	_, err = inboxStore.Write(&inbox.Note{
		ID:             "note-e2e-2",
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-uncorroborated-2",
		Branch:         "main",
		CreatedAt:      now.Add(-30 * time.Minute),
		Items: []inbox.NoteItem{
			{
				Type:    dossiers.EntryTypeInvariant,
				Title:   "Missing package must never panic on nil config",
				Why:     "unverified hypothesis from exploration",
				Do:      "check nil config in internal/missing/nonexistent.go",
				Anchors: []string{"internal/missing/nonexistent.go"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Write note-e2e-2: %v", err)
	}

	// Note 3: Exact duplicate of got-existing1 from an unexposed session -> merged deterministically via OpAmend.
	_, err = inboxStore.Write(&inbox.Note{
		ID:             "note-e2e-3",
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-dup-3",
		Branch:         "main",
		CreatedAt:      now.Add(-20 * time.Minute),
		Items: []inbox.NoteItem{
			{
				Type:     dossiers.EntryTypeGotcha,
				Title:    "Close SQLite rows before commit",
				Why:      "prevents SQLITE_BUSY under WAL",
				Do:       "call rows.Close() before tx.Commit()",
				Anchors:  []string{"internal/db/store.go#QueryRows"},
				Verified: "2026-10-16 (test-pass)",
			},
		},
	})
	if err != nil {
		t.Fatalf("Write note-e2e-3: %v", err)
	}

	// Note 4: Workstation-local user: item (~/.ssh/config) + corroborated repo: gotcha item.
	_, err = inboxStore.Write(&inbox.Note{
		ID:             "note-e2e-4",
		Source:         inbox.SourceTurnNote,
		ConversationID: "conv-mixed-4",
		Branch:         "main",
		CreatedAt:      now.Add(-10 * time.Minute),
		Items: []inbox.NoteItem{
			{
				Type:  dossiers.EntryTypeGotcha,
				Scope: "user:preferences/ssh",
				Title: "Local ~/.ssh/config ControlMaster multiplexing",
				Why:   "speeds up workstation git fetches",
				Do:    "keep ControlPersist 10m in ~/.ssh/config",
			},
			{
				Type:     dossiers.EntryTypeGotcha,
				Title:    "QueryRows requires explicit context timeout",
				Why:      "prevents unbounded query stalls",
				Do:       "wrap ctx with context.WithTimeout before calling QueryRows",
				Anchors:  []string{"internal/db/store.go#QueryRows"},
				Verified: "2026-10-16 (anchor-verified)",
			},
		},
	})
	if err != nil {
		t.Fatalf("Write note-e2e-4: %v", err)
	}

	execPipe := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)
	execPipe.Now = func() time.Time { return now }

	// Step A: Run with DryRun=true first and verify zero disk writes.
	dryRes, err := execPipe.RunConsolidationWithOptions(ctx, tick.ConsolidationOptions{
		Force:  true,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("RunConsolidationWithOptions(DryRun) failed: %v", err)
	}
	if !dryRes.DryRun || dryRes.Skipped {
		t.Fatalf("expected non-skipped dry-run result, got %+v", dryRes)
	}
	if dryRes.EchoesDropped != 1 {
		t.Errorf("dryRun EchoesDropped = %d, want 1", dryRes.EchoesDropped)
	}
	if dryRes.DowngradedToIssue != 1 {
		t.Errorf("dryRun DowngradedToIssue = %d, want 1", dryRes.DowngradedToIssue)
	}
	if dryRes.ExactDuplicatesAmend != 1 {
		t.Errorf("dryRun ExactDuplicatesAmend = %d, want 1", dryRes.ExactDuplicatesAmend)
	}
	if dryRes.EstimatedInputTokens <= 0 {
		t.Errorf("dryRun EstimatedInputTokens = %d, want > 0", dryRes.EstimatedInputTokens)
	}
	if _, statErr := os.Stat(consolidate.UserTierBaseDir(dataDir, repoID)); !os.IsNotExist(statErr) {
		t.Fatalf("expected user-dossiers not to be written during dry-run")
	}
	unconsolidatedAfterDry, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated after dry-run: %v", err)
	}
	if len(unconsolidatedAfterDry) != 4 {
		t.Fatalf("expected all 4 notes to remain unconsolidated after dry-run, got %d", len(unconsolidatedAfterDry))
	}

	// Step B: Run real consolidation.
	res, err := execPipe.RunConsolidation(ctx, true)
	if err != nil {
		t.Fatalf("RunConsolidation(true) failed: %v", err)
	}
	if res.Skipped {
		t.Fatalf("expected consolidation not to be skipped, got %+v", res)
	}
	if res.ProcessedNotes != 4 {
		t.Errorf("ProcessedNotes = %d, want 4", res.ProcessedNotes)
	}
	if res.CandidateItems != 5 {
		t.Errorf("CandidateItems = %d, want 5", res.CandidateItems)
	}
	if res.EchoesDropped != 1 {
		t.Errorf("EchoesDropped = %d, want 1", res.EchoesDropped)
	}
	if res.DowngradedToIssue != 1 {
		t.Errorf("DowngradedToIssue = %d, want 1", res.DowngradedToIssue)
	}
	if res.ExactDuplicatesAmend != 1 {
		t.Errorf("ExactDuplicatesAmend = %d, want 1", res.ExactDuplicatesAmend)
	}
	if res.OpsAdded != 3 {
		t.Errorf("OpsAdded = %d, want 3 (1 user gotcha + 1 downgraded open-issue + 1 repo gotcha)", res.OpsAdded)
	}
	if res.OpsAmended != 1 {
		t.Errorf("OpsAmended = %d, want 1", res.OpsAmended)
	}
	if res.UserBatchID == "" {
		t.Errorf("expected non-empty UserBatchID")
	}
	if res.RepoBatchID == "" {
		t.Errorf("expected non-empty RepoBatchID")
	}
	if !res.WorktreeCommitted || res.WorktreeCommitSHA == "" {
		t.Errorf("expected WorktreeCommitted=true and non-empty WorktreeCommitSHA, got Committed=%v SHA=%q", res.WorktreeCommitted, res.WorktreeCommitSHA)
	}

	// 1. Verify user: item is written to <dataDir>/<repoID>/user-dossiers/ and NOT .memory/ops/.
	userDossierPath := filepath.Join(consolidate.UserTierBaseDir(dataDir, repoID), "dossiers", "preferences", "ssh.md")
	userDossierBytes, err := os.ReadFile(userDossierPath)
	if err != nil {
		t.Fatalf("reading user dossier %s: %v", userDossierPath, err)
	}
	if !strings.Contains(string(userDossierBytes), "ControlMaster") {
		t.Errorf("expected user dossier to contain 'ControlMaster', got:\n%s", string(userDossierBytes))
	}

	repoProjected, err := dossiers.NewProjector(ops.NewReader(filepath.Join(repoRoot, ".memory", "ops"))).Project(ctx)
	if err != nil {
		t.Fatalf("projecting repo ops: %v", err)
	}
	if _, leaked := repoProjected["preferences/ssh"]; leaked {
		t.Fatalf("user: scope preferences/ssh leaked into shared .memory/ops!")
	}

	// 2. Verify repo: ops are rendered in .memory/dossiers/ and .memory/ATLAS.md.
	dbDossierPath := filepath.Join(repoRoot, ".memory", "dossiers", "db.md")
	dbDossierBytes, err := os.ReadFile(dbDossierPath)
	if err != nil {
		t.Fatalf("reading repo db dossier %s: %v", dbDossierPath, err)
	}
	if !strings.Contains(string(dbDossierBytes), "QueryRows requires explicit context timeout") {
		t.Errorf("expected db dossier to contain new gotcha, got:\n%s", string(dbDossierBytes))
	}
	if !strings.Contains(string(dbDossierBytes), "note-e2e-3") {
		t.Errorf("expected db dossier got-existing1 to include amended evidence note-e2e-3, got:\n%s", string(dbDossierBytes))
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".memory", "ATLAS.md")); err != nil {
		t.Fatalf("expected .memory/ATLAS.md to exist: %v", err)
	}

	// 3. Verify all 4 inbox notes are marked Consolidated == true.
	remainingNotes, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated: %v", err)
	}
	if len(remainingNotes) != 0 {
		t.Fatalf("expected 0 unconsolidated notes after run, got %d", len(remainingNotes))
	}

	// 4. Verify <dataDir>/<repoID>/index.db contains both repo: entries and the user: entry.
	idx, err := index.NewIndex(index.IndexPath(dataDir, repoID))
	if err != nil {
		t.Fatalf("opening index.db: %v", err)
	}
	defer idx.Close()

	if _, err := idx.GetEntry(ctx, existingEntryID); err != nil {
		t.Errorf("expected existing repo entry %s in index.db: %v", existingEntryID, err)
	}
	userSearchHits, err := idx.SearchEntries(ctx, "ControlMaster")
	if err != nil || len(userSearchHits) == 0 {
		t.Errorf("expected user entry 'ControlMaster' in index.db, hits=%v err=%v", userSearchHits, err)
	}

	// 5. Verify <dataDir>/<repoID>/state/freshness.json is written with UnconsolidatedNotes == 0 and non-empty LastConsolidationAt.
	freshness, err := tick.LoadFreshnessState(dataDir, repoID)
	if err != nil {
		t.Fatalf("LoadFreshnessState: %v", err)
	}
	if freshness.UnconsolidatedNotes != 0 {
		t.Errorf("freshness.UnconsolidatedNotes = %d, want 0", freshness.UnconsolidatedNotes)
	}
	if freshness.LastConsolidationAt != now.Format(time.RFC3339) {
		t.Errorf("freshness.LastConsolidationAt = %q, want %q", freshness.LastConsolidationAt, now.Format(time.RFC3339))
	}
	if freshness.ActiveEntries < 3 {
		t.Errorf("freshness.ActiveEntries = %d, want >= 3", freshness.ActiveEntries)
	}
}

func TestRunConsolidation_QuotaGuardSkipsWhenExhausted(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-quota-guard"

	inboxStore := inbox.NewStore(dataDir, repoID)
	_, err := inboxStore.Write(&inbox.Note{
		ID:        "note-quota-1",
		Source:    inbox.SourceTurnNote,
		Branch:    "main",
		CreatedAt: now,
		Items: []inbox.NoteItem{
			{
				Type:  dossiers.EntryTypeGotcha,
				Title: "Quota guard test item",
				Do:    "Wait for quota",
			},
		},
	})
	if err != nil {
		t.Fatalf("Write note: %v", err)
	}

	epStore := episode.NewStore(dataDir, repoID)
	execPipe := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)
	execPipe.Now = func() time.Time { return now }
	execPipe.SetHousekeeper(&noopHousekeeper{})
	execPipe.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
		return &telemetry.QuotaStatus{
			Buckets: []telemetry.QuotaBucket{
				{ID: "gemini-flash", Name: "Gemini Flash", RemainingFraction: 0.10},
			},
		}, nil
	}

	res, err := execPipe.RunConsolidation(ctx, true)
	if err != nil {
		t.Fatalf("RunConsolidation failed: %v", err)
	}
	if !res.Skipped || res.SkipReason != "quota_or_budget_exhausted" {
		t.Fatalf("expected Skipped=true SkipReason='quota_or_budget_exhausted', got %+v", res)
	}
}

type noopHousekeeper struct{}

func (n *noopHousekeeper) Run(context.Context, harness.Stage, any) (harness.Result, error) {
	return harness.Result{}, nil
}
