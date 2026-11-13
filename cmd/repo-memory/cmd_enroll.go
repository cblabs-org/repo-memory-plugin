/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_enroll.go
 * Description: Enrolls a Git repository for memory tracking, stages the
 *              workspace plugin at .agents/plugins/repo-memory (scrubbing any
 *              legacy .agents/plugins.json entries), switches any enrolled
 *              Principal Architect workspace to external memory mode, and performs
 *              automatic Day-1 seeding of PA history and curated documentation.
 *
 * Exported Types / Functions:
 * - StagedWorkspacePluginRel:        Relative repository path for the staged workspace plugin.
 * - StagedWorkspacePluginDir():      Resolves <repoRoot>/.agents/plugins/repo-memory.
 * - StageWorkspacePlugin():          Copies the workspace plugin bundle into .agents/plugins/repo-memory.
 * - RunEnroll():                     Executes full repository enrollment and Day-1 seeding.
 * - RunInit():                       Alias for RunEnroll.
 * - EnsureWorkspacePluginConfigured(): Stages .agents/plugins/repo-memory and configures git excludes.
 * - RemoveWorkspacePluginConfigured(): Removes .agents/plugins/repo-memory and cleans legacy plugins.json.
 * - SyncPAMemoryMode():              Updates memory_mode in Principal Architect .state/state.db.
 */

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/packaging/antigravity"
	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

var (
	enrollNoSeedFlag     bool     // When true, skips all automatic Day-1 PA and doc seeding during enroll.
	enrollNoSeedPAFlag   bool     // When true, skips Day-1 Principal Architect report seeding and episode import.
	enrollNoSeedDocsFlag bool     // When true, skips Day-1 seeding of curated repository docs and skills.
	enrollDocSeedGlobs   []string // Additional doc_seeds globs to persist into .memory/config.yaml on enroll.
	enrollTurnNotesMode  string   // Optional turn_notes mode ("on", "fallback-only", "off") for .memory/config.yaml.
	enrollPAMemoryMode   string   // PA handoff mode ("external" or "keep"; defaults to "external").
	enrollPushGapsFlag   bool     // When true, sets gaps.push = true in .memory/config.yaml.
)

// enrollCmd defines the `repo-memory enroll` subcommand.
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

// initCmd is a retained alias for enrollCmd for backwards compatibility.
var initCmd = enrollCmd

func init() {
	enrollCmd.Flags().BoolVar(&enrollNoSeedFlag, "no-seed", false, "Skip automatic Day-1 seeding of docs and Principal Architect history")
	enrollCmd.Flags().BoolVar(&enrollNoSeedPAFlag, "no-seed-pa", false, "Skip automatic Day-1 seeding of Principal Architect history")
	enrollCmd.Flags().BoolVar(&enrollNoSeedDocsFlag, "no-seed-docs", false, "Skip automatic Day-1 seeding of repository markdown docs and skills")
	enrollCmd.Flags().StringSliceVar(&enrollDocSeedGlobs, "doc-seed", nil, "Additional document glob(s) to record under doc_seeds in .memory/config.yaml and seed on Day 1")
	enrollCmd.Flags().StringVar(&enrollTurnNotesMode, "turn-notes", "", "Configure end-of-turn distillation mode in .memory/config.yaml (on, fallback-only, off)")
	enrollCmd.Flags().StringVar(&enrollPAMemoryMode, "pa-memory-mode", "external", "Principal Architect memory_mode handoff when PA is present (external, keep)")
	enrollCmd.Flags().BoolVar(&enrollPushGapsFlag, "push-gaps", false, "Enable pushing coverage gap hints in context (sets gaps.push: true in .memory/config.yaml)")
}

// RunEnroll executes repository enrollment, workspace plugin registration, Principal Architect
// memory_mode handoff, and automatic Day-1 seeding of PA completion reports and repository docs.
func RunEnroll(cmd *cobra.Command, targetDir string) error {
	// --- Step 1: Resolve repository root and ensure .memory layout ---
	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		repoRoot = targetDir
	}
	repoRoot, err = filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolving repository root: %w", err)
	}

	memDir := filepath.Join(repoRoot, ".memory")
	if fi, statErr := os.Stat(memDir); statErr == nil && fi.IsDir() {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: .memory/ directory already exists (inherited from template?)\n")
	}

	if err := config.EnsureRepoLayout(repoRoot); err != nil {
		return fmt.Errorf("ensuring repo layout: %w", err)
	}

	createdInstructionFile, err := config.EnsureInstructionBlock(repoRoot)
	if err != nil {
		return fmt.Errorf("updating %s: %w", config.InstructionFilename, err)
	}

	if err := config.CreateEnrolledMarker(repoRoot); err != nil {
		return fmt.Errorf("creating enrolled marker: %w", err)
	}

	// --- Step 2: Stage .agents/plugins/repo-memory and update global registry ---
	if daemonDir, err := ResolveAntigravityPluginDir(); err == nil {
		if execPath, execErr := os.Executable(); execErr == nil && !isTestExecutable(execPath) {
			if absBin, absErr := filepath.Abs(execPath); absErr == nil {
				if evalBin, evalErr := filepath.EvalSymlinks(absBin); evalErr == nil {
					absBin = evalBin
				}
				_ = ensurePluginBundleInstalled(daemonDir, absBin)
			}
		}
	}

	if err := EnsureWorkspacePluginConfigured(repoRoot); err != nil {
		return fmt.Errorf("configuring workspace plugin: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Workspace plugin staged for this repository at .agents/plugins/repo-memory")

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
	if createdInstructionFile {
		if err := config.SetCreatedInstructionFile(dataDir, repoRoot, true); err != nil {
			return fmt.Errorf("recording instruction file creation in registry: %w", err)
		}
	}

	// --- Step 3: Apply repo config overrides, hand off PA memory_mode, and run Day-1 seeding ---
	if err := applyEnrollRepoConfigOverrides(cmd, repoRoot); err != nil {
		return fmt.Errorf("applying enroll config overrides: %w", err)
	}

	paMode := strings.ToLower(strings.TrimSpace(enrollPAMemoryMode))
	if paMode == "" || paMode == "auto" {
		paMode = "external"
	}
	if paMode != "keep" && SyncPAMemoryMode(repoRoot, paMode) {
		fmt.Fprintf(cmd.OutOrStdout(), "Switched Principal Architect memory_mode -> %s\n", paMode)
	}

	if !enrollNoSeedFlag {
		bootstrapRepoMemoryOnEnroll(cmd, repoRoot, dataDir, repoID)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Enrolled repository %s (repo-id: %s)\n", repoRoot, repoID)
	return nil
}

// RunInit is an alias for RunEnroll retained for backwards compatibility.
func RunInit(cmd *cobra.Command, targetDir string) error {
	return RunEnroll(cmd, targetDir)
}

// applyEnrollRepoConfigOverrides persists any CLI flags (--turn-notes, --doc-seed, --push-gaps)
// into <repoRoot>/.memory/config.yaml while preserving existing subsystem and spend settings.
func applyEnrollRepoConfigOverrides(cmd *cobra.Command, repoRoot string) error {
	// --- Step 1: Check whether any CLI override flags were supplied ---
	hasTurnNotes := strings.TrimSpace(enrollTurnNotesMode) != ""
	hasDocSeeds := len(enrollDocSeedGlobs) > 0
	hasPushGaps := enrollPushGapsFlag || (cmd != nil && cmd.Flags().Changed("push-gaps"))
	if !hasTurnNotes && !hasDocSeeds && !hasPushGaps {
		return nil
	}

	cfg, err := config.LoadRepoConfig(repoRoot)
	if err != nil && !os.IsNotExist(err) {
		cfg = config.DefaultRepoConfig()
	}

	// --- Step 2: Apply turn-notes, doc-seed, and push-gaps overrides ---
	if hasTurnNotes {
		mode := strings.ToLower(strings.TrimSpace(enrollTurnNotesMode))
		switch mode {
		case "on", "fallback-only", "fallback_only", "off":
			if mode == "fallback_only" {
				mode = "fallback-only"
			}
			cfg.TurnNotes = mode
		default:
			return fmt.Errorf("invalid --turn-notes mode %q (expected on, fallback-only, or off)", enrollTurnNotesMode)
		}
	}

	if hasDocSeeds {
		seen := make(map[string]bool, len(cfg.DocSeeds))
		for _, g := range cfg.DocSeeds {
			seen[strings.TrimSpace(g)] = true
		}
		for _, raw := range enrollDocSeedGlobs {
			for _, part := range strings.Split(raw, ",") {
				g := strings.TrimSpace(part)
				if g != "" && !seen[g] {
					seen[g] = true
					cfg.DocSeeds = append(cfg.DocSeeds, g)
				}
			}
		}
	}

	if hasPushGaps {
		cfg.Gaps.Push = enrollPushGapsFlag
	}

	// --- Step 3: Persist updated repository configuration ---
	return config.SaveRepoConfig(repoRoot, cfg)
}

// SyncPAMemoryMode checks whether a Principal Architect SQLite state database exists
// under .agents/{plugins,skills}/principal-architect-workspace/.state/state.db and
// updates repo_config.memory_mode to mode ("external" on enroll, "builtin" on unenroll).
func SyncPAMemoryMode(repoRoot, mode string) bool {
	// --- Step 1: Scan candidate Principal Architect state.db locations ---
	updated := false
	nowISO := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	for _, rel := range []string{
		filepath.Join(".agents", "plugins", "principal-architect-workspace", ".state", "state.db"),
		filepath.Join(".agents", "skills", "principal-architect-workspace", ".state", "state.db"),
	} {
		dbPath := filepath.Join(repoRoot, rel)
		info, err := os.Stat(dbPath)
		if err != nil || info.IsDir() {
			continue
		}

		// --- Step 2: Open SQLite database and upsert repo_config.memory_mode ---
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			continue
		}
		_, _ = db.Exec("PRAGMA busy_timeout = 5000;")

		var tableName string
		rowErr := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='repo_config'").Scan(&tableName)
		if rowErr == nil && tableName == "repo_config" {
			_, execErr := db.Exec(
				`INSERT INTO repo_config (key, value, updated_at) VALUES ('memory_mode', ?, ?)
				 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
				mode,
				nowISO,
			)
			if execErr != nil {
				_, execErr = db.Exec(
					`INSERT INTO repo_config (key, value) VALUES ('memory_mode', ?)
					 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
					mode,
				)
			}
			if execErr == nil {
				updated = true
			}
		}
		_ = db.Close()
	}
	return updated
}

// bootstrapRepoMemoryOnEnroll performs automatic Day-1 seeding from Principal Architect
// workspaces and curated repository docs when present, projects dossiers, updates GEMINI.md,
// and builds the SQLite FTS5/BM25 index.
func bootstrapRepoMemoryOnEnroll(cmd *cobra.Command, repoRoot, dataDir, repoID string) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	configPath := filepath.Join(repoRoot, ".memory", "config.yaml")

	// --- Step 1: Seed PA completion reports and import PA episodes if a PA workspace exists ---
	if !enrollNoSeedPAFlag {
		paDirs := ResolvePAWorkspaceDirs(repoRoot)
		if len(paDirs) > 0 {
			if paReports, err := SeedPAWorkspace(ctx, repoRoot, paDirs, opsDir, configPath, "", false, false); err == nil && paReports > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Seeded Principal Architect history from %d reports\n", paReports)
			}
			if importedCount, err := ImportPAWorkspace(repoRoot, dataDir, repoID, "", false); err == nil && importedCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Imported %d Principal Architect files as episodes\n", importedCount)
			}
		}
	}

	// --- Step 2: Seed curated repository markdown docs and skills if present ---
	if !enrollNoSeedDocsFlag {
		repoCfg, err := config.LoadRepoConfig(repoRoot)
		if err != nil && !os.IsNotExist(err) {
			repoCfg = config.DefaultRepoConfig()
		}
		if candDocs, globErr := config.ResolveDocSeedGlobs(repoRoot, repoCfg); globErr == nil && len(candDocs) > 0 {
			scopeCfg, _ := config.LoadScopeConfig(configPath)
			opsReader := ops.NewReader(opsDir)
			opsWriter := ops.NewWriter(opsDir)
			docRes, docErr := seed.SeedDocs(ctx, seed.DocSeedOptions{
				RepoRoot: repoRoot,
				OpsDir:   opsDir,
				DocPaths: candDocs,
				RepoCfg:  repoCfg,
				DryRun:   false,
			}, scopeCfg, opsReader, opsWriter)
			if docErr == nil && docRes != nil {
				totalMutated := docRes.EntriesAdded + docRes.EntriesAmend + docRes.EntriesRetire
				if totalMutated > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Seeded %d entries from %d repository docs\n", totalMutated, docRes.FilesSeeded)
				}
			}
		}
	}

	// --- Step 3: Project dossiers, refresh GEMINI.md Atlas, rebuild index.db, and refresh freshness.json ---
	opsReader := ops.NewReader(opsDir)
	proj := dossiers.NewProjector(opsReader)
	if dossiersMap, projErr := proj.Project(ctx); projErr == nil && len(dossiersMap) > 0 {
		writer := dossiers.NewWriter(filepath.Join(repoRoot, ".memory"))
		if writeErr := writer.WriteAll(dossiersMap); writeErr == nil {
			_, _ = config.EnsureInstructionBlock(repoRoot)
		}
	}

	dbPath := index.IndexPath(dataDir, repoID)
	if mkErr := os.MkdirAll(filepath.Dir(dbPath), 0755); mkErr == nil {
		if idx, idxErr := index.NewIndex(dbPath); idxErr == nil {
			_ = idx.RebuildFromOpLog(ctx, opsReader)
			_ = idx.Close()
		}
	}
	tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, nil).RefreshFreshnessState(ctx, false)
}

// StagedWorkspacePluginRel is the repository-relative path where Antigravity 1.5.8+
// auto-discovers workspace-scoped plugins as real (non-symlink) directories.
const StagedWorkspacePluginRel = ".agents/plugins/repo-memory"

// StagedWorkspacePluginDir returns the staged workspace plugin path inside repoRoot.
func StagedWorkspacePluginDir(repoRoot string) string {
	return filepath.Join(repoRoot, ".agents", "plugins", "repo-memory")
}

// StageWorkspacePlugin copies the repo-memory workspace plugin bundle into
// <repoRoot>/.agents/plugins/repo-memory as a real (non-symlink) directory tree.
// It unlinks any pre-existing symlink at the target path, preserves the top-level
// directory inode when already present, dereferences any symlinks inside the source
// bundle, strips packaging artifacts, and falls back to embedded templates when the
// global daemon workspace-plugin directory has not been installed on disk.
func StageWorkspacePlugin(repoRoot string) error {
	// --- Step 1: Resolve installed workspace-plugin source directory if present ---
	var wsSrcDir string
	if daemonDir, err := ResolveAntigravityPluginDir(); err == nil && strings.TrimSpace(daemonDir) != "" {
		if absDaemon, absErr := filepath.Abs(daemonDir); absErr == nil {
			wsSrcDir = ResolveWorkspacePluginDir(absDaemon)
		}
	}

	// --- Step 2: Prepare <repoRoot>/.agents/plugins/repo-memory without following symlinks ---
	stagedDir := StagedWorkspacePluginDir(repoRoot)
	if err := os.MkdirAll(filepath.Dir(stagedDir), 0755); err != nil {
		return fmt.Errorf("creating .agents/plugins directory: %w", err)
	}

	if fi, lstatErr := os.Lstat(stagedDir); lstatErr == nil {
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			if err := os.Remove(stagedDir); err != nil {
				return fmt.Errorf("removing legacy symlink or non-directory at %s: %w", stagedDir, err)
			}
		}
	}

	if err := os.MkdirAll(stagedDir, 0755); err != nil {
		return fmt.Errorf("creating staged plugin directory %s: %w", stagedDir, err)
	}

	if existing, err := os.ReadDir(stagedDir); err == nil {
		for _, entry := range existing {
			if err := os.RemoveAll(filepath.Join(stagedDir, entry.Name())); err != nil {
				return fmt.Errorf("cleaning stale staged entry %s: %w", entry.Name(), err)
			}
		}
	}

	// --- Step 3: Populate staged directory from installed workspace-plugin or embedded templates ---
	srcManifest := ""
	if wsSrcDir != "" {
		srcManifest = filepath.Join(wsSrcDir, "plugin.json")
	}
	if fi, err := os.Stat(srcManifest); err == nil && !fi.IsDir() {
		if err := copyTreeDereferenced(wsSrcDir, "", stagedDir); err != nil {
			return fmt.Errorf("staging workspace plugin from %s: %w", wsSrcDir, err)
		}
		flatAgentPath := filepath.Join(stagedDir, "agents", "repo-memory-distiller.md")
		if _, statErr := os.Stat(flatAgentPath); os.IsNotExist(statErr) {
			if err := os.MkdirAll(filepath.Dir(flatAgentPath), 0755); err != nil {
				return fmt.Errorf("creating staged agents directory: %w", err)
			}
			if err := os.WriteFile(flatAgentPath, []byte(antigravity.GenerateDistillerAgentMD()), 0644); err != nil {
				return fmt.Errorf("writing staged distiller agent: %w", err)
			}
		}
		return nil
	}

	binPath := "repo-memory"
	if execPath, err := os.Executable(); err == nil {
		if absBin, absErr := filepath.Abs(execPath); absErr == nil {
			if evalBin, evalErr := filepath.EvalSymlinks(absBin); evalErr == nil {
				absBin = evalBin
			}
			binPath = absBin
		}
	}

	hooksContent := antigravity.GenerateHooksJSON(binPath)
	if err := ValidateHooksJSON([]byte(hooksContent)); err != nil {
		return fmt.Errorf("validating embedded hooks template: %w", err)
	}

	agentsDir := filepath.Join(stagedDir, "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		return fmt.Errorf("creating staged agents directory: %w", err)
	}

	fallbackFiles := map[string]string{
		filepath.Join(stagedDir, "plugin.json"):              antigravity.GenerateWorkspacePluginJSONWithVersion(Version),
		filepath.Join(stagedDir, "hooks.json"):               hooksContent,
		filepath.Join(stagedDir, "mcp_config.json"):          antigravity.GenerateMCPConfigJSON(binPath),
		filepath.Join(agentsDir, "repo-memory-distiller.md"): antigravity.GenerateDistillerAgentMD(),
	}
	for path, content := range fallbackFiles {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing fallback staged file %s: %w", path, err)
		}
	}
	return nil
}

// copyTreeDereferenced recursively copies regular files and directories from srcRoot/srcRel
// into dstRoot/srcRel as real directories and regular files, dereferencing any symlinks and
// skipping packaging artifacts, legacy skills directories, and duplicate distiller subdirectories.
func copyTreeDereferenced(srcRoot, srcRel, dstRoot string) error {
	curSrc := srcRoot
	if srcRel != "" {
		curSrc = filepath.Join(srcRoot, srcRel)
	}
	entries, err := os.ReadDir(curSrc)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if isIgnoredPluginArtifact(name) {
			continue
		}
		rel := name
		if srcRel != "" {
			rel = filepath.Join(srcRel, name)
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "skills" || relSlash == "agents/repo-memory-distiller" {
			continue
		}
		srcPath := filepath.Join(srcRoot, rel)
		dstPath := filepath.Join(dstRoot, rel)
		info, err := os.Stat(srcPath)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyTreeDereferenced(srcRoot, rel, dstRoot); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if info.Mode().Perm()&0111 != 0 {
			mode = 0755
		}
		if err := os.WriteFile(dstPath, data, mode); err != nil {
			return err
		}
	}
	return nil
}

// EnsureWorkspacePluginConfigured stages the repo-memory workspace plugin inside
// <repoRoot>/.agents/plugins/repo-memory, scrubs any legacy repo-memory entries from
// <repoRoot>/.agents/plugins.json, and adds the staged directory to .git/info/exclude.
func EnsureWorkspacePluginConfigured(repoRoot string) error {
	// --- Step 1: Reject git-tracked plugin manifests containing machine-specific paths ---
	cmdTrackedJSON := exec.Command("git", "ls-files", "--error-unmatch", filepath.Join(".agents", "plugins.json"))
	cmdTrackedJSON.Dir = repoRoot
	if err := cmdTrackedJSON.Run(); err == nil {
		return fmt.Errorf(".agents/plugins.json is tracked in git — it contains machine-specific paths and should not be committed. Remove it from git tracking: git rm --cached .agents/plugins.json")
	}

	cmdTrackedStaged := exec.Command("git", "ls-files", "--error-unmatch", filepath.Join(".agents", "plugins", "repo-memory", "plugin.json"))
	cmdTrackedStaged.Dir = repoRoot
	if err := cmdTrackedStaged.Run(); err == nil {
		return fmt.Errorf(".agents/plugins/repo-memory is tracked in git — staged workspace plugins contain machine-specific paths and must not be committed. Remove it from git tracking: git rm -r --cached .agents/plugins/repo-memory")
	}

	// --- Step 2: Stage real plugin directory and scrub legacy .agents/plugins.json entries ---
	if err := StageWorkspacePlugin(repoRoot); err != nil {
		return err
	}

	if err := scrubRepoMemoryFromPluginsJSON(repoRoot, true); err != nil {
		return err
	}

	// --- Step 3: Ensure .agents/plugins/repo-memory/ and .agents/plugins.json are git-excluded ---
	if err := ensurePluginsGitExcluded(repoRoot); err != nil {
		return fmt.Errorf("updating .git/info/exclude: %w", err)
	}

	return nil
}

// scrubRepoMemoryFromPluginsJSON removes legacy repo-memory workspace-plugin entries from
// <repoRoot>/.agents/plugins.json while preserving sibling entries and unrelated keys.
// It never creates .agents/plugins.json if absent, and deletes the file if no entries or keys remain.
func scrubRepoMemoryFromPluginsJSON(repoRoot string, strictParse bool) error {
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
		if !strictParse {
			return nil
		}
		return fmt.Errorf("cannot parse existing .agents/plugins.json: %w — please fix or remove the file manually", err)
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
	removedAny := false
	for _, item := range rawEntries {
		if m, ok := item.(map[string]any); ok {
			if p, ok := m["path"].(string); ok {
				if isRepoMemoryWorkspacePluginEntry(p, expectedPath) {
					removedAny = true
					continue
				}
			}
		}
		remaining = append(remaining, item)
	}

	if !removedAny {
		return nil
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

// RemoveWorkspacePluginConfigured removes the staged <repoRoot>/.agents/plugins/repo-memory
// directory (or legacy symlink), cleans up empty .agents/plugins / .agents parents, and scrubs
// any legacy repo-memory entries from <repoRoot>/.agents/plugins.json while preserving siblings.
func RemoveWorkspacePluginConfigured(repoRoot string) error {
	// --- Step 1: Remove staged plugin directory or symlink ---
	stagedDir := StagedWorkspacePluginDir(repoRoot)
	if fi, err := os.Lstat(stagedDir); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			if rmErr := os.Remove(stagedDir); rmErr != nil && !os.IsNotExist(rmErr) {
				return fmt.Errorf("removing %s: %w", stagedDir, rmErr)
			}
		} else {
			if rmErr := os.RemoveAll(stagedDir); rmErr != nil {
				return fmt.Errorf("removing %s: %w", stagedDir, rmErr)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspecting %s: %w", stagedDir, err)
	}

	// Best-effort cleanup of empty .agents/plugins and .agents directories without disturbing siblings.
	pluginsParent := filepath.Join(repoRoot, ".agents", "plugins")
	if entries, err := os.ReadDir(pluginsParent); err == nil && len(entries) == 0 {
		_ = os.Remove(pluginsParent)
	}
	agentsDir := filepath.Join(repoRoot, ".agents")
	if entries, err := os.ReadDir(agentsDir); err == nil && len(entries) == 0 {
		_ = os.Remove(agentsDir)
	}

	// --- Step 2: Scrub any legacy repo-memory entries from .agents/plugins.json ---
	return scrubRepoMemoryFromPluginsJSON(repoRoot, true)
}

// ensurePluginsGitExcluded appends /.agents/plugins/repo-memory/ and .agents/plugins.json
// to <repoRoot>/.git/info/exclude (supporting git worktrees) when absent.
func ensurePluginsGitExcluded(repoRoot string) error {
	// --- Step 1: Resolve .git/info/exclude path for standard repos and worktrees ---
	gitDot := filepath.Join(repoRoot, ".git")
	fi, err := os.Stat(gitDot)
	if err != nil {
		return nil
	}

	var excludePath string
	if fi.IsDir() {
		excludePath = filepath.Join(gitDot, "info", "exclude")
	} else {
		out, cmdErr := exec.Command("git", "-C", repoRoot, "rev-parse", "--git-path", "info/exclude").Output()
		if cmdErr != nil {
			return nil
		}
		resolved := strings.TrimSpace(string(out))
		if resolved == "" {
			return nil
		}
		if filepath.IsAbs(resolved) {
			excludePath = resolved
		} else {
			excludePath = filepath.Join(repoRoot, resolved)
		}
	}

	infoDir := filepath.Dir(excludePath)
	if info, statErr := os.Stat(infoDir); statErr != nil || !info.IsDir() {
		return nil
	}

	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// --- Step 2: Check for existing exclude entries and append missing patterns ---
	content := string(data)
	hasStagedExclude := false
	hasLegacyJSONExclude := false
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "/.agents/plugins/repo-memory/", ".agents/plugins/repo-memory/", "/.agents/plugins/repo-memory", ".agents/plugins/repo-memory":
			hasStagedExclude = true
		case ".agents/plugins.json", "/.agents/plugins.json":
			hasLegacyJSONExclude = true
		}
	}

	if hasStagedExclude && hasLegacyJSONExclude {
		return nil
	}

	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if !hasStagedExclude && !hasLegacyJSONExclude {
		content += "# repo-memory: local staged workspace plugin\n"
	}
	if !hasStagedExclude {
		content += "/.agents/plugins/repo-memory/\n"
	}
	if !hasLegacyJSONExclude {
		content += ".agents/plugins.json\n"
	}
	return os.WriteFile(excludePath, []byte(content), 0644)
}

// isRepoMemoryWorkspacePluginEntry reports whether entryPath refers to a legacy or current
// repo-memory workspace plugin path.
func isRepoMemoryWorkspacePluginEntry(entryPath, expectedPath string) bool {
	if strings.TrimSpace(entryPath) == "" {
		return false
	}
	if expectedPath != "" {
		if filepath.Clean(entryPath) == filepath.Clean(expectedPath) {
			return true
		}
		if filepath.Clean(entryPath) == filepath.Join(filepath.Clean(expectedPath), "repo-memory") {
			return true
		}
	}
	norm := strings.TrimRight(filepath.ToSlash(strings.TrimSpace(entryPath)), "/")
	return norm == ".agents/plugins/repo-memory" ||
		strings.HasSuffix(norm, "/repo-memory-daemon/workspace-plugin") ||
		strings.HasSuffix(norm, "/repo-memory-daemon/workspace-plugin/repo-memory") ||
		strings.HasSuffix(norm, "/workspace-plugin/repo-memory") ||
		strings.HasSuffix(norm, "/.agents/plugins/repo-memory")
}

// isTestExecutable reports whether execPath appears to be a temporary `go test` binary.
func isTestExecutable(execPath string) bool {
	cleaned := filepath.Clean(execPath)
	base := filepath.Base(cleaned)
	return strings.HasSuffix(base, ".test") ||
		strings.HasSuffix(base, ".test.exe") ||
		strings.Contains(cleaned, string(filepath.Separator)+"go-build")
}

// ensurePluginBundleInstalled checks whether the daemon and workspace plugin bundles
// (including the repo-memory-admin skill) are present and up to date at daemonDir, and
// invokes RunInstall(daemonDir, binPath) to self-heal if any artifact is missing or outdated.
func ensurePluginBundleInstalled(daemonDir, binPath string) error {
	// --- Step 1: Check for required plugin manifests and skill files ---
	wsDir := ResolveWorkspacePluginDir(daemonDir)
	requiredFiles := []string{
		filepath.Join(daemonDir, "plugin.json"),
		filepath.Join(daemonDir, "skills", "repo-memory-admin", "SKILL.md"),
		filepath.Join(wsDir, "plugin.json"),
		filepath.Join(wsDir, "hooks.json"),
		filepath.Join(wsDir, "mcp_config.json"),
		filepath.Join(wsDir, "agents", "repo-memory-distiller.md"),
	}
	needsInstall := false
	for _, p := range requiredFiles {
		if _, err := os.Stat(p); err != nil {
			needsInstall = true
			break
		}
	}
	if !needsInstall {
		if _, err := os.Stat(filepath.Join(wsDir, "agents", "repo-memory-distiller")); err == nil {
			needsInstall = true
		}
	}

	// --- Step 2: Check for version drift when binary Version is stamped ---
	if !needsInstall && Version != "" && Version != "dev" {
		installedVer := ReadInstalledPluginVersion(daemonDir)
		expectedVer := antigravity.NormalizePluginVersion(Version)
		if installedVer != expectedVer {
			needsInstall = true
		}
	}

	if !needsInstall {
		// Even when all bundle files exist (e.g., extracted from a portable .zip archive),
		// ensure repo-memory-daemon is enabled in the global config.json.
		parentDir := filepath.Dir(daemonDir)
		configDir := filepath.Dir(parentDir)
		configPath := filepath.Join(configDir, "config.json")
		enableDaemonInGlobalConfig(configPath, filepath.Base(parentDir) == "plugins")
		return nil
	}
	return RunInstall(daemonDir, binPath)
}
