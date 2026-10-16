package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

var (
	seedRepoDir      string
	seedPaDir        string
	seedOpsDir       string
	seedConfigPath   string
	seedBeforeCommit string
	seedDryRun       bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed memory ops from Principal Architect completion reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := gitops.FindRepoRoot(seedRepoDir)
		if err != nil {
			return fmt.Errorf("finding repo root: %w", err)
		}

		// Resolve relative paths against repo root or cwd when explicitly passed
		resolve := func(p string, flagChanged bool) string {
			if filepath.IsAbs(p) {
				return p
			}
			inRepo := filepath.Join(repoRoot, p)
			if !flagChanged {
				return inRepo
			}
			if _, err := os.Stat(inRepo); err == nil {
				return inRepo
			}
			if _, err := os.Stat(p); err == nil {
				if abs, err := filepath.Abs(p); err == nil {
					return abs
				}
				return p
			}
			if _, err := os.Stat(filepath.Dir(filepath.Clean(p))); err == nil {
				if abs, err := filepath.Abs(p); err == nil {
					return abs
				}
				return p
			}
			return inRepo
		}
		resolvedPaDir := resolve(seedPaDir, cmd.Flags().Changed("pa-dir"))
		resolvedOpsDir := resolve(seedOpsDir, cmd.Flags().Changed("ops-dir"))
		resolvedConfigPath := resolve(seedConfigPath, cmd.Flags().Changed("config"))

		resolvedBefore := seedBeforeCommit
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
			OpsDir:       resolvedOpsDir,
		}

		scopeCfg, err := config.LoadScopeConfig(resolvedConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("loading scope config: %w", err)
		}

		parsed, err := seed.ScanAndParse(resolvedPaDir, opts)
		if err != nil {
			return fmt.Errorf("scanning workorders: %w", err)
		}

		// Derive anchors from parsed results
		var allAnchors []string
		for _, pw := range parsed {
			for _, c := range pw.Report.Changes {
				allAnchors = append(allAnchors, c.Path)
			}
		}
		moduleScopes := config.DeriveModuleScopes(allAnchors)

		resolver := config.NewScopeResolver(scopeCfg, moduleScopes, nil)

		var opsWriter *ops.Writer
		if !seedDryRun {
			opsWriter = ops.NewWriter(resolvedOpsDir)
		}

		if seedDryRun {
			fmt.Printf("DRY RUN: Seeding from %s (no files will be written)\n", resolvedPaDir)
		} else {
			fmt.Printf("Seeding from %s to %s...\n", resolvedPaDir, resolvedOpsDir)
		}

		// Seed from pre-parsed results
		return seed.SeedFromParsed(parsed, opsWriter, resolver, opts)
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
