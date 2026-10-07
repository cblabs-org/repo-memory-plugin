package antigravity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestPreInvocation_FastExits(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		payload string
		env     map[string]string
		want    string
	}{
		{
			name:    "InvocationNum > 0",
			payload: `{"invocationNum": 1}`,
			want:    "{}",
		},
		{
			name:    "REPO_MEMORY_DISABLED",
			payload: `{"invocationNum": 0}`,
			env:     map[string]string{"REPO_MEMORY_DISABLED": "1"},
			want:    "{}",
		},
		{
			name:    "REPO_MEMORY_HOUSEKEEPING",
			payload: `{"invocationNum": 0}`,
			env:     map[string]string{"REPO_MEMORY_HOUSEKEEPING": "1"},
			want:    "{}",
		},
		{
			name:    "No Workspace",
			payload: `{"invocationNum": 0, "workspacePaths": []}`,
			want:    "{}",
		},
		{
			name:    "Not Enrolled",
			payload: `{"invocationNum": 0, "workspacePaths": ["/tmp/not-enrolled-repo"]}`,
			want:    "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			out, err := HandlePreInvocation(ctx, []byte(tt.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(out) != tt.want {
				t.Errorf("got %q, want %q", string(out), tt.want)
			}
		})
	}
}

func TestPreInvocation_Normalization(t *testing.T) {
	payload := []byte(`{
		"conversation_id": "conv-1",
		"transcript_path": "/tmp/transcript",
		"workspace_paths": ["/tmp/workspace"]
	}`)
	
	var input PreInvocationPayload
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatal(err)
	}
	input.Normalize()
	
	if input.ConversationID != "conv-1" {
		t.Errorf("got %q, want conv-1", input.ConversationID)
	}
	if input.TranscriptPath != "/tmp/transcript" {
		t.Errorf("got %q, want /tmp/transcript", input.TranscriptPath)
	}
	if len(input.WorkspacePaths) != 1 || input.WorkspacePaths[0] != "/tmp/workspace" {
		t.Errorf("got %v, want [/tmp/workspace]", input.WorkspacePaths)
	}
}

func setupEnrolledWorkspace(t *testing.T) (string, string) {
	t.Helper()
	workspace := t.TempDir()
	dataDir := t.TempDir()
	
	os.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Cleanup(func() { os.Unsetenv("REPO_MEMORY_DATA_DIR") })

	if err := config.CreateEnrolledMarker(workspace); err != nil {
		t.Fatal(err)
	}
	if err := config.AddToRegistry(dataDir, []string{workspace}, "test-repo", workspace); err != nil {
		t.Fatal(err)
	}
	
	idxPath := filepath.Join(dataDir, "test-repo", "index.db")
	if err := os.MkdirAll(filepath.Dir(idxPath), 0755); err != nil {
		t.Fatal(err)
	}
	
	return workspace, idxPath
}

func TestPreInvocation_TaskStart_And_Empty(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)
	
	// Test empty index
	payload := `{"invocationNum": 0, "initialNumSteps": 0, "workspacePaths": ["` + workspace + `"]}`
	out, err := HandlePreInvocation(ctx, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "{}" {
		t.Errorf("empty index: got %q, want {}", string(out))
	}

	// Seed index
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	
	entry := dossiers.Entry{
		ID:           "test-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "test context text",
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Task start with user prompt matches the scope/text
	payload2 := `{"invocationNum": 0, "initialNumSteps": 0, "userPrompt": "test context", "workspacePaths": ["` + workspace + `"]}`
	out2, err := HandlePreInvocation(ctx, []byte(payload2))
	if err != nil {
		t.Fatal(err)
	}
	
	var resp InjectStepsResponse
	if err := json.Unmarshal(out2, &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.InjectSteps) == 0 {
		t.Fatalf("expected injectSteps, got empty")
	}
	
	step := resp.InjectSteps[0]
	if step.SystemMessage == nil {
		t.Fatalf("expected systemMessage entry, got nil")
	}
	
	sysMsg := step.SystemMessage.SystemMessage
	if sysMsg == "" {
		t.Errorf("systemMessage text is empty")
	}
	
	// Check tag
	if !strings.Contains(sysMsg, "[repo-memory test-1]") {
		t.Errorf("systemMessage missing tag, got: %s", sysMsg)
	}
}

func TestPreInvocation_AboutToEdit(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)

	// Create real file in workspace
	filePath := filepath.Join(workspace, "file.go")
	if err := os.WriteFile(filePath, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Seed index with entry anchored to file.go
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := dossiers.Entry{
		ID:           "edit-entry-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "Must preserve file.go invariants",
		Anchors:      []string{"file.go"},
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpsertEntryAnchors(ctx, entry.ID, entry.Anchors); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Create transcript with prior tool touch and latest thinking mentioning "edit file.go"
	transcriptDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")

	step0 := map[string]any{"step_index": 0, "type": "USER_INPUT", "content": "Please inspect file.go"}
	step1 := map[string]any{
		"step_index": 1,
		"type":       "PLANNER_RESPONSE",
		"tool_calls": []map[string]any{
			{
				"function": map[string]any{
					"name":      "view_file",
					"arguments": fmt.Sprintf(`{"AbsolutePath": %q}`, filePath),
				},
			},
		},
	}
	step2 := map[string]any{"step_index": 2, "type": "TOOL_RESPONSE", "content": "package main"}
	step3 := map[string]any{
		"step_index": 3,
		"type":       "PLANNER_RESPONSE",
		"thinking":   "Now I will edit file.go to implement changes.",
		"content":    "Starting edit.",
	}

	b0, _ := json.Marshal(step0)
	b1, _ := json.Marshal(step1)
	b2, _ := json.Marshal(step2)
	b3, _ := json.Marshal(step3)

	content := strings.Join([]string{string(b0), string(b1), string(b2), string(b3)}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	payload := `{
		"conversationId": "conv-about-to-edit",
		"invocationNum": 0,
		"initialNumSteps": 4,
		"transcriptPath": "` + transcriptPath + `",
		"workspacePaths": ["` + workspace + `"]
	}`

	out, err := HandlePreInvocation(ctx, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}

	var resp InjectStepsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(out))
	}

	var ephemeralMsg string
	var hasSystemMsg bool
	for _, step := range resp.InjectSteps {
		if step.EphemeralMessage != "" {
			ephemeralMsg = step.EphemeralMessage
		}
		if step.SystemMessage != nil {
			hasSystemMsg = true
		}
	}

	if !hasSystemMsg {
		t.Errorf("expected first-touch systemMessage to be present")
	}

	wantEphemeral := "[repo-memory] Check injected entries above before editing file.go"
	if ephemeralMsg != wantEphemeral {
		t.Errorf("got ephemeralMessage %q, want %q", ephemeralMsg, wantEphemeral)
	}

	// Negative case: thinking mentions a file with no entries in context
	linesNoEntries := []string{
		`{"step_index": 0, "type": "PLANNER_RESPONSE", "thinking": "Now I will edit other.go to implement changes.", "content": "Starting edit."}`,
	}
	if err := os.WriteFile(transcriptPath, []byte(strings.Join(linesNoEntries, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	outNoEntries, err := HandlePreInvocation(ctx, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}

	var respNoEntries InjectStepsResponse
	_ = json.Unmarshal(outNoEntries, &respNoEntries)
	for _, step := range respNoEntries.InjectSteps {
		if step.EphemeralMessage != "" {
			t.Errorf("expected no ephemeralMessage for unseeded file, got %q", step.EphemeralMessage)
		}
	}
}
