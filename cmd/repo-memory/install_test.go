package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

)

func TestInstallIdempotencyAndLayout(t *testing.T) {
	tempPluginDir := t.TempDir()
	binPath := "/test/bin/repo-memory"

	// First install run
	if err := RunInstall(tempPluginDir, binPath); err != nil {
		t.Fatalf("first RunInstall failed: %v", err)
	}

	// Verify required subdirectories exist
	expectedDirs := []string{
		filepath.Join(tempPluginDir, "sidecars", "tick"),
		filepath.Join(tempPluginDir, "workspace-plugin", "repo-memory"),
		filepath.Join(tempPluginDir, "workspace-plugin", "repo-memory", "agents"),
		filepath.Join(tempPluginDir, "workspace-plugin", "repo-memory", "agents", "repo-memory-distiller"),
	}
	for _, dir := range expectedDirs {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("expected directory %s to exist: %v", dir, err)
		} else if !info.IsDir() {
			t.Errorf("expected %s to be a directory", dir)
		}
	}

	// Verify required JSON files exist and are valid JSON
	expectedFiles := []struct {
		relPath     string
		mustContain string
	}{
		{
			relPath:     "plugin.json",
			mustContain: "repo-memory-daemon",
		},
		{
			relPath:     "config.json",
			mustContain: `"enabled": true`,
		},
		{
			relPath:     filepath.Join("sidecars", "tick", "sidecar.json"),
			mustContain: "Repo Memory Tick",
		},
		{
			relPath:     filepath.Join("workspace-plugin", "repo-memory", "plugin.json"),
			mustContain: "repo-memory",
		},
		{
			relPath:     filepath.Join("workspace-plugin", "repo-memory", "hooks.json"),
			mustContain: "/test/bin/repo-memory hook antigravity",
		},
		{
			relPath:     filepath.Join("workspace-plugin", "repo-memory", "mcp_config.json"),
			mustContain: "/test/bin/repo-memory",
		},
	}

	for _, ef := range expectedFiles {
		fullPath := filepath.Join(tempPluginDir, ef.relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", fullPath, err)
			continue
		}
		var parsed interface{}
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Errorf("file %s contains invalid JSON: %v", fullPath, err)
		}
		if !strings.Contains(string(data), ef.mustContain) {
			t.Errorf("file %s does not contain expected substring %q", fullPath, ef.mustContain)
		}
	}

	// Verify repo-memory-distiller agent markdown files exist in both directory and flat forms
	distillerPaths := []string{
		filepath.Join(tempPluginDir, "workspace-plugin", "repo-memory", "agents", "repo-memory-distiller", "agent.md"),
		filepath.Join(tempPluginDir, "workspace-plugin", "repo-memory", "agents", "repo-memory-distiller.md"),
	}
	for _, dp := range distillerPaths {
		data, err := os.ReadFile(dp)
		if err != nil {
			t.Errorf("expected distiller agent file %s to exist: %v", dp, err)
			continue
		}
		if !strings.Contains(string(data), "name: repo-memory-distiller") || !strings.Contains(string(data), "echo_token:") {
			t.Errorf("distiller agent file %s missing expected content", dp)
		}
	}

	// Verify sidecar args and restart_policy
	sidecarPath := filepath.Join(tempPluginDir, "sidecars", "tick", "sidecar.json")
	sidecarData, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("reading sidecar.json: %v", err)
	}
	var sidecarMap map[string]interface{}
	if err := json.Unmarshal(sidecarData, &sidecarMap); err != nil {
		t.Fatalf("unmarshaling sidecar.json: %v", err)
	}
	if sidecarMap["builtin"] != "schedule" {
		t.Errorf("expected builtin 'schedule', got %v", sidecarMap["builtin"])
	}
	if sidecarMap["restart_policy"] != "always" {
		t.Errorf("expected restart_policy 'always', got %v", sidecarMap["restart_policy"])
	}
	if _, ok := sidecarMap["agent_permissions"]; ok {
		t.Errorf("expected no agent_permissions in sidecar.json")
	}
	args, ok := sidecarMap["args"].([]interface{})
	if !ok || len(args) < 3 {
		t.Fatalf("expected sidecar args list with at least 3 items, got %v", sidecarMap["args"])
	}
	if args[0] != "*/1 * * * *" || args[1] != binPath || args[2] != "tick" {
		t.Errorf("unexpected sidecar args: %v", args)
	}

	// Second install run (upgrade in place / idempotency) with new binary path
	newBinPath := "/new/path/repo-memory"
	if err := RunInstall(tempPluginDir, newBinPath); err != nil {
		t.Fatalf("second RunInstall failed: %v", err)
	}

	updatedSidecar, err := os.ReadFile(sidecarPath)
	if err != nil {
		t.Fatalf("reading updated sidecar.json: %v", err)
	}
	if !strings.Contains(string(updatedSidecar), newBinPath) {
		t.Errorf("expected updated sidecar to contain new binary path %s", newBinPath)
	}
}

func TestUninstall(t *testing.T) {
	tempPluginDir := t.TempDir()
	tempDataDir := t.TempDir()

	// Install something into plugin dir
	if err := RunInstall(tempPluginDir, "/dummy/bin"); err != nil {
		t.Fatalf("RunInstall failed: %v", err)
	}
	// Create a dummy file in data dir
	dummyFile := filepath.Join(tempDataDir, "test.txt")
	if err := os.WriteFile(dummyFile, []byte("data"), 0644); err != nil {
		t.Fatalf("writing dummy data file: %v", err)
	}

	// 1. Uninstall without purge
	if err := RunUninstall(tempPluginDir, tempDataDir, false); err != nil {
		t.Fatalf("RunUninstall (no purge) failed: %v", err)
	}

	// Plugin dir should be removed
	if _, err := os.Stat(tempPluginDir); !os.IsNotExist(err) {
		t.Errorf("expected plugin dir %s to be removed, but stat was: %v", tempPluginDir, err)
	}

	// Data dir should still exist
	if _, err := os.Stat(dummyFile); err != nil {
		t.Errorf("expected data file %s to still exist: %v", dummyFile, err)
	}

	// 2. Uninstall with purge
	if err := RunUninstall(tempPluginDir, tempDataDir, true); err != nil {
		t.Fatalf("RunUninstall (with purge) failed: %v", err)
	}
	// Data dir should now be removed
	if _, err := os.Stat(tempDataDir); !os.IsNotExist(err) {
		t.Errorf("expected data dir %s to be purged, but stat was: %v", tempDataDir, err)
	}

	// 3. Uninstall on non-existent directories should succeed cleanly (idempotent)
	if err := RunUninstall(tempPluginDir, tempDataDir, true); err != nil {
		t.Errorf("RunUninstall on already removed directories failed: %v", err)
	}
}

func TestResolveAntigravityPluginDir(t *testing.T) {
	// Test REPO_MEMORY_PLUGIN_DIR override
	customDir := "/custom/plugin/dir"
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", customDir)
	res, err := ResolveAntigravityPluginDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res != customDir {
		t.Errorf("expected %s, got %s", customDir, res)
	}

	// Test ANTIGRAVITY_CONFIG_DIR override
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", "")
	t.Setenv("ANTIGRAVITY_CONFIG_DIR", "/custom/config")
	res, err = ResolveAntigravityPluginDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := filepath.Join("/custom/config", "plugins", "repo-memory-daemon")
	if res != expected {
		t.Errorf("expected %s, got %s", expected, res)
	}
}

func TestResolveWorkspacePluginDir(t *testing.T) {
	daemonDir := "/custom/daemon"
	res := ResolveWorkspacePluginDir(daemonDir)
	expected := filepath.Join(daemonDir, "workspace-plugin", "repo-memory")
	if res != expected {
		t.Errorf("expected %s, got %s", expected, res)
	}
}

func TestMigrationFromLegacyLayout(t *testing.T) {
	t.Setenv("REPO_MEMORY_DATA_DIR", t.TempDir())
	parentDir := t.TempDir()
	legacyDir := filepath.Join(parentDir, "repo-memory")
	if err := os.MkdirAll(filepath.Join(legacyDir, "sub"), 0755); err != nil {
		t.Fatalf("creating legacy dir: %v", err)
	}
	dummyFile := filepath.Join(legacyDir, "sub", "old_config.txt")
	if err := os.WriteFile(dummyFile, []byte("legacy data"), 0644); err != nil {
		t.Fatalf("writing dummy legacy file: %v", err)
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	daemonDir := filepath.Join(parentDir, "repo-memory-daemon")
	binPath := "/test/bin/repo-memory"

	if err := RunInstall(daemonDir, binPath); err != nil {
		t.Fatalf("RunInstall failed during migration: %v", err)
	}

	// Legacy dir should no longer exist
	if _, err := os.Stat(legacyDir); !os.IsNotExist(err) {
		t.Errorf("expected legacy dir %s to be migrated, but it still exists", legacyDir)
	}

	// Backup dir should exist with today's date
	expectedBackupDir := filepath.Join(tempHome, ".gemini", "antigravity", "sidecar_data", "repo-memory-daemon", "backups", "repo-memory.bak-"+time.Now().Format("20060102"))
	if _, err := os.Stat(expectedBackupDir); err != nil {
		t.Errorf("expected backup dir %s to exist: %v", expectedBackupDir, err)
	}
	backedUpFile := filepath.Join(expectedBackupDir, "sub", "old_config.txt")
	if content, err := os.ReadFile(backedUpFile); err != nil || string(content) != "legacy data" {
		t.Errorf("expected backed up file to exist with legacy data: %v", err)
	}

	// Daemon dir should be cleanly installed
	if _, err := os.Stat(filepath.Join(daemonDir, "plugin.json")); err != nil {
		t.Errorf("expected daemon plugin.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(daemonDir, "config.json")); err != nil {
		t.Errorf("expected daemon config.json to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(daemonDir, "workspace-plugin", "repo-memory", "plugin.json")); err != nil {
		t.Errorf("expected workspace plugin.json to exist: %v", err)
	}
}

func TestSubcommandsExecution(t *testing.T) {
	subcommands := []struct {
		name string
		args []string
	}{
		{"hook", []string{"hook", "antigravity", "Stop"}},
		{"tick", []string{"tick"}},
		{"run", []string{"run"}},
		{"mcp", []string{"mcp"}},
		{"report", []string{"report"}},
		{"status", []string{"status"}},
		{"reindex", []string{"reindex"}},
		{"doctor", []string{"doctor"}},
	}

	for _, sc := range subcommands {
		t.Run(sc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(sc.args)

			err := rootCmd.Execute()
			if err != nil {
				t.Fatalf("command %v failed: %v", sc.args, err)
			}
		})
	}
}

func TestInstallFlagValidation(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"install", "--harness", "invalid-harness"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error for unsupported harness, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported harness") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUninstallFlagValidation(t *testing.T) {
	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetErr(buf)
	rootCmd.SetArgs([]string{"uninstall", "--harness", "invalid-harness"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatalf("expected error for unsupported harness, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported harness") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidateHooksJSON(t *testing.T) {
	// Valid
	validJSON := []byte(`{
		"repo-memory": {
			"enabled": true,
			"PreInvocation": [{"type": "command", "command": "repo-memory hook antigravity PreInvocation"}],
			"PreToolUse": [{"matcher": "run_command", "hooks": [{"type": "command", "command": "repo-memory hook antigravity PreToolUse"}]}]
		}
	}`)
	if err := ValidateHooksJSON(validJSON); err != nil {
		t.Errorf("expected valid JSON to pass validation, got: %v", err)
	}

	// Empty object {}
	emptyJSON := []byte(`{}`)
	if err := ValidateHooksJSON(emptyJSON); err == nil {
		t.Error("expected empty object {} to fail validation")
	}

	// Empty repo-memory block
	emptyBlock := []byte(`{"repo-memory": {"enabled": true}}`)
	if err := ValidateHooksJSON(emptyBlock); err == nil {
		t.Error("expected empty hook group to fail validation")
	}

	// Malformed JSON
	malformed := []byte(`{invalid`)
	if err := ValidateHooksJSON(malformed); err == nil {
		t.Error("expected malformed JSON to fail validation")
	}
}

