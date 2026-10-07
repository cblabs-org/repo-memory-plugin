package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestPostToolUse_FastExits(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		payload string
		env     map[string]string
		want    string
	}{
		{
			name:    "REPO_MEMORY_DISABLED",
			payload: `{"toolResult": {"status": "error"}}`,
			env:     map[string]string{"REPO_MEMORY_DISABLED": "1"},
			want:    "{}",
		},
		{
			name:    "REPO_MEMORY_HOUSEKEEPING",
			payload: `{"toolResult": {"status": "error"}}`,
			env:     map[string]string{"REPO_MEMORY_HOUSEKEEPING": "1"},
			want:    "{}",
		},
		{
			name:    "Non-error status",
			payload: `{"toolResult": {"status": "success"}}`,
			want:    "{}",
		},
		{
			name:    "No Workspace",
			payload: `{"toolResult": {"status": "error"}, "workspacePaths": []}`,
			want:    "{}",
		},
		{
			name:    "Not Enrolled",
			payload: `{"toolResult": {"status": "error"}, "workspacePaths": ["/tmp/not-enrolled-repo"]}`,
			want:    "{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			out, err := HandlePostToolUse(ctx, []byte(tt.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(out) != tt.want {
				t.Errorf("got %q, want %q", string(out), tt.want)
			}
		})
	}
}

func TestPostToolUse_Normalization(t *testing.T) {
	payload := []byte(`{
		"conversation_id": "conv-1",
		"transcript_path": "/tmp/transcript",
		"workspace_paths": ["/tmp/workspace"],
		"toolCall": {"name": "run_command"}
	}`)

	var input PostToolUsePayload
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
	if input.ToolName != "run_command" {
		t.Errorf("got %q, want run_command", input.ToolName)
	}
}

func TestPostToolUse_ErrorMatching(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)

	// Seed index with failure signature and error pattern
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}

	entry := dossiers.Entry{
		ID:           "fix-panic-1",
		Type:         dossiers.EntryTypeFailureSignature,
		DossierScope: "test",
		Text:         "Check nil pointer before calling method",
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := idx.InsertErrorPattern(ctx, "fix-panic-1", "go", "panic: runtime error", "test"); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Error with matching command prefix and pattern
	payloadMatch := `{
		"conversationId": "conv-123",
		"toolName": "run_command",
		"toolCall": {
			"name": "run_command",
			"args": {"CommandLine": "go test ./..."}
		},
		"toolResult": {
			"status": "error",
			"output": "panic: runtime error: invalid memory address"
		},
		"workspacePaths": ["` + workspace + `"]
	}`

	out, err := HandlePostToolUse(ctx, []byte(payloadMatch))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp InjectStepsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(out))
	}
	if len(resp.InjectSteps) != 1 {
		t.Fatalf("expected 1 injectStep, got %d", len(resp.InjectSteps))
	}

	ephemeral := resp.InjectSteps[0].EphemeralMessage
	if ephemeral == "" {
		t.Fatal("expected ephemeralMessage, got empty")
	}
	if !strings.Contains(ephemeral, "[repo-memory fix-panic-1]") {
		t.Errorf("expected tag [repo-memory fix-panic-1] in ephemeralMessage, got %s", ephemeral)
	}
	if !strings.Contains(ephemeral, "Check nil pointer") {
		t.Errorf("expected entry text in ephemeralMessage, got %s", ephemeral)
	}

	// Error with unknown command prefix -> returns {}
	payloadNoMatch := `{
		"conversationId": "conv-123",
		"toolName": "run_command",
		"toolCall": {
			"name": "run_command",
			"args": {"CommandLine": "cargo build"}
		},
		"toolResult": {
			"status": "error",
			"output": "cargo error: could not compile"
		},
		"workspacePaths": ["` + workspace + `"]
	}`

	outNoMatch, err := HandlePostToolUse(ctx, []byte(payloadNoMatch))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(outNoMatch) != "{}" {
		t.Errorf("expected {}, got %s", string(outNoMatch))
	}
}
