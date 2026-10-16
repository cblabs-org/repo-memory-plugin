package tick_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func setupFallbackTestEnv(t *testing.T, repoID string) (string, string, *jobs.Store, *episode.Store) {
	t.Helper()
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	if err := os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, "internal", "service"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal", "service", "a.go"), []byte("package service\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "internal", "service", "b.go"), []byte("package service\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repoDataDir := filepath.Join(dataDir, repoID)
	if err := os.MkdirAll(repoDataDir, 0755); err != nil {
		t.Fatal(err)
	}
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = jobStore.Close()
	})

	epStore := episode.NewStore(dataDir, repoID)
	return dataDir, repoRoot, jobStore, epStore
}

func TestRunSidecarFallback_IdleMilestoneSessionDistillation(t *testing.T) {
	dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-idle")
	repoID := "repo-fb-idle"
	convID := "conv-idle-milestone"
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	// Mark is at step 1; steps 2, 3, 4 are uncovered (2 distinct edits + 1 test step).
	initialMark := &state.Mark{
		ConversationID: convID,
		StepIndex:      1,
	}
	if err := state.SaveMark(dataDir, repoID, initialMark); err != nil {
		t.Fatal(err)
	}

	ep := &episode.Episode{
		Header: episode.Header{
			EpisodeID: "ep-idle-1",
			SessionID: convID,
			RepoID:    repoID,
			Branch:    "main",
			HeadEnd:   "deadbeef1234567890abcdef1234567890abcdef",
			Started:   now.Add(-4 * time.Hour),
			Ended:     now.Add(-3 * time.Hour), // ended 3 hours ago (>= 2h threshold)
			Checkpoints: map[string]string{
				"Model Knowledge": "- Service handlers require non-nil context.",
			},
		},
		Steps: []harness.Step{
			{
				Index:   1,
				Kind:    harness.StepKindExposure,
				Via:     "userMessage",
				Trigger: "task_start",
				Claims:  []string{"inv-01J9ABC"},
			},
			{
				Index: 2,
				Kind:  harness.StepKindEdit,
				File:  "internal/service/a.go",
				Text:  "Updated handler A",
			},
			{
				Index: 3,
				Kind:  harness.StepKindEdit,
				File:  "internal/service/b.go",
				Text:  "Updated handler B",
			},
			{
				Index:   4,
				Kind:    harness.StepKindTest,
				Command: "go test ./...",
				Passed:  5,
				Outcome: "success",
			},
		},
	}
	if _, err := epStore.WriteEpisode(ep); err != nil {
		t.Fatal(err)
	}

	fakeHK := fake.NewFakeHousekeeper()
	fakeHK.SetResult(harness.StageDistill, harness.Result{
		Payload: "```repo-memory-note\n" +
			"- type: invariant\n" +
			"  title: Service handlers require non-nil context\n" +
			"  why: Nil context panics during cancellation propagation in service handlers.\n" +
			"  do: Always pass context.Background() when caller context is nil.\n" +
			"  anchors:\n" +
			"    - internal/service/a.go\n" +
			"    - internal/service/missing.go\n" +
			"  step_citations: [2, 4]\n" +
			"```\n" +
			"echo_token: {{echo_token}}\n",
		Usage: harness.Usage{InputTokens: 500, OutputTokens: 120},
	})

	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
	exec.SetHousekeeper(fakeHK)
	exec.Now = func() time.Time { return now }
	exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
		return &telemetry.QuotaStatus{
			Buckets: []telemetry.QuotaBucket{
				{ID: "gemini-flash", Name: "Gemini Flash", RemainingFraction: 0.80},
			},
		}, nil
	}

	n, err := exec.RunSidecarFallback(context.Background())
	if err != nil {
		t.Fatalf("RunSidecarFallback failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 note written, got %d", n)
	}
	if len(fakeHK.RunHistory) != 1 || fakeHK.RunHistory[0] != harness.StageDistill {
		t.Fatalf("expected StageDistill invocation, got %v", fakeHK.RunHistory)
	}

	notes, err := inbox.NewStore(dataDir, repoID).ListUnconsolidated()
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 unconsolidated inbox note, got %d", len(notes))
	}
	note := notes[0]
	if note.Source != inbox.SourceSidecar {
		t.Errorf("got note.Source %q, want %q", note.Source, inbox.SourceSidecar)
	}
	if note.ConversationID != convID {
		t.Errorf("got note.ConversationID %q, want %q", note.ConversationID, convID)
	}
	if note.StepsCovered != [2]int{2, 4} {
		t.Errorf("got note.StepsCovered %v, want [2 4]", note.StepsCovered)
	}
	if len(note.ExposureList) != 1 || note.ExposureList[0].EntryID != "inv-01J9ABC" {
		t.Errorf("expected ExposureList with inv-01J9ABC, got %+v", note.ExposureList)
	}
	if len(note.AnchorChecks) != 2 || !note.AnchorChecks[0].Exists || note.AnchorChecks[1].Exists {
		t.Errorf("unexpected note.AnchorChecks: %+v", note.AnchorChecks)
	}
	if len(note.Items) != 1 || note.Items[0].Title != "Service handlers require non-nil context" {
		t.Errorf("unexpected note.Items: %+v", note.Items)
	}

	updatedMark, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedMark.StepIndex != 4 {
		t.Errorf("expected mark.StepIndex advanced to 4, got %d", updatedMark.StepIndex)
	}
	if updatedMark.StepTimestamp != now.Add(-3*time.Hour).UTC().Format(time.RFC3339) {
		t.Errorf("expected mark.StepTimestamp %q, got %q", now.Add(-3*time.Hour).UTC().Format(time.RFC3339), updatedMark.StepTimestamp)
	}
	if !updatedMark.LastFallbackAt.Equal(now) {
		t.Errorf("expected mark.LastFallbackAt=%v, got %v", now, updatedMark.LastFallbackAt)
	}
	if !updatedMark.LastNoteAt.Equal(now) || updatedMark.NoteCount != 1 || updatedMark.NotesToday != 1 {
		t.Errorf("expected NoteCount=1, NotesToday=1, LastNoteAt=%v, got %+v", now, updatedMark)
	}
}

func TestRunSidecarFallback_NoneOutputAdvancesMarkWithoutIncrementingNoteCount(t *testing.T) {
	dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-none")
	repoID := "repo-fb-none"
	convID := "conv-idle-none"
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	initialMark := &state.Mark{
		ConversationID:      convID,
		StepIndex:           1,
		PendingContinuation: true,
		PendingRequestStep:  2,
	}
	if err := state.SaveMark(dataDir, repoID, initialMark); err != nil {
		t.Fatal(err)
	}

	ep := &episode.Episode{
		Header: episode.Header{
			EpisodeID: "ep-none-1",
			SessionID: convID,
			RepoID:    repoID,
			Started:   now.Add(-4 * time.Hour),
			Ended:     now.Add(-3 * time.Hour),
		},
		Steps: []harness.Step{
			{Index: 1, Kind: harness.StepKindUser, Text: "initial"},
			{
				Index:    2,
				Kind:     harness.StepKindCommand,
				Command:  "git commit -m 'chore: minor fix'",
				Commit:   "1234567890abcdef1234567890abcdef12345678",
				RawData:  map[string]any{"created_at": "2026-10-16T08:55:00Z"},
			},
		},
	}
	if _, err := epStore.WriteEpisode(ep); err != nil {
		t.Fatal(err)
	}

	fakeHK := fake.NewFakeHousekeeper()
	fakeHK.SetResult(harness.StageDistill, harness.Result{
		Payload: "NONE\necho_token: {{echo_token}}\n",
		Usage:   harness.Usage{InputTokens: 200, OutputTokens: 20},
	})

	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
	exec.SetHousekeeper(fakeHK)
	exec.Now = func() time.Time { return now }
	exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
		return &telemetry.QuotaStatus{
			Buckets: []telemetry.QuotaBucket{{ID: "flash", RemainingFraction: 0.90}},
		}, nil
	}

	n, err := exec.RunSidecarFallback(context.Background())
	if err != nil {
		t.Fatalf("RunSidecarFallback failed: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 notes written for NONE output, got %d", n)
	}

	updatedMark, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if updatedMark.StepIndex != 2 {
		t.Errorf("expected mark.StepIndex advanced to 2, got %d", updatedMark.StepIndex)
	}
	if updatedMark.StepTimestamp != "2026-10-16T08:55:00Z" {
		t.Errorf("expected mark.StepTimestamp '2026-10-16T08:55:00Z', got %q", updatedMark.StepTimestamp)
	}
	if updatedMark.PendingContinuation || updatedMark.PendingRequestStep != 0 {
		t.Errorf("expected PendingContinuation=false and PendingRequestStep=0, got %+v", updatedMark)
	}
	if !updatedMark.LastFallbackAt.Equal(now) {
		t.Errorf("expected mark.LastFallbackAt=%v, got %v", now, updatedMark.LastFallbackAt)
	}
	if updatedMark.NoteCount != 0 || updatedMark.NotesToday != 0 || !updatedMark.LastNoteAt.IsZero() {
		t.Errorf("expected NoteCount=0, NotesToday=0, and zero LastNoteAt on NONE output, got %+v", updatedMark)
	}
}

func TestRunSidecarFallback_ActiveSessionSkipped(t *testing.T) {
	dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-active")
	repoID := "repo-fb-active"
	convID := "conv-active-10m"
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	ep := &episode.Episode{
		Header: episode.Header{
			EpisodeID: "ep-active-1",
			SessionID: convID,
			RepoID:    repoID,
			Started:   now.Add(-20 * time.Minute),
			Ended:     now.Add(-10 * time.Minute), // ended 10m ago (< 2h threshold)
		},
		Steps: []harness.Step{
			{Index: 1, Kind: harness.StepKindEdit, File: "internal/service/a.go"},
			{Index: 2, Kind: harness.StepKindEdit, File: "internal/service/b.go"},
			{Index: 3, Kind: harness.StepKindTest, Command: "go test ./...", Outcome: "success"},
		},
	}
	if _, err := epStore.WriteEpisode(ep); err != nil {
		t.Fatal(err)
	}

	fakeHK := fake.NewFakeHousekeeper()
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
	exec.SetHousekeeper(fakeHK)
	exec.Now = func() time.Time { return now }
	exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
		return &telemetry.QuotaStatus{
			Buckets: []telemetry.QuotaBucket{{ID: "flash", RemainingFraction: 0.90}},
		}, nil
	}

	n, err := exec.RunSidecarFallback(context.Background())
	if err != nil {
		t.Fatalf("RunSidecarFallback failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 notes for active session, got %d", n)
	}
	if len(fakeHK.RunHistory) != 0 {
		t.Errorf("expected Housekeeper not to be called for active session, got %v", fakeHK.RunHistory)
	}
}

func TestRunSidecarFallback_SubagentAndFallbackOnlyProcessedImmediately(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	t.Run("subagent processed before 2h idle", func(t *testing.T) {
		dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-subagent")
		repoID := "repo-fb-subagent"
		subConvID := "conv-child-subagent"

		if err := state.RecordSubagent(context.Background(), dataDir, repoID, subConvID, "conv-parent", "worker"); err != nil {
			t.Fatal(err)
		}

		ep := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "ep-sub-1",
				SessionID: subConvID,
				RepoID:    repoID,
				Started:   now.Add(-5 * time.Minute),
				Ended:     now.Add(-2 * time.Minute), // only 2 minutes ago
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindCommand, Command: "git commit -m 'feat: subagent commit'", Commit: "1234567890abcdef1234567890abcdef12345678"},
			},
		}
		if _, err := epStore.WriteEpisode(ep); err != nil {
			t.Fatal(err)
		}

		fakeHK := fake.NewFakeHousekeeper()
		fakeHK.SetResult(harness.StageDistill, harness.Result{
			Payload: "```repo-memory-note\n" +
				"- type: gotcha\n" +
				"  title: Subagent worker commit convention\n" +
				"  why: Subagents commit directly on their workorder branch.\n" +
				"  do: Verify workorder branch before committing.\n" +
				"  anchors: [internal/service/a.go]\n" +
				"  step_citations: [1]\n" +
				"```\n" +
				"echo_token: {{echo_token}}\n",
		})

		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
		exec.SetHousekeeper(fakeHK)
		exec.Now = func() time.Time { return now }
		exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
			return &telemetry.QuotaStatus{
				Buckets: []telemetry.QuotaBucket{{ID: "flash", RemainingFraction: 0.75}},
			}, nil
		}

		n, err := exec.RunSidecarFallback(context.Background())
		if err != nil {
			t.Fatalf("RunSidecarFallback failed: %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 note for subagent session, got %d", n)
		}
	})

	t.Run("fallback-only mode processed before 2h idle", func(t *testing.T) {
		dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-mode")
		repoID := "repo-fb-mode"
		convID := "conv-fallback-only"

		cfg := config.DefaultRepoConfig()
		cfg.TurnNotes = "fallback-only"
		if err := config.SaveRepoConfig(repoRoot, cfg); err != nil {
			t.Fatal(err)
		}

		ep := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "ep-fbonly-1",
				SessionID: convID,
				RepoID:    repoID,
				Started:   now.Add(-5 * time.Minute),
				Ended:     now.Add(-1 * time.Minute), // only 1 minute ago
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindCommand, Command: "git commit -m 'fix: quick fix'", Commit: "abcdef1234567890abcdef1234567890abcdef12"},
			},
		}
		if _, err := epStore.WriteEpisode(ep); err != nil {
			t.Fatal(err)
		}

		fakeHK := fake.NewFakeHousekeeper()
		fakeHK.SetResult(harness.StageDistill, harness.Result{
			Payload: "```repo-memory-note\n" +
				"- type: decision\n" +
				"  title: Use fallback-only mode for batch distillation\n" +
				"  why: Avoids interactive turn continuation latency.\n" +
				"  do: \"Set turn_notes to fallback-only in .memory/config.yaml.\"\n" +
				"  anchors: [internal/service/a.go]\n" +
				"  step_citations: [1]\n" +
				"```\n" +
				"echo_token: {{echo_token}}\n",
		})

		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
		exec.SetHousekeeper(fakeHK)
		exec.Now = func() time.Time { return now }
		exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
			return &telemetry.QuotaStatus{
				Buckets: []telemetry.QuotaBucket{{ID: "flash", RemainingFraction: 0.75}},
			}, nil
		}

		n, err := exec.RunSidecarFallback(context.Background())
		if err != nil {
			t.Fatalf("RunSidecarFallback failed: %v", err)
		}
		if n != 1 {
			t.Fatalf("expected 1 note for fallback-only session, got %d", n)
		}
	})
}

func TestRunSidecarFallback_QuotaGuardSkipsWhenLow(t *testing.T) {
	dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-quota")
	repoID := "repo-fb-quota"
	convID := "conv-low-quota"
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	initialMark := &state.Mark{
		ConversationID: convID,
		StepIndex:      1,
	}
	if err := state.SaveMark(dataDir, repoID, initialMark); err != nil {
		t.Fatal(err)
	}

	ep := &episode.Episode{
		Header: episode.Header{
			EpisodeID: "ep-quota-1",
			SessionID: convID,
			RepoID:    repoID,
			Started:   now.Add(-4 * time.Hour),
			Ended:     now.Add(-3 * time.Hour),
		},
		Steps: []harness.Step{
			{Index: 1, Kind: harness.StepKindUser, Text: "initial"},
			{Index: 2, Kind: harness.StepKindCommand, Command: "git commit -m 'feat: milestone'", Commit: "1111222233334444555566667777888899990000"},
		},
	}
	if _, err := epStore.WriteEpisode(ep); err != nil {
		t.Fatal(err)
	}

	fakeHK := fake.NewFakeHousekeeper()
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
	exec.SetHousekeeper(fakeHK)
	exec.Now = func() time.Time { return now }
	exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
		return &telemetry.QuotaStatus{
			Buckets: []telemetry.QuotaBucket{
				{ID: "gemini-flash", Name: "Gemini 3.0 Flash", RemainingFraction: 0.15}, // < 0.25
			},
		}, nil
	}

	n, err := exec.RunSidecarFallback(context.Background())
	if err != nil {
		t.Fatalf("RunSidecarFallback failed: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 notes written when quota < 0.25, got %d", n)
	}
	if len(fakeHK.RunHistory) != 0 {
		t.Errorf("expected Housekeeper not to be invoked when quota < 0.25, got %v", fakeHK.RunHistory)
	}

	afterMark, err := state.LoadMark(dataDir, repoID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if afterMark.StepIndex != 1 {
		t.Errorf("expected mark.StepIndex to remain 1 when skipped by quota guard, got %d", afterMark.StepIndex)
	}
}

func TestRunSidecarFallback_PrepareValidatorRejectionQuarantined(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

	t.Run("missing echo_token is quarantined", func(t *testing.T) {
		dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-quarantine-token")
		repoID := "repo-fb-quarantine-token"
		convID := "conv-bad-token"

		ep := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "ep-bad-1",
				SessionID: convID,
				RepoID:    repoID,
				Started:   now.Add(-4 * time.Hour),
				Ended:     now.Add(-3 * time.Hour),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindCommand, Command: "git commit -m 'test'", Commit: "1111222233334444555566667777888899990000"},
			},
		}
		if _, err := epStore.WriteEpisode(ep); err != nil {
			t.Fatal(err)
		}

		fakeHK := fake.NewFakeHousekeeper()
		fakeHK.SetResult(harness.StageDistill, harness.Result{
			Payload: "```repo-memory-note\n" +
				"- type: gotcha\n" +
				"  title: Missing echo token item\n" +
				"  why: Some reason\n" +
				"  do: Some action\n" +
				"  anchors: [internal/service/a.go]\n" +
				"  step_citations: [1]\n" +
				"```\n", // NO echo_token line!
		})

		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
		exec.SetHousekeeper(fakeHK)
		exec.Now = func() time.Time { return now }
		exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
			return &telemetry.QuotaStatus{
				Buckets: []telemetry.QuotaBucket{{ID: "flash", RemainingFraction: 0.90}},
			}, nil
		}

		n, err := exec.RunSidecarFallback(context.Background())
		if err != nil {
			t.Fatalf("RunSidecarFallback failed: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 notes written when validation fails, got %d", n)
		}

		notes, _ := inbox.NewStore(dataDir, repoID).ListAll()
		if len(notes) != 0 {
			t.Errorf("expected 0 inbox notes, got %d", len(notes))
		}

		qDir := filepath.Join(dataDir, repoID, "quarantine")
		entries, err := os.ReadDir(qDir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("expected 1 quarantine file in %s, got %v (err=%v)", qDir, entries, err)
		}
	})

	t.Run("citing stubbed file-read step is quarantined", func(t *testing.T) {
		dataDir, repoRoot, jobStore, epStore := setupFallbackTestEnv(t, "repo-fb-quarantine-stub")
		repoID := "repo-fb-quarantine-stub"
		convID := "conv-bad-stub-cite"

		ep := &episode.Episode{
			Header: episode.Header{
				EpisodeID: "ep-bad-2",
				SessionID: convID,
				RepoID:    repoID,
				Started:   now.Add(-4 * time.Hour),
				Ended:     now.Add(-3 * time.Hour),
			},
			Steps: []harness.Step{
				{Index: 1, Kind: harness.StepKindRead, File: ".agents/skills/pa/SKILL.md", Text: "Always follow skill instructions."},
				{Index: 2, Kind: harness.StepKindCommand, Command: "git commit -m 'test'", Commit: "1111222233334444555566667777888899990000"},
			},
		}
		if _, err := epStore.WriteEpisode(ep); err != nil {
			t.Fatal(err)
		}

		fakeHK := fake.NewFakeHousekeeper()
		fakeHK.SetResult(harness.StageDistill, harness.Result{
			Payload: "```repo-memory-note\n" +
				"- type: invariant\n" +
				"  title: Cited stubbed skill step\n" +
				"  why: Extracted from skill read\n" +
				"  do: Do not cite step 1\n" +
				"  anchors: [internal/service/a.go]\n" +
				"  step_citations: [1]\n" + // step 1 is a stubbed StepKindRead!
				"```\n" +
				"echo_token: {{echo_token}}\n",
		})

		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
		exec.SetHousekeeper(fakeHK)
		exec.Now = func() time.Time { return now }
		exec.QuotaChecker = func(context.Context) (*telemetry.QuotaStatus, error) {
			return &telemetry.QuotaStatus{
				Buckets: []telemetry.QuotaBucket{{ID: "flash", RemainingFraction: 0.90}},
			}, nil
		}

		n, err := exec.RunSidecarFallback(context.Background())
		if err != nil {
			t.Fatalf("RunSidecarFallback failed: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 notes written when citing stubbed step, got %d", n)
		}

		qDir := filepath.Join(dataDir, repoID, "quarantine")
		entries, err := os.ReadDir(qDir)
		if err != nil || len(entries) != 1 {
			t.Fatalf("expected 1 quarantine file in %s, got %v (err=%v)", qDir, entries, err)
		}
		qBytes, _ := os.ReadFile(filepath.Join(qDir, entries[0].Name()))
		if !strings.Contains(string(qBytes), "stubbed") {
			t.Errorf("expected quarantine reason to mention stubbed step citation, got:\n%s", string(qBytes))
		}
	})
}
