package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/packaging/antigravity"
	"github.com/spf13/cobra"
)

var installHarness string

// ResolveAntigravityPluginDir resolves the target plugin directory for the Antigravity daemon:
// 1. REPO_MEMORY_PLUGIN_DIR or ANTIGRAVITY_PLUGIN_DIR environment variables (for testing/custom roots)
// 2. ANTIGRAVITY_CONFIG_DIR/plugins/repo-memory-daemon
// 3. User home directory ~/.gemini/config/plugins/repo-memory-daemon
func ResolveAntigravityPluginDir() (string, error) {
	if custom := os.Getenv("REPO_MEMORY_PLUGIN_DIR"); custom != "" {
		return custom, nil
	}
	if custom := os.Getenv("ANTIGRAVITY_PLUGIN_DIR"); custom != "" {
		return custom, nil
	}
	if configDir := os.Getenv("ANTIGRAVITY_CONFIG_DIR"); configDir != "" {
		return filepath.Join(configDir, "plugins", "repo-memory-daemon"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving user home directory: %w", err)
	}
	return filepath.Join(home, ".gemini", "config", "plugins", "repo-memory-daemon"), nil
}

// ResolveWorkspacePluginDir resolves the nested workspace-plugin directory inside the daemon directory.
func ResolveWorkspacePluginDir(daemonDir string) string {
	return filepath.Join(daemonDir, "workspace-plugin", "repo-memory")
}

// ValidateHooksJSON parses hooks.json data and verifies it has at least one valid hook entry.
func ValidateHooksJSON(data []byte) error {
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(parsed) == 0 {
		return errors.New("hooks configuration is empty ({})")
	}

	totalHooks := 0
	for _, v := range parsed {
		var hookGroup map[string]json.RawMessage
		if err := json.Unmarshal(v, &hookGroup); err == nil {
			for k, g := range hookGroup {
				if k == "enabled" {
					continue
				}
				var arr []json.RawMessage
				if err := json.Unmarshal(g, &arr); err == nil && len(arr) > 0 {
					totalHooks += len(arr)
				}
			}
		} else {
			var arr []json.RawMessage
			if err := json.Unmarshal(v, &arr); err == nil && len(arr) > 0 {
				totalHooks += len(arr)
			}
		}
	}

	if totalHooks == 0 {
		return errors.New("hooks configuration contains no hook entries")
	}
	return nil
}

// RunInstall writes the plugin configuration, hooks, MCP server, and sidecar definitions.
// It is idempotent and can be safely re-run to upgrade configurations in place.
func RunInstall(pluginDir, binPath string) error {
	if pluginDir == "" {
		return errors.New("plugin directory cannot be empty")
	}
	if binPath == "" {
		return errors.New("binary path cannot be empty")
	}

	// 0. Migration: check if legacy repo-memory directory exists alongside repo-memory-daemon
	parentDir := filepath.Dir(pluginDir)
	legacyDir := filepath.Join(parentDir, "repo-memory")
	if legacyDir != pluginDir {
		if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
			fmt.Println("Migrating from legacy plugin layout (repo-memory/ → repo-memory-daemon/)")
			backupName := fmt.Sprintf("repo-memory.bak-%s", time.Now().Format("20060102"))
			backupDir := filepath.Join(parentDir, backupName)
			if _, err := os.Stat(backupDir); err == nil {
				for i := 1; ; i++ {
					candidate := filepath.Join(parentDir, fmt.Sprintf("%s-%d", backupName, i))
					if _, err := os.Stat(candidate); os.IsNotExist(err) {
						backupDir = candidate
						break
					}
				}
			}
			if err := os.Rename(legacyDir, backupDir); err != nil {
				return fmt.Errorf("backing up legacy plugin directory: %w", err)
			}
			fmt.Printf("Legacy plugin backed up to %s. Remove after verifying the new layout.\n", backupDir)
		}
	}

	wsDir := ResolveWorkspacePluginDir(pluginDir)

	// 1. Create necessary directory hierarchy
	subdirs := []string{
		pluginDir,
		filepath.Join(pluginDir, "sidecars", "tick"),
		wsDir,
	}
	for _, dir := range subdirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// 2. Generate JSON file contents using templates with absolute binary path
	hooksContent := antigravity.GenerateHooksJSON(binPath)
	if err := ValidateHooksJSON([]byte(hooksContent)); err != nil {
		return fmt.Errorf("validating generated hooks template: %w", err)
	}

	daemonPluginContent := antigravity.GenerateDaemonPluginJSON()
	configContent := antigravity.GenerateConfigJSON()
	sidecarContent := antigravity.GenerateSidecarJSON(binPath)

	workspacePluginContent := antigravity.GenerateWorkspacePluginJSON()
	mcpContent := antigravity.GenerateMCPConfigJSON(binPath)

	files := map[string]string{
		// Daemon files
		filepath.Join(pluginDir, "plugin.json"):                      daemonPluginContent,
		filepath.Join(pluginDir, "config.json"):                      configContent,
		filepath.Join(pluginDir, "sidecars", "tick", "sidecar.json"): sidecarContent,
		// Workspace files
		filepath.Join(wsDir, "plugin.json"):                          workspacePluginContent,
		filepath.Join(wsDir, "mcp_config.json"):                      mcpContent,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing file %s: %w", path, err)
		}
	}

	// 3. Write and verify hooks.json with safety validation
	hooksPath := filepath.Join(wsDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(hooksContent), 0644); err != nil {
		return fmt.Errorf("writing file %s: %w", hooksPath, err)
	}

	// Safety check: read back, parse as JSON, verify at least one hook entry
	readBack, err := os.ReadFile(hooksPath)
	if err != nil {
		_ = os.Remove(hooksPath)
		return fmt.Errorf("reading back %s for verification: %w", hooksPath, err)
	}
	if err := ValidateHooksJSON(readBack); err != nil {
		_ = os.Remove(hooksPath)
		return fmt.Errorf("safety check failed for %s: %w", hooksPath, err)
	}

	return nil
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install repo-memory plugin and harness configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		if installHarness != "antigravity" {
			return fmt.Errorf("unsupported harness %q (only 'antigravity' is supported in Release 1)", installHarness)
		}

		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("determining executable path: %w", err)
		}
		absBinPath, err := filepath.Abs(execPath)
		if err != nil {
			return fmt.Errorf("resolving absolute executable path: %w", err)
		}
		if evalPath, err := filepath.EvalSymlinks(absBinPath); err == nil {
			absBinPath = evalPath
		}

		pluginDir, err := ResolveAntigravityPluginDir()
		if err != nil {
			return err
		}

		if err := RunInstall(pluginDir, absBinPath); err != nil {
			return err
		}

		wsDir := ResolveWorkspacePluginDir(pluginDir)
		fmt.Printf("Successfully installed repo-memory daemon plugin to %s\n", pluginDir)
		fmt.Printf("Workspace plugin bundle at %s\n", wsDir)
		fmt.Printf("Binary configured at: %s\n\n", absBinPath)
		fmt.Println("Repositories must be enrolled with `repo-memory enroll` to activate hooks and memory.")
		fmt.Println("IMPORTANT: The background tick sidecar is NOT automatically enabled.")
		fmt.Println("To enable the scheduled background worker, open the Antigravity Automations dashboard:")
		fmt.Println("  [Open Automations Dashboard](sidecar://dashboard)")
		fmt.Println("and enable \"Repo Memory Tick\" (repo-memory-daemon/tick).")

		return nil
	},
}

func init() {
	installCmd.Flags().StringVar(&installHarness, "harness", "antigravity", "Target agent harness (antigravity)")
}
