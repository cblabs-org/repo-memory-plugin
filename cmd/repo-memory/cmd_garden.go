/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_garden.go
 * Description: CLI subcommand for running dossier gardening (broken-anchor
 *              cleanup, duplicate merge, token-cap splitting, and doc freshness).
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

var (
	gardenRepoDir   string // Target repository root directory (--repo).
	gardenDryRun    bool   // Whether to preview gardening operations without writing (--dry-run).
	gardenJSON      bool   // Whether to output structured JSON result (--json).
	gardenMaxTokens int    // Maximum estimated tokens per dossier before splitting (--max-tokens).
)

// gardenCmd defines the `repo-memory garden` subcommand.
var gardenCmd = &cobra.Command{
	Use:   "garden [repo-dir]",
	Short: "Run dossier gardening (broken-anchor cleanup, duplicate merge, cap split, and doc freshness)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve target repository root and repository ID ---
		targetDir := "."
		if gardenRepoDir != "" {
			targetDir = gardenRepoDir
		} else if len(args) > 0 {
			targetDir = args[0]
		}

		repoRoot, err := gitops.FindRepoRoot(targetDir)
		if err != nil {
			absDir, absErr := filepath.Abs(targetDir)
			if absErr != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}
			repoRoot = absDir
		}

		dataDir := config.DataDir()
		repoID := ""
		if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil {
			repoID = entry.RepoID
		}
		if repoID == "" {
			var genErr error
			repoID, genErr = gitops.GenerateRepoID(repoRoot)
			if genErr != nil {
				return fmt.Errorf("generating repo ID: %w", genErr)
			}
		}

		// --- Step 2: Execute dossier gardening pass ---
		epStore := episode.NewStore(dataDir, repoID)
		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)

		res, err := exec.RunGardening(cmd.Context(), tick.GardeningOptions{
			DryRun:           gardenDryRun,
			MaxDossierTokens: gardenMaxTokens,
		})
		if err != nil {
			return err
		}

		// --- Step 3: Format and emit gardening summary ---
		if gardenJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		if gardenDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: ")
		}
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"Gardening complete: %d ops (%d broken-anchor retired, %d anchors pruned, %d duplicates merged, %d dossiers split)\n",
			res.OpsEmitted,
			len(res.RetiredBrokenAnchors),
			len(res.PrunedAnchorEntries),
			len(res.MergedDuplicates),
			len(res.SplitDossiers),
		)
		return nil
	},
}

func init() {
	gardenCmd.Flags().StringVar(&gardenRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	gardenCmd.Flags().BoolVar(&gardenDryRun, "dry-run", false, "Preview gardening operations without writing to .memory/ops/")
	gardenCmd.Flags().BoolVar(&gardenJSON, "json", false, "Output structured JSON result")
	gardenCmd.Flags().IntVar(&gardenMaxTokens, "max-tokens", 3000, "Maximum estimated tokens per dossier before splitting")
}
