/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_install.go
 * Description: Installs and upgrades the Antigravity two-plugin layout
 *              (global repo-memory-daemon with tick sidecar and repo-memory-admin
 *              skill + nested workspace-plugin/repo-memory with hooks, MCP server,
 *              and repo-memory-distiller agent), and generates portable archive
 *              bundles for drop-in .zip distribution.
 *
 * Exported Types / Functions:
 * - ResolveAntigravityPluginDir(): Resolves the target global daemon plugin directory.
 * - ResolveWorkspacePluginDir():   Resolves the nested workspace-plugin directory.
 * - ReadInstalledPluginVersion():  Reads the semantic version string from an installed daemon plugin.json.
 * - ValidateHooksJSON():           Verifies hooks.json is valid JSON with at least one hook entry.
 * - RunInstall():                  Writes and validates the daemon and workspace plugin bundles for a resolved binary path.
 * - RunPortableInstall():          Writes a self-contained portable plugin bundle using ~ / $HOME runner paths.
 */

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/packaging/antigravity"
	"github.com/spf13/cobra"
)

var (
	installHarness      string // Target agent harness flag (--harness; defaults to "antigravity").
	installPortableFlag bool   // When true (--portable), writes a machine-independent bundle for .zip distribution.
	installTargetOS     string // Target OS for --portable bundle generation (--os; defaults to runtime.GOOS).
)

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

// ReadInstalledPluginVersion reads the "version" field from <pluginDir>/plugin.json.
// It returns an empty string if the manifest does not exist or cannot be parsed.
func ReadInstalledPluginVersion(pluginDir string) string {
	if strings.TrimSpace(pluginDir) == "" {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(pluginDir, "plugin.json"))
	if err != nil {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return ""
	}
	if v, ok := parsed["version"].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// ValidateHooksJSON parses hooks.json data and verifies it has at least one valid hook entry.
func ValidateHooksJSON(data []byte) error {
	// --- Step 1: Unmarshal top-level JSON map and reject empty payloads ---
	var parsed map[string]json.RawMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	if len(parsed) == 0 {
		return errors.New("hooks configuration is empty ({})")
	}

	// --- Step 2: Count configured hook entries across nested and flat structures ---
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

	// --- Step 1: Migrate legacy repo-memory plugin directory and update global config.json ---
	parentDir := filepath.Dir(pluginDir)
	legacyDir := filepath.Join(parentDir, "repo-memory")
	if legacyDir != pluginDir {
		if info, err := os.Stat(legacyDir); err == nil && info.IsDir() {
			fmt.Println("Migrating from legacy plugin layout (repo-memory/ → repo-memory-daemon/)")
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("resolving user home directory for backup: %w", err)
			}
			backupBase := filepath.Join(home, ".gemini", "antigravity", "sidecar_data", "repo-memory-daemon", "backups")
			if err := os.MkdirAll(backupBase, 0755); err != nil {
				return fmt.Errorf("creating backup dir: %w", err)
			}
			backupName := fmt.Sprintf("repo-memory.bak-%s", time.Now().Format("20060102"))
			backupDir := filepath.Join(backupBase, backupName)
			if _, err := os.Stat(backupDir); err == nil {
				for i := 1; ; i++ {
					candidate := filepath.Join(backupBase, fmt.Sprintf("%s-%d", backupName, i))
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

	configDir := filepath.Dir(parentDir)
	configPath := filepath.Join(configDir, "config.json")
	enableDaemonInGlobalConfig(configPath, filepath.Base(parentDir) == "plugins")

	wsDir := ResolveWorkspacePluginDir(pluginDir)

	// --- Step 2: Create necessary directory hierarchy ---
	subdirs := []string{
		pluginDir,
		filepath.Join(pluginDir, "sidecars", "tick"),
		filepath.Join(pluginDir, "skills", "repo-memory-admin"),
		wsDir,
		filepath.Join(wsDir, "agents"),
		filepath.Join(wsDir, "agents", "repo-memory-distiller"),
	}
	for _, dir := range subdirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// Clean up any legacy duplicate skill directory inside workspace-plugin
	_ = os.RemoveAll(filepath.Join(wsDir, "skills"))

	// --- Step 3: Generate JSON and Markdown file contents using templates with absolute binary path ---
	hooksContent := antigravity.GenerateHooksJSON(binPath)
	if err := ValidateHooksJSON([]byte(hooksContent)); err != nil {
		return fmt.Errorf("validating generated hooks template: %w", err)
	}

	daemonPluginContent := antigravity.GenerateDaemonPluginJSONWithVersion(Version)
	configContent := antigravity.GenerateConfigJSON()
	sidecarContent := antigravity.GenerateSidecarJSON(binPath)
	adminSkillContent := antigravity.GenerateAdminSkillMD(binPath)

	workspacePluginContent := antigravity.GenerateWorkspacePluginJSONWithVersion(Version)
	mcpContent := antigravity.GenerateMCPConfigJSON(binPath)
	distillerAgentContent := antigravity.GenerateDistillerAgentMD()

	files := map[string]string{
		// Daemon files
		filepath.Join(pluginDir, "plugin.json"):                             daemonPluginContent,
		filepath.Join(pluginDir, "config.json"):                             configContent,
		filepath.Join(pluginDir, "sidecars", "tick", "sidecar.json"):        sidecarContent,
		filepath.Join(pluginDir, "skills", "repo-memory-admin", "SKILL.md"): adminSkillContent,
		// Workspace files
		filepath.Join(wsDir, "plugin.json"):                                 workspacePluginContent,
		filepath.Join(wsDir, "mcp_config.json"):                             mcpContent,
		filepath.Join(wsDir, "agents", "repo-memory-distiller", "agent.md"): distillerAgentContent,
		filepath.Join(wsDir, "agents", "repo-memory-distiller.md"):          distillerAgentContent,
	}

	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing file %s: %w", path, err)
		}
	}

	// --- Step 4: Write and verify hooks.json with read-back safety validation ---
	hooksPath := filepath.Join(wsDir, "hooks.json")
	if err := os.WriteFile(hooksPath, []byte(hooksContent), 0644); err != nil {
		return fmt.Errorf("writing file %s: %w", hooksPath, err)
	}

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

// RunPortableInstall writes a machine-independent plugin bundle to pluginDir suitable for
// packaging as a drop-in .zip archive. It embeds portable ~ / $HOME runner references and
// writes bin/run-repo-memory.sh (on Unix) without mutating the build host's config.json.
func RunPortableInstall(pluginDir, targetOS, version string) error {
	if strings.TrimSpace(pluginDir) == "" {
		return errors.New("plugin directory cannot be empty")
	}
	if strings.TrimSpace(targetOS) == "" {
		targetOS = runtime.GOOS
	}

	// --- Step 1: Create daemon, bin, sidecar, skill, and workspace-plugin directories ---
	wsDir := ResolveWorkspacePluginDir(pluginDir)
	subdirs := []string{
		pluginDir,
		filepath.Join(pluginDir, "bin"),
		filepath.Join(pluginDir, "sidecars", "tick"),
		filepath.Join(pluginDir, "skills", "repo-memory-admin"),
		wsDir,
		filepath.Join(wsDir, "agents"),
		filepath.Join(wsDir, "agents", "repo-memory-distiller"),
	}
	for _, dir := range subdirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	// --- Step 2: Select portable binary and hook runner paths for the target OS ---
	hookRunnerPath := "~/.gemini/config/plugins/repo-memory-daemon/bin/run-repo-memory.sh"
	skillBinPath := "~/.gemini/config/plugins/repo-memory-daemon/bin/repo-memory"
	if strings.EqualFold(targetOS, "windows") {
		hookRunnerPath = `~/.gemini/config/plugins/repo-memory-daemon/bin/repo-memory.exe`
		skillBinPath = `~/.gemini/config/plugins/repo-memory-daemon/bin/repo-memory.exe`
	} else {
		runnerPath := filepath.Join(pluginDir, "bin", "run-repo-memory.sh")
		if err := os.WriteFile(runnerPath, []byte(antigravity.GeneratePortableRunnerScript()), 0755); err != nil {
			return fmt.Errorf("writing portable runner script %s: %w", runnerPath, err)
		}
	}

	// --- Step 3: Render and write portable JSON and Markdown manifests ---
	hooksContent := antigravity.GenerateHooksJSON(hookRunnerPath)
	if err := ValidateHooksJSON([]byte(hooksContent)); err != nil {
		return fmt.Errorf("validating generated portable hooks template: %w", err)
	}

	adminSkillContent := antigravity.GenerateAdminSkillMD(skillBinPath)
	files := map[string]string{
		filepath.Join(pluginDir, "plugin.json"):                             antigravity.GenerateDaemonPluginJSONWithVersion(version),
		filepath.Join(pluginDir, "config.json"):                             antigravity.GenerateConfigJSON(),
		filepath.Join(pluginDir, "sidecars", "tick", "sidecar.json"):        antigravity.GeneratePortableSidecarJSON(targetOS),
		filepath.Join(pluginDir, "skills", "repo-memory-admin", "SKILL.md"): adminSkillContent,
		filepath.Join(wsDir, "plugin.json"):                                 antigravity.GenerateWorkspacePluginJSONWithVersion(version),
		filepath.Join(wsDir, "mcp_config.json"):                             antigravity.GeneratePortableMCPConfigJSON(targetOS),
		filepath.Join(wsDir, "hooks.json"):                                  hooksContent,
		filepath.Join(wsDir, "agents", "repo-memory-distiller", "agent.md"): antigravity.GenerateDistillerAgentMD(),
		filepath.Join(wsDir, "agents", "repo-memory-distiller.md"):          antigravity.GenerateDistillerAgentMD(),
	}

	for filePath, content := range files {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing file %s: %w", filePath, err)
		}
	}

	return nil
}

// installCmd defines the `repo-memory install` subcommand.
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install repo-memory plugin and harness configurations",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Validate target harness and resolve plugin directory ---
		if installHarness != "antigravity" {
			return fmt.Errorf("unsupported harness %q (only 'antigravity' is supported in Release 1)", installHarness)
		}

		pluginDir, err := ResolveAntigravityPluginDir()
		if err != nil {
			return err
		}

		// --- Step 2: Handle portable archive bundle generation mode (--portable) ---
		if installPortableFlag {
			if err := RunPortableInstall(pluginDir, installTargetOS, Version); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Generated portable repo-memory plugin bundle (%s, version %s) at %s\n",
				installTargetOS, antigravity.NormalizePluginVersion(Version), pluginDir)
			return nil
		}

		// --- Step 3: Resolve executable path and run host installation ---
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

		if err := RunInstall(pluginDir, absBinPath); err != nil {
			return err
		}

		// --- Step 4: Print post-install onboarding instructions ---
		wsDir := ResolveWorkspacePluginDir(pluginDir)
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Successfully installed repo-memory daemon plugin (v%s) to %s\n",
			antigravity.NormalizePluginVersion(Version), pluginDir)
		fmt.Fprintf(out, "Workspace plugin bundle at %s\n", wsDir)
		fmt.Fprintf(out, "Binary configured at: %s\n\n", absBinPath)
		fmt.Fprintln(out, "To enroll a repository from chat, use `/repo-memory-admin enroll this repo` (or run `repo-memory enroll .`).")
		fmt.Fprintln(out, "IMPORTANT: The background tick sidecar is NOT automatically enabled.")
		fmt.Fprintln(out, "To enable the scheduled background worker, open the Antigravity Automations dashboard:")
		fmt.Fprintln(out, "  [Open Automations Dashboard](sidecar://dashboard)")
		fmt.Fprintln(out, "and enable \"Repo Memory Tick\" (repo-memory-daemon/tick).")

		return nil
	},
}

// enableDaemonInGlobalConfig ensures `plugins["repo-memory-daemon"] = {"enabled": true}`
// is recorded in <configDir>/config.json (creating config.json or the "plugins" map if
// createIfMissing is true) and notifies a running Antigravity Language Server via
// JetboxWriteState when ANTIGRAVITY_LS_ADDRESS and ANTIGRAVITY_CSRF_TOKEN are present.
func enableDaemonInGlobalConfig(configPath string, createIfMissing bool) {
	// --- Step 1: Read existing config.json or initialize a new map when installing under plugins/ ---
	var config map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if !os.IsNotExist(err) || !createIfMissing {
			return
		}
		config = make(map[string]interface{})
	} else {
		if err := json.Unmarshal(data, &config); err != nil || config == nil {
			return
		}
	}

	// --- Step 2: Ensure "plugins" map exists and enable repo-memory-daemon ---
	plugins, ok := config["plugins"].(map[string]interface{})
	if !ok || plugins == nil {
		if !createIfMissing {
			return
		}
		plugins = make(map[string]interface{})
	}
	delete(plugins, "repo-memory")
	plugins["repo-memory-daemon"] = map[string]interface{}{"enabled": true}
	config["plugins"] = plugins

	out, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return
	}
	out = append(out, '\n')

	if err := atomicWriteConfig(configPath, data, out); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update config.json: %v\n", err)
		return
	}

	// --- Step 3: Notify running Language Server via JetboxWriteState if inside an active session ---
	lsAddr := strings.TrimSpace(os.Getenv("ANTIGRAVITY_LS_ADDRESS"))
	if lsAddr == "" {
		lsAddr = "localhost:5387"
	}
	csrfToken := strings.TrimSpace(os.Getenv("ANTIGRAVITY_CSRF_TOKEN"))
	if csrfToken != "" {
		payload, err := json.Marshal(map[string]any{
			"userConfig": map[string]any{
				"plugins": map[string]any{
					"repo-memory-daemon": map[string]any{"enabled": true},
				},
			},
		})
		if err == nil {
			req, reqErr := http.NewRequest(
				http.MethodPost,
				fmt.Sprintf("http://%s/exa.language_server_pb.LanguageServerService/JetboxWriteState", lsAddr),
				bytes.NewReader(payload),
			)
			if reqErr == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-codeium-csrf-token", csrfToken)
				client := &http.Client{Timeout: 2 * time.Second}
				if resp, doErr := client.Do(req); doErr == nil && resp != nil {
					_ = resp.Body.Close()
				}
			}
		}
	}
}

// atomicWriteConfig writes config data atomically: back up the original (when present),
// write to a temp file in the same directory, then rename into place.
// This prevents corruption from crashes or concurrent IDE writes.
func atomicWriteConfig(configPath string, originalData, newData []byte) error {
	// --- Step 1: Ensure parent directory exists and back up any original config.json ---
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	backupPath := configPath + ".bak"
	if len(originalData) > 0 {
		if err := os.WriteFile(backupPath, originalData, 0644); err != nil {
			return fmt.Errorf("backing up config.json: %w", err)
		}
	}

	// --- Step 2: Write updated payload to a temporary file in the same directory ---
	tmp, err := os.CreateTemp(dir, "config-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(newData); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	// --- Step 3: Atomically rename temporary file over config.json and remove backup ---
	if err := os.Rename(tmpPath, configPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp to config.json: %w", err)
	}

	_ = os.Remove(backupPath)
	return nil
}

func init() {
	installCmd.Flags().StringVar(&installHarness, "harness", "antigravity", "Target agent harness (antigravity)")
	installCmd.Flags().BoolVar(&installPortableFlag, "portable", false, "Generate a portable plugin bundle using ~ / $HOME runner paths for .zip distribution")
	installCmd.Flags().StringVar(&installTargetOS, "os", runtime.GOOS, "Target OS when generating a --portable bundle (darwin, linux, windows)")
}
