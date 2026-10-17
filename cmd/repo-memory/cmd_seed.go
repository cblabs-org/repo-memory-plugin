/*
 * repo-memory-plugin (c) 2026 CBLabs
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_seed.go
 * Description: Implements the `repo-memory seed` command and helper to seed memory
 *              ops from Principal Architect WORKORDER.md and PROJECT.md reports across
 *              both .agents/plugins/principal-architect-workspace and
 *              .agents/skills/principal-architect-workspace.
 *
 * Exported Types / Functions:
 * - ResolvePAWorkspaceDirs(): Discovers active PA workspace directories under a repo root.
 * - SeedPAWorkspace():        Scans PA workspace directories, emits seed ops, and returns counts.
 */

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

var (
	seedRepoDir      string // Target repository root path for `repo-memory seed --repo`.
	seedPaDir        string // Explicit PA workspace path for `repo-memory seed --pa-dir`.
	seedOpsDir       string // Target `.memory/ops/` path for `repo-memory seed --ops-dir`.
	seedConfigPath   string // Path to `.memory/config.yaml` for `repo-memory seed --config`.
	seedBeforeCommit string // Optional Git commit SHA/ref for time-cut seeding.
	seedDryRun       bool   // Whether to preview seeded ops without writing to disk.
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed memory ops from Principal Architect completion reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := gitops.FindRepoRoot(seedRepoDir)
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		// Resolve relative paths against repo root or cwd when explicitly passed.
		resolve := func(p string, flagChanged bool) string {
			if filepath.IsAbs(p) {
				return p
			}
			inRepo := filepath.Join(repoRoot, p)
			if !flagChanged {
				return inRepo
			}
			if _, statErr := os.Stat(inRepo); statErr == nil {
				return inRepo
			}
			if _, statErr := os.Stat(p); statErr == nil {
				if abs, absErr := filepath.Abs(p); absErr == nil {
					return abs
				}
				return p
			}
			if _, statErr := os.Stat(filepath.Dir(filepath.Clean(p))); statErr == nil {
				if abs, absErr := filepath.Abs(p); absErr == nil {
					return abs
				}
				return p
			}
			return inRepo
		}

		var paDirs []string
		if cmd.Flags().Changed("pa-dir") {
			paDirs = []string{resolve(seedPaDir, true)}
		} else {
			paDirs = ResolvePAWorkspaceDirs(repoRoot)
			if len(paDirs) == 0 {
				paDirs = []string{resolve(seedPaDir, false)}
			}
		}

		resolvedOpsDir := resolve(seedOpsDir, cmd.Flags().Changed("ops-dir"))
		resolvedConfigPath := resolve(seedConfigPath, cmd.Flags().Changed("config"))

		_, err = SeedPAWorkspace(
			cmd.Context(),
			repoRoot,
			paDirs,
			resolvedOpsDir,
			resolvedConfigPath,
			seedBeforeCommit,
			seedDryRun,
			true,
		)
		return err
	},
}

func init() {
	seedCmd.Flags().StringVar(&seedRepoDir, "repo", ".", "Path to the target repository root")
	seedCmd.Flags().StringVar(&seedPaDir, "pa-dir", ".agents/skills/principal-architect-workspace/", "Path to the PA workspace")
	seedCmd.Flags().StringVar(&seedOpsDir, "ops-dir", ".memory/ops/", "Path to the ops directory")
	seedCmd.Flags().StringVar(&seedConfigPath, "config", ".memory/config.yaml", "Path to config.yaml")
	seedCmd.Flags().StringVar(&seedBeforeCommit, "before", "", "Git commit for time-cut seeding")
	seedCmd.Flags().BoolVar(&seedDryRun, "dry-run", false, "Print what would be seeded without writing")
}

// ResolvePAWorkspaceDirs returns existing Principal Architect workspace directories
// under repoRoot in priority order (.agents/plugins/principal-architect-workspace,
// then .agents/skills/principal-architect-workspace).
func ResolvePAWorkspaceDirs(repoRoot string) []string {
	var dirs []string
	for _, rel := range []string{
		filepath.Join(".agents", "plugins", "principal-architect-workspace"),
		filepath.Join(".agents", "skills", "principal-architect-workspace"),
	} {
		cand := filepath.Join(repoRoot, rel)
		if info, err := os.Stat(cand); err == nil && info.IsDir() {
			dirs = append(dirs, cand)
		}
	}
	return dirs
}

// SeedPAWorkspace scans one or more PA workspace directories, writes deduplicated
// seed ops to opsDir (unless dryRun is true), projects dossiers when new ops are written,
// and returns the number of parsed reports.
func SeedPAWorkspace(
	ctx context.Context,
	repoRoot string,
	paDirs []string,
	opsDir string,
	configPath string,
	beforeCommit string,
	dryRun bool,
	verbose bool,
) (int, error) {
	// --- Step 1: Resolve optional beforeCommit ref ---
	resolvedBefore := beforeCommit
	if resolvedBefore != "" {
		if err := exec.Command("git", "-C", repoRoot, "cat-file", "-e", resolvedBefore+"^{commit}").Run(); err != nil {
			if err := exec.Command("git", "-C", repoRoot, "cat-file", "-e", "baseline^{commit}").Run(); err == nil {
				resolvedBefore = "baseline"
			} else {
				resolvedBefore = "HEAD"
			}
		}
	}

	opts := seed.SeedOptions{
		RepoRoot:     repoRoot,
		BeforeCommit: resolvedBefore,
		OpsDir:       opsDir,
	}

	scopeCfg, err := config.LoadScopeConfig(configPath)
	if err != nil && !os.IsNotExist(err) {
		return 0, fmt.Errorf("loading scope config: %w", err)
	}

	// --- Step 2: Scan and parse reports across all candidate PA directories ---
	var allParsed []*seed.ParsedWorkorder
	for _, paDir := range paDirs {
		parsed, scanErr := seed.ScanAndParse(paDir, opts)
		if scanErr != nil {
			return 0, fmt.Errorf("scanning workorders in %s: %w", paDir, scanErr)
		}
		allParsed = append(allParsed, parsed...)
	}

	var allAnchors []string
	for _, pw := range allParsed {
		for _, c := range pw.Report.Changes {
			allAnchors = append(allAnchors, c.Path)
		}
	}
	moduleScopes := config.DeriveModuleScopes(allAnchors)
	resolver := config.NewScopeResolver(scopeCfg, moduleScopes, nil)

	var opsWriter *ops.Writer
	if !dryRun {
		opsWriter = ops.NewWriter(opsDir)
	}

	if verbose && len(paDirs) > 0 {
		if dryRun {
			fmt.Printf("DRY RUN: Seeding from %s (no files will be written)\n", paDirs[0])
		} else {
			fmt.Printf("Seeding from %s to %s...\n", paDirs[0], opsDir)
		}
	}

	// --- Step 3: Emit deduplicated seed ops and project dossiers ---
	if err := seed.SeedFromParsed(allParsed, opsWriter, resolver, opts); err != nil {
		return len(allParsed), err
	}

	if !dryRun && len(allParsed) > 0 {
		opsReader := ops.NewReader(opsDir)
		proj := dossiers.NewProjector(opsReader)
		if dossiersMap, projErr := proj.Project(ctx); projErr == nil && len(dossiersMap) > 0 {
			memBaseDir := filepath.Dir(opsDir)
			writer := dossiers.NewWriter(memBaseDir)
			_ = writer.WriteAll(dossiersMap)
		}
	}

	return len(allParsed), nil
}
