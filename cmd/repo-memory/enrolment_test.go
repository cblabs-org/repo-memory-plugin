/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
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
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", t.TempDir())

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

	// Verify stdout mentions staged workspace plugin
	if !strings.Contains(stdout.String(), "Workspace plugin staged for this repository at .agents/plugins/repo-memory") {
		t.Errorf("expected output to mention staged workspace plugin, got: %s", stdout.String())
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

	// 1c. Verify .agents/plugins/repo-memory is staged as a real directory (no symlinks),
	// .agents/plugins.json was not created, and .git/info/exclude excludes it.
	stagedDir := filepath.Join(repoDir, ".agents", "plugins", "repo-memory")
	fi, err := os.Lstat(stagedDir)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", stagedDir, err)
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Errorf("expected %s to be a real non-symlink directory", stagedDir)
	}
	for _, rel := range []string{"plugin.json", "hooks.json", "mcp_config.json", filepath.Join("agents", "repo-memory-distiller.md")} {
		if _, err := os.Stat(filepath.Join(stagedDir, rel)); err != nil {
			t.Errorf("expected staged file %s to exist: %v", rel, err)
		}
	}
	if _, err := os.Stat(filepath.Join(stagedDir, "agents", "repo-memory-distiller")); !os.IsNotExist(err) {
		t.Errorf("expected duplicate agents/repo-memory-distiller directory not to exist")
	}
	_ = filepath.WalkDir(stagedDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr == nil && d.Type()&os.ModeSymlink != 0 {
			t.Errorf("unexpected symlink inside staged plugin tree: %s", path)
		}
		return nil
	})
	pluginsPath := filepath.Join(repoDir, ".agents", "plugins.json")
	if _, err := os.Stat(pluginsPath); !os.IsNotExist(err) {
		t.Errorf("expected .agents/plugins.json not to exist on fresh enroll")
	}
	agentsGitignore := filepath.Join(repoDir, ".git", "info", "exclude")
	agentsGitignoreData, err := os.ReadFile(agentsGitignore)
	if err != nil {
		t.Fatalf("expected .git/info/exclude to exist: %v", err)
	}
	if !strings.Contains(string(agentsGitignoreData), "/.agents/plugins/repo-memory/") {
		t.Errorf("expected .git/info/exclude to contain /.agents/plugins/repo-memory/, got: %s", string(agentsGitignoreData))
	}
	gitStatusCmd := exec.Command("git", "status", "--porcelain")
	gitStatusCmd.Dir = repoDir
	statusOut, err := gitStatusCmd.Output()
	if err != nil {
		t.Fatalf("git status failed: %v", err)
	}
	if strings.Contains(string(statusOut), ".agents/") {
		t.Errorf("expected .agents/plugins/repo-memory to be ignored by git status, got:\n%s", string(statusOut))
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
	if !entry.CreatedInstructionFile {
		t.Errorf("expected CreatedInstructionFile=true for fresh repo")
	}
	geminiPath := filepath.Join(repoDir, config.InstructionFilename)
	geminiData, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatalf("expected GEMINI.md to be created on fresh repo enroll: %v", err)
	}
	if !strings.Contains(string(geminiData), config.ManagedBlockBeginV2) {
		t.Errorf("expected GEMINI.md to contain v2 managed block, got: %s", string(geminiData))
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

	// Verify GEMINI.md now contains both original content and v2 managed block
	geminiData, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatal(err)
	}
	geminiStr := string(geminiData)
	if !strings.Contains(geminiStr, "# Existing Project Guidelines") {
		t.Errorf("lost existing GEMINI.md content: %s", geminiStr)
	}
	if !strings.Contains(geminiStr, config.ManagedBlockBeginV2) || !strings.Contains(geminiStr, "@[memory atlas](.memory/ATLAS.md)") {
		t.Errorf("expected GEMINI.md to contain v2 managed block, got: %s", geminiStr)
	}

	// Unenrolling should remove the managed block and preserve the pre-existing user content byte-for-byte
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&stdout)
	if err := RunUnenroll(cmdUnenroll, repoDir, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}
	restoredGemini, err := os.ReadFile(geminiPath)
	if err != nil {
		t.Fatalf("expected pre-existing GEMINI.md to remain after unenroll: %v", err)
	}
	if string(restoredGemini) != initialGemini {
		t.Errorf("expected GEMINI.md to match initial content byte-for-byte:\ngot:  %q\nwant: %q", string(restoredGemini), initialGemini)
	}
}

func TestLeaveSubcommand_WithoutAndWithPurge(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", t.TempDir())

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

	// .agents/plugins/repo-memory must be removed
	stagedDir := StagedWorkspacePluginDir(repoDir)
	if _, err := os.Lstat(stagedDir); !os.IsNotExist(err) {
		t.Errorf("expected .agents/plugins/repo-memory to be deleted after unenroll")
	}

	// Marker must be removed
	if config.IsEnrolledFast(repoDir) {
		t.Errorf("expected marker to be removed after unenroll")
	}
	if _, err := os.Stat(config.EnrolledMarkerPath(repoDir)); !os.IsNotExist(err) {
		t.Errorf("expected marker file to not exist after unenroll")
	}
	// GEMINI.md created by enroll must be deleted on unenroll
	if _, err := os.Stat(filepath.Join(repoDir, config.InstructionFilename)); !os.IsNotExist(err) {
		t.Errorf("expected GEMINI.md created by enroll to be deleted on unenroll")
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
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", t.TempDir())
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
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PostInvocation"}); err != nil {
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
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PostInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) == "{}" || !strings.Contains(out.String(), "PostInvocation") {
		t.Errorf("expected hook to proceed to stub when enrolled, got %q", out.String())
	}

	// 3. REPO_MEMORY_DISABLED=1: hook must return {}
	t.Setenv("REPO_MEMORY_DISABLED", "1")
	out.Reset()
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PostInvocation"}); err != nil {
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
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PostInvocation"}); err != nil {
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
	if err := hookCmd.RunE(hookCmd, []string{"antigravity", "PostInvocation"}); err != nil {
		t.Fatalf("hookCmd failed: %v", err)
	}
	if strings.TrimSpace(out.String()) != "{}" {
		t.Errorf("expected {} when repo disabled, got %q", out.String())
	}
}

func TestEnrollAndUnenrollAliases(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", t.TempDir())

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
	stagedDir := StagedWorkspacePluginDir(repoDir)
	if _, err := os.Stat(stagedDir); err != nil {
		t.Errorf("expected .agents/plugins/repo-memory to exist after init alias: %v", err)
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
	if _, err := os.Lstat(stagedDir); !os.IsNotExist(err) {
		t.Errorf("expected .agents/plugins/repo-memory to be deleted after leave alias")
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
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", t.TempDir())

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	// Pre-populate .agents/plugins.json with an external plugin AND a legacy repo-memory entry
	agentsDir := filepath.Join(repoDir, ".agents")
	if err := os.MkdirAll(filepath.Join(agentsDir, "plugins"), 0755); err != nil {
		t.Fatal(err)
	}
	existingJSON := `{
  "entries": [
    {
      "path": "/opt/other-plugin"
    },
    {
      "path": "/old/path/repo-memory-daemon/workspace-plugin"
    }
  ]
}
`
	pluginsPath := filepath.Join(agentsDir, "plugins.json")
	if err := os.WriteFile(pluginsPath, []byte(existingJSON), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-populate a legacy symlink at .agents/plugins/repo-memory pointing to an external dir
	symlinkTarget := t.TempDir()
	sentinel := filepath.Join(symlinkTarget, "preserve.txt")
	if err := os.WriteFile(sentinel, []byte("keep-me"), 0644); err != nil {
		t.Fatal(err)
	}
	stagedDir := StagedWorkspacePluginDir(repoDir)
	if err := os.Symlink(symlinkTarget, stagedDir); err != nil {
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

	// Verify .git/info/exclude preserves local-secret.json and adds staged exclusion
	giData, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(giData), "local-secret.json") || !strings.Contains(string(giData), "/.agents/plugins/repo-memory/") {
		t.Errorf("unexpected .git/info/exclude content: %s", string(giData))
	}

	// Verify symlink was unlinked (without deleting symlink target contents) and replaced with a real dir
	if sentinelData, err := os.ReadFile(sentinel); err != nil || string(sentinelData) != "keep-me" {
		t.Errorf("expected symlink target file to be untouched, got err=%v data=%q", err, string(sentinelData))
	}
	fi, err := os.Lstat(stagedDir)
	if err != nil || fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		t.Fatalf("expected %s to be replaced by a real directory", stagedDir)
	}

	// Verify .agents/plugins.json preserves /opt/other-plugin AND scrubbed legacy workspace-plugin
	pData, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(pData), "/opt/other-plugin") {
		t.Errorf("expected existing entry /opt/other-plugin to be preserved: %s", string(pData))
	}
	if strings.Contains(string(pData), "workspace-plugin") {
		t.Errorf("expected legacy workspace-plugin entry to be scrubbed on enroll: %s", string(pData))
	}

	// Re-run enroll (idempotency check)
	if err := RunEnroll(cmd, repoDir); err != nil {
		t.Fatalf("second RunEnroll failed: %v", err)
	}
	pData2, _ := os.ReadFile(pluginsPath)
	if !strings.Contains(string(pData2), "/opt/other-plugin") || strings.Contains(string(pData2), "workspace-plugin") {
		t.Errorf("unexpected plugins.json after idempotent RunEnroll: %s", string(pData2))
	}

	// Unenroll: should remove .agents/plugins/repo-memory while keeping /opt/other-plugin
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&buf)
	if err := RunUnenroll(cmdUnenroll, repoDir, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}
	if _, err := os.Lstat(stagedDir); !os.IsNotExist(err) {
		t.Errorf("expected staged directory %s to be removed on unenroll", stagedDir)
	}
	pData3, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("expected plugins.json to still exist because other entries remained: %v", err)
	}
	if !strings.Contains(string(pData3), "/opt/other-plugin") {
		t.Errorf("expected /opt/other-plugin to remain: %s", string(pData3))
	}

	// Also verify that when .agents/plugins.json ONLY contains legacy repo-memory, RunEnroll deletes it
	if err := os.WriteFile(pluginsPath, []byte(`{"entries":[{"path":"/old/repo-memory-daemon/workspace-plugin"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := RunEnroll(cmd, repoDir); err != nil {
		t.Fatalf("RunEnroll (empty plugins.json cleanup) failed: %v", err)
	}
	if _, err := os.Stat(pluginsPath); !os.IsNotExist(err) {
		t.Errorf("expected .agents/plugins.json to be deleted when only legacy repo-memory entry existed")
	}
}

func TestEnableDisableRepo_PluginsJSON(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", t.TempDir())

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

	stagedDir := StagedWorkspacePluginDir(repoDir)
	if _, err := os.Stat(stagedDir); err != nil {
		t.Fatalf("expected %s to exist: %v", stagedDir, err)
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
	if _, err := os.Lstat(stagedDir); !os.IsNotExist(err) {
		t.Errorf("expected %s to be deleted on disable --repo", stagedDir)
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
	if _, err := os.Stat(filepath.Join(stagedDir, "plugin.json")); err != nil {
		t.Errorf("expected %s to be re-staged on enable --repo: %v", stagedDir, err)
	}
}

func TestDoctor_Checks(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	// Set custom plugin dir and install a daemon bundle at v0.2.0
	pluginDir := t.TempDir()
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", pluginDir)

	origVersion := Version
	Version = "v0.2.0"
	defer func() { Version = origVersion }()

	origFix := doctorFixFlag
	doctorFixFlag = false
	t.Cleanup(func() { doctorFixFlag = origFix })

	if err := RunInstall(pluginDir, "/test/bin/repo-memory"); err != nil {
		t.Fatalf("RunInstall failed: %v", err)
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	// Case 1: Repo enrolled but .agents/plugins/repo-memory missing
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
	if !strings.Contains(buf.String(), "staged workspace plugin is missing at .agents/plugins/repo-memory") {
		t.Errorf("expected missing staged plugin warning, got: %s", buf.String())
	}

	// Case 2: .agents/plugins/repo-memory is a symlink -> doctor warns
	stagedDir := StagedWorkspacePluginDir(repoDir)
	if err := os.MkdirAll(filepath.Dir(stagedDir), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(ResolveWorkspacePluginDir(pluginDir), stagedDir); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), ".agents/plugins/repo-memory is a symlink") {
		t.Errorf("expected symlink warning, got: %s", buf.String())
	}
	_ = os.Remove(stagedDir)

	// Case 3: Real staged dir at stale version + legacy .agents/plugins.json entry -> doctor warns
	if err := EnsureWorkspacePluginConfigured(repoDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stagedDir, "plugin.json"), []byte(`{"name":"repo-memory","version":"0.0.1"}`), 0644); err != nil {
		t.Fatal(err)
	}
	pluginsPath := filepath.Join(repoDir, ".agents", "plugins.json")
	if err := os.WriteFile(pluginsPath, []byte(`{"entries":[{"path":"/old/repo-memory-daemon/workspace-plugin"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "staged workspace plugin version (0.0.1) does not match daemon version (0.2.0)") ||
		!strings.Contains(buf.String(), "legacy repo-memory entry found in .agents/plugins.json") {
		t.Errorf("expected version drift and legacy plugins.json warnings, got:\n%s", buf.String())
	}

	// Case 4: Second enrolled repo registered in registry.json missing staged plugin -> doctor detects & doctor --fix repairs BOTH
	repoDir2 := t.TempDir()
	runGit(t, repoDir2, "init")
	runGit(t, repoDir2, "config", "user.name", "Test User")
	runGit(t, repoDir2, "config", "user.email", "test@example.com")
	runGit(t, repoDir2, "commit", "--allow-empty", "-m", "initial commit")
	if err := config.EnsureRepoLayout(repoDir2); err != nil {
		t.Fatal(err)
	}
	if err := config.CreateEnrolledMarker(repoDir2); err != nil {
		t.Fatal(err)
	}
	if err := config.AddToRegistry(dataDir, []string{repoDir2}, "repo-2-id", repoDir2); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "has stale or un-staged workspace plugin") {
		t.Errorf("expected registry repo warning for repoDir2, got:\n%s", buf.String())
	}

	doctorFixFlag = true
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor --fix failed: %v", err)
	}
	doctorFixFlag = false

	// Verify both repoDir and repoDir2 now have real v0.2.0 staged plugins and repoDir's legacy plugins.json is deleted
	for _, r := range []string{repoDir, repoDir2} {
		sDir := StagedWorkspacePluginDir(r)
		sfi, err := os.Lstat(sDir)
		if err != nil || sfi.Mode()&os.ModeSymlink != 0 || !sfi.IsDir() {
			t.Errorf("expected repaired real directory at %s (err=%v)", sDir, err)
		}
		if gotVer := ReadInstalledPluginVersion(sDir); gotVer != "0.2.0" {
			t.Errorf("expected repaired version 0.2.0 at %s, got %q", sDir, gotVer)
		}
	}
	if _, err := os.Stat(pluginsPath); !os.IsNotExist(err) {
		t.Errorf("expected legacy plugins.json to be removed after doctor --fix")
	}

	// Clean doctor run after repair
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected clean doctor output after --fix, got: %s", buf.String())
	}

	// Case 5: GEMINI.md has unbalanced or duplicate managed block markers
	geminiPath := filepath.Join(repoDir, config.InstructionFilename)
	if err := os.WriteFile(geminiPath, []byte("# Guidelines\n<!-- repo-memory:begin v2 -->\nunclosed block\n"), 0644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "⚠ GEMINI.md managed block error:") || !strings.Contains(buf.String(), "unbalanced") {
		t.Errorf("expected doctor warning for unbalanced GEMINI.md markers, got: %s", buf.String())
	}

	// Case 6: GEMINI.md has duplicate begin markers
	if err := os.WriteFile(geminiPath, []byte("<!-- repo-memory:begin v2 -->\n<!-- repo-memory:begin v2 -->\n<!-- repo-memory:end -->\n"), 0644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "⚠ GEMINI.md managed block error:") || !strings.Contains(buf.String(), "duplicate") {
		t.Errorf("expected doctor warning for duplicate GEMINI.md markers, got: %s", buf.String())
	}
}

func TestDoctor_GraduatedSkillsLint(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("REPO_MEMORY_DATA_DIR", dataDir)

	pluginDir := t.TempDir()
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", pluginDir)
	if err := os.MkdirAll(filepath.Join(pluginDir, "workspace-plugin"), 0755); err != nil {
		t.Fatal(err)
	}

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Test User")
	runGit(t, repoDir, "config", "user.email", "test@example.com")
	runGit(t, repoDir, "commit", "--allow-empty", "-m", "initial commit")

	var buf bytes.Buffer
	cmdEnroll := enrollCmd
	cmdEnroll.SetOut(&buf)
	if err := RunEnroll(cmdEnroll, repoDir); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	repoID, _ := gitops.GenerateRepoID(repoDir)
	opsDir := filepath.Join(repoDir, ".memory", "ops")
	now := time.Now().UTC()
	addOp := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      "rec-doc-01",
		DossierScope: "build",
		Timestamp:    now,
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"entry_type":      "recipe",
			"kind":            "procedural",
			"text":            "Run build and test\n1. Run `go build ./...`\n2. Run `go test ./...`",
			"anchors":         []string{"go.mod"},
			"render_as_skill": "rm-build-test",
		},
	}
	if err := ops.NewWriter(opsDir).Write(ops.NewBatchWithTime([]ops.Op{addOp}, now)); err != nil {
		t.Fatal(err)
	}

	if _, err := skills.SyncGraduatedSkills(context.Background(), repoDir, dataDir, repoID, false); err != nil {
		t.Fatalf("SyncGraduatedSkills failed: %v", err)
	}

	skillPath := filepath.Join(repoDir, ".agents", "skills", "rm-build-test", "SKILL.md")
	origBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("reading graduated skill failed: %v", err)
	}

	// Corrupt the generated skill on disk while retaining generated_by: repo-memory
	if err := os.WriteFile(skillPath, []byte(string(origBytes)+"\n drifted manual line\n"), 0644); err != nil {
		t.Fatal(err)
	}

	buf.Reset()
	cmdDoctor := doctorCmd
	cmdDoctor.SetOut(&buf)
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if !strings.Contains(buf.String(), "⚠ Graduated skill .agents/skills/rm-build-test/SKILL.md is out of sync") {
		t.Errorf("expected doctor lint warning for drifted skill, got: %s", buf.String())
	}

	// Restore via SyncGraduatedSkills -> doctor should be completely clean
	if _, err := skills.SyncGraduatedSkills(context.Background(), repoDir, dataDir, repoID, false); err != nil {
		t.Fatalf("SyncGraduatedSkills failed: %v", err)
	}
	buf.Reset()
	if err := RunDoctor(cmdDoctor, repoDir); err != nil {
		t.Fatalf("RunDoctor failed: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("expected clean doctor output after SyncGraduatedSkills, got: %s", buf.String())
	}

	// Also seed a branch-local entry in .memory/local/ops/feat-branch and a user-tier entry
	// to verify RunStatus and RunReport project and count across tiers.
	branchOpsDir := filepath.Join(repoDir, ".memory", "local", "ops", "feat-branch")
	if err := os.MkdirAll(branchOpsDir, 0755); err != nil {
		t.Fatal(err)
	}
	branchAddOp := ops.Op{
		Type:         ops.OpAdd,
		EntryID:      "inv-branch-01",
		DossierScope: "branch-scope",
		Timestamp:    now,
		Source:       ops.SourceConsolidation,
		Fields: map[string]any{
			"entry_type": "invariant",
			"kind":       "semantic",
			"text":       "Branch-specific invariant for feat-branch",
			"anchors":    []string{"go.mod"},
		},
	}
	if err := ops.NewWriter(branchOpsDir).Write(ops.NewBatchWithTime([]ops.Op{branchAddOp}, now)); err != nil {
		t.Fatal(err)
	}

	// Also verify status and report commands output v2.2 information cleanly across tiers
	buf.Reset()
	cmdStatus := statusCmd
	cmdStatus.SetOut(&buf)
	if err := RunStatus(cmdStatus, repoDir, false); err != nil {
		t.Fatalf("RunStatus failed: %v", err)
	}
	if !strings.Contains(buf.String(), "memory:") ||
		!strings.Contains(buf.String(), "Dossiers:   2 active (2 active entries)") ||
		!strings.Contains(buf.String(), "Skills:     1 graduated") {
		t.Errorf("unexpected status output:\n%s", buf.String())
	}

	buf.Reset()
	cmdReport := reportCmd
	cmdReport.SetOut(&buf)
	if err := RunReport(cmdReport, repoDir, false); err != nil {
		t.Fatalf("RunReport failed: %v", err)
	}
	if !strings.Contains(buf.String(), "memory:") ||
		!strings.Contains(buf.String(), "Entries:    2 active (semantic: 1, procedural: 1, episodic: 0)") ||
		!strings.Contains(buf.String(), "rm-build-test") {
		t.Errorf("unexpected report output:\n%s", buf.String())
	}
}

