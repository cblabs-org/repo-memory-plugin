package main

import (
	"encoding/json"
	"fmt"
	"os"
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

// initCmd is retained as an alias variable for backwards compatibility.
var initCmd = enrollCmd

const mapIncludeLine = "@[memory map](.memory/views/MAP.md)"

// RunEnroll executes repository enrolment.
func RunEnroll(cmd *cobra.Command, targetDir string) error {
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	// 1. Check if .memory/ directory exists before enroll (template warning)
	memDir := filepath.Join(repoRoot, ".memory")
	if fi, err := os.Stat(memDir); err == nil && fi.IsDir() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: .memory/ directory already exists (inherited from template?)\n")
	}

	// 2. Create .memory/ directory and .memory/.gitignore (ignoring .cache/)
	if err := config.EnsureRepoLayout(repoRoot); err != nil {
		return fmt.Errorf("ensuring repo layout: %w", err)
	}

	// 2b. Create .memory/.enrolled marker file
	if err := config.CreateEnrolledMarker(repoRoot); err != nil {
		return fmt.Errorf("creating enrolled marker: %w", err)
	}

	// 2c. Configure .agents/plugins.json and .agents/.gitignore
	if err := EnsureWorkspacePluginConfigured(repoRoot); err != nil {
		return fmt.Errorf("configuring workspace plugin: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Workspace plugin enabled for this repository via .agents/plugins.json")

	// 3. Append MAP include line to GEMINI.md if missing or create GEMINI.md
	geminiPath := filepath.Join(repoRoot, "GEMINI.md")
	if err := ensureMapIncludeInGemini(geminiPath); err != nil {
		return fmt.Errorf("updating GEMINI.md: %w", err)
	}

	// 4. Generate Repo ID
	repoID, err := gitops.GenerateRepoID(repoRoot)
	if err != nil {
		return fmt.Errorf("generating repo ID: %w", err)
	}

	// 5. Ensure data-dir layout for this repository
	dataDir := config.DataDir()
	if err := config.EnsureDataDirLayout(dataDir, repoID); err != nil {
		return fmt.Errorf("ensuring data dir layout: %w", err)
	}

	// 6. Add to <data-dir>/registry.json
	worktrees, _ := gitops.GetWorktreePaths(repoRoot)
	workspacePaths := append([]string{repoRoot}, worktrees...)
	if err := config.AddToRegistry(dataDir, workspacePaths, repoID, repoRoot); err != nil {
		return fmt.Errorf("adding to registry: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Enrolled repository %s (repo-id: %s)\n", repoRoot, repoID)
	return nil
}

// RunInit executes repository enrolment (alias for RunEnroll).
func RunInit(cmd *cobra.Command, targetDir string) error {
	return RunEnroll(cmd, targetDir)
}

func ensureMapIncludeInGemini(geminiPath string) error {
	data, err := os.ReadFile(geminiPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create GEMINI.md with MAP include line
			return os.WriteFile(geminiPath, []byte(mapIncludeLine+"\n"), 0644)
		}
		return err
	}

	content := string(data)
	if strings.Contains(content, mapIncludeLine) || strings.Contains(content, ".memory/views/MAP.md") {
		// Already included
		return nil
	}

	// Append MAP include line
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += mapIncludeLine + "\n"

	return os.WriteFile(geminiPath, []byte(content), 0644)
}

// EnsureWorkspacePluginConfigured writes or updates .agents/plugins.json and .agents/.gitignore.
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

	var configMap map[string]any
	if data, err := os.ReadFile(pluginsPath); err == nil {
		if err := json.Unmarshal(data, &configMap); err != nil {
			configMap = make(map[string]any)
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

	// Ensure plugins.json is in .agents/.gitignore
	if err := ensurePluginsGitignored(agentsDir); err != nil {
		return fmt.Errorf("updating .agents/.gitignore: %w", err)
	}

	return nil
}

// RemoveWorkspacePluginConfigured removes the repo-memory workspace-plugin entry from .agents/plugins.json.
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

func ensurePluginsGitignored(agentsDir string) error {
	gitignorePath := filepath.Join(agentsDir, ".gitignore")
	data, err := os.ReadFile(gitignorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(gitignorePath, []byte("plugins.json\n"), 0644)
		}
		return err
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "plugins.json" || trimmed == "/plugins.json" {
			return nil
		}
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "plugins.json\n"
	return os.WriteFile(gitignorePath, []byte(content), 0644)
}

func isRepoMemoryWorkspacePluginEntry(entryPath, expectedPath string) bool {
	if entryPath == "" {
		return false
	}
	if expectedPath != "" && filepath.Clean(entryPath) == filepath.Clean(expectedPath) {
		return true
	}
	if expectedPath != "" && filepath.Clean(entryPath) == filepath.Join(filepath.Clean(expectedPath), "repo-memory") {
		return true
	}
	cleaned := filepath.Clean(entryPath)
	if strings.Contains(cleaned, "repo-memory") && strings.Contains(cleaned, "workspace-plugin") {
		return true
	}
	return false
}
