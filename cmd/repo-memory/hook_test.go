package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

func TestHookCmd_Stop_Enrolled(t *testing.T) {
	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = ""

	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init", "-b", "main")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	testFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("writing README: %v", err)
	}
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "initial commit")

	repoID, err := gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatalf("GenerateRepoID: %v", err)
	}

	// Machine config
	machCfg := config.DefaultMachineConfig()
	machCfg.Enabled = true
	if err := config.SaveMachineConfig(dataDir, machCfg); err != nil {
		t.Fatalf("SaveMachineConfig: %v", err)
	}

	// Registry
	if err := config.AddToRegistry(dataDir, []string{repoDir}, repoID, repoDir); err != nil {
		t.Fatalf("AddToRegistry: %v", err)
	}

	// Repo config
	repoCfg := config.DefaultRepoConfig()
	enabled := true
	repoCfg.Enabled = &enabled
	if err := config.SaveRepoConfig(repoDir, repoCfg); err != nil {
		t.Fatalf("SaveRepoConfig: %v", err)
	}

	// Create enrolled marker and run in repoDir
	if err := config.CreateEnrolledMarker(repoDir); err != nil {
		t.Fatalf("CreateEnrolledMarker: %v", err)
	}
	origWd, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origWd)

	// Create fake transcript
	transcriptDir := filepath.Join(t.TempDir(), "brain", "conv-cli-stop", ".system_generated", "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("mkdir transcript: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")
	transcriptContent := "{\"step_index\": 0, \"type\": \"USER_INPUT\", \"content\": \"hello from CLI\"}\n"
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}

	payload := `{"conversationId":"conv-cli-stop","transcriptPath":"` + transcriptPath + `","workspacePaths":["` + repoDir + `"]}`

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	stdin.WriteString(payload)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	err = hookCmd.RunE(hookCmd, []string{"antigravity", "Stop"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("expected {}, got %q", stdout.String())
	}

	// Verify spool entry
	spoolDir := filepath.Join(dataDir, repoID, "spool")
	reader := spool.NewReader(spoolDir)
	items, err := reader.ReadEntries()
	if err != nil || len(items) != 1 {
		t.Fatalf("expected 1 spool entry, got items: %v, err: %v", items, err)
	}
	if items[0].Entry.ConversationID != "conv-cli-stop" {
		t.Errorf("expected conv-cli-stop, got %s", items[0].Entry.ConversationID)
	}

	// Verify queue copy
	queueFile := filepath.Join(dataDir, repoID, "queue", "conv-cli-stop.jsonl")
	data, err := os.ReadFile(queueFile)
	if err != nil || string(data) != transcriptContent {
		t.Errorf("queue file content mismatch: got %q, want %q", string(data), transcriptContent)
	}
}

func TestHookCmd_Stop_Unenrolled(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	origWd, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origWd)

	payload := `{"conversationId":"conv-cli-unenrolled","workspacePaths":["` + repoDir + `"]}`

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	stdin.WriteString(payload)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	err := hookCmd.RunE(hookCmd, []string{"antigravity", "Stop"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("expected {}, got %q", stdout.String())
	}

	// Ensure no spool entry was created
	entries, _ := os.ReadDir(dataDir)
	for _, entry := range entries {
		if entry.IsDir() {
			spoolEntries, _ := os.ReadDir(filepath.Join(dataDir, entry.Name(), "spool"))
			if len(spoolEntries) > 0 {
				t.Errorf("expected no spool entries for unenrolled workspace")
			}
		}
	}
}

func TestHookCmd_Stop_KillSwitch(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Setenv("REPO_MEMORY_DISABLED", "1")

	repoDir := t.TempDir()
	_ = config.CreateEnrolledMarker(repoDir)
	origWd, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origWd)

	payload := `{"conversationId":"conv-cli-kill","workspacePaths":["` + repoDir + `"]}`

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	stdin.WriteString(payload)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	err := hookCmd.RunE(hookCmd, []string{"antigravity", "Stop"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("expected {}, got %q", stdout.String())
	}
}

func TestHookCmd_Stop_HousekeepingGuard(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Setenv("REPO_MEMORY_HOUSEKEEPING", "1")

	repoDir := t.TempDir()
	_ = config.CreateEnrolledMarker(repoDir)
	origWd, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(origWd)

	payload := `{"conversationId":"conv-cli-housekeeping","workspacePaths":["` + repoDir + `"]}`

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	stdin.WriteString(payload)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	err := hookCmd.RunE(hookCmd, []string{"antigravity", "Stop"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("expected {}, got %q", stdout.String())
	}
}

func TestHookCmd_PostToolUse(t *testing.T) {
	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = ""

	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	_ = config.CreateEnrolledMarker(repoDir)

	payload := `{"conversationId":"conv-cli-post","toolResult":{"status":"success"},"workspacePaths":["` + repoDir + `"]}`

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	stdin.WriteString(payload)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	err := hookCmd.RunE(hookCmd, []string{"antigravity", "PostToolUse"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != "{}" {
		t.Errorf("expected {}, got %q", stdout.String())
	}
}

func TestHookCmd_PreToolUse(t *testing.T) {
	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = ""

	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	_ = config.CreateEnrolledMarker(repoDir)

	// Default allow
	payload := `{"conversationId":"conv-cli-pretool","toolName":"view_file","workspacePaths":["` + repoDir + `"]}`

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	stdin.WriteString(payload)

	hookCmd.SetIn(&stdin)
	hookCmd.SetOut(&stdout)

	err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreToolUse"})
	if err != nil {
		t.Fatalf("hookCmd.RunE failed: %v", err)
	}

	if strings.TrimSpace(stdout.String()) != `{"decision":"allow"}` {
		t.Errorf("expected %q, got %q", `{"decision":"allow"}`, stdout.String())
	}
}

