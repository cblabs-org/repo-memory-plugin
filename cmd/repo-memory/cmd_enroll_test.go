package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
)

func TestEnrollUnenrollFlow(t *testing.T) {
	// 1. Create temp dirs for plugin and repo
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	
	tempPluginDir := filepath.Join(t.TempDir(), "plugins", "repo-memory-daemon")
	tempDataDir := filepath.Join(t.TempDir(), "data")
	tempRepo := filepath.Join(t.TempDir(), "test-repo")
	if err := os.MkdirAll(tempRepo, 0755); err != nil {
		t.Fatalf("creating test repo dir: %v", err)
	}

	runGit(t, tempRepo, "init")
	runGit(t, tempRepo, "config", "user.name", "Test User")
	runGit(t, tempRepo, "config", "user.email", "test@example.com")
	runGit(t, tempRepo, "commit", "--allow-empty", "-m", "initial commit")

	// 2. Set REPO_MEMORY_PLUGIN_DIR to temp plugin dir
	t.Setenv("REPO_MEMORY_PLUGIN_DIR", tempPluginDir)
	t.Setenv("REPO_MEMORY_DATA_DIR", tempDataDir)

	fakeBinPath := filepath.Join(t.TempDir(), "bin", "repo-memory")
	if err := os.MkdirAll(filepath.Dir(fakeBinPath), 0755); err != nil {
		t.Fatalf("creating bin dir: %v", err)
	}
	if err := os.WriteFile(fakeBinPath, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatalf("writing fake bin: %v", err)
	}

	// 3. Run install (creates daemon + workspace-plugin layout)
	if err := RunInstall(tempPluginDir, fakeBinPath); err != nil {
		t.Fatalf("RunInstall failed: %v", err)
	}

	// 4. Verify directory structure matches target
	wsPluginDir := filepath.Join(tempPluginDir, "workspace-plugin", "repo-memory")
	requiredPaths := []string{
		filepath.Join(tempPluginDir, "plugin.json"),
		filepath.Join(tempPluginDir, "config.json"),
		filepath.Join(tempPluginDir, "sidecars", "tick", "sidecar.json"),
		filepath.Join(wsPluginDir, "plugin.json"),
		filepath.Join(wsPluginDir, "hooks.json"),
		filepath.Join(wsPluginDir, "mcp_config.json"),
	}
	for _, p := range requiredPaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist: %v", p, err)
		}
	}

	// Verify daemon plugin.json
	daemonJSONData, err := os.ReadFile(filepath.Join(tempPluginDir, "plugin.json"))
	if err != nil {
		t.Fatalf("reading daemon plugin.json: %v", err)
	}
	var daemonMap map[string]any
	if err := json.Unmarshal(daemonJSONData, &daemonMap); err != nil || daemonMap["name"] != "repo-memory-daemon" {
		t.Errorf("expected daemon plugin name 'repo-memory-daemon', got: %v", daemonMap["name"])
	}

	// Verify sidecar.json has NO agent_permissions
	sidecarData, err := os.ReadFile(filepath.Join(tempPluginDir, "sidecars", "tick", "sidecar.json"))
	if err != nil {
		t.Fatalf("reading sidecar.json: %v", err)
	}
	var sidecarMap map[string]any
	if err := json.Unmarshal(sidecarData, &sidecarMap); err != nil {
		t.Fatalf("parsing sidecar.json: %v", err)
	}
	if _, ok := sidecarMap["agent_permissions"]; ok {
		t.Errorf("sidecar.json should NOT contain agent_permissions")
	}

	// Verify workspace plugin.json
	wsJSONData, err := os.ReadFile(filepath.Join(wsPluginDir, "plugin.json"))
	if err != nil {
		t.Fatalf("reading workspace plugin.json: %v", err)
	}
	var wsMap map[string]any
	if err := json.Unmarshal(wsJSONData, &wsMap); err != nil || wsMap["name"] != "repo-memory" {
		t.Errorf("expected workspace plugin name 'repo-memory', got: %v", wsMap["name"])
	}

	// 5. Run enroll on temp repo
	var stdout bytes.Buffer
	cmdEnroll := enrollCmd
	cmdEnroll.SetOut(&stdout)
	if err := RunEnroll(cmdEnroll, tempRepo); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Verify .memory/.enrolled marker
	if !config.IsEnrolledFast(tempRepo) {
		t.Errorf("expected IsEnrolledFast to return true after enroll")
	}

	// 6. Verify .agents/plugins.json exists and points to workspace-plugin
	pluginsPath := filepath.Join(tempRepo, ".agents", "plugins.json")
	pluginsData, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", pluginsPath, err)
	}
	var pluginsConfig map[string]any
	if err := json.Unmarshal(pluginsData, &pluginsConfig); err != nil {
		t.Fatalf("parsing plugins.json: %v", err)
	}
	entries, ok := pluginsConfig["entries"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("expected non-empty entries in plugins.json, got: %v", pluginsConfig["entries"])
	}
	expectedWsPath := filepath.Join(tempPluginDir, "workspace-plugin")
	firstEntry, ok := entries[0].(map[string]any)
	if !ok || firstEntry["path"] != expectedWsPath {
		t.Errorf("expected path %q in plugins.json, got: %v", expectedWsPath, firstEntry["path"])
	}

	// 7. Verify .git/info/exclude contains plugins.json
	gitignorePath := filepath.Join(tempRepo, ".git", "info", "exclude")
	gitignoreData, err := os.ReadFile(gitignorePath)
	if err != nil {
		t.Fatalf("expected %s to exist: %v", gitignorePath, err)
	}
	if !strings.Contains(string(gitignoreData), "plugins.json") {
		t.Errorf("expected %s to contain plugins.json, got: %s", gitignorePath, string(gitignoreData))
	}

	// 8. Run unenroll
	stdout.Reset()
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&stdout)
	if err := RunUnenroll(cmdUnenroll, tempRepo, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}

	// 9. Verify .agents/plugins.json entry removed
	if config.IsEnrolledFast(tempRepo) {
		t.Errorf("expected IsEnrolledFast to return false after unenroll")
	}
	if data, err := os.ReadFile(pluginsPath); err == nil {
		// If file still exists, verify entry was removed
		var remainingConfig map[string]any
		if err := json.Unmarshal(data, &remainingConfig); err == nil {
			if remainingEntries, ok := remainingConfig["entries"].([]any); ok {
				for _, item := range remainingEntries {
					if m, ok := item.(map[string]any); ok {
						if p, ok := m["path"].(string); ok && strings.Contains(p, "repo-memory") {
							t.Errorf("expected repo-memory entry to be removed, found: %v", p)
						}
					}
				}
			}
		}
	}

	// 10. Run install again (idempotent)
	if err := RunInstall(tempPluginDir, fakeBinPath); err != nil {
		t.Fatalf("second RunInstall failed: %v", err)
	}

	// 11. Verify layout unchanged
	for _, p := range requiredPaths {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to still exist after second install: %v", p, err)
		}
	}
}

func TestEnrollUnenrollFlow_PreservesOtherPlugins(t *testing.T) {
	tempPluginDir := filepath.Join(t.TempDir(), "plugins", "repo-memory-daemon")
	tempDataDir := filepath.Join(t.TempDir(), "data")
	tempRepo := filepath.Join(t.TempDir(), "test-repo")
	if err := os.MkdirAll(tempRepo, 0755); err != nil {
		t.Fatalf("creating test repo dir: %v", err)
	}

	runGit(t, tempRepo, "init")
	runGit(t, tempRepo, "config", "user.name", "Test User")
	runGit(t, tempRepo, "config", "user.email", "test@example.com")
	runGit(t, tempRepo, "commit", "--allow-empty", "-m", "initial commit")

	t.Setenv("REPO_MEMORY_PLUGIN_DIR", tempPluginDir)
	t.Setenv("REPO_MEMORY_DATA_DIR", tempDataDir)

	fakeBinPath := filepath.Join(t.TempDir(), "bin", "repo-memory")
	_ = os.MkdirAll(filepath.Dir(fakeBinPath), 0755)
	_ = os.WriteFile(fakeBinPath, []byte("#!/bin/sh\nexit 0\n"), 0755)
	if err := RunInstall(tempPluginDir, fakeBinPath); err != nil {
		t.Fatalf("RunInstall failed: %v", err)
	}

	// Pre-populate .agents/plugins.json with a third-party plugin
	agentsDir := filepath.Join(tempRepo, ".agents")
	_ = os.MkdirAll(agentsDir, 0755)
	otherPlugin := map[string]any{
		"entries": []any{
			map[string]any{"path": "/opt/custom-other-plugin"},
		},
	}
	otherBytes, _ := json.MarshalIndent(otherPlugin, "", "  ")
	if err := os.WriteFile(filepath.Join(agentsDir, "plugins.json"), otherBytes, 0644); err != nil {
		t.Fatalf("writing pre-existing plugins.json: %v", err)
	}

	// Enroll
	var stdout bytes.Buffer
	cmdEnroll := enrollCmd
	cmdEnroll.SetOut(&stdout)
	if err := RunEnroll(cmdEnroll, tempRepo); err != nil {
		t.Fatalf("RunEnroll failed: %v", err)
	}

	// Verify both entries present
	pluginsPath := filepath.Join(agentsDir, "plugins.json")
	data, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("reading plugins.json: %v", err)
	}
	var loaded map[string]any
	_ = json.Unmarshal(data, &loaded)
	entries := loaded["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries in plugins.json, got %d", len(entries))
	}

	// Unenroll
	cmdUnenroll := unenrollCmd
	cmdUnenroll.SetOut(&stdout)
	if err := RunUnenroll(cmdUnenroll, tempRepo, false); err != nil {
		t.Fatalf("RunUnenroll failed: %v", err)
	}

	// Verify other plugin preserved, plugins.json still exists
	dataAfter, err := os.ReadFile(pluginsPath)
	if err != nil {
		t.Fatalf("expected plugins.json to still exist because other plugin was present: %v", err)
	}
	var loadedAfter map[string]any
	_ = json.Unmarshal(dataAfter, &loadedAfter)
	entriesAfter := loadedAfter["entries"].([]any)
	if len(entriesAfter) != 1 {
		t.Fatalf("expected exactly 1 entry preserved, got %d", len(entriesAfter))
	}
	entryMap := entriesAfter[0].(map[string]any)
	if entryMap["path"] != "/opt/custom-other-plugin" {
		t.Errorf("expected /opt/custom-other-plugin preserved, got %v", entryMap["path"])
	}
}

func TestMigrationPath(t *testing.T) {
	// Set up layout where old repo-memory directory exists alongside new install target
	parentPluginsDir := t.TempDir()
	legacyPluginDir := filepath.Join(parentPluginsDir, "repo-memory")
	if err := os.MkdirAll(legacyPluginDir, 0755); err != nil {
		t.Fatalf("creating legacy dir: %v", err)
	}
	legacyFile := filepath.Join(legacyPluginDir, "legacy_config.json")
	if err := os.WriteFile(legacyFile, []byte(`{"version": 1}`), 0644); err != nil {
		t.Fatalf("writing legacy file: %v", err)
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	daemonPluginDir := filepath.Join(parentPluginsDir, "repo-memory-daemon")
	fakeBinPath := filepath.Join(t.TempDir(), "bin", "repo-memory")
	_ = os.MkdirAll(filepath.Dir(fakeBinPath), 0755)
	_ = os.WriteFile(fakeBinPath, []byte("#!/bin/sh\nexit 0\n"), 0755)

	// Run install targeting repo-memory-daemon
	if err := RunInstall(daemonPluginDir, fakeBinPath); err != nil {
		t.Fatalf("RunInstall failed: %v", err)
	}

	// Legacy directory should be removed/renamed
	if _, err := os.Stat(legacyPluginDir); !os.IsNotExist(err) {
		t.Errorf("expected legacy plugin directory %s to no longer exist", legacyPluginDir)
	}

	// Backup directory should exist
	expectedBackupName := fmt.Sprintf("repo-memory.bak-%s", time.Now().Format("20060102"))
	backupDir := filepath.Join(tempHome, ".gemini", "antigravity", "sidecar_data", "repo-memory-daemon", "backups", expectedBackupName)
	if _, err := os.Stat(backupDir); err != nil {
		t.Fatalf("expected backup directory %s to exist: %v", backupDir, err)
	}

	// Backup should contain the old legacy contents
	backedUpLegacyFile := filepath.Join(backupDir, "legacy_config.json")
	data, err := os.ReadFile(backedUpLegacyFile)
	if err != nil || string(data) != `{"version": 1}` {
		t.Errorf("expected legacy content preserved in backup: %v", err)
	}

	// New daemon plugin directory should be cleanly installed
	if _, err := os.Stat(filepath.Join(daemonPluginDir, "plugin.json")); err != nil {
		t.Errorf("daemon plugin.json not found: %v", err)
	}
	if _, err := os.Stat(filepath.Join(daemonPluginDir, "workspace-plugin", "repo-memory", "plugin.json")); err != nil {
		t.Errorf("workspace plugin.json not found: %v", err)
	}
}
