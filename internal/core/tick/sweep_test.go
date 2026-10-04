package tick

import (
	"strings"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
)

type mockAttributor struct {
	sessions []AttributedSession
	err      error
}

func (m *mockAttributor) AttributedSessions(ctx context.Context, enrolledRepos map[string]config.RegistryEntry) ([]AttributedSession, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.sessions, nil
}

func TestSweep_WithAttributor(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	repoID := "repo-xyz"

	// Create source transcripts
	srcDir := filepath.Join(tempDir, "brain")
	_ = os.MkdirAll(srcDir, 0755)

	t1Path := filepath.Join(srcDir, "t1.jsonl")
	_ = os.WriteFile(t1Path, []byte("{\"step\":0,\"content\":\"turn 1\"}\n"), 0644)

	tSubPath := filepath.Join(srcDir, "tsub.jsonl")
	_ = os.WriteFile(tSubPath, []byte("{\"step\":0,\"content\":\"subagent turn\"}\n"), 0644)

	tActivePath := filepath.Join(srcDir, "tactive.jsonl")
	_ = os.WriteFile(tActivePath, []byte("{\"step\":0,\"content\":\"active turn\"}\n"), 0644)

	enrolledRepos := map[string]config.RegistryEntry{
		"/workspace/repo": {
			RepoID:   repoID,
			RootPath: "/workspace/repo",
		},
	}

	mockAttr := &mockAttributor{
		sessions: []AttributedSession{
			{
				ConversationID:       "conv-idle",
				ParentConversationID: "",
				RepoID:               repoID,
				RepoRoot:             "/workspace/repo",
				TranscriptPath:       t1Path,
				Idle:                 true,
				Source:               "test",
			},
			{
				ConversationID:       "conv-subagent",
				ParentConversationID: "conv-idle",
				RepoID:               repoID,
				RepoRoot:             "/workspace/repo",
				TranscriptPath:       tSubPath,
				Idle:                 true,
				Source:               "test",
			},
			{
				ConversationID:       "conv-active",
				ParentConversationID: "",
				RepoID:               repoID,
				RepoRoot:             "/workspace/repo",
				TranscriptPath:       tActivePath,
				Idle:                 false, // Not idle
				Source:               "test",
			},
			{
				ConversationID:       "conv-missing-transcript",
				ParentConversationID: "",
				RepoID:               repoID,
				RepoRoot:             "/workspace/repo",
				TranscriptPath:       "", // Missing transcript
				Idle:                 true,
				Source:               "test",
			},
		},
	}

	ctx := context.Background()
	if err := Sweep(ctx, dataDir, enrolledRepos, mockAttr); err != nil {
		t.Fatalf("Sweep failed: %v", err)
	}

	queueDir := filepath.Join(dataDir, repoID, "queue")

	// 1. Verify conv-idle was copied into queue
	idleDst := filepath.Join(queueDir, "conv-idle.jsonl")
	if _, err := os.Stat(idleDst); err != nil {
		t.Errorf("expected conv-idle to be copied to %s, got: %v", idleDst, err)
	}

	// 2. Verify conv-subagent was copied into queue
	subDst := filepath.Join(queueDir, "conv-subagent.jsonl")
	if _, err := os.Stat(subDst); err != nil {
		t.Errorf("expected conv-subagent to be copied to %s, got: %v", subDst, err)
	}

	// 3. Verify conv-active was NOT copied (it's not idle)
	activeDst := filepath.Join(queueDir, "conv-active.jsonl")
	if _, err := os.Stat(activeDst); !os.IsNotExist(err) {
		t.Errorf("expected conv-active NOT to be copied because it is active")
	}

	// 4. Verify capture jobs were enqueued
	jobDBPath := filepath.Join(dataDir, repoID, "jobs.sqlite")
	store, err := jobs.Open(jobDBPath)
	if err != nil {
		t.Fatalf("opening job db: %v", err)
	}
	defer store.Close()

	jobsList, err := store.List(ctx, jobs.Filter{})
	if err != nil {
		t.Fatalf("listing jobs: %v", err)
	}

	if len(jobsList) != 2 {
		t.Fatalf("expected 2 jobs queued (conv-idle, conv-subagent), got %d", len(jobsList))
	}

	// Find the subagent job and verify its payload contains parent_conversation_id
	var foundSubagentJob bool
	for _, j := range jobsList {
		if strings.HasPrefix(j.IdempotencyKey, "sweep-conv-subagent") {
			foundSubagentJob = true
			var payload map[string]any
			if err := json.Unmarshal([]byte(j.Payload), &payload); err != nil {
				t.Fatalf("parsing subagent job payload: %v", err)
			}
			if payload["parent_conversation_id"] != "conv-idle" {
				t.Errorf("expected parent_conversation_id 'conv-idle', got %v", payload["parent_conversation_id"])
			}
			if payload["conversation_id"] != "conv-subagent" {
				t.Errorf("expected conversation_id 'conv-subagent', got %v", payload["conversation_id"])
			}
		}
	}

	if !foundSubagentJob {
		t.Error("job for conv-subagent was not found in jobs table")
	}
}

func TestSweep_EmptyAttributors(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	enrolledRepos := map[string]config.RegistryEntry{}

	ctx := context.Background()
	// Calling with empty slice and no defaults should cleanly succeed as no-op
	if err := Sweep(ctx, dataDir, enrolledRepos); err != nil {
		t.Errorf("expected nil error for empty sweep, got: %v", err)
	}
}

func TestSweep_ArtifactCapture(t *testing.T) {
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, "data")
	repoID := "repo-xyz"

	// Create brain dir with artifacts and transcript
	brainDir := filepath.Join(tempDir, "brain", "conv-idle-art")
	sysLogsDir := filepath.Join(brainDir, ".system_generated", "logs")
	if err := os.MkdirAll(sysLogsDir, 0755); err != nil {
		t.Fatalf("creating sysLogsDir: %v", err)
	}

	transcriptPath := filepath.Join(sysLogsDir, "transcript_full.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("{\"step\":0,\"content\":\"hello\"}\n"), 0644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}

	designMd := filepath.Join(brainDir, "design.md")
	if err := os.WriteFile(designMd, []byte("# System Design\n"), 0644); err != nil {
		t.Fatalf("writing design.md: %v", err)
	}

	designMeta := filepath.Join(brainDir, "design.md.metadata.json")
	if err := os.WriteFile(designMeta, []byte(`{"title":"Design"}`), 0644); err != nil {
		t.Fatalf("writing design.md.metadata.json: %v", err)
	}

	enrolledRepos := map[string]config.RegistryEntry{
		"/workspace/repo": {
			RepoID:   repoID,
			RootPath: "/workspace/repo",
		},
	}

	mockAttr := &mockAttributor{
		sessions: []AttributedSession{
			{
				ConversationID: "conv-idle-art",
				RepoID:         repoID,
				RepoRoot:       "/workspace/repo",
				TranscriptPath: transcriptPath,
				Idle:           true,
				Source:         "test",
			},
		},
	}

	ctx := context.Background()
	if err := Sweep(ctx, dataDir, enrolledRepos, mockAttr); err != nil {
		t.Fatalf("Sweep failed: %v", err)
	}

	// Verify transcript was copied into queue
	queueFile := filepath.Join(dataDir, repoID, "queue", "conv-idle-art.jsonl")
	if _, err := os.Stat(queueFile); err != nil {
		t.Fatalf("queue file not found: %v", err)
	}

	// Verify artifacts were copied into evidence/artifacts/conv-idle-art
	dstArtifactDir := filepath.Join(dataDir, repoID, "evidence", "artifacts", "conv-idle-art")
	if _, err := os.Stat(filepath.Join(dstArtifactDir, "design.md")); err != nil {
		t.Fatalf("design.md was not copied to evidence store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstArtifactDir, "design.md.metadata.json")); err != nil {
		t.Fatalf("design.md.metadata.json was not copied to evidence store: %v", err)
	}
}

