/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_enroll.go
 * Description: Enrolls a Git repository for memory tracking, configures the
 *              workspace plugin in .agents/plugins.json, switches any enrolled
 *              Principal Architect workspace to external memory mode, and performs
 *              automatic Day-1 seeding of PA history and curated documentation.
 *
 * Exported Types / Functions:
 * - RunEnroll():                      Executes full repository enrollment and Day-1 seeding.
 * - RunInit():                        Alias for RunEnroll.
 * - EnsureWorkspacePluginConfigured(): Registers workspace-plugin in .agents/plugins.json.
 * - RemoveWorkspacePluginConfigured(): Removes workspace-plugin from .agents/plugins.json.
 * - SyncPAMemoryMode():               Updates memory_mode in Principal Architect .state/state.db.
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

	// --- Step 2: Configure .agents/plugins.json and global registry ---
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

// EnsureWorkspacePluginConfigured registers the repo-memory workspace plugin path inside
// <repoRoot>/.agents/plugins.json and adds .agents/plugins.json to .git/info/exclude.
func EnsureWorkspacePluginConfigured(repoRoot string) error {
	// --- Step 1: Resolve daemon workspace-plugin path and .agents/plugins.json ---
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

	// --- Step 2: Merge workspace-plugin entry into .agents/plugins.json ---
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

	// --- Step 3: Ensure .agents/plugins.json is git-excluded ---
	if err := ensurePluginsGitExcluded(repoRoot); err != nil {
		return fmt.Errorf("updating .git/info/exclude: %w", err)
	}

	return nil
}

// RemoveWorkspacePluginConfigured removes the repo-memory workspace-plugin entry from
// <repoRoot>/.agents/plugins.json, deleting the file if no other entries remain.
func RemoveWorkspacePluginConfigured(repoRoot string) error {
	// --- Step 1: Read and parse .agents/plugins.json ---
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

	// --- Step 2: Filter out repo-memory workspace-plugin entries ---
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

	// --- Step 3: Remove empty plugins.json or write remaining entries ---
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

// ensurePluginsGitExcluded appends .agents/plugins.json to <repoRoot>/.git/info/exclude if absent.
func ensurePluginsGitExcluded(repoRoot string) error {
	// --- Step 1: Verify .git/info directory exists and read current exclude file ---
	excludePath := filepath.Join(repoRoot, ".git", "info", "exclude")

	infoDir := filepath.Dir(excludePath)
	if info, err := os.Stat(infoDir); err != nil || !info.IsDir() {
		return nil
	}

	data, err := os.ReadFile(excludePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// --- Step 2: Check for existing entry and append if missing ---
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

// isRepoMemoryWorkspacePluginEntry reports whether entryPath matches the repo-memory workspace plugin.
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
	}
	needsInstall := false
	for _, p := range requiredFiles {
		if _, err := os.Stat(p); err != nil {
			needsInstall = true
			break
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
		return nil
	}
	return RunInstall(daemonDir, binPath)
}
