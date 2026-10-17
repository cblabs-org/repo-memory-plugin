/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_seed_docs.go
 * Description: Implements the `repo-memory seed-docs` CLI command, which seeds
 *              dossier entries from curated repository markdown documentation
 *              and skill files and projects updated dossiers to disk.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

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
	seedDocsRepoDir    string // Target repository root path for `repo-memory seed-docs --repo`.
	seedDocsOpsDir     string // Target `.memory/ops/` path for `repo-memory seed-docs --ops-dir`.
	seedDocsConfigPath string // Path to `.memory/config.yaml` for `repo-memory seed-docs --config`.
	seedDocsDryRun     bool   // When true, previews seeded doc entries without writing to disk.
	seedDocsJSON       bool   // When true, emits the seeding summary as indented JSON.
)

// seedDocsCmd defines the `repo-memory seed-docs` subcommand.
var seedDocsCmd = &cobra.Command{
	Use:   "seed-docs [doc-paths...]",
	Short: "Seed dossier entries from curated repository markdown documentation and skills",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve repository root, ops directory, and config paths ---
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

		// --- Step 2: Load repository and scope configurations ---
		repoCfg, err := config.LoadRepoConfig(resolvedConfigPath)
		if err != nil && !os.IsNotExist(err) {
			repoCfg, _ = config.LoadRepoConfig(repoRoot)
		}

		scopeCfg, err := config.LoadScopeConfig(resolvedConfigPath)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("loading scope config: %w", err)
		}

		// --- Step 3: Seed document entries into the operation log ---
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

		// --- Step 4: Reproject dossiers when new operations are written ---
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

		// --- Step 5: Render JSON or human-readable summary ---
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
