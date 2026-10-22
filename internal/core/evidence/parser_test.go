/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package evidence

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

func TestParseTranscript_FileWithCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptPath := filepath.Join(tmpDir, "transcript.jsonl")

	transcriptContent := `{"step_index":0,"source":"USER_EXPLICIT","type":"USER_INPUT","status":"DONE","created_at":"2026-10-04T12:00:00Z","content":"Please check the system"}
{"step_index":1,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-04T12:01:00Z","tool_calls":[{"name":"view_file","args":{"AbsolutePath":"/path/to/file.go"}}]}
{"step_index":2,"source":"SYSTEM","type":"CHECKPOINT","status":"DONE","created_at":"2026-10-04T12:02:00Z","content":"### 1. User Knowledge\n- User wants clean tests\n\n### 2. Model Knowledge\n- Boundary test forbids adapters\n\n### 3. Work Accomplished\n- Wrote parser\n\n### 4. Current Work\n- Writing tests\n\n### 5. Outstanding User Requests\n- Fix capture to evidence\n\n### 6. Files and Code\n- internal/core/tick/pipeline.go"}
{"step_index":3,"source":"MODEL","type":"PLANNER_RESPONSE","status":"DONE","created_at":"2026-10-04T12:03:00Z","content":"Done with the work"}
`
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("writing test transcript: %v", err)
	}

	parsed, err := ParseTranscript(transcriptPath)
	if err != nil {
		t.Fatalf("ParseTranscript failed: %v", err)
	}

	if len(parsed.Steps) != 4 {
		t.Errorf("expected 4 steps, got %d", len(parsed.Steps))
	}

	if parsed.Steps[0].Kind != harness.StepKindUser {
		t.Errorf("step 0 kind: got %s, want %s", parsed.Steps[0].Kind, harness.StepKindUser)
	}
	if parsed.Steps[1].Kind != harness.StepKindRead || parsed.Steps[1].File != "/path/to/file.go" {
		t.Errorf("step 1 kind/file: got %s / %s", parsed.Steps[1].Kind, parsed.Steps[1].File)
	}

	if !parsed.HasCheckpoint {
		t.Error("expected HasCheckpoint to be true")
	}

	if len(parsed.Checkpoints) == 0 {
		t.Fatal("expected checkpoints map to be non-empty")
	}

	if parsed.Checkpoints[SectionUserKnowledge] != "- User wants clean tests" {
		t.Errorf("User Knowledge: got %q", parsed.Checkpoints[SectionUserKnowledge])
	}
	if parsed.Checkpoints[SectionModelKnowledge] != "- Boundary test forbids adapters" {
		t.Errorf("Model Knowledge: got %q", parsed.Checkpoints[SectionModelKnowledge])
	}
	if parsed.Checkpoints[SectionWorkAccomplished] != "- Wrote parser" {
		t.Errorf("Work Accomplished: got %q", parsed.Checkpoints[SectionWorkAccomplished])
	}
	if parsed.Checkpoints[SectionCurrentWork] != "- Writing tests" {
		t.Errorf("Current Work: got %q", parsed.Checkpoints[SectionCurrentWork])
	}
	if parsed.Checkpoints[SectionOutstandingUserRequests] != "- Fix capture to evidence" {
		t.Errorf("Outstanding User Requests: got %q", parsed.Checkpoints[SectionOutstandingUserRequests])
	}
	if parsed.Checkpoints[SectionFilesAndCode] != "- internal/core/tick/pipeline.go" {
		t.Errorf("Files and Code: got %q", parsed.Checkpoints[SectionFilesAndCode])
	}

	// Test BuildEpisode
	entry := spool.Entry{
		ConversationID: "conv-12345",
		TranscriptPath: transcriptPath,
		Branch:         "main",
		Head:           "commit-abc",
		Timestamp:      time.Now().UTC(),
	}

	ep := BuildEpisode(entry, parsed, "repo-test")
	if ep.Header.SessionID != "conv-12345" {
		t.Errorf("session ID: got %s, want conv-12345", ep.Header.SessionID)
	}
	if ep.Header.RepoID != "repo-test" {
		t.Errorf("repo ID: got %s, want repo-test", ep.Header.RepoID)
	}
	if ep.Header.Status != "parsed" {
		t.Errorf("status: got %s, want parsed", ep.Header.Status)
	}
	if ep.Header.StepCount != 4 {
		t.Errorf("step count: got %d, want 4", ep.Header.StepCount)
	}
	if ep.Header.Source != transcriptPath {
		t.Errorf("source: got %s, want %s", ep.Header.Source, transcriptPath)
	}
	if len(ep.Header.Checkpoints) != len(parsed.Checkpoints) {
		t.Errorf("header checkpoints count mismatch")
	}
}
