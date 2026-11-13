/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_doctor.go
 * Description: CLI subcommand and diagnostic runner for verifying repository
 *              memory enrollment, staged .agents/plugins/repo-memory workspace
 *              plugin health, GEMINI.md managed block integrity, and graduated
 *              skills health, with optional --fix auto-repair across enrolled repos.
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
	"sort"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/cblabs-org/repo-memory-plugin/packaging/antigravity"
	"github.com/spf13/cobra"
)

var (
	doctorRepoDir string // Optional --repo directory override for `repo-memory doctor`.
	doctorFixFlag bool   // When true (--fix), repairs missing/symlinked/stale staged plugins across enrolled repos.
)

// doctorCmd defines the `repo-memory doctor` subcommand.
var doctorCmd = &cobra.Command{
	Use:   "doctor [path]",
	Short: "Verify repository memory installation and health",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if strings.TrimSpace(doctorRepoDir) != "" {
			targetDir = doctorRepoDir
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetDir = args[0]
		}
		return RunDoctor(cmd, targetDir)
	},
}

func init() {
	doctorCmd.Flags().StringVar(&doctorRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	doctorCmd.Flags().BoolVar(&doctorFixFlag, "fix", false, "Automatically repair missing, symlinked, or stale staged workspace plugins across enrolled repositories")
}

// RunDoctor verifies repository memory installation and health.
func RunDoctor(cmd *cobra.Command, targetDir string) error {
	// --- Step 1: Resolve repository root and installed daemon version ---
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	daemonDir, daemonErr := ResolveAntigravityPluginDir()
	installedDaemonVer := ""
	if daemonErr == nil {
		installedDaemonVer = ReadInstalledPluginVersion(daemonDir)
	}
	expectedVer := antigravity.NormalizePluginVersion(Version)

	// --- Step 2: Check installed daemon plugin bundle version against binary version ---
	if daemonErr == nil && installedDaemonVer != "" && Version != "" && Version != "dev" && installedDaemonVer != expectedVer {
		if doctorFixFlag {
			if execPath, execErr := os.Executable(); execErr == nil && !isTestExecutable(execPath) {
				absBin := execPath
				if a, aErr := filepath.Abs(execPath); aErr == nil {
					absBin = a
				}
				if e, eErr := filepath.EvalSymlinks(absBin); eErr == nil {
					absBin = e
				}
				if installErr := RunInstall(daemonDir, absBin); installErr == nil {
					installedDaemonVer = ReadInstalledPluginVersion(daemonDir)
					fmt.Fprintf(cmd.OutOrStdout(), "✓ Updated daemon plugin bundle to v%s\n", installedDaemonVer)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "⚠ Failed to update daemon plugin bundle: %v\n", installErr)
				}
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠ Plugin bundle version (%s) does not match binary version (%s).\n", installedDaemonVer, expectedVer)
				fmt.Fprintln(cmd.OutOrStdout(), "  Run: repo-memory install --harness antigravity")
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠ Plugin bundle version (%s) does not match binary version (%s).\n", installedDaemonVer, expectedVer)
			fmt.Fprintln(cmd.OutOrStdout(), "  Run: repo-memory install --harness antigravity")
		}
	}

	// --- Step 3: Check .memory enrollment markers and staged workspace plugin for target repo ---
	memDir := filepath.Join(repoRoot, ".memory")
	enrolledMarker := filepath.Join(memDir, ".enrolled")

	memInfo, errMem := os.Stat(memDir)
	hasMemoryDir := errMem == nil && memInfo.IsDir()
	_, errEnrolled := os.Stat(enrolledMarker)
	hasEnrolledMarker := errEnrolled == nil

	if hasMemoryDir && hasEnrolledMarker {
		repoCfg, _ := config.LoadRepoConfig(repoRoot)
		if repoCfg.IsEnabled() {
			issues := inspectRepoWorkspacePlugin(repoRoot, daemonDir, expectedVer)
			if len(issues) > 0 {
				if doctorFixFlag {
					if fixErr := EnsureWorkspacePluginConfigured(repoRoot); fixErr != nil {
						return fmt.Errorf("re-staging workspace plugin for %s: %w", repoRoot, fixErr)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "✓ Re-staged workspace plugin for %s\n", repoRoot)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "⚠ repo-memory is enrolled but workspace plugin needs attention (%s).\n", strings.Join(issues, "; "))
					fmt.Fprintln(cmd.OutOrStdout(), "  Run: repo-memory doctor --fix  (or: repo-memory enroll .)")
				}
			}
		}
	}

	// --- Step 4: Inspect and optionally repair other enrolled repos in registry.json ---
	dataDir := config.DataDir()
	excludePath := ""
	if hasMemoryDir && hasEnrolledMarker {
		excludePath = repoRoot
	}
	for _, candRoot := range collectEnrolledRegistryRepos(dataDir, excludePath) {
		issues := inspectRepoWorkspacePlugin(candRoot, daemonDir, expectedVer)
		if len(issues) == 0 {
			continue
		}
		if doctorFixFlag {
			if fixErr := EnsureWorkspacePluginConfigured(candRoot); fixErr != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "⚠ Failed to re-stage workspace plugin for %s: %v\n", candRoot, fixErr)
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Re-staged workspace plugin for %s\n", candRoot)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "⚠ Enrolled repo %s has stale or un-staged workspace plugin (%s). Run: repo-memory doctor --fix\n", candRoot, strings.Join(issues, "; "))
		}
	}

	// --- Step 5: Validate GEMINI.md managed instruction block ---
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

	// --- Step 6: Run graduated skills lint checks ---
	if hasMemoryDir {
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

// inspectRepoWorkspacePlugin checks a single enrolled repository's .agents/plugins/repo-memory
// directory and .agents/plugins.json for missing files, symlinks, duplicate distiller agents,
// version drift, or legacy plugins.json entries.
func inspectRepoWorkspacePlugin(repoRoot, daemonDir, expectedBinaryVer string) []string {
	var issues []string

	targetVer := ReadInstalledPluginVersion(daemonDir)
	if targetVer == "" && Version != "" && Version != "dev" {
		targetVer = expectedBinaryVer
	}

	stagedDir := StagedWorkspacePluginDir(repoRoot)
	fi, lstatErr := os.Lstat(stagedDir)
	switch {
	case os.IsNotExist(lstatErr):
		issues = append(issues, "staged workspace plugin is missing at .agents/plugins/repo-memory")
	case lstatErr != nil:
		issues = append(issues, fmt.Sprintf("cannot inspect .agents/plugins/repo-memory: %v", lstatErr))
	case fi.Mode()&os.ModeSymlink != 0:
		issues = append(issues, ".agents/plugins/repo-memory is a symlink (ignored by Antigravity 1.5.8+; must be a real directory)")
	case !fi.IsDir():
		issues = append(issues, ".agents/plugins/repo-memory is not a directory")
	default:
		requiredRel := []string{
			"plugin.json",
			"hooks.json",
			"mcp_config.json",
			filepath.Join("agents", "repo-memory-distiller.md"),
		}
		for _, rel := range requiredRel {
			full := filepath.Join(stagedDir, rel)
			lfi, err := os.Lstat(full)
			if os.IsNotExist(err) {
				issues = append(issues, fmt.Sprintf("missing staged file .agents/plugins/repo-memory/%s", filepath.ToSlash(rel)))
				continue
			}
			if err == nil && lfi.Mode()&os.ModeSymlink != 0 {
				issues = append(issues, fmt.Sprintf("symlinked file at .agents/plugins/repo-memory/%s", filepath.ToSlash(rel)))
			}
		}
		if _, err := os.Lstat(filepath.Join(stagedDir, "agents", "repo-memory-distiller")); err == nil {
			issues = append(issues, "duplicate agent directory .agents/plugins/repo-memory/agents/repo-memory-distiller fails strict validation")
		}
		if targetVer != "" {
			stagedVer := ReadInstalledPluginVersion(stagedDir)
			if stagedVer != targetVer {
				issues = append(issues, fmt.Sprintf("staged workspace plugin version (%s) does not match daemon version (%s)", stagedVer, targetVer))
			}
		}
	}

	// Check for lingering legacy repo-memory entries in .agents/plugins.json.
	pluginsPath := filepath.Join(repoRoot, ".agents", "plugins.json")
	if data, err := os.ReadFile(pluginsPath); err == nil {
		var configMap map[string]any
		if err := json.Unmarshal(data, &configMap); err == nil {
			if rawEntries, ok := configMap["entries"].([]any); ok {
				expectedLegacyPath := ""
				if daemonDir != "" {
					if abs, absErr := filepath.Abs(daemonDir); absErr == nil {
						expectedLegacyPath = filepath.Join(abs, "workspace-plugin")
					}
				}
				for _, item := range rawEntries {
					if m, ok := item.(map[string]any); ok {
						if p, ok := m["path"].(string); ok && isRepoMemoryWorkspacePluginEntry(p, expectedLegacyPath) {
							issues = append(issues, "legacy repo-memory entry found in .agents/plugins.json (deprecated in Antigravity 1.5.8+)")
							break
						}
					}
				}
			}
		}
	}

	return issues
}

// collectEnrolledRegistryRepos returns sorted, deduplicated repository roots from registry.json
// that exist on disk, are currently enrolled, and remain enabled in .memory/config.yaml.
func collectEnrolledRegistryRepos(dataDir, excludeRepoRoot string) []string {
	reg, err := config.LoadRegistry(dataDir)
	if err != nil || len(reg) == 0 {
		return nil
	}

	excludeCanon := ""
	if strings.TrimSpace(excludeRepoRoot) != "" {
		excludeCanon = canonicalRepoPath(excludeRepoRoot)
	}

	seen := make(map[string]bool)
	var roots []string
	for wsPath, entry := range reg {
		cand := strings.TrimSpace(entry.RootPath)
		if cand == "" {
			cand = strings.TrimSpace(wsPath)
		}
		if cand == "" {
			continue
		}
		canon := canonicalRepoPath(cand)
		if canon == "" || seen[canon] || (excludeCanon != "" && canon == excludeCanon) {
			continue
		}
		seen[canon] = true

		fi, statErr := os.Stat(cand)
		if statErr != nil || !fi.IsDir() {
			continue
		}
		if !config.IsEnrolledFast(cand) {
			continue
		}
		if cfg, _ := config.LoadRepoConfig(cand); !cfg.IsEnabled() {
			continue
		}
		roots = append(roots, cand)
	}
	sort.Strings(roots)
	return roots
}

// canonicalRepoPath normalizes p and resolves any macOS/OS symlinks (e.g. /var -> /private/var).
func canonicalRepoPath(p string) string {
	clean := filepath.Clean(strings.TrimSpace(p))
	if clean == "" || clean == "." {
		return ""
	}
	if eval, err := filepath.EvalSymlinks(clean); err == nil && eval != "" {
		return eval
	}
	return clean
}
