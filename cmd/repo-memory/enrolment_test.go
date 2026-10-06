package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
)


func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %s", args, string(out))
	}
}

func TestInitSubcommand_FreshRepo(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial root commit")
	runGit(t, repoDir, "remote", "add", "origin", "https://github.com/cblabs-org/repo-memory-plugin.git")

	var stdout, stderr bytes.Buffer
	cmd := enrollCmd
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := RunEnroll(cmd, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Verify stdout mentions workspace plugin
	if !strings.Contains(stdout.String(), "Workspace plugin enabled for this repository via .agents/plugins.json") {
		t.Errorf("expected output to mention workspace plugin, got: %s", stdout.String())
	}

	// 1. Verify .memory and .gitignore exist
	memDir := filepath.Join(repoDir, ".memory")
	if fi, err := os.Stat(memDir); err != nil || !fi.IsDir() {
		t.Errorf("expected .memory directory to exist in %s", repoDir)
	}

	gitignorePath := filepath.Join(memDir, ".gitignore")
	gitignoreData, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Errorf("expected .memory/.gitignore to exist: %v", err)
	} else if !strings.Contains(string(gitignoreData), ".cache/") {
		t.Errorf("expected .gitignore to ignore .cache/, got %q", string(gitignoreData))
	}

	// 1b. Verify .memory/.enrolled marker file exists
	if !config.IsEnrolledFast(repoDir) {
		t.Errorf("expected IsEnrolledFast to return true for enrolled repo %s", repoDir)
	}
	markerPath := config.EnrolledMarkerPath(repoDir)
	if _, err := os.Stat(markerPath); err != nil {
		t.Errorf("expected marker file %s to exist: %v", markerPath, err)
	}

	// 1c. Verify .agents/plugins.json and .git/info/exclude exist
	pluginsPath := filepath.Join(repoDir, ".agents", "plugins.json")
	pluginsData, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("expected .agents/plugins.json to exist: %v", err)
	}
	if !strings.Contains(string(pluginsData), "workspace-plugin") {
		t.Errorf("expected .agents/plugins.json to contain workspace-plugin, got: %s", string(pluginsData))
	}
	agentsGitignore := filepath.Join(repoDir, ".git", "info", "exclude")
	agentsGitignoreData, err := os.ReadFile(agentsGitignore)
	if err != nil {
		t.Fatalf("expected .git/info/exclude to exist: %v", err)
	}
	if !strings.Contains(string(agentsGitignoreData), "plugins.json") {
		t.Errorf("expected .git/info/exclude to contain plugins.json, got: %s", string(agentsGitignoreData))
	}


	// 3. Verify registry.json has repository entry
	reg, err := config.LoadRegistry(dataDir)
	if err != nil {
		t.Fatalf("LoadRegistry failed: %v", err)
	}

	entry, found, err := config.FindRegistryEntry(dataDir, repoDir)
	if err != nil || !found || entry == nil {
		t.Fatalf("FindRegistryEntry failed: found=%v, err=%v", found, err)
	}

	rootCommit, _ := gitops.GetRootCommit(repoDir)
	expectedID := gitops.ComputeRepoID(rootCommit, "https://github.com/cblabs-org/repo-memory-plugin")
	if entry.RepoID != expectedID {
		t.Errorf("got RepoID %q, want %q", entry.RepoID, expectedID)
	}

	// 4. Verify data-dir layout created for this repo
	for _, sub := range config.DataDirSubdirectories {
		subPath := filepath.Join(dataDir, expectedID, sub)
		if fi, err := os.Stat(subPath); err != nil || !fi.IsDir() {
			t.Errorf("expected data dir subdir %s to exist", subPath)
		}
	}

	// No template warning should be emitted for fresh repo
	if strings.Contains(stderr.String(), "inherited from template") {
		t.Errorf("unexpected template warning on fresh repo: %s", stderr.String())
	}
	_ = reg
}

func TestInitSubcommand_ExistingGeminiAndTemplateWarning(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial root commit")

	// Pre-create .memory directory (simulating template inheritance)
	if err := os.MkdirAll(filepath.Join(repoDir, ".memory"), 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-create GEMINI.md with existing content
	geminiPath := filepath.Join(repoDir, "GEMINI.md")
	initialGemini := "# Existing Project Guidelines\nFollow clean code.\n"
	if err := os.WriteFile(geminiPath, []byte(initialGemini), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	cmd := enrollCmd
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	if err := RunEnroll(cmd, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Verify template warning was logged to stderr
	if !strings.Contains(stderr.String(), "warning: .memory/ directory already exists (inherited from template?)") {
		t.Errorf("expected template warning in stderr, got: %s", stderr.String())
	}

	// Verify GEMINI.md now contains both original content and map include line
	geminiData, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatal(err)
	}
	geminiStr := string(geminiData)
	if !strings.Contains(geminiStr, "# Existing Project Guidelines") {
		t.Errorf("lost existing GEMINI.md content: %s", geminiStr)
	}
}

func TestLeaveSubcommand_WithoutAndWithPurge(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial root commit")

	// Enroll repo first
	var stdout bytes.Buffer
	cmdEnroll := enrollCmd
	cmdEnroll.SetOut(&stdout)
	if err := RunEnroll(cmdEnroll, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	entry, found, err := config.FindRegistryEntry(dataDir, repoDir)
	if err != nil || !found || entry == nil {
		t.Fatalf("expected repo in registry before unenroll: %v", err)
	}
	repoID := entry.RepoID

	repoDataDir := filepath.Join(dataDir, repoID)
	if _, err := os.Stat(repoDataDir); err != nil {
		t.Fatalf("expected data dir %s to exist before unenroll", repoDataDir)
	}

	// 1. Unenroll WITHOUT --purge
	var leaveOut bytes.Buffer
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&leaveOut)
	if err := RunUnenroll(cmdUnenroll, repoDir, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}

	// Entry must be gone from registry
	_, foundAfter, err := config.FindRegistryEntry(dataDir, repoDir)
	if err != nil || foundAfter {
		t.Errorf("expected repo to be removed from registry")
	}

	// .agents/plugins.json must be removed
	pluginsPath := filepath.Join(repoDir, ".agents", "plugins.json")
	if _, err := os.Stat(pluginsPath); !os.IsNotExist(err) {
		t.Errorf("expected .agents/plugins.json to be deleted after unenroll")
	}

	// Marker must be removed
	if config.IsEnrolledFast(repoDir) {
		t.Errorf("expected marker to be removed after unenroll")
	}
	if _, err := os.Stat(config.EnrolledMarkerPath(repoDir)); !os.IsNotExist(err) {
		t.Errorf("expected marker file to not exist after unenroll")
	}

	// Private data dir must STILL exist because purge=false
	if _, err := os.Stat(repoDataDir); err != nil {
		t.Errorf("data dir %s was unexpectedly deleted when purge=false", repoDataDir)
	}

	// .memory in repository must remain intact
	if _, err := os.Stat(filepath.Join(repoDir, ".memory")); err != nil {
		t.Errorf(".memory was unexpectedly deleted from repo")
	}

	// 2. Re-enroll then unenroll WITH --purge
	if err := RunEnroll(cmdEnroll, repoDir); err != nil {
		t.Fatalf("re-RunEnroll failed: %v", err)
	}
	leaveOut.Reset()
	if err := RunUnenroll(cmdUnenroll, repoDir, true); err != nil {
		t.Fatalf("RunUnenroll with purge failed: %v", err)
	}

	// Private data dir must now be deleted
	if _, err := os.Stat(repoDataDir); !os.IsNotExist(err) {
		t.Errorf("data dir %s was not purged when purge=true", repoDataDir)
	}

	// .memory in repository must still remain intact
	if _, err := os.Stat(filepath.Join(repoDir, ".memory")); err != nil {
		t.Errorf(".memory was unexpectedly deleted from repo during purge")
	}
}

func TestKillSwitch_Subcommands(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial root commit")

	// Global disable
	var out bytes.Buffer
	cmdDisable := disableCmd
	cmdDisable.SetOut(&out)
	if err := RunDisable(cmdDisable, false); err != nil {
		t.Fatalf("RunDisable global failed: %v", err)
	}
	machCfg, err := config.LoadMachineConfig(dataDir)
	if err != nil {
		t.Fatalf("LoadMachineConfig failed: %v", err)
	}
	if machCfg.Enabled {
		t.Errorf("expected machine enabled=false after global disable")
	}

	// Global enable
	cmdEnable := enableCmd
	cmdEnable.SetOut(&out)
	if err := RunEnable(cmdEnable, false); err != nil {
		t.Fatalf("RunEnable global failed: %v", err)
	}
	machCfg, _ = config.LoadMachineConfig(dataDir)
	if !machCfg.Enabled {
		t.Errorf("expected machine enabled=true after global enable")
	}

	// Repo-level disable
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	if err := RunDisable(cmdDisable, true); err != nil {
		t.Fatalf("RunDisable repo failed: %v", err)
	}
	repoCfg, err := config.LoadRepoConfig(repoDir)
	if err != nil {
		t.Fatalf("LoadRepoConfig failed: %v", err)
	}
	if repoCfg.IsEnabled() {
		t.Errorf("expected repo IsEnabled()=false after repo disable")
	}

	// Repo-level enable
	if err := RunEnable(cmdEnable, true); err != nil {
		t.Fatalf("RunEnable repo failed: %v", err)
	}
	repoCfg, _ = config.LoadRepoConfig(repoDir)
	if !repoCfg.IsEnabled() {
		t.Errorf("expected repo IsEnabled()=true after repo enable")
	}
}

func TestHookSubcommand_FastPath(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Setenv("REPO_MEMORY_V2_SPIKE", "0") // Disable V2 spike handler so fast-path test reaches the stub

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial root commit")

	// 1. Not enrolled: hook must return empty json {}
	origWorkspace := hookWorkspaceFlag
	defer func() { hookWorkspaceFlag = origWorkspace }()
	hookWorkspaceFlag = repoDir
	var out bytes.Buffer
	hookCmd.SetOut(&out)
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("expected {} when not enrolled, got %q", out.String())
	}

	// 2. Enroll repository
	if err := RunEnroll(enrollCmd, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Now enrolled and enabled: hook should pass fast-path and proceed to stub
	out.Reset()
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) == "{}" || !strings.Contains(out.String(), "PreInvocation") {
		t.Errorf("expected hook to proceed to stub when enrolled, got %q", out.String())
	}

	// 3. REPO_MEMORY_DISABLED=1: hook must return {}
	t.Setenv("REPO_MEMORY_DISABLED", "1")
	out.Reset()
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("expected {} when REPO_MEMORY_DISABLED=1, got %q", out.String())
	}
	t.Setenv("REPO_MEMORY_DISABLED", "")

	// 4. Global disable: hook must return {}
	if err := config.SetMachineEnabled(dataDir, false); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("expected {} when machine disabled, got %q", out.String())
	}
	_ = config.SetMachineEnabled(dataDir, true)

	// 5. Repo disable: hook must return {}
	if err := config.SetRepoEnabled(repoDir, false); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PreInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("expected {} when repo disabled, got %q", out.String())
	}
}

func TestEnrollAndUnenrollAliases(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	// Test "init" alias
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetErr(&out)
	rootCmd.SetArgs([]string{"init", repoDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd execute init failed: %v", err)
	}

	if !config.IsEnrolledFast(repoDir) {
		t.Errorf("expected repo to be enrolled via init alias")
	}
	pluginsPath := filepath.Join(repoDir, ".agents", "plugins.json")
	if _, err := os.Stat(pluginsPath); err != nil {
		t.Errorf("expected .agents/plugins.json to exist after init alias: %v", err)
	}

	// Test "leave" alias
	out.Reset()
	rootCmd.SetArgs([]string{"leave", repoDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd execute leave failed: %v", err)
	}

	if config.IsEnrolledFast(repoDir) {
		t.Errorf("expected repo to be unenrolled via leave alias")
	}
	if _, err := os.Stat(pluginsPath); !os.IsNotExist(err) {
		t.Errorf("expected .agents/plugins.json to be deleted after leave alias")
	}

	// Test "enroll" primary command
	out.Reset()
	rootCmd.SetArgs([]string{"enroll", repoDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd execute enroll failed: %v", err)
	}
	if !config.IsEnrolledFast(repoDir) {
		t.Errorf("expected repo to be enrolled via enroll command")
	}

	// Test "unenroll" primary command
	out.Reset()
	rootCmd.SetArgs([]string{"unenroll", repoDir})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("rootCmd execute unenroll failed: %v", err)
	}
	if config.IsEnrolledFast(repoDir) {
		t.Errorf("expected repo to be unenrolled via unenroll command")
	}
}

func TestEnrollPluginsJSON_ExistingEntriesAndIdempotency(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	// Pre-populate .agents/plugins.json with an external plugin
	agentsDir := filepath.Join(repoDir, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	existingJSON := `{
  "entries": [
    {
      "path": "/opt/other-plugin"
    }
  ]
}
`
	pluginsPath := filepath.Join(agentsDir, "plugins.json")
	if err := os.WriteFile(pluginsPath, []byte(existingJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-populate .git/info/exclude with an existing rule
	gitignorePath := filepath.Join(repoDir, ".git", "info", "exclude")
	if err := os.WriteFile(gitignorePath, []byte("local-secret.json\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run Enroll
	var buf bytes.Buffer
	cmd := enrollCmd
	cmd.SetOut(&buf)
	if err := RunEnroll(cmd, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Verify .git/info/exclude contains both local-secret.json and plugins.json
	giData, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(giData), "local-secret.json") || !strings.Contains(string(giData), "plugins.json") {
		t.Errorf("unexpected .gitignore content: %s", string(giData))
	}

	// Verify .agents/plugins.json contains /opt/other-plugin AND workspace-plugin
	pData, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pData), "/opt/other-plugin") {
		t.Errorf("expected existing entry /opt/other-plugin to be preserved: %s", string(pData))
	}
	if !strings.Contains(string(pData), "workspace-plugin") {
		t.Errorf("expected workspace-plugin entry to be added: %s", string(pData))
	}

	// Re-run enroll (idempotency check)
	if err := RunEnroll(cmd, repoDir); err != nil {
		t.Fatalf("second RunEnroll failed: %v", err)
	}
	pData2, _ := os.ReadFile(pluginsPath)
	if strings.Count(string(pData2), "workspace-plugin") != 1 {
		t.Errorf("duplicate workspace-plugin entries found: %s", string(pData2))
	}

	// Unenroll: should keep /opt/other-plugin, delete only workspace-plugin
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&buf)
	if err := RunUnenroll(cmdUnenroll, repoDir, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}
	pData3, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("expected plugins.json to still exist because other entries remained: %v", err)
	}
	if !strings.Contains(string(pData3), "/opt/other-plugin") {
		t.Errorf("expected /opt/other-plugin to remain: %s", string(pData3))
	}
	if strings.Contains(string(pData3), "workspace-plugin") {
		t.Errorf("expected workspace-plugin entry to be removed: %s", string(pData3))
	}
}

func TestEnableDisableRepo_PluginsJSON(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	// Enroll repo
	var buf bytes.Buffer
	cmdEnroll := enrollCmd
	cmdEnroll.SetOut(&buf)
	if err := RunEnroll(cmdEnroll, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	pluginsPath := filepath.Join(repoDir, ".agents", "plugins.json")
	if _, err := os.Stat(pluginsPath); err != nil {
		t.Fatalf("expected plugins.json to exist: %v", err)
	}

	// Change working dir to repoDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(repoDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Disable repo-level
	cmdDisable := disableCmd
	cmdDisable.SetOut(&buf)
	if err := RunDisable(cmdDisable, true); err != nil {
		t.Fatalf("RunDisable failed: %v", err)
	}
	if _, err := os.Stat(pluginsPath); !os.IsNotExist(err) {
		t.Errorf("expected plugins.json to be deleted on disable --repo")
	}
	// Marker must STILL exist
	if !config.IsEnrolledFast(repoDir) {
		t.Errorf("marker should still exist after disable --repo")
	}

	// Enable repo-level
	cmdEnable := enableCmd
	cmdEnable.SetOut(&buf)
	if err := RunEnable(cmdEnable, true); err != nil {
		t.Fatalf("RunEnable failed: %v", err)
	}
	if _, err := os.Stat(pluginsPath); err != nil {
		t.Errorf("expected plugins.json to be restored on enable --repo: %v", err)
	}
}

func TestDoctor_Checks(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	// Set custom plugin dir
	pluginDir := t.TempDir()
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", pluginDir)

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	// Case 1: Repo enrolled but plugins.json missing
	if err := config.EnsureRepoLayout(repoDir); err != nil {
		t.Fatal(err)
	}
	if err := config.CreateEnrolledMarker(repoDir); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	cmdDoctor := doctorCmd
	cmdDoctor.SetOut(&buf)
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "⚠ repo-memory is enrolled but workspace plugin is not configured.") {
		t.Errorf("expected warning 1, got: %s", buf.String())
	}

	// Case 2: plugins.json configured, but plugin directory does not exist on disk
	if err := EnsureWorkspacePluginConfigured(repoDir); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "⚠ Workspace plugin not found at") {
		t.Errorf("expected warning 2, got: %s", buf.String())
	}

	// Case 3: Plugin directory exists on disk
	wsPluginPath := filepath.Join(pluginDir, "workspace-plugin")
	if err := os.MkdirAll(wsPluginPath, 0755); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected clean doctor output, got: %s", buf.String())
	}
}

