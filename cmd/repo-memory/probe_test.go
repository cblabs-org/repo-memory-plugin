package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
)

func TestProbe_PreToolUse_Decisions(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	repoID, err := gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatalf("GenerateRepoID failed: %v", err)
	}

	testPayload := []byte(`{"conversationId":"test-conv-1","workspacePaths":["` + repoDir + `"],"toolCall":{"name":"run_command","args":{"CommandLine":"echo hello"}}}`)

	fixedTime := time.Date(2026, 10, 1, 12, 0, 0, 123456789, time.UTC)
	origTimeNow := probeTimeNow
	probeTimeNow = func() time.Time { return fixedTime }
	defer func() { probeTimeNow = origTimeNow }()

	expectedTimestamp := fixedTime.UTC().Format("20060102T150405.000000000Z")

	tests := []struct {
		name             string
		decisionFlag     string
		expectedDecision string
		expectedReason   string
	}{
		{
			name:             "default allow",
			decisionFlag:     "allow",
			expectedDecision: "allow",
			expectedReason:   "probe allow",
		},
		{
			name:             "deny decision",
			decisionFlag:     "deny",
			expectedDecision: "deny",
			expectedReason:   "probe deny",
		},
		{
			name:             "ask decision",
			decisionFlag:     "ask",
			expectedDecision: "ask",
			expectedReason:   "probe ask",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			origDecision := hookProbeDecisionFlag
			origWorkspace := hookWorkspaceFlag
			defer func() {
				hookProbeDecisionFlag = origDecision
				hookWorkspaceFlag = origWorkspace
			}()

			hookProbeDecisionFlag = tc.decisionFlag
			hookWorkspaceFlag = repoDir

			out, err := handleProbe("PreToolUse", testPayload)
			if err != nil {
				t.Fatalf("handleProbe failed: %v", err)
			}

			var resp map[string]string
			if err := json.Unmarshal(out, &resp); err != nil {
				t.Fatalf("failed to unmarshal output JSON: %v, raw: %s", err, string(out))
			}

			if resp["decision"] != tc.expectedDecision {
				t.Errorf("expected decision %q, got %q", tc.expectedDecision, resp["decision"])
			}
			if resp["reason"] != tc.expectedReason {
				t.Errorf("expected reason %q, got %q", tc.expectedReason, resp["reason"])
			}

			// Verify probe file was written to <data-dir>/<repo-id>/logs/probe_PreToolUse_<timestamp>.json
			expectedLogFile := filepath.Join(dataDir, repoID, "logs", "probe_PreToolUse_"+expectedTimestamp+".json")
			content, err := os.ReadFile(expectedLogFile)
			if err != nil {
				t.Fatalf("expected probe log file %s to exist: %v", expectedLogFile, err)
			}
			if string(content) != string(testPayload) {
				t.Errorf("probe log content mismatch: got %q, want %q", string(content), string(testPayload))
			}
		})
	}
}

func TestProbe_PreToolUse_InvalidDecision(t *testing.T) {
	origDecision := hookProbeDecisionFlag
	defer func() { hookProbeDecisionFlag = origDecision }()

	hookProbeDecisionFlag = "invalid_option"
	_, err := handleProbe("PreToolUse", []byte(`{}`))
	if err == nil {
		t.Fatal("expected error for invalid --probe-decision, got nil")
	}
	if !strings.Contains(err.Error(), "invalid --probe-decision") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestProbe_OtherEvents_NoOp(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	repoID, err := gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatalf("GenerateRepoID failed: %v", err)
	}

	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = repoDir

	events := []string{"PreInvocation", "PostToolUse", "Stop", "SessionStart"}
	for _, event := range events {
		t.Run(event, func(t *testing.T) {
			payload := []byte(`{"conversationId":"conv-` + event + `"}`)
			out, err := handleProbe(event, payload)
			if err != nil {
				t.Fatalf("handleProbe(%s) failed: %v", event, err)
			}

			if strings.TrimSpace(string(out)) != "{}" {
				t.Errorf("expected no-op output '{}' for event %s, got: %s", event, string(out))
			}

			// Check that log file exists in logs directory
			logsDir := filepath.Join(dataDir, repoID, "logs")
			entries, err := os.ReadDir(logsDir)
			if err != nil {
				t.Fatalf("failed to read logs directory %s: %v", logsDir, err)
			}

			found := false
			prefix := "probe_" + event + "_"
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), prefix) && strings.HasSuffix(entry.Name(), ".json") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected log file starting with %s in %s", prefix, logsDir)
			}
		})
	}
}

func TestProbe_TranscriptSnapshot(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	repoID, err := gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatalf("GenerateRepoID failed: %v", err)
	}

	// Create fake active transcript file
	transcriptDir := filepath.Join(repoDir, ".gemini", "antigravity")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript.jsonl")
	transcriptContent := "{\"step_index\": 1, \"type\": \"USER_INPUT\", \"content\": \"hello\"}\n{\"step_index\": 2, \"type\": \"PLANNER_RESPONSE\", \"content\": \"hi\"}\n"
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("failed to write transcript: %v", err)
	}

	fixedTime := time.Date(2026, 10, 1, 14, 30, 0, 0, time.UTC)
	origTimeNow := probeTimeNow
	probeTimeNow = func() time.Time { return fixedTime }
	defer func() { probeTimeNow = origTimeNow }()

	expectedTimestamp := fixedTime.UTC().Format("20060102T150405.000000000Z")

	origWorkspace := hookWorkspaceFlag
	origDecision := hookProbeDecisionFlag
	defer func() {
		hookWorkspaceFlag = origWorkspace
		hookProbeDecisionFlag = origDecision
	}()

	hookWorkspaceFlag = repoDir
	hookProbeDecisionFlag = "allow"

	payload := []byte(`{"conversationId":"test-transcript-conv","transcriptPath":"` + transcriptPath + `"}`)
	out, err := handleProbe("PreToolUse", payload)
	if err != nil {
		t.Fatalf("handleProbe failed: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if resp["decision"] != "allow" {
		t.Errorf("expected allow, got %v", resp["decision"])
	}

	// Verify transcript snapshot file exists
	expectedSnapshotPath := filepath.Join(dataDir, repoID, "logs", "probe_PreToolUse_"+expectedTimestamp+"_transcript.jsonl")
	copiedData, err := os.ReadFile(expectedSnapshotPath)
	if err != nil {
		t.Fatalf("expected transcript snapshot %s to exist: %v", expectedSnapshotPath, err)
	}
	if string(copiedData) != transcriptContent {
		t.Errorf("transcript content mismatch: got %q, want %q", string(copiedData), transcriptContent)
	}

	// Verify probe json was also written
	expectedLogPath := filepath.Join(dataDir, repoID, "logs", "probe_PreToolUse_"+expectedTimestamp+".json")
	logData, err := os.ReadFile(expectedLogPath)
	if err != nil {
		t.Fatalf("expected probe log %s to exist: %v", expectedLogPath, err)
	}
	if string(logData) != string(payload) {
		t.Errorf("log data mismatch: got %q, want %q", string(logData), string(payload))
	}
}

func TestProbe_TranscriptSnapshot_Nonexistent(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	nonexistentTranscript := filepath.Join(repoDir, "does_not_exist.jsonl")

	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = repoDir

	payload := []byte(`{"conversationId":"test-nonexistent","transcriptPath":"` + nonexistentTranscript + `"}`)
	out, err := handleProbe("Stop", payload)
	if err != nil {
		t.Fatalf("handleProbe failed: %v", err)
	}
	if strings.TrimSpace(string(out)) != "{}" {
		t.Errorf("expected {}, got %s", string(out))
	}

	// Verify probe log was written, but no transcript file was created
	repoID, _ := gitops.GenerateRepoID(repoDir)
	logsDir := filepath.Join(dataDir, repoID, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		t.Fatalf("failed to read logs: %v", err)
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_transcript.jsonl") {
			t.Errorf("unexpected transcript snapshot found for nonexistent file: %s", entry.Name())
		}
	}
}

func TestProbe_WorkspaceResolution(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	customRepoID := "custom-enrolled-repo-id-12345"

	// Enrol in registry
	if err := config.AddToRegistry(dataDir, []string{repoDir}, customRepoID, repoDir); err != nil {
		t.Fatalf("AddToRegistry failed: %v", err)
	}

	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = repoDir

	payload := []byte(`{"conversationId":"conv-registry"}`)
	_, err := handleProbe("PreInvocation", payload)
	if err != nil {
		t.Fatalf("handleProbe failed: %v", err)
	}

	// Check that log file is under <data-dir>/<customRepoID>/logs/
	enrolledLogsDir := filepath.Join(dataDir, customRepoID, "logs")
	entries, err := os.ReadDir(enrolledLogsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected probe log in enrolled repo dir %s, got err: %v", enrolledLogsDir, err)
	}

	// Test direct repo_id in payload
	payloadWithRepoID := []byte(`{"repo_id":"payload-repo-id-999"}`)
	hookWorkspaceFlag = ""
	_, err = handleProbe("PreInvocation", payloadWithRepoID)
	if err != nil {
		t.Fatalf("handleProbe with payload repo_id failed: %v", err)
	}

	directLogsDir := filepath.Join(dataDir, "payload-repo-id-999", "logs")
	entriesDirect, err := os.ReadDir(directLogsDir)
	if err != nil || len(entriesDirect) == 0 {
		t.Fatalf("expected probe log in direct repo dir %s, got err: %v", directLogsDir, err)
	}
}

func TestProbe_CobraCLI_Execution(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	repoID, err := gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatalf("GenerateRepoID failed: %v", err)
	}

	// Execute via cobra command RunE
	var stdout, stdin bytes.Buffer
	stdin.WriteString(`{"conversationId":"cli-conv","workspacePaths":["` + repoDir + `"]}`)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	// Set flags
	hookProbeFlag = true
	hookProbeDecisionFlag = "deny"
	hookWorkspaceFlag = repoDir

	defer func() {
		hookProbeFlag = false
		hookProbeDecisionFlag = "allow"
		hookWorkspaceFlag = ""
	}()

	err = hookCmd.RunE(hookCmd, []string{"antigravity", "PreToolUse"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	var resp map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse stdout JSON: %v, raw: %q", err, stdout.String())
	}

	if resp["decision"] != "deny" {
		t.Errorf("expected decision deny, got %q", resp["decision"])
	}
	if resp["reason"] != "probe deny" {
		t.Errorf("expected reason 'probe deny', got %q", resp["reason"])
	}

	// Verify log file was written
	logsDir := filepath.Join(dataDir, repoID, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected probe log in %s, got err: %v", logsDir, err)
	}
}

func TestProbe_Disabled_FastPath(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir() // Not enrolled

	var stdout, stdin bytes.Buffer
	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	// Probe is false
	hookProbeFlag = false
	hookWorkspaceFlag = repoDir
	defer func() { hookWorkspaceFlag = "" }()

	err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreToolUse"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != `{"decision":"allow"}` {
		t.Errorf("expected fast-path fail-open '{\"decision\":\"allow\"}', got %q", stdout.String())
	}

	// Verify non-PreToolUse events return {}
	stdout.Reset()
	err = hookCmd.RunE(hookCmd, []string{"antigravity", "PreInvocation"})
	if err != nil {
		t.Fatalf("hookCmd.RunE PreInvocation failed: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("expected fast-path no-op '{}', got %q", stdout.String())
	}
}
