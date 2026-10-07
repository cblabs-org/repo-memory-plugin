package antigravity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

func TestPreToolUse_AllowByDefault(t *testing.T) {
	ctx := context.Background()

	cases := []struct {
		name    string
		payload string
	}{
		{"empty json", `{}`},
		{"empty string", ``},
		{"invalid json", `{not-json`},
		{"minimal payload", `{"conversationId": "c1"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := HandlePreToolUse(ctx, []byte(tc.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(out) != string(allowResponse) {
				t.Errorf("got %q, want %q", string(out), string(allowResponse))
			}
		})
	}
}

func TestPreToolUse_FastExits(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		payload string
		env     map[string]string
	}{
		{
			name:    "REPO_MEMORY_DISABLED",
			payload: `{"toolName": "replace_file_content", "toolCall": {"name": "replace_file_content", "args": {"TargetFile": "/path/to/file.go"}}}`,
			env:     map[string]string{"REPO_MEMORY_DISABLED": "1"},
		},
		{
			name:    "REPO_MEMORY_HOUSEKEEPING",
			payload: `{"toolName": "replace_file_content", "toolCall": {"name": "replace_file_content", "args": {"TargetFile": "/path/to/file.go"}}}`,
			env:     map[string]string{"REPO_MEMORY_HOUSEKEEPING": "1"},
		},
		{
			name:    "Non-edit tool view_file",
			payload: `{"toolName": "view_file", "toolCall": {"name": "view_file", "args": {"AbsolutePath": "/path/to/file.go"}}}`,
		},
		{
			name:    "Non-edit tool run_command",
			payload: `{"toolName": "run_command", "toolCall": {"name": "run_command", "args": {"CommandLine": "rm -rf /"}}}`,
		},
		{
			name:    "Not enrolled workspace",
			payload: `{"toolName": "replace_file_content", "toolCall": {"name": "replace_file_content", "args": {"TargetFile": "/tmp/test.go"}}, "workspacePaths": ["/tmp/not-enrolled-repo"]}`,
		},
		{
			name:    "No target file in args",
			payload: `{"toolName": "replace_file_content", "toolCall": {"name": "replace_file_content", "args": {}}, "workspacePaths": ["/tmp/workspace"]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			out, err := HandlePreToolUse(ctx, []byte(tt.payload))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(out) != string(allowResponse) {
				t.Errorf("got %q, want %q", string(out), string(allowResponse))
			}
		})
	}
}

func TestPreToolUse_Normalization(t *testing.T) {
	payload := []byte(`{
		"conversation_id": "conv-42",
		"transcript_path": "/tmp/transcript.jsonl",
		"workspace_paths": ["/tmp/workspace"],
		"toolCall": {"name": "replace_file_content"}
	}`)

	var input PreToolUsePayload
	if err := json.Unmarshal(payload, &input); err != nil {
		t.Fatal(err)
	}
	input.Normalize()

	if input.ConversationID != "conv-42" {
		t.Errorf("got %q, want conv-42", input.ConversationID)
	}
	if input.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("got %q, want /tmp/transcript.jsonl", input.TranscriptPath)
	}
	if len(input.WorkspacePaths) != 1 || input.WorkspacePaths[0] != "/tmp/workspace" {
		t.Errorf("got %v, want [/tmp/workspace]", input.WorkspacePaths)
	}
	if input.ToolName != "replace_file_content" {
		t.Errorf("got %q, want replace_file_content", input.ToolName)
	}
}

func TestPreToolUse_EditWithNoGate(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)

	// Create an empty index (no entries)
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	idx.Close()

	payload := `{
		"conversationId": "conv-test",
		"toolName": "replace_file_content",
		"toolCall": {
			"name": "replace_file_content",
			"args": {"TargetFile": "` + filepath.Join(workspace, "safe_file.go") + `"}
		},
		"workspacePaths": ["` + workspace + `"]
	}`

	out, err := HandlePreToolUse(ctx, []byte(payload))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string(allowResponse) {
		t.Errorf("got %q, want %q", string(out), string(allowResponse))
	}
}

func TestPreToolUse_EditWithGate(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)

	// Create target file on disk so anchor verification passes
	targetFile := filepath.Join(workspace, "critical_config.go")
	if err := os.WriteFile(targetFile, []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Seed index with gate: edit entry anchored to target file
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}

	entry := dossiers.Entry{
		ID:           "gate-crit-01",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "global",
		Text:         "Do not modify critical config without architectural review gate: edit",
		Anchors:      []string{"critical_config.go"},
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpsertEntryAnchors(ctx, entry.ID, entry.Anchors); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// 1. Test replace_file_content with absolute TargetFile path
	payloadReplace := `{
		"conversationId": "conv-test-gate",
		"toolName": "replace_file_content",
		"toolCall": {
			"name": "replace_file_content",
			"args": {"TargetFile": "` + targetFile + `"}
		},
		"workspacePaths": ["` + workspace + `"]
	}`

	out, err := HandlePreToolUse(ctx, []byte(payloadReplace))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var resp PreToolUseResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(out))
	}

	if resp.Decision != "force_ask" {
		t.Errorf("expected decision 'force_ask', got %q", resp.Decision)
	}
	if !strings.Contains(resp.Reason, "[REPO-MEMORY") {
		t.Errorf("expected '[REPO-MEMORY' tag in reason, got %q", resp.Reason)
	}
	if !strings.Contains(resp.Reason, "gate-crit-01") {
		t.Errorf("expected entry ID 'gate-crit-01' in reason, got %q", resp.Reason)
	}
	if !strings.Contains(resp.Reason, "Do not modify critical config without architectural review") {
		t.Errorf("expected gate text in reason, got %q", resp.Reason)
	}

	// 2. Re-issued edit with same conversationID — gate is pending, still force_ask.
	payloadWrite := `{
		"conversationId": "conv-test-gate",
		"toolName": "write_to_file",
		"toolCall": {
			"name": "write_to_file",
			"args": {"TargetFile": "critical_config.go"}
		},
		"workspacePaths": ["` + workspace + `"]
	}`

	outWrite, err := HandlePreToolUse(ctx, []byte(payloadWrite))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var respWrite PreToolUseResponse
	if err := json.Unmarshal(outWrite, &respWrite); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(outWrite))
	}

	if respWrite.Decision != "force_ask" {
		t.Errorf("expected decision 'force_ask' for re-issued edit (pending, not yet approved), got %q", respWrite.Decision)
	}

	// 3. Simulate PostToolUse with successful edit — acknowledges pending gates.
	postPayload := `{
		"conversationId": "conv-test-gate",
		"toolName": "write_to_file",
		"toolCall": {
			"name": "write_to_file",
			"args": {"TargetFile": "critical_config.go"}
		},
		"toolResult": {"status": "success", "output": ""},
		"workspacePaths": ["` + workspace + `"]
	}`
	_, err = HandlePostToolUse(ctx, []byte(postPayload))
	if err != nil {
		t.Fatalf("unexpected error from PostToolUse: %v", err)
	}

	// 4. Now PreToolUse should allow — gates are acknowledged.
	outAfter, err := HandlePreToolUse(ctx, []byte(payloadWrite))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var respAfter PreToolUseResponse
	if err := json.Unmarshal(outAfter, &respAfter); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(outAfter))
	}

	if respAfter.Decision != "allow" {
		t.Errorf("expected decision 'allow' after PostToolUse approval, got %q", respAfter.Decision)
	}

	// 5. Different conversationID — gate should trigger again.
	payloadNew := `{
		"conversationId": "conv-test-gate-2",
		"toolName": "write_to_file",
		"toolCall": {
			"name": "write_to_file",
			"args": {"TargetFile": "critical_config.go"}
		},
		"workspacePaths": ["` + workspace + `"]
	}`

	outNew, err := HandlePreToolUse(ctx, []byte(payloadNew))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var respNew PreToolUseResponse
	if err := json.Unmarshal(outNew, &respNew); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(outNew))
	}

	if respNew.Decision != "force_ask" {
		t.Errorf("expected decision 'force_ask' for new conversation, got %q", respNew.Decision)
	}
	if !strings.Contains(respNew.Reason, "[REPO-MEMORY gate-crit-01]") {
		t.Errorf("expected '[REPO-MEMORY gate-crit-01]' in reason, got %q", respNew.Reason)
	}
}
