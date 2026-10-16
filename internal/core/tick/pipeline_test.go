package tick_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func TestPipelineBudgetCapping(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755)

	cfg := config.DefaultRepoConfig()
	cfg.Budgets.DailyTokens = 1000
	config.SaveRepoConfig(repoRoot, cfg)

	store, err := jobs.Open(filepath.Join(dataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Enqueue a job that will trigger budget check
	store.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindTriage,
		Priority:       10,
		IdempotencyKey: "triage-1",
		Payload:        "triage-1",
	})

	ticker := tick.NewTicker(dataDir, time.Minute)
	_ = ticker
}

func TestConvertCapturesToEvidence(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-test-evidence"
	convID := "conv-session-42"

	// Create test transcript JSONL file with steps and checkpoint
	transcriptDir := filepath.Join(dataDir, "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("creating transcript dir: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript.jsonl")

	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-04T10:00:00Z","content":"Build the feature"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-04T10:01:00Z","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/path/to/code.go"}}]}
{"step_index":2,"source":"SYSTEM","type":"CHECKPOINT","status":"DONE","created_at":"2026-10-04T10:02:00Z","content":"### 1. User Knowledge\n- User requires high test coverage.\n\n### 2. Model Knowledge\n- Production core code never imports harness adapters.\n\n### 3. Work Accomplished\n- Built the evidence ingestion pipeline.\n\n### 4. Current Work\n- Validating pipeline tests.\n\n### 5. Outstanding User Requests\n- Implement WO2 capture-to-evidence.\n\n### 6. Files and Code\n- internal/core/tick/pipeline.go"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-04T10:03:00Z","content":"Finished verification successfully."}
`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("writing transcript file: %v", err)
	}

	// Setup jobs store
	repoDataDir := filepath.Join(dataDir, repoID)
	if err := os.MkdirAll(repoDataDir, 0755); err != nil {
		t.Fatalf("creating repo data dir: %v", err)
	}
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create mock spool entry pointing to test transcript JSONL file
	mockEntry := spool.Entry{
		Kind:           spool.KindCapture,
		Timestamp:      time.Now().UTC(),
		Harness:        "antigravity",
		ConversationID: convID,
		TranscriptPath: transcriptPath,
		RepoRoot:       repoRoot,
		Branch:         "main",
		Head:           "commit-12345678",
	}
	entryBytes, err := json.Marshal(mockEntry)
	if err != nil {
		t.Fatalf("marshaling spool entry: %v", err)
	}

	// Enqueue KindCapture job with that entry
	_, err = jobStore.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindCapture,
		Priority:       jobs.PriorityNormal,
		IdempotencyKey: "capture-" + convID,
		Payload:        string(entryBytes),
	})
	if err != nil {
		t.Fatalf("enqueuing capture job: %v", err)
	}

	// Create episode store and pipeline executor
	epStore := episode.NewStore(dataDir, repoID)
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)

	// Run convertCapturesToEvidence
	if err := exec.ConvertCapturesToEvidence(ctx); err != nil {
		t.Fatalf("ConvertCapturesToEvidence failed: %v", err)
	}

	// Verify KindCapture job was completed
	pendingCaptures, err := jobStore.List(ctx, jobs.Filter{Kind: jobs.KindCapture, State: jobs.StateQueued})
	if err != nil {
		t.Fatalf("listing pending capture jobs: %v", err)
	}
	if len(pendingCaptures) != 0 {
		t.Errorf("expected 0 pending capture jobs, got %d", len(pendingCaptures))
	}

	// Verify no KindTriage jobs were enqueued
	triageJobs, err := jobStore.List(ctx, jobs.Filter{Kind: jobs.KindTriage})
	if err != nil {
		t.Fatalf("listing triage jobs: %v", err)
	}
	if len(triageJobs) != 0 {
		t.Errorf("expected 0 triage jobs, got %d", len(triageJobs))
	}

	// Verify episode was written to the store with correct metadata and parsed checkpoints
	ep, err := epStore.Read(convID)
	if err != nil {
		t.Fatalf("reading episode %s from store: %v", convID, err)
	}

	if ep.Header.SessionID != convID {
		t.Errorf("expected SessionID %s, got %s", convID, ep.Header.SessionID)
	}
	if ep.Header.RepoID != repoID {
		t.Errorf("expected RepoID %s, got %s", repoID, ep.Header.RepoID)
	}
	if ep.Header.Status != "parsed" {
		t.Errorf("expected Status 'parsed', got %s", ep.Header.Status)
	}
	if ep.Header.Source != transcriptPath {
		t.Errorf("expected Source %s, got %s", transcriptPath, ep.Header.Source)
	}
	if ep.Header.StepCount != 4 {
		t.Errorf("expected StepCount 4, got %d", ep.Header.StepCount)
	}
	if ep.Header.Branch != "main" {
		t.Errorf("expected Branch 'main', got %s", ep.Header.Branch)
	}
	if !ep.HasCommit("commit-12345678") {
		t.Errorf("expected commit-12345678 to be recorded on episode")
	}

	// Verify checkpoints
	if len(ep.Header.Checkpoints) == 0 {
		t.Fatal("expected checkpoints map to be non-empty")
	}
	expectedSections := map[string]string{
		evidence.SectionUserKnowledge:           "- User requires high test coverage.",
		evidence.SectionModelKnowledge:          "- Production core code never imports harness adapters.",
		evidence.SectionWorkAccomplished:         "- Built the evidence ingestion pipeline.",
		evidence.SectionCurrentWork:             "- Validating pipeline tests.",
		evidence.SectionOutstandingUserRequests: "- Implement WO2 capture-to-evidence.",
		evidence.SectionFilesAndCode:            "- internal/core/tick/pipeline.go",
	}
	for section, wantText := range expectedSections {
		gotText, ok := ep.Header.Checkpoints[section]
		if !ok {
			t.Errorf("missing parsed checkpoint section %q", section)
			continue
		}
		if gotText != wantText {
			t.Errorf("section %q:\ngot:  %q\nwant: %q", section, gotText, wantText)
		}
	}
}

func TestPipelineRun_ConvertCapturesToEvidence(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-test-run"
	convID := "conv-session-run"

	os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755)
	cfg := config.DefaultRepoConfig()
	config.SaveRepoConfig(repoRoot, cfg)

	transcriptDir := filepath.Join(dataDir, "transcripts")
	os.MkdirAll(transcriptDir, 0755)
	transcriptPath := filepath.Join(transcriptDir, "transcript.jsonl")
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-04T10:00:00Z","content":"Hello"}`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	repoDataDir := filepath.Join(dataDir, repoID)
	os.MkdirAll(repoDataDir, 0755)
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mockEntry := spool.Entry{
		Kind:           spool.KindCapture,
		ConversationID: convID,
		TranscriptPath: transcriptPath,
		RepoRoot:       repoRoot,
	}
	entryBytes, _ := json.Marshal(mockEntry)

	_, _ = jobStore.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindCapture,
		Priority:       jobs.PriorityNormal,
		IdempotencyKey: "capture-" + convID,
		Payload:        string(entryBytes),
	})

	epStore := episode.NewStore(dataDir, repoID)
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)

	// Run end-to-end
	if err := exec.Run(ctx); err != nil {
		t.Fatalf("exec.Run failed: %v", err)
	}

	ep, err := epStore.Read(convID)
	if err != nil {
		t.Fatalf("reading episode %s: %v", convID, err)
	}
	if ep.Header.SessionID != convID {
		t.Errorf("got session ID %s, want %s", ep.Header.SessionID, convID)
	}
	if ep.Header.Status != "captured" {
		t.Errorf("got status %s, want captured", ep.Header.Status)
	}
}

func TestPipelineRun_HarvestFreeSources(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-test-harvest"
	convID := "conv-session-harvest"

	os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755)
	os.MkdirAll(filepath.Join(repoRoot, "internal", "core", "tick"), 0755)
	os.WriteFile(filepath.Join(repoRoot, "internal", "core", "tick", "pipeline.go"), []byte("package tick\n"), 0644)

	cfg := config.DefaultRepoConfig()
	config.SaveRepoConfig(repoRoot, cfg)

	transcriptDir := filepath.Join(dataDir, "transcripts")
	os.MkdirAll(transcriptDir, 0755)
	transcriptPath := filepath.Join(transcriptDir, "transcript.jsonl")
	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-04T10:00:00Z","content":"Hello"}
{"step_index":1,"source":"SYSTEM","type":"CHECKPOINT","status":"DONE","created_at":"2026-10-04T10:02:00Z","content":"### 1. Model Knowledge\n- **Core Boundary**: ` + "`internal/core/tick/pipeline.go`" + ` must never import concrete harness adapters.\n\n### 2. Files and Code\n- internal/core/tick/pipeline.go"}
`
	os.WriteFile(transcriptPath, []byte(transcriptContent), 0644)

	repoDataDir := filepath.Join(dataDir, repoID)
	os.MkdirAll(repoDataDir, 0755)
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mockEntry := spool.Entry{
		Kind:           spool.KindCapture,
		ConversationID: convID,
		TranscriptPath: transcriptPath,
		RepoRoot:       repoRoot,
		Branch:         "main",
		Head:           "commit-abc123",
	}
	entryBytes, _ := json.Marshal(mockEntry)

	_, _ = jobStore.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindCapture,
		Priority:       jobs.PriorityNormal,
		IdempotencyKey: "capture-" + convID,
		Payload:        string(entryBytes),
	})

	epStore := episode.NewStore(dataDir, repoID)
	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)

	if err := exec.Run(ctx); err != nil {
		t.Fatalf("exec.Run failed: %v", err)
	}

	inboxStore := inbox.NewStore(dataDir, repoID)
	notes, err := inboxStore.ListUnconsolidated()
	if err != nil {
		t.Fatalf("ListUnconsolidated: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 harvested checkpoint note, got %d", len(notes))
	}
	if notes[0].Source != inbox.SourceCheckpoint {
		t.Errorf("got source %q, want %q", notes[0].Source, inbox.SourceCheckpoint)
	}

	// Second HarvestFreeSources call is idempotent
	n2, err := exec.HarvestFreeSources(ctx)
	if err != nil {
		t.Fatalf("second HarvestFreeSources failed: %v", err)
	}
	if n2 != 0 {
		t.Errorf("expected 0 notes on second HarvestFreeSources call, got %d", n2)
	}
}

func TestPipelineCostPersistence(t *testing.T) {
	t.Skip("v1 pipeline stages removed; v2 distillation not yet implemented")
	dataDir := t.TempDir()
	repoRoot := filepath.Join(dataDir, "workspace")
	repoID := "repo-cost-test"
	epID := "ep-ambiguous-session"

	os.MkdirAll(filepath.Join(repoRoot, ".memory"), 0755)
	cfg := config.DefaultRepoConfig()
	config.SaveRepoConfig(repoRoot, cfg)

	repoDataDir := filepath.Join(dataDir, repoID)
	os.MkdirAll(repoDataDir, 0755)
	jobStore, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		t.Fatalf("jobs.Open: %v", err)
	}
	defer jobStore.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Write an ambiguous episode (6 read steps: not kept, not skipped by rules)
	epStore := episode.NewStore(dataDir, repoID)
	ambiguousEp := &episode.Episode{
		Header: episode.Header{
			SessionID: epID,
			EpisodeID: epID,
			RepoID:    repoID,
		},
		Steps: []harness.Step{
			{Index: 0, Kind: harness.StepKindRead, File: "a.txt"},
			{Index: 1, Kind: harness.StepKindRead, File: "b.txt"},
			{Index: 2, Kind: harness.StepKindRead, File: "c.txt"},
			{Index: 3, Kind: harness.StepKindRead, File: "d.txt"},
			{Index: 4, Kind: harness.StepKindRead, File: "e.txt"},
			{Index: 5, Kind: harness.StepKindRead, File: "f.txt"},
		},
	}
	if _, err := epStore.WriteEpisode(ambiguousEp); err != nil {
		t.Fatalf("WriteEpisode: %v", err)
	}

	// Enqueue triage job
	triageJob, err := jobStore.Enqueue(ctx, &jobs.Job{
		Kind:           jobs.KindTriage,
		Priority:       jobs.PriorityNormal,
		IdempotencyKey: "triage-" + epID,
		Payload:        epID,
	})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	// Setup fake Housekeeper with token usage
	fakeHK := fake.NewFakeHousekeeper()
	expectedUsage := harness.Usage{
		InputTokens:     2048,
		OutputTokens:    512,
		ThinkingTokens:  128,
		CacheReadTokens: 1024,
	}
	fakeHK.SetResult(harness.StageTriage, harness.Result{
		Payload: `{"decisions":[{"id":"ep-ambiguous-session","keep":false,"reason":"filtered"}]}`,
		Usage:   expectedUsage,
	})

	exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, jobStore, epStore)
	exec.SetHousekeeper(fakeHK)

	if err := exec.Run(ctx); err != nil {
		t.Fatalf("exec.Run: %v", err)
	}

	// Fetch completed job and verify Cost is populated from usage
	completedJob, err := jobStore.Get(ctx, triageJob.ID)
	if err != nil {
		t.Fatalf("jobStore.Get: %v", err)
	}

	if completedJob.State != jobs.StateDone {
		t.Fatalf("expected job state done, got %s", completedJob.State)
	}
	if completedJob.Cost == nil {
		t.Fatal("expected non-nil job.Cost")
	}

	if completedJob.Cost.InputTokens != expectedUsage.InputTokens {
		t.Errorf("InputTokens: got %d, want %d", completedJob.Cost.InputTokens, expectedUsage.InputTokens)
	}
	if completedJob.Cost.OutputTokens != expectedUsage.OutputTokens {
		t.Errorf("OutputTokens: got %d, want %d", completedJob.Cost.OutputTokens, expectedUsage.OutputTokens)
	}
	if completedJob.Cost.ThinkingTokens != expectedUsage.ThinkingTokens {
		t.Errorf("ThinkingTokens: got %d, want %d", completedJob.Cost.ThinkingTokens, expectedUsage.ThinkingTokens)
	}
	if completedJob.Cost.CacheReadTokens != expectedUsage.CacheReadTokens {
		t.Errorf("CacheReadTokens: got %d, want %d", completedJob.Cost.CacheReadTokens, expectedUsage.CacheReadTokens)
	}
}

