/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

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
	"github.com/cblabs-org/repo-memory-plugin/internal/core/state"
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
	payloadMatch := fmt.Sprintf(`{
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
		"workspacePaths": [%q]
	}`, workspace)

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
	payloadNoMatch := fmt.Sprintf(`{
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
		"workspacePaths": [%q]
	}`, workspace)

	outNoMatch, err := HandlePostToolUse(ctx, []byte(payloadNoMatch))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(outNoMatch) != "{}" {
		t.Errorf("expected {}, got %s", string(outNoMatch))
	}
}

func TestPostToolUse_InvokeSubagentRegistry(t *testing.T) {
	ctx := context.Background()
	dataDir, repoDir, repoID := setupEnrolledRepo(t)
	if err := config.CreateEnrolledMarker(repoDir); err != nil {
		t.Fatalf("CreateEnrolledMarker: %v", err)
	}

	parentID := "3ce5d33d-ed0b-4d4a-8f21-5c33e7ce80c5"
	childID := "e145e070-4d7a-42d4-89f1-305f1d520548"

	payload := fmt.Sprintf(`{
		"conversationId": %q,
		"toolName": "invoke_subagent",
		"toolCall": {
			"name": "invoke_subagent",
			"args": {"TypeName": "self", "Role": "WO1 Worker Agent", "Prompt": "Execute WO1"}
		},
		"toolResult": {
			"status": "success",
			"output": "{\"conversationId\": \"%s\", \"status\": \"running\"}"
		},
		"workspacePaths": [%q]
	}`, parentID, childID, repoDir)

	out, err := HandlePostToolUse(ctx, []byte(payload), WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("HandlePostToolUse invoke_subagent failed: %v", err)
	}
	if string(out) != "{}" {
		t.Errorf("expected {}, got %s", string(out))
	}

	isSub, gotParent, err := state.IsSubagent(ctx, dataDir, repoID, childID)
	if err != nil {
		t.Fatalf("IsSubagent failed: %v", err)
	}
	if !isSub || gotParent != parentID {
		t.Errorf("expected child %s registered under parent %s, got isSub=%v parent=%q", childID, parentID, isSub, gotParent)
	}

	children, err := state.ListSubagentChildren(ctx, dataDir, repoID, parentID)
	if err != nil || len(children) != 1 || children[0] != childID {
		t.Errorf("expected children [%s], got %v (err=%v)", childID, children, err)
	}
}

func TestPostToolUse_RecipeCommandOutcomeTracking(t *testing.T) {
	ctx := context.Background()
	dataDir, repoDir, repoID := setupEnrolledRepo(t)
	if err := config.CreateEnrolledMarker(repoDir); err != nil {
		t.Fatalf("CreateEnrolledMarker: %v", err)
	}

	idxPath := index.IndexPath(dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		t.Fatal(err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	recipe := dossiers.Entry{
		ID:           "rec-test-suite",
		Type:         dossiers.EntryTypeRecipe,
		Kind:         dossiers.CognitiveKindProcedural,
		DossierScope: "core",
		Text:         "Run Go unit tests\n1. Run `go vet ./...`\n2. Run `go test ./...`",
		Anchors:      []string{"go.mod"},
	}
	if err := idx.UpsertEntry(ctx, recipe); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Success in conv-1
	payload1 := fmt.Sprintf(`{
		"conversationId": "conv-1",
		"toolName": "run_command",
		"toolCall": {
			"name": "run_command",
			"args": {"CommandLine": "go test ./..."}
		},
		"toolResult": {
			"status": "success",
			"output": "Command exited with code 0\nok"
		},
		"workspacePaths": [%q]
	}`, repoDir)
	if _, err := HandlePostToolUse(ctx, []byte(payload1), WithDataDir(dataDir)); err != nil {
		t.Fatalf("HandlePostToolUse conv-1 failed: %v", err)
	}

	// Success in conv-2 -> graduates skill
	payload2 := fmt.Sprintf(`{
		"conversationId": "conv-2",
		"toolName": "bash",
		"toolCall": {
			"name": "bash",
			"args": {"command": "go vet ./... && go test ./..."}
		},
		"toolResult": {
			"status": "success",
			"output": "PASS"
		},
		"workspacePaths": [%q]
	}`, repoDir)
	if _, err := HandlePostToolUse(ctx, []byte(payload2), WithDataDir(dataDir)); err != nil {
		t.Fatalf("HandlePostToolUse conv-2 failed: %v", err)
	}

	idx, err = index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	got, err := idx.GetEntry(ctx, "rec-test-suite")
	idx.Close()
	if err != nil {
		t.Fatal(err)
	}
	if got.SuccessCount != 2 || got.FailureCount != 0 || got.StaleSuspect {
		t.Errorf("expected SuccessCount=2 FailureCount=0 StaleSuspect=false, got %+v", got)
	}
}
