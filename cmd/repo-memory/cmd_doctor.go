package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor [path]",
	Short: "Verify repository memory installation and health",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}
		return RunDoctor(cmd, targetDir)
	},
}

// RunDoctor verifies repository memory installation and health.
func RunDoctor(cmd *cobra.Command, targetDir string) error {
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	memDir := filepath.Join(repoRoot, ".memory")
	enrolledMarker := filepath.Join(memDir, ".enrolled")

	memInfo, errMem := os.Stat(memDir)
	hasMemoryDir := errMem == nil && memInfo.IsDir()
	_, errEnrolled := os.Stat(enrolledMarker)
	hasEnrolledMarker := errEnrolled == nil

	if hasMemoryDir && hasEnrolledMarker {
		pluginsPath := filepath.Join(repoRoot, ".agents", "plugins.json")
		var pluginPath string
		foundReference := false

		if data, err := os.ReadFile(pluginsPath); err == nil {
			var configMap map[string]any
			if err := json.Unmarshal(data, &configMap); err == nil {
				if rawEntries, ok := configMap["entries"].([]any); ok {
					var expectedPath string
					if daemonDir, err := ResolveAntigravityPluginDir(); err == nil {
						if abs, err := filepath.Abs(daemonDir); err == nil {
							expectedPath = filepath.Join(abs, "workspace-plugin")
						}
					}
					for _, item := range rawEntries {
						if m, ok := item.(map[string]any); ok {
							if p, ok := m["path"].(string); ok {
								if isRepoMemoryWorkspacePluginEntry(p, expectedPath) {
									foundReference = true
									pluginPath = p
									break
								}
							}
						}
					}
				}
			}
		}

		if !foundReference {
			fmt.Fprintln(cmd.OutOrStdout(), "⚠ repo-memory is enrolled but workspace plugin is not configured.")
			fmt.Fprintln(cmd.OutOrStdout(), "  Run: repo-memory enroll .")
		} else {
			if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠ Workspace plugin not found at %s.\n", pluginPath)
				fmt.Fprintln(cmd.OutOrStdout(), "  Run: repo-memory install --harness antigravity")
			}
		}
	}

	return nil
}
