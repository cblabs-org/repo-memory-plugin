package episode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestBuildSessionSummary_StoredEpisodeAndNotes(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "test-repo-id"

	store := NewStore(dataDir, repoID)
	exitZero := 0
	exitOne := 1
	ep := NewEpisode(
		Header{
			EpisodeID: "ep-01JABCDEF1234567890",
			SessionID: "sess-worker-01",
			Harness:   HarnessAntigravity,
			RepoID:    repoID,
			Started:   time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC),
			Ended:     time.Date(2026, 4, 15, 10, 15, 0, 0, time.UTC),
			Status:    "parsed",
		},
		harness.Step{Index: 1, Kind: harness.StepKindEdit, File: "internal/core/episode/summary.go"},
		harness.Step{Index: 2, Kind: harness.StepKindEdit, File: "cmd/repo-memory/cmd_summary.go"},
		harness.Step{Index: 3, Kind: harness.StepKindCommand, Command: "go test ./internal/core/episode/...", ExitCode: &exitOne, Excerpt: "FAIL\tinternal/core/episode [build failed]"},
		harness.Step{Index: 4, Kind: harness.StepKindCommand, Command: "go test -race ./internal/core/episode/...", ExitCode: &exitZero, Excerpt: "ok  \tinternal/core/episode\t0.123s"},
	)

	if err := store.Write(ep); err != nil {
		t.Fatalf("failed to write episode: %v", err)
	}

	notesDir := filepath.Join(dataDir, repoID, "evidence", "notes")
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	noteObj := SummaryNote{
		ID:             "note-01JXYZ9876543210",
		Source:         "turn-note",
		ConversationID: "sess-worker-01",
		Items: []SummaryNoteItem{
			{
				Type:  dossiers.EntryTypeInvariant,
				Scope: "internal/core/episode",
				Title: "Episode package must never import internal/core/evidence to avoid cyclic dependencies.",
			},
		},
	}
	noteBytes, _ := json.MarshalIndent(noteObj, "", "  ")
	if err := os.WriteFile(filepath.Join(notesDir, "note-01JXYZ9876543210.json"), noteBytes, 0o644); err != nil {
		t.Fatalf("failed to write note file: %v", err)
	}

	summary, err := SummarizeSessionInput(dataDir, repoID, repoRoot, "sess-worker-01", 400)
	if err != nil {
		t.Fatalf("SummarizeSessionInput failed: %v", err)
	}

	if summary.SessionID != "sess-worker-01" {
		t.Errorf("SessionID = %q, want %q", summary.SessionID, "sess-worker-01")
	}
	if summary.VerificationOutcome != "verified" {
		t.Errorf("VerificationOutcome = %q, want %q", summary.VerificationOutcome, "verified")
	}
	if len(summary.FilesTouched) != 2 {
		t.Fatalf("FilesTouched len = %d (%v), want 2", len(summary.FilesTouched), summary.FilesTouched)
	}
	if len(summary.NotesCaptured) != 1 || !strings.Contains(summary.NotesCaptured[0], "note-01JXYZ9876543210 [invariant] (internal/core/episode)") {
		t.Errorf("NotesCaptured = %v, want formatted note line", summary.NotesCaptured)
	}
	if len(summary.FailuresEncountered) != 1 || !strings.Contains(summary.FailuresEncountered[0], "exit 1") {
		t.Errorf("FailuresEncountered = %v, want 1 exit 1 failure", summary.FailuresEncountered)
	}
	if len(summary.CommandsRun) != 2 {
		t.Errorf("CommandsRun = %v, want 2 commands", summary.CommandsRun)
	}
	if summary.TokenCount <= 0 || summary.TokenCount > 400 {
		t.Errorf("TokenCount = %d, want in (0, 400]", summary.TokenCount)
	}
	if dossiers.EstimateTokens(summary.SummaryMarkdown) != summary.TokenCount {
		t.Errorf("EstimateTokens(SummaryMarkdown) = %d, TokenCount = %d", dossiers.EstimateTokens(summary.SummaryMarkdown), summary.TokenCount)
	}
}

func TestSummarizeSessionInput_FlatJSONEpisodeAndNotes(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-flat"

	epDir := filepath.Join(dataDir, "repos", repoID, "evidence", "episodes")
	notesDir := filepath.Join(dataDir, "repos", repoID, "evidence", "notes")
	if err := os.MkdirAll(epDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(notesDir, 0o755); err != nil {
		t.Fatal(err)
	}

	epPayload := map[string]any{
		"schema_version":        "v1",
		"session_id":            "sess-flat-01",
		"harness":               "claude-code",
		"timestamp":             "2026-04-16T12:00:00Z",
		"touched_files":         []string{"internal/core/delivery/workorder_brief.go"},
		"tool_sequence":         []string{"go vet ./...", "go test ./..."},
		"verification_commands": []string{"go test ./..."},
		"tool_errors": []map[string]any{
			{
				"command":       "go vet ./...",
				"exit_code":     2,
				"stderr_digest": "undefined: EstimateTokens",
			},
		},
		"note_ids":         []string{"note-flat-1", "note-unloaded-2"},
		"terminal_success": true,
	}
	epBytes, _ := json.MarshalIndent(epPayload, "", "  ")
	if err := os.WriteFile(filepath.Join(epDir, "sess-flat-01.json"), epBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	notePayload := map[string]any{
		"id":         "note-flat-1",
		"session_id": "sess-flat-01",
		"kind":       "gotcha",
		"scope":      "internal/core/delivery",
		"statement":  "Avoid cyclic imports between delivery and coverage.",
	}
	noteBytes, _ := json.MarshalIndent(notePayload, "", "  ")
	if err := os.WriteFile(filepath.Join(notesDir, "note-flat-1.json"), noteBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := SummarizeSessionInput(dataDir, repoID, repoRoot, "sess-flat-01", 400)
	if err != nil {
		t.Fatalf("SummarizeSessionInput failed: %v", err)
	}
	if summary.VerificationOutcome != "verified" {
		t.Errorf("VerificationOutcome = %q, want verified", summary.VerificationOutcome)
	}
	if len(summary.NotesCaptured) != 2 {
		t.Fatalf("NotesCaptured = %v, want 2 notes", summary.NotesCaptured)
	}
	if !strings.Contains(summary.NotesCaptured[0], "note-flat-1 [gotcha] (internal/core/delivery): Avoid cyclic imports") {
		t.Errorf("unexpected NotesCaptured[0]: %q", summary.NotesCaptured[0])
	}
	if summary.NotesCaptured[1] != "note-unloaded-2" {
		t.Errorf("unexpected NotesCaptured[1]: %q", summary.NotesCaptured[1])
	}
}

func TestSummarizeSessionInput_RawTranscriptDirAndFile(t *testing.T) {
	dataDir := t.TempDir()
	repoRoot := t.TempDir()
	repoID := "repo-raw"

	convDir := filepath.Join(t.TempDir(), "conv-uuid-1234")
	logsDir := filepath.Join(convDir, ".system_generated", "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	targetFileAbs := filepath.Join(repoRoot, "internal", "core", "dossiers", "lint.go")
	lines := []string{
		`{"step_index":1,"source":"USER_EXPLICIT","type":"USER_INPUT","created_at":"2026-04-16T09:00:00Z","content":"Export EstimateTokens"}`,
		fmt.Sprintf(`{"step_index":2,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-04-16T09:01:00Z","tool_calls":[{"name":"edit_file","args":{"Path":%q}}]}`, targetFileAbs),
		`{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","created_at":"2026-04-16T09:02:00Z","tool_calls":[{"name":"run_command","args":{"CommandLine":"go test ./internal/core/dossiers/..."}}]}`,
		`{"step_index":4,"source":"SYSTEM","type":"TOOL_OUTPUT","status":"DONE","created_at":"2026-04-16T09:02:05Z","content":"ok  \tinternal/core/dossiers\t0.02s"}`,
	}
	transcriptPath := filepath.Join(logsDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	summary, err := SummarizeSessionInput(dataDir, repoID, repoRoot, convDir, 400)
	if err != nil {
		t.Fatalf("SummarizeSessionInput on convDir failed: %v", err)
	}
	if summary.SessionID != "conv-uuid-1234" {
		t.Errorf("SessionID = %q, want conv-uuid-1234", summary.SessionID)
	}
	if summary.VerificationOutcome != "verified" {
		t.Errorf("VerificationOutcome = %q, want verified", summary.VerificationOutcome)
	}
	if len(summary.FilesTouched) != 1 || summary.FilesTouched[0] != "internal/core/dossiers/lint.go" {
		t.Errorf("FilesTouched = %v, want [internal/core/dossiers/lint.go]", summary.FilesTouched)
	}

	claudeFile := filepath.Join(t.TempDir(), "claude-sess-99.jsonl")
	claudeLines := []string{
		`{"sessionId":"claude-sess-99","timestamp":"2026-04-16T11:00:00Z","type":"assistant","message":{"content":[{"type":"tool_use","id":"tu_1","name":"Edit","input":{"file_path":"cmd/repo-memory/main.go"}},{"type":"tool_use","id":"tu_2","name":"Bash","input":{"command":"go build ./cmd/repo-memory"}}]}}`,
		`{"sessionId":"claude-sess-99","timestamp":"2026-04-16T11:00:05Z","type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tu_2","is_error":true,"content":"main.go:12:2: undefined: summaryCmd\nexit status 2"}]}}`,
	}
	if err := os.WriteFile(claudeFile, []byte(strings.Join(claudeLines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ccSummary, err := SummarizeSessionInput(dataDir, repoID, repoRoot, claudeFile, 400)
	if err != nil {
		t.Fatalf("SummarizeSessionInput on claudeFile failed: %v", err)
	}
	if ccSummary.SessionID != "claude-sess-99" {
		t.Errorf("SessionID = %q, want claude-sess-99", ccSummary.SessionID)
	}
	if ccSummary.VerificationOutcome != "failed" {
		t.Errorf("VerificationOutcome = %q, want failed", ccSummary.VerificationOutcome)
	}
	if len(ccSummary.FailuresEncountered) != 1 || !strings.Contains(ccSummary.FailuresEncountered[0], "exit 2") {
		t.Errorf("FailuresEncountered = %v, want exit 2 failure", ccSummary.FailuresEncountered)
	}
}

func TestBuildSessionSummary_TokenBudgetEnforcement(t *testing.T) {
	var steps []harness.Step
	exitErr := 1
	exitOK := 0
	for i := 0; i < 120; i++ {
		steps = append(steps, harness.Step{
			Index: i*2 + 1,
			Kind:  harness.StepKindEdit,
			File:  fmt.Sprintf("internal/core/subsystem_%03d/deeply/nested/component_file_%03d.go", i, i),
		})
		steps = append(steps, harness.Step{
			Index:    i*2 + 2,
			Kind:     harness.StepKindCommand,
			Command:  fmt.Sprintf("go test -v -race ./internal/core/subsystem_%03d/...", i),
			ExitCode: &exitErr,
			Excerpt:  fmt.Sprintf("FAIL: TestSubsystem%03d encountered an unexpected timeout while waiting for database lock", i),
		})
	}
	steps = append(steps, harness.Step{
		Index:    300,
		Kind:     harness.StepKindCommand,
		Command:  "go test ./...",
		ExitCode: &exitOK,
		Excerpt:  "ok",
	})

	var notes []*SummaryNote
	for i := 0; i < 30; i++ {
		notes = append(notes, &SummaryNote{
			ID:             fmt.Sprintf("note-%03d", i),
			ConversationID: "sess-huge",
			Items: []SummaryNoteItem{
				{
					Type:  dossiers.EntryTypeGotcha,
					Scope: fmt.Sprintf("internal/core/subsystem_%03d", i),
					Title: fmt.Sprintf("Subsystem %03d requires explicit context cancellation before closing SQLite connections.", i),
				},
			},
		})
	}

	ep := NewEpisode(Header{
		EpisodeID: "ep-huge",
		SessionID: "sess-huge",
		Harness:   HarnessAntigravity,
		Started:   time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC),
		Ended:     time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC),
	}, steps...)

	s400 := BuildSessionSummary(ep, notes, 400)
	if s400.TokenCount > 400 {
		t.Errorf("s400.TokenCount = %d, exceeds 400", s400.TokenCount)
	}
	if !strings.Contains(s400.SummaryMarkdown, "more)") {
		t.Errorf("expected truncation indicator (+N more) in s400.SummaryMarkdown:\n%s", s400.SummaryMarkdown)
	}

	s90 := BuildSessionSummary(ep, notes, 90)
	if s90.TokenCount > 90 {
		t.Errorf("s90.TokenCount = %d, exceeds 90", s90.TokenCount)
	}
}
