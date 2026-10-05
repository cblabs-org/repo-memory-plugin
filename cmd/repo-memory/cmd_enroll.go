package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

var enrollCmd = &cobra.Command{
	Use:     "enroll [path]",
	Short:   "Enroll a repository for memory tracking",
	Aliases: []string{"init"},
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}
		return RunEnroll(cmd, targetDir)
	},
}

var initCmd = enrollCmd

func RunEnroll(cmd *cobra.Command, targetDir string) error {
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	memDir := filepath.Join(repoRoot, ".memory")
	if fi, err := os.Stat(memDir); err == nil && fi.IsDir() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: .memory/ directory already exists (inherited from template?)\n")
	}

	if err := config.EnsureRepoLayout(repoRoot); err != nil {
		return fmt.Errorf("ensuring repo layout: %w", err)
	}

	if err := config.CreateEnrolledMarker(repoRoot); err != nil {
		return fmt.Errorf("creating enrolled marker: %w", err)
	}

	if err := EnsureWorkspacePluginConfigured(repoRoot); err != nil {
		return fmt.Errorf("configuring workspace plugin: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Workspace plugin enabled for this repository via .agents/plugins.json")

	repoID, err := gitops.GenerateRepoID(repoRoot)
	if err != nil {
		return fmt.Errorf("generating repo ID: %w", err)
	}

	dataDir := config.DataDir()
	if err := config.EnsureDataDirLayout(dataDir, repoID); err != nil {
		return fmt.Errorf("ensuring data dir layout: %w", err)
	}

	worktrees, _ := gitops.GetWorktreePaths(repoRoot)
	workspacePaths := append([]string{repoRoot}, worktrees...)
	if err := config.AddToRegistry(dataDir, workspacePaths, repoID, repoRoot); err != nil {
		return fmt.Errorf("adding to registry: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Enrolled repository %s (repo-id: %s)\n", repoRoot, repoID)
	return nil
}

func RunInit(cmd *cobra.Command, targetDir string) error {
	return RunEnroll(cmd, targetDir)
}

func EnsureWorkspacePluginConfigured(repoRoot string) error {
	daemonDir, err := ResolveAntigravityPluginDir()
	if err != nil {
		return fmt.Errorf("resolving daemon plugin directory: %w", err)
	}
	daemonDir, err = filepath.Abs(daemonDir)
	if err != nil {
		return fmt.Errorf("resolving absolute daemon plugin directory: %w", err)
	}
	wsPluginPath := filepath.Join(daemonDir, "workspace-plugin")

	agentsDir := filepath.Join(repoRoot, ".agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("creating .agents directory: %w", err)
	}

	pluginsPath := filepath.Join(agentsDir, "plugins.json")

	cmdGit := exec.Command("git", "ls-files", "--error-unmatch", pluginsPath)
	cmdGit.Dir = repoRoot
	if err := cmdGit.Run(); err == nil {
		return fmt.Errorf(".agents/plugins.json is tracked in git — it contains machine-specific paths and should not be committed. Remove it from git tracking: git rm --cached .agents/plugins.json")
	}

	var configMap map[string]any
	if data, err := os.ReadFile(pluginsPath); err == nil {
		if err := json.Unmarshal(data, &configMap); err != nil {
			return fmt.Errorf("cannot parse existing .agents/plugins.json: %w — please fix or remove the file manually", err)
		}
	} else if os.IsNotExist(err) {
		configMap = make(map[string]any)
	} else {
		return fmt.Errorf("reading %s: %w", pluginsPath, err)
	}

	var rawEntries []any
	if entriesVal, ok := configMap["entries"]; ok {
		if list, ok := entriesVal.([]any); ok {
			rawEntries = list
		}
	}

	alreadyPresent := false
	for _, item := range rawEntries {
		if m, ok := item.(map[string]any); ok {
			if p, ok := m["path"].(string); ok {
				if isRepoMemoryWorkspacePluginEntry(p, wsPluginPath) {
					alreadyPresent = true
					break
				}
			}
		}
	}

	if !alreadyPresent {
		rawEntries = append(rawEntries, map[string]any{
			"path": wsPluginPath,
		})
		configMap["entries"] = rawEntries

		marshaled, err := json.MarshalIndent(configMap, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling %s: %w", pluginsPath, err)
		}
		if err := os.WriteFile(pluginsPath, append(marshaled, '\n'), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", pluginsPath, err)
		}
	}

	if err := ensurePluginsGitExcluded(repoRoot); err != nil {
		return fmt.Errorf("updating .git/info/exclude: %w", err)
	}

	return nil
}

func RemoveWorkspacePluginConfigured(repoRoot string) error {
	pluginsPath := filepath.Join(repoRoot, ".agents", "plugins.json")
	data, err := os.ReadFile(pluginsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", pluginsPath, err)
	}

	var configMap map[string]any
	if err := json.Unmarshal(data, &configMap); err != nil {
		return fmt.Errorf("parsing %s: %w", pluginsPath, err)
	}

	var expectedPath string
	if daemonDir, err := ResolveAntigravityPluginDir(); err == nil {
		if abs, err := filepath.Abs(daemonDir); err == nil {
			expectedPath = filepath.Join(abs, "workspace-plugin")
		}
	}

	var rawEntries []any
	if entriesVal, ok := configMap["entries"]; ok {
		if list, ok := entriesVal.([]any); ok {
			rawEntries = list
		}
	}

	var remaining []any
	for _, item := range rawEntries {
		if m, ok := item.(map[string]any); ok {
			if p, ok := m["path"].(string); ok {
				if isRepoMemoryWorkspacePluginEntry(p, expectedPath) {
					continue
				}
			}
		}
		remaining = append(remaining, item)
	}

	hasOtherKeys := false
	for k, v := range configMap {
		if k == "entries" {
			continue
		}
		if v != nil {
			if arr, ok := v.([]any); ok && len(arr) > 0 {
				hasOtherKeys = true
			} else if !ok {
				hasOtherKeys = true
			}
		}
	}

	if len(remaining) == 0 && !hasOtherKeys {
		if err := os.Remove(pluginsPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("deleting %s: %w", pluginsPath, err)
		}
		return nil
	}

	configMap["entries"] = remaining
	marshaled, err := json.MarshalIndent(configMap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling %s: %w", pluginsPath, err)
	}
	return os.WriteFile(pluginsPath, append(marshaled, '\n'), 0644)
}

func ensurePluginsGitExcluded(repoRoot string) error {
	excludePath := filepath.Join(repoRoot, ".git", "info", "exclude")
	
	infoDir := filepath.Dir(excludePath)
	if info, err := os.Stat(infoDir); err != nil || !info.IsDir() {
		return nil
	}

	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == ".agents/plugins.json" || trimmed == "/.agents/plugins.json" {
			return nil
		}
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "# repo-memory: machine-specific workspace plugin path\n.agents/plugins.json\n"
	return os.WriteFile(excludePath, []byte(content), 0644)
}

func isRepoMemoryWorkspacePluginEntry(entryPath, expectedPath string) bool {
	if entryPath == "" || expectedPath == "" {
		return false
	}
	if filepath.Clean(entryPath) == filepath.Clean(expectedPath) {
		return true
	}
	if filepath.Clean(entryPath) == filepath.Join(filepath.Clean(expectedPath), "repo-memory") {
		return true
	}
	return false
}
