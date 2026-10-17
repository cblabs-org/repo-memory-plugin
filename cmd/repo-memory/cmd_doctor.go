/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_doctor.go
 * Description: CLI subcommand and diagnostic runner for verifying repository
 *              memory enrollment, workspace plugin linkage, GEMINI.md managed
 *              block integrity, and graduated skills health.
 *
 * Exported Types / Functions:
 * - RunDoctor(): Verifies repository memory installation and health for a target directory.
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/spf13/cobra"
)

// doctorCmd defines the `repo-memory doctor` subcommand.
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
	// --- Step 1: Resolve repository root and check .memory enrollment markers ---
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

	// --- Step 2: Verify .agents/plugins.json workspace plugin registration ---
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

	// --- Step 3: Validate GEMINI.md managed instruction block ---
	geminiPath := filepath.Join(repoRoot, config.InstructionFilename)
	if data, err := os.ReadFile(geminiPath); err == nil {
		content := string(data)
		if err := config.ValidateInstructionBlock(content); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠ GEMINI.md managed block error: %v\n", err)
			fmt.Fprintln(cmd.OutOrStdout(), "  Fix the markers manually in GEMINI.md, then run: repo-memory enroll .")
		} else if strings.Contains(content, "<!-- repo-memory:begin v1") {
			fmt.Fprintln(cmd.OutOrStdout(), "⚠ GEMINI.md contains a v1 managed block; run 'repo-memory enroll .' to upgrade to v2.")
		}
	}

	// --- Step 4: Run graduated skills lint checks ---
	if hasMemoryDir {
		dataDir := config.DataDir()
		repoID := ""
		if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil {
			repoID = entry.RepoID
		}
		if repoID == "" {
			repoID, _ = gitops.GenerateRepoID(repoRoot)
		}
		if skillWarnings, err := skills.CheckGraduatedSkillsLint(cmd.Context(), repoRoot, dataDir, repoID); err == nil {
			for _, w := range skillWarnings {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠ %s\n", w)
			}
		}
	}

	return nil
}
