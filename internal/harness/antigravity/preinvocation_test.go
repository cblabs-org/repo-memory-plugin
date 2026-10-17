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
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
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
	payload := fmt.Sprintf(`{"invocationNum": 0, "initialNumSteps": 0, "workspacePaths": [%q]}`, workspace)
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
	payload2 := fmt.Sprintf(`{"invocationNum": 0, "initialNumSteps": 0, "userPrompt": "test context", "workspacePaths": [%q]}`, workspace)
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
	if step.SystemMessage == nil || step.SystemMessage.SystemMessage == "" {
		t.Fatalf("expected systemMessage entry, got nil/empty")
	}
	if len(step.SystemMessage.Metadata) > 0 {
		t.Errorf("expected nil/empty metadata on systemMessage, got %v", step.SystemMessage.Metadata)
	}
	sysMsg := step.SystemMessage.SystemMessage
	if step.EphemeralMessage != "" {
		t.Errorf("expected empty ephemeralMessage on task-start, got %q", step.EphemeralMessage)
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

	payload := fmt.Sprintf(`{
		"conversationId": "conv-about-to-edit",
		"invocationNum": 0,
		"initialNumSteps": 4,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, transcriptPath, workspace)

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
			if step.SystemMessage != nil {
				t.Errorf("expected about-to-edit step to have nil systemMessage, got %+v", step.SystemMessage)
			}
			ephemeralMsg = step.EphemeralMessage
		}
		if step.SystemMessage != nil && step.SystemMessage.SystemMessage != "" {
			if step.EphemeralMessage != "" {
				t.Errorf("expected first-touch step to have empty ephemeralMessage, got %q", step.EphemeralMessage)
			}
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

func TestPreInvocation_CompactionAndReinjection(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)

	// Seed index with entry
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := dossiers.Entry{
		ID:           "reinj-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "Must preserve state after compaction",
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	dataDir := os.Getenv("REPO_MEMORY_DATA_DIR")
	repoID := "test-repo"
	convID := "conv-compaction"

	// 1. Manually create a ledger with a past injection
	ledger := delivery.NewLedger(convID)
	ledger.RecordInjection(delivery.InjectionRecord{
		EntryID:   "reinj-1",
		Channel:   delivery.ChannelUser,
		Moment:    delivery.MomentTaskStart,
		StepIndex: 2,
	})
	ledger.RecordInjection(delivery.InjectionRecord{
		EntryID:   "summarized-1",
		Channel:   delivery.ChannelUser,
		Moment:    delivery.MomentTaskStart,
		StepIndex: 1,
	})
	// Simulate seen dossiers to verify dedup is cleared
	ledger.MarkDossierSeen("core")
	if err := SaveLedger(dataDir, repoID, ledger); err != nil {
		t.Fatal(err)
	}

	// 2. Create transcript with CHECKPOINT
	transcriptDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")

	content := `{"step_index": 0, "type": "USER_INPUT", "content": "hello"}
{"step_index": 1, "type": "PLANNER_RESPONSE", "content": "world"}
CHECKPOINT
{"step_index": 4, "type": "SYSTEM", "content": "compacted history summary mentioning [repo-memory summarized-1]"}
`
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	payload := fmt.Sprintf(`{
		"conversationId": "conv-compaction",
		"invocationNum": 0,
		"initialNumSteps": 5,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, transcriptPath, workspace)

	out, err := HandlePreInvocation(ctx, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}

	var resp InjectStepsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(out))
	}

	if len(resp.InjectSteps) == 0 {
		t.Fatalf("expected inject steps, got none")
	}

	if resp.InjectSteps[0].SystemMessage == nil || !strings.Contains(resp.InjectSteps[0].SystemMessage.SystemMessage, "Must preserve state after compaction") {
		t.Errorf("expected reinjection text in systemMessage, got: %+v", resp.InjectSteps[0].SystemMessage)
	}
	if resp.InjectSteps[0].EphemeralMessage != "" {
		t.Errorf("expected empty ephemeralMessage on reinjection, got: %s", resp.InjectSteps[0].EphemeralMessage)
	}

	// verify dedup is cleared and non-reinjected pre-checkpoint entries are not resurrected
	loaded, _ := LoadLedger(dataDir, repoID, convID)
	if loaded.HasSeenDossier("core") {
		t.Errorf("expected seen dossier to be cleared after compaction")
	}
	if loaded.HasInjected("summarized-1") {
		t.Errorf("expected pre-checkpoint non-reinjected entry summarized-1 not to be resurrected into InjectedSet")
	}
}

func TestPreInvocation_AntigravityNativeToolCalls(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)

	// Create real file in workspace
	filePath := filepath.Join(workspace, "service.go")
	if err := os.WriteFile(filePath, []byte("package service\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Seed index with entry anchored to service.go
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := dossiers.Entry{
		ID:           "native-entry-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "Must preserve service.go invariants",
		Anchors:      []string{"service.go"},
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpsertEntryAnchors(ctx, entry.ID, entry.Anchors); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	// Create transcript using Antigravity native format: {"name": "...", "args": {...}}
	transcriptDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")

	step0 := map[string]any{"step_index": 0, "type": "USER_INPUT", "content": "Please check service.go"}
	step1 := map[string]any{
		"step_index": 1,
		"type":       "PLANNER_RESPONSE",
		"tool_calls": []map[string]any{
			{
				"name": "view_file",
				"args": map[string]any{
					"AbsolutePath": filePath,
				},
			},
		},
	}
	step2 := map[string]any{"step_index": 2, "type": "TOOL_RESPONSE", "content": "package service"}
	step3 := map[string]any{
		"step_index": 3,
		"type":       "PLANNER_RESPONSE",
		"thinking":   "Now I will edit service.go to make the changes.",
		"content":    "Making edit.",
	}

	b0, _ := json.Marshal(step0)
	b1, _ := json.Marshal(step1)
	b2, _ := json.Marshal(step2)
	b3, _ := json.Marshal(step3)

	content := strings.Join([]string{string(b0), string(b1), string(b2), string(b3)}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	payload := fmt.Sprintf(`{
		"conversationId": "conv-native-tool-calls",
		"invocationNum": 0,
		"initialNumSteps": 4,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, transcriptPath, workspace)

	out, err := HandlePreInvocation(ctx, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}

	var resp InjectStepsResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v, raw: %s", err, string(out))
	}

	var hasContext, hasWarning bool
	for _, step := range resp.InjectSteps {
		if step.SystemMessage != nil && strings.Contains(step.SystemMessage.SystemMessage, "Must preserve service.go invariants") {
			if step.EphemeralMessage != "" {
				t.Errorf("expected empty ephemeralMessage on context step, got %q", step.EphemeralMessage)
			}
			hasContext = true
		}
		if strings.Contains(step.EphemeralMessage, "before editing service.go") {
			if step.SystemMessage != nil {
				t.Errorf("expected nil systemMessage on about-to-edit step, got %+v", step.SystemMessage)
			}
			hasWarning = true
		}
	}

	if !hasContext {
		t.Errorf("expected context injection for service.go, got steps: %+v", resp.InjectSteps)
	}
	if !hasWarning {
		t.Errorf("expected warning ephemeralMessage for service.go, got steps: %+v", resp.InjectSteps)
	}
}

func TestPreInvocation_RewindRearmsDedup(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)
	dataDir := os.Getenv("REPO_MEMORY_DATA_DIR")
	repoID := "test-repo"
	convID := "conv-rewind-rearm"

	filePath := filepath.Join(workspace, "service.go")
	if err := os.WriteFile(filePath, []byte("package service\n"), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}
	entry := dossiers.Entry{
		ID:           "native-entry-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core",
		Text:         "Must preserve service.go invariants",
		Anchors:      []string{"service.go"},
	}
	if err := idx.UpsertEntry(ctx, entry); err != nil {
		t.Fatal(err)
	}
	if err := idx.UpsertEntryAnchors(ctx, entry.ID, entry.Anchors); err != nil {
		t.Fatal(err)
	}
	idx.Close()

	transcriptDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")

	step0 := map[string]any{"step_index": 0, "type": "USER_INPUT", "content": "Please check service.go"}
	step1 := map[string]any{
		"step_index": 1,
		"type":       "PLANNER_RESPONSE",
		"tool_calls": []map[string]any{
			{
				"name": "view_file",
				"args": map[string]any{
					"AbsolutePath": filePath,
				},
			},
		},
	}
	step2 := map[string]any{"step_index": 2, "type": "TOOL_RESPONSE", "content": "package service"}
	step3 := map[string]any{"step_index": 3, "type": "PLANNER_RESPONSE", "content": "Checked service.go"}

	b0, _ := json.Marshal(step0)
	b1, _ := json.Marshal(step1)
	b2, _ := json.Marshal(step2)
	b3, _ := json.Marshal(step3)

	content4 := strings.Join([]string{string(b0), string(b1), string(b2), string(b3)}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content4), 0644); err != nil {
		t.Fatal(err)
	}

	payload4 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 1,
		"initialNumSteps": 4,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	// 1. Initial invocation with 4-step transcript touching service.go
	out1, err := HandlePreInvocation(ctx, []byte(payload4))
	if err != nil {
		t.Fatal(err)
	}
	var resp1 InjectStepsResponse
	if err := json.Unmarshal(out1, &resp1); err != nil {
		t.Fatalf("unmarshal out1: %v", err)
	}
	if len(resp1.InjectSteps) == 0 || resp1.InjectSteps[0].SystemMessage == nil || !strings.Contains(resp1.InjectSteps[0].SystemMessage.SystemMessage, "Must preserve service.go invariants") {
		t.Fatalf("expected systemMessage injection on initial call, got %+v", resp1.InjectSteps)
	}
	if resp1.InjectSteps[0].EphemeralMessage != "" {
		t.Errorf("expected empty ephemeralMessage, got %q", resp1.InjectSteps[0].EphemeralMessage)
	}

	loaded1, err := LoadLedger(dataDir, repoID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded1.LastKnownStepCount != 4 {
		t.Errorf("got LastKnownStepCount = %d, want 4", loaded1.LastKnownStepCount)
	}
	if !loaded1.InjectedSet["native-entry-1"] {
		t.Errorf("expected InjectedSet[\"native-entry-1\"] == true")
	}

	// 2. Subsequent invocation with the same 4-step transcript is deduplicated
	out2, err := HandlePreInvocation(ctx, []byte(payload4))
	if err != nil {
		t.Fatal(err)
	}
	if string(out2) != "{}" {
		t.Errorf("expected deduplicated '{}' on second invocation, got %s", string(out2))
	}

	// 3. Rewind transcript to 2 steps (len(steps) < ledger.LastKnownStepCount)
	content2 := strings.Join([]string{string(b0), string(b1)}, "\n") + "\n"
	if err := os.WriteFile(transcriptPath, []byte(content2), 0644); err != nil {
		t.Fatal(err)
	}

	payload2 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 1,
		"initialNumSteps": 2,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	out3, err := HandlePreInvocation(ctx, []byte(payload2))
	if err != nil {
		t.Fatal(err)
	}
	var resp3 InjectStepsResponse
	if err := json.Unmarshal(out3, &resp3); err != nil {
		t.Fatalf("unmarshal out3: %v", err)
	}
	if len(resp3.InjectSteps) == 0 || resp3.InjectSteps[0].SystemMessage == nil || !strings.Contains(resp3.InjectSteps[0].SystemMessage.SystemMessage, "Must preserve service.go invariants") {
		t.Fatalf("expected systemMessage re-injection after rewind, got %+v", resp3.InjectSteps)
	}
	if resp3.InjectSteps[0].EphemeralMessage != "" {
		t.Errorf("expected empty ephemeralMessage after rewind, got %q", resp3.InjectSteps[0].EphemeralMessage)
	}

	loaded3, err := LoadLedger(dataDir, repoID, convID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded3.LastKnownStepCount != 2 {
		t.Errorf("got LastKnownStepCount = %d after rewind, want 2", loaded3.LastKnownStepCount)
	}
	if loaded3.LastProcessedStepIndex != 1 {
		t.Errorf("got LastProcessedStepIndex = %d after rewind, want 1", loaded3.LastProcessedStepIndex)
	}
	if !loaded3.InjectedSet["native-entry-1"] {
		t.Errorf("expected InjectedSet[\"native-entry-1\"] == true after rewind re-injection")
	}
}

func TestPreInvocation_MultiInvocationLifecycle(t *testing.T) {
	ctx := context.Background()
	workspace, idxPath := setupEnrolledWorkspace(t)
	convID := "conv-multi-invocation-lifecycle"

	tokenRelPath := filepath.Join("internal", "core", "auth", "token.go")
	tokenAbsPath := filepath.Join(workspace, tokenRelPath)
	if err := os.MkdirAll(filepath.Dir(tokenAbsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tokenAbsPath, []byte("package auth\n\nfunc SignToken() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	idx, err := index.NewIndex(idxPath)
	if err != nil {
		t.Fatal(err)
	}

	taskEntry := dossiers.Entry{
		ID:           "task-entry-1",
		Type:         dossiers.EntryTypeInvariant,
		DossierScope: "core/auth",
		Text:         "Use HMAC-SHA256 for session tokens in auth module",
	}
	touchEntry := dossiers.Entry{
		ID:           "touch-entry-1",
		Type:         dossiers.EntryTypeGotcha,
		DossierScope: "core/auth",
		Text:         "Never mutate claims map in place inside internal/core/auth/token.go",
		Anchors:      []string{"internal/core/auth/token.go"},
	}
	dossierEntry := dossiers.Entry{
		ID:           "dossier-entry-1",
		Type:         dossiers.EntryTypeRuledOut,
		DossierScope: "core/auth",
		Text:         "Always validate token expiry before signature verification in core/auth",
	}

	for _, ent := range []dossiers.Entry{taskEntry, touchEntry, dossierEntry} {
		if err := idx.UpsertEntry(ctx, ent); err != nil {
			t.Fatal(err)
		}
		if len(ent.Anchors) > 0 {
			if err := idx.UpsertEntryAnchors(ctx, ent.ID, ent.Anchors); err != nil {
				t.Fatal(err)
			}
		}
	}
	idx.Close()

	transcriptDir := filepath.Join(t.TempDir(), "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatal(err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")

	marshalStep := func(m map[string]any) string {
		b, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	writeTranscript := func(lines ...string) {
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(transcriptPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Phase 1 (Turn 1, invocationNum: 0, initialNumSteps: 1 — MomentTaskStart)
	step0 := marshalStep(map[string]any{
		"step_index": 0,
		"type":       "USER_INPUT",
		"content":    "Update HMAC-SHA256 session tokens",
	})
	writeTranscript(step0)

	payload1 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 0,
		"initialNumSteps": 1,
		"userPrompt": "Update HMAC-SHA256 session tokens",
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	out1, err := HandlePreInvocation(ctx, []byte(payload1))
	if err != nil {
		t.Fatalf("Phase 1 HandlePreInvocation error: %v", err)
	}
	var resp1 InjectStepsResponse
	if err := json.Unmarshal(out1, &resp1); err != nil {
		t.Fatalf("Phase 1 unmarshal: %v (raw: %s)", err, string(out1))
	}
	if len(resp1.InjectSteps) != 1 {
		t.Fatalf("Phase 1 expected 1 injectStep, got %d: %+v", len(resp1.InjectSteps), resp1.InjectSteps)
	}
	if resp1.InjectSteps[0].EphemeralMessage != "" {
		t.Errorf("Phase 1 expected empty EphemeralMessage, got %q", resp1.InjectSteps[0].EphemeralMessage)
	}
	if resp1.InjectSteps[0].SystemMessage == nil || !strings.Contains(resp1.InjectSteps[0].SystemMessage.SystemMessage, "[repo-memory task-entry-1]") {
		t.Errorf("Phase 1 expected SystemMessage containing [repo-memory task-entry-1], got %+v", resp1.InjectSteps[0].SystemMessage)
	}

	// Append injected SystemMessage as persistent SYSTEM_MESSAGE step (step_index: 1),
	// followed by view_file on internal/core/auth/token.go (step_index: 2),
	// TOOL_RESPONSE (step_index: 3), and PLANNER_RESPONSE (step_index: 4) with edit intent.
	step1 := marshalStep(map[string]any{
		"step_index": 1,
		"type":       "SYSTEM_MESSAGE",
		"content":    resp1.InjectSteps[0].SystemMessage.SystemMessage,
	})
	step2 := marshalStep(map[string]any{
		"step_index": 2,
		"type":       "PLANNER_RESPONSE",
		"tool_calls": []map[string]any{
			{
				"name": "view_file",
				"args": map[string]any{
					"AbsolutePath": tokenAbsPath,
				},
			},
		},
	})
	step3 := marshalStep(map[string]any{
		"step_index": 3,
		"type":       "TOOL_RESPONSE",
		"content":    "package auth\n\nfunc SignToken() {}\n",
	})
	step4 := marshalStep(map[string]any{
		"step_index": 4,
		"type":       "PLANNER_RESPONSE",
		"thinking":   "Now I will edit internal/core/auth/token.go to update token signing.",
		"content":    "Editing token.go.",
	})
	writeTranscript(step0, step1, step2, step3, step4)

	// Phase 2 (Turn 1, invocationNum: 1, initialNumSteps: 5 — MomentFirstTouch + MomentFirstEntry + MomentAboutToEdit)
	payload2 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 1,
		"initialNumSteps": 5,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	out2, err := HandlePreInvocation(ctx, []byte(payload2))
	if err != nil {
		t.Fatalf("Phase 2 HandlePreInvocation error: %v", err)
	}
	var resp2 InjectStepsResponse
	if err := json.Unmarshal(out2, &resp2); err != nil {
		t.Fatalf("Phase 2 unmarshal: %v (raw: %s)", err, string(out2))
	}

	var phase2SysMsgs []string
	var phase2EphemeralMsgs []string
	for _, st := range resp2.InjectSteps {
		if st.SystemMessage != nil && st.SystemMessage.SystemMessage != "" {
			if st.EphemeralMessage != "" {
				t.Errorf("Phase 2 expected SystemMessage step to have empty EphemeralMessage, got %q", st.EphemeralMessage)
			}
			phase2SysMsgs = append(phase2SysMsgs, st.SystemMessage.SystemMessage)
		}
		if st.EphemeralMessage != "" {
			if st.SystemMessage != nil {
				t.Errorf("Phase 2 expected EphemeralMessage step to have nil SystemMessage, got %+v", st.SystemMessage)
			}
			phase2EphemeralMsgs = append(phase2EphemeralMsgs, st.EphemeralMessage)
		}
	}

	if len(phase2SysMsgs) != 1 {
		t.Errorf("Phase 2 expected all SystemMessage injections to coalesce into 1 step, got %d: %v", len(phase2SysMsgs), phase2SysMsgs)
	}
	combinedPhase2Sys := strings.Join(phase2SysMsgs, "\n")
	if strings.Count(combinedPhase2Sys, "--- repo-memory context ---") != 1 {
		t.Errorf("Phase 2 expected exactly 1 context header in coalesced SystemMessage, got: %s", combinedPhase2Sys)
	}
	if strings.Contains(combinedPhase2Sys, "task-entry-1") {
		t.Errorf("Phase 2 expected task-entry-1 to be suppressed by InjectedSet, got: %s", combinedPhase2Sys)
	}
	if !strings.Contains(combinedPhase2Sys, "[repo-memory touch-entry-1]") {
		t.Errorf("Phase 2 expected touch-entry-1 in SystemMessage, got: %s", combinedPhase2Sys)
	}
	if !strings.Contains(combinedPhase2Sys, "[repo-memory dossier-entry-1]") {
		t.Errorf("Phase 2 expected dossier-entry-1 in SystemMessage, got: %s", combinedPhase2Sys)
	}

	transcriptBytesPhase2, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(transcriptBytesPhase2), "[repo-memory task-entry-1]") {
		t.Errorf("Phase 2 expected task-entry-1 SYSTEM_MESSAGE step to remain present in transcript_full.jsonl")
	}

	wantAboutToEdit := "[repo-memory] Check injected entries above before editing token.go"
	if len(phase2EphemeralMsgs) != 1 || phase2EphemeralMsgs[0] != wantAboutToEdit {
		t.Errorf("Phase 2 got EphemeralMessages %v, want [%q]", phase2EphemeralMsgs, wantAboutToEdit)
	}

	// Phase 3 (Turn 1, invocationNum: 2, initialNumSteps: 7 — Mid-turn Deduplication)
	step5 := marshalStep(map[string]any{
		"step_index": 5,
		"type":       "SYSTEM_MESSAGE",
		"content":    combinedPhase2Sys,
	})
	step6 := marshalStep(map[string]any{
		"step_index": 6,
		"type":       "PLANNER_RESPONSE",
		"thinking":   "Finished reviewing.",
		"content":    "Done.",
	})
	writeTranscript(step0, step1, step2, step3, step4, step5, step6)

	payload3 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 2,
		"initialNumSteps": 7,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	out3, err := HandlePreInvocation(ctx, []byte(payload3))
	if err != nil {
		t.Fatalf("Phase 3 HandlePreInvocation error: %v", err)
	}
	if string(out3) != "{}" {
		t.Errorf("Phase 3 expected deduplicated \"{}\", got %s", string(out3))
	}

	transcriptBytesPhase3, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, wantTag := range []string{"[repo-memory task-entry-1]", "[repo-memory touch-entry-1]", "[repo-memory dossier-entry-1]"} {
		if !strings.Contains(string(transcriptBytesPhase3), wantTag) {
			t.Errorf("Phase 3 expected %s to remain in transcript_full.jsonl", wantTag)
		}
	}

	// Phase 4 (Turn 2, invocationNum: 0, initialNumSteps: 9 after CHECKPOINT — Compaction Re-injection)
	step8 := marshalStep(map[string]any{
		"step_index": 8,
		"type":       "SYSTEM",
		"content":    "Compacted conversation summary",
	})
	writeTranscript(step0, step1, step2, step3, step4, step5, step6, "CHECKPOINT", step8)

	payload4 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 0,
		"initialNumSteps": 9,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	out4, err := HandlePreInvocation(ctx, []byte(payload4))
	if err != nil {
		t.Fatalf("Phase 4 HandlePreInvocation error: %v", err)
	}
	var resp4 InjectStepsResponse
	if err := json.Unmarshal(out4, &resp4); err != nil {
		t.Fatalf("Phase 4 unmarshal: %v (raw: %s)", err, string(out4))
	}
	if len(resp4.InjectSteps) == 0 {
		t.Fatalf("Phase 4 expected compaction re-injection step, got empty")
	}
	reinjStep := resp4.InjectSteps[0]
	if reinjStep.EphemeralMessage != "" {
		t.Errorf("Phase 4 expected empty EphemeralMessage on compaction re-injection, got %q", reinjStep.EphemeralMessage)
	}
	if reinjStep.SystemMessage == nil {
		t.Fatalf("Phase 4 expected non-nil SystemMessage on compaction re-injection")
	}
	for _, wantID := range []string{"task-entry-1", "touch-entry-1", "dossier-entry-1"} {
		if !strings.Contains(reinjStep.SystemMessage.SystemMessage, wantID) {
			t.Errorf("Phase 4 expected re-injected SystemMessage to contain %q, got: %s", wantID, reinjStep.SystemMessage.SystemMessage)
		}
	}
	if strings.Contains(reinjStep.SystemMessage.SystemMessage, "about-to-edit") || strings.Contains(reinjStep.SystemMessage.SystemMessage, "before editing") {
		t.Errorf("Phase 4 expected about-to-edit not to be re-injected, got: %s", reinjStep.SystemMessage.SystemMessage)
	}

	dataDir := os.Getenv("REPO_MEMORY_DATA_DIR")
	repoID := "test-repo"
	loadedPhase4, err := LoadLedger(dataDir, repoID, convID)
	if err != nil {
		t.Fatalf("Phase 4 LoadLedger error: %v", err)
	}
	if loadedPhase4.HasInjected("about-to-edit:internal/core/auth/token.go") {
		t.Errorf("Phase 4 expected pre-checkpoint non-reinjected about-to-edit key not to be resurrected into InjectedSet")
	}
	if loadedPhase4.LastHandledCheckpointStep < 0 {
		t.Errorf("Phase 4 expected LastHandledCheckpointStep >= 0, got %d", loadedPhase4.LastHandledCheckpointStep)
	}

	// Phase 5 (Turn 3, Transcript Rewind to 3 steps — DetectRewind Re-arms Dedup)
	writeTranscript(step0, step1, step2)

	payload5 := fmt.Sprintf(`{
		"conversationId": %q,
		"invocationNum": 1,
		"initialNumSteps": 3,
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, convID, transcriptPath, workspace)

	out5, err := HandlePreInvocation(ctx, []byte(payload5))
	if err != nil {
		t.Fatalf("Phase 5 HandlePreInvocation error: %v", err)
	}
	var resp5 InjectStepsResponse
	if err := json.Unmarshal(out5, &resp5); err != nil {
		t.Fatalf("Phase 5 unmarshal: %v (raw: %s)", err, string(out5))
	}
	var phase5SysMsgs []string
	for _, st := range resp5.InjectSteps {
		if st.EphemeralMessage != "" {
			t.Errorf("Phase 5 expected empty EphemeralMessage after rewind, got %q", st.EphemeralMessage)
		}
		if st.SystemMessage != nil && st.SystemMessage.SystemMessage != "" {
			phase5SysMsgs = append(phase5SysMsgs, st.SystemMessage.SystemMessage)
		}
	}
	combinedPhase5Sys := strings.Join(phase5SysMsgs, "\n")
	for _, wantID := range []string{"touch-entry-1", "dossier-entry-1"} {
		if !strings.Contains(combinedPhase5Sys, wantID) {
			t.Errorf("Phase 5 expected rewind re-injection of %q in SystemMessage, got: %s", wantID, combinedPhase5Sys)
		}
	}

	loadedPhase5, err := LoadLedger(dataDir, repoID, convID)
	if err != nil {
		t.Fatalf("Phase 5 LoadLedger error: %v", err)
	}
	if loadedPhase5.LastHandledCheckpointStep != -1 {
		t.Errorf("Phase 5 expected LastHandledCheckpointStep reset to -1 after rewinding past checkpoint, got %d", loadedPhase5.LastHandledCheckpointStep)
	}
	if loadedPhase5.HasInjected("about-to-edit:internal/core/auth/token.go") {
		t.Errorf("Phase 5 expected rewound about-to-edit key not to be resurrected into InjectedSet")
	}
}

