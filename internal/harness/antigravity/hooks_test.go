package antigravity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

func runGitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{"-c", "user.name=test", "-c", "user.email=test@example.com"}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\nOutput: %s", strings.Join(args, " "), err, string(out))
	}
}

// setupEnrolledRepo creates an enrolled git repository in temp dirs for testing.
func setupEnrolledRepo(t *testing.T) (dataDir, repoDir, repoID string) {
	t.Helper()
	dataDir = t.TempDir()
	repoDir = t.TempDir()

	runGitInDir(t, repoDir, "init", "-b", "main")
	testFile := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("writing README.md: %v", err)
	}
	runGitInDir(t, repoDir, "add", "README.md")
	runGitInDir(t, repoDir, "commit", "-m", "initial commit")

	var err error
	repoID, err = gitops.GenerateRepoID(repoDir)
	if err != nil {
		t.Fatalf("GenerateRepoID: %v", err)
	}

	// 1. Machine config enabled
	machCfg := config.DefaultMachineConfig()
	machCfg.Enabled = true
	if err := config.SaveMachineConfig(dataDir, machCfg); err != nil {
		t.Fatalf("SaveMachineConfig: %v", err)
	}

	// 2. Repo enrolled in registry
	if err := config.AddToRegistry(dataDir, []string{repoDir}, repoID, repoDir); err != nil {
		t.Fatalf("AddToRegistry: %v", err)
	}

	// 3. Repo config enabled
	repoCfg := config.DefaultRepoConfig()
	enabled := true
	repoCfg.Enabled = &enabled
	if err := config.SaveRepoConfig(repoDir, repoCfg); err != nil {
		t.Fatalf("SaveRepoConfig: %v", err)
	}

	return dataDir, repoDir, repoID
}

func TestStopHook_CompletePipeline(t *testing.T) {
	dataDir, repoDir, repoID := setupEnrolledRepo(t)

	// Create fake transcript
	transcriptDir := filepath.Join(t.TempDir(), "brain", "conv-stop-1", ".system_generated", "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("creating transcript dir: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")
	transcriptContent := "{\"step_index\": 0, \"type\": \"USER_INPUT\", \"content\": \"hello\"}\n{\"step_index\": 1, \"type\": \"PLANNER_RESPONSE\", \"content\": \"hi\"}\n"
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}

	rawPayload := []byte(fmt.Sprintf(`{
		"conversationId": "conv-stop-1",
		"transcriptPath": %q,
		"workspacePaths": [%q]
	}`, transcriptPath, repoDir))

	ctx := context.Background()
	out, err := HandleStop(ctx, rawPayload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("HandleStop failed: %v", err)
	}

	if string(out) != "{}" {
		t.Errorf("expected out '{}', got %s", string(out))
	}

	// Verify spool entry was written
	spoolDir := filepath.Join(dataDir, repoID, "spool")
	reader := spool.NewReader(spoolDir)
	items, err := reader.ReadEntries()
	if err != nil {
		t.Fatalf("scanning spool: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 spool entry, got %d", len(items))
	}

	entry := items[0].Entry
	if entry.Kind != spool.KindCapture {
		t.Errorf("expected kind %s, got %s", spool.KindCapture, entry.Kind)
	}
	if entry.Harness != "antigravity" {
		t.Errorf("expected harness antigravity, got %s", entry.Harness)
	}
	if entry.ConversationID != "conv-stop-1" {
		t.Errorf("expected conv-stop-1, got %s", entry.ConversationID)
	}
	if entry.Branch != "main" {
		t.Errorf("expected branch main, got %s", entry.Branch)
	}
	if entry.Head == "" {
		t.Errorf("expected non-empty head commit")
	}

	// Verify transcript was incrementally copied into queue/
	queueFile := filepath.Join(dataDir, repoID, "queue", "conv-stop-1.jsonl")
	copiedData, err := os.ReadFile(queueFile)
	if err != nil {
		t.Fatalf("reading queue file %s: %v", queueFile, err)
	}
	if string(copiedData) != transcriptContent {
		t.Errorf("queue file content mismatch:\ngot: %s\nwant: %s", string(copiedData), transcriptContent)
	}

	// Verify state cursor
	stateFile := filepath.Join(dataDir, repoID, "state", "cursor_conv-stop-1.json")
	cursor, err := spool.LoadCopyState(stateFile)
	if err != nil || cursor == nil {
		t.Fatalf("failed to load cursor: %v", err)
	}
	if cursor.ByteOffset != int64(len(transcriptContent)) {
		t.Errorf("cursor offset = %d, want %d", cursor.ByteOffset, len(transcriptContent))
	}

	// Now append to the transcript and call Stop again (incremental test)
	turn2Content := "{\"step_index\": 2, \"type\": \"USER_INPUT\", \"content\": \"turn 2\"}\n"
	f, err := os.OpenFile(transcriptPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("opening transcript for append: %v", err)
	}
	if _, err := f.WriteString(turn2Content); err != nil {
		t.Fatalf("appending to transcript: %v", err)
	}
	f.Close()

	payloadTurn2 := &StopPayload{
		ConversationID: "conv-stop-1",
		TranscriptPath: transcriptPath,
		WorkspacePaths: []string{repoDir},
	}

	res2, err := HandleStopEvent(ctx, payloadTurn2, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("HandleStopEvent turn 2 failed: %v", err)
	}
	if res2.Skipped {
		t.Fatalf("unexpected skipped on turn 2: %s", res2.Reason)
	}
	if res2.CopyResult == nil {
		t.Fatalf("expected CopyResult on turn 2")
	}
	if res2.CopyResult.FullCopy {
		t.Errorf("expected incremental copy on turn 2, got full copy")
	}
	if res2.CopyResult.BytesCopied != int64(len(turn2Content)) {
		t.Errorf("expected %d bytes copied, got %d", len(turn2Content), res2.CopyResult.BytesCopied)
	}

	// Verify full content in queue file
	copiedData2, err := os.ReadFile(queueFile)
	if err != nil {
		t.Fatalf("reading queue file turn 2: %v", err)
	}
	expectedFullContent := transcriptContent + turn2Content
	if string(copiedData2) != expectedFullContent {
		t.Errorf("queue content after turn 2 mismatch:\ngot: %s\nwant: %s", string(copiedData2), expectedFullContent)
	}
}

func TestStopHook_KillSwitch_DisabledEnv(t *testing.T) {
	dataDir, repoDir, repoID := setupEnrolledRepo(t)

	t.Setenv("REPO_MEMORY_DISABLED", "1")

	payload := &StopPayload{
		ConversationID: "conv-disabled-env",
		WorkspacePaths: []string{repoDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected hook to be skipped due to REPO_MEMORY_DISABLED=1")
	}
	if !strings.Contains(res.Reason, "REPO_MEMORY_DISABLED") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}

	// Ensure no spool entry was written
	spoolDir := filepath.Join(dataDir, repoID, "spool")
	entries, _ := os.ReadDir(spoolDir)
	if len(entries) > 0 {
		t.Errorf("expected 0 spool entries when killed, got %d", len(entries))
	}
}

func TestStopHook_KillSwitch_MachineConfig(t *testing.T) {
	dataDir, repoDir, _ := setupEnrolledRepo(t)

	// Disable via machine config
	machCfg := config.DefaultMachineConfig()
	machCfg.Enabled = false
	if err := config.SaveMachineConfig(dataDir, machCfg); err != nil {
		t.Fatalf("SaveMachineConfig: %v", err)
	}

	payload := &StopPayload{
		ConversationID: "conv-mach-disabled",
		WorkspacePaths: []string{repoDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected hook to be skipped due to machine config disabled")
	}
	if !strings.Contains(res.Reason, "machine config disabled") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}
}

func TestStopHook_KillSwitch_RepoConfig(t *testing.T) {
	dataDir, repoDir, _ := setupEnrolledRepo(t)

	// Disable via repo config
	repoCfg := config.DefaultRepoConfig()
	disabled := false
	repoCfg.Enabled = &disabled
	if err := config.SaveRepoConfig(repoDir, repoCfg); err != nil {
		t.Fatalf("SaveRepoConfig: %v", err)
	}

	payload := &StopPayload{
		ConversationID: "conv-repo-disabled",
		WorkspacePaths: []string{repoDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected hook to be skipped due to repo config disabled")
	}
	if !strings.Contains(res.Reason, "repo config disabled") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}
}

func TestStopHook_HousekeepingGuard_EnvVar(t *testing.T) {
	dataDir, repoDir, repoID := setupEnrolledRepo(t)

	t.Setenv("REPO_MEMORY_HOUSEKEEPING", "1")

	payload := &StopPayload{
		ConversationID: "conv-housekeeping",
		WorkspacePaths: []string{repoDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected hook to be skipped due to REPO_MEMORY_HOUSEKEEPING=1")
	}
	if !strings.Contains(res.Reason, "REPO_MEMORY_HOUSEKEEPING") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}

	// Verify no spool entries
	spoolDir := filepath.Join(dataDir, repoID, "spool")
	entries, _ := os.ReadDir(spoolDir)
	if len(entries) > 0 {
		t.Errorf("expected 0 spool entries during housekeeping run, got %d", len(entries))
	}
}

func TestStopHook_HousekeepingGuard_RunsDirectory(t *testing.T) {
	dataDir, repoDir, repoID := setupEnrolledRepo(t)

	// Path inside runs/ directory
	runsDir := filepath.Join(repoDir, "runs", repoID)
	if err := os.MkdirAll(runsDir, 0755); err != nil {
		t.Fatalf("mkdir runs: %v", err)
	}

	payload := &StopPayload{
		ConversationID: "conv-runs-dir",
		WorkspacePaths: []string{runsDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected hook to be skipped due to runs/ directory")
	}
	if !strings.Contains(res.Reason, "housekeeping directory") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}
}

func TestStopHook_EnrolmentCheck_UnenrolledWorkspace(t *testing.T) {
	dataDir, _, _ := setupEnrolledRepo(t)

	// Separate non-enrolled directory
	unenrolledDir := t.TempDir()

	payload := &StopPayload{
		ConversationID: "conv-unenrolled",
		WorkspacePaths: []string{unenrolledDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Skipped {
		t.Fatal("expected hook to be skipped for unenrolled workspace")
	}
	if !strings.Contains(res.Reason, "not enrolled") {
		t.Errorf("unexpected reason: %s", res.Reason)
	}
}

func TestStopHook_ExecutionBudgetUnder200ms(t *testing.T) {
	dataDir, repoDir, _ := setupEnrolledRepo(t)

	transcriptDir := filepath.Join(t.TempDir(), "brain", "conv-budget", ".system_generated", "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("creating transcript dir: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")
	transcriptContent := strings.Repeat("{\"step_index\": 0, \"type\": \"USER_INPUT\", \"content\": \"data\"}\n", 100)
	if err := os.WriteFile(transcriptPath, []byte(transcriptContent), 0644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}

	payload := &StopPayload{
		ConversationID: "conv-budget",
		TranscriptPath: transcriptPath,
		WorkspacePaths: []string{repoDir},
	}

	budget := 200 * time.Millisecond
	if runtime.GOOS == "windows" {
		budget = 2 * time.Second
	}
	start := time.Now()
	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir), WithTimeout(budget))
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("HandleStopEvent failed: %v", err)
	}
	if res.Skipped {
		t.Fatalf("unexpected skip: %s", res.Reason)
	}

	t.Logf("Stop hook execution time: %v (recorded internal duration: %v)", duration, res.Duration)
	if duration >= budget {
		t.Errorf("Stop hook took %v, exceeded %v execution budget!", duration, budget)
	}
}

func TestStopHook_ArtifactCapture(t *testing.T) {
	dataDir, repoDir, repoID := setupEnrolledRepo(t)

	// Create fake transcript
	brainDir := filepath.Join(t.TempDir(), "brain", "conv-artifact-1")
	transcriptDir := filepath.Join(brainDir, ".system_generated", "logs")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("creating transcript dir: %v", err)
	}
	transcriptPath := filepath.Join(transcriptDir, "transcript_full.jsonl")
	if err := os.WriteFile(transcriptPath, []byte("{\"step_index\": 0}\n"), 0644); err != nil {
		t.Fatalf("writing transcript: %v", err)
	}

	// Create sample artifacts in brainDir
	planPath := filepath.Join(brainDir, "plan.md")
	if err := os.WriteFile(planPath, []byte("# Implementation Plan\n"), 0644); err != nil {
		t.Fatalf("writing plan.md: %v", err)
	}
	metaPath := filepath.Join(brainDir, "plan.md.metadata.json")
	if err := os.WriteFile(metaPath, []byte(`{"title":"Plan"}`), 0644); err != nil {
		t.Fatalf("writing metadata.json: %v", err)
	}

	payload := &StopPayload{
		ConversationID:        "conv-artifact-1",
		TranscriptPath:        transcriptPath,
		ArtifactDirectoryPath: brainDir,
		WorkspacePaths:        []string{repoDir},
	}

	res, err := HandleStopEvent(context.Background(), payload, WithDataDir(dataDir))
	if err != nil {
		t.Fatalf("HandleStopEvent failed: %v", err)
	}
	if res.Skipped {
		t.Fatalf("unexpected skip: %s", res.Reason)
	}
	if res.ArtifactsCopied != 2 {
		t.Fatalf("expected 2 artifacts copied, got %d", res.ArtifactsCopied)
	}

	dstArtifactDir := filepath.Join(dataDir, repoID, "evidence", "artifacts", "conv-artifact-1")
	if _, err := os.Stat(filepath.Join(dstArtifactDir, "plan.md")); err != nil {
		t.Fatalf("plan.md not found in evidence store: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dstArtifactDir, "plan.md.metadata.json")); err != nil {
		t.Fatalf("plan.md.metadata.json not found in evidence store: %v", err)
	}
}
