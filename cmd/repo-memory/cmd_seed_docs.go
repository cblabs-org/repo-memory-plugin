package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

var (
	seedDocsRepoDir    string
	seedDocsOpsDir     string
	seedDocsConfigPath string
	seedDocsDryRun     bool
	seedDocsJSON       bool
)

var seedDocsCmd = &cobra.Command{
	Use:   "seed-docs [doc-paths...]",
	Short: "Seed dossier entries from curated repository markdown documentation and skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, err := gitops.FindRepoRoot(seedDocsRepoDir)
		if err != nil {
			if absDir, absErr := filepath.Abs(seedDocsRepoDir); absErr == nil {
				repoRoot = absDir
			} else {
				repoRoot = seedDocsRepoDir
			}
		}

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
		resolvedOpsDir := resolve(seedDocsOpsDir, cmd.Flags().Changed("ops-dir"))
		resolvedConfigPath := resolve(seedDocsConfigPath, cmd.Flags().Changed("config"))

		repoCfg, err := config.LoadRepoConfig(resolvedConfigPath)
		if err != nil && !os.IsNotExist(err) {
			repoCfg, _ = config.LoadRepoConfig(repoRoot)
		}

		scopeCfg, err := config.LoadScopeConfig(resolvedConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("loading scope config: %w", err)
		}

		opsReader := ops.NewReader(resolvedOpsDir)
		var opsWriter *ops.Writer
		if !seedDocsDryRun {
			opsWriter = ops.NewWriter(resolvedOpsDir)
		}

		opts := seed.DocSeedOptions{
			RepoRoot: repoRoot,
			OpsDir:   resolvedOpsDir,
			DocPaths: args,
			RepoCfg:  repoCfg,
			DryRun:   seedDocsDryRun,
		}

		res, err := seed.SeedDocs(cmd.Context(), opts, scopeCfg, opsReader, opsWriter)
		if err != nil {
			return err
		}

		totalMutated := res.EntriesAdded + res.EntriesAmend + res.EntriesRetire
		if !seedDocsDryRun && totalMutated > 0 {
			proj := dossiers.NewProjector(opsReader)
			dossiersMap, projErr := proj.Project(cmd.Context())
			if projErr != nil {
				return fmt.Errorf("projecting dossiers after seed-docs: %w", projErr)
			}
			memBaseDir := filepath.Dir(resolvedOpsDir)
			writer := dossiers.NewWriter(memBaseDir)
			if writeErr := writer.WriteAll(dossiersMap); writeErr != nil {
				return fmt.Errorf("writing projected dossiers: %w", writeErr)
			}
		}

		if seedDocsJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		if seedDocsDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: ")
		}
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"Seeded %d entries (%d added, %d amended, %d retired) from %d/%d docs (%d unchanged skipped)\n",
			totalMutated,
			res.EntriesAdded,
			res.EntriesAmend,
			res.EntriesRetire,
			res.FilesSeeded,
			res.FilesScanned,
			res.FilesSkipped,
		)
		return nil
	},
}

func init() {
	seedDocsCmd.Flags().StringVar(&seedDocsRepoDir, "repo", ".", "Path to the target repository root")
	seedDocsCmd.Flags().StringVar(&seedDocsOpsDir, "ops-dir", ".memory/ops/", "Path to the ops directory")
	seedDocsCmd.Flags().StringVar(&seedDocsConfigPath, "config", ".memory/config.yaml", "Path to config.yaml")
	seedDocsCmd.Flags().BoolVar(&seedDocsDryRun, "dry-run", false, "Preview what would be seeded without writing")
	seedDocsCmd.Flags().BoolVar(&seedDocsJSON, "json", false, "Emit structured JSON summary")
}
