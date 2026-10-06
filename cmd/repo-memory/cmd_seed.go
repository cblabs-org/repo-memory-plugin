package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

var (
	seedPaDir       string
	seedOpsDir      string
	seedConfigPath  string
	seedBeforeCommit string
	seedDryRun      bool
)

var seedCmd = &cobra.Command{
	Use:   "seed",
	Short: "Seed memory ops from Principal Architect completion reports",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := gitops.FindRepoRoot(".")
		if err == nil {
			if seedPaDir == ".agents/skills/principal-architect-workspace/" {
				seedPaDir = filepath.Join(repoRoot, seedPaDir)
			}
			if seedOpsDir == ".memory/ops/" {
				seedOpsDir = filepath.Join(repoRoot, seedOpsDir)
			}
			if seedConfigPath == ".memory/config.yaml" {
				seedConfigPath = filepath.Join(repoRoot, seedConfigPath)
			}
		}

		opts := seed.SeedOptions{
			RepoRoot:     repoRoot,
			BeforeCommit: seedBeforeCommit,
			OpsDir:       seedOpsDir,
		}

		scopeCfg, err := config.LoadScopeConfig(seedConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("loading scope config: %w", err)
		}
		anchors, err := seed.ScanAllAnchors(seedPaDir, opts)
		if err != nil {
			return fmt.Errorf("scanning anchors: %w", err)
		}
		moduleScopes := config.DeriveModuleScopes(anchors)

		resolver := config.NewScopeResolver(scopeCfg, moduleScopes, nil)

		var opsWriter *ops.Writer
		if !seedDryRun {
			opsWriter = ops.NewWriter(seedOpsDir)
		}

		if seedDryRun {
			fmt.Printf("DRY RUN: Seeding from %s (no files will be written)\n", seedPaDir)
		} else {
			fmt.Printf("Seeding from %s to %s...\n", seedPaDir, seedOpsDir)
		}

		if seedBeforeCommit != "" {
			return seed.SeedWithTimeCut(seedPaDir, opsWriter, resolver, opts)
		}
		return seed.SeedFromWorkorders(seedPaDir, opsWriter, resolver, opts)
	},
}

func init() {
	seedCmd.Flags().StringVar(&seedPaDir, "pa-dir", ".agents/skills/principal-architect-workspace/", "Path to the PA workspace")
	seedCmd.Flags().StringVar(&seedOpsDir, "ops-dir", ".memory/ops/", "Path to the ops directory")
	seedCmd.Flags().StringVar(&seedConfigPath, "config", ".memory/config.yaml", "Path to config.yaml")
	seedCmd.Flags().StringVar(&seedBeforeCommit, "before", "", "Git commit for time-cut seeding")
	seedCmd.Flags().BoolVar(&seedDryRun, "dry-run", false, "Print what would be seeded without writing")
}
