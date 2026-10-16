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
	gardenRepoDir   string
	gardenDryRun    bool
	gardenJSON      bool
	gardenMaxTokens int
)

var gardenCmd = &cobra.Command{
	Use:   "garden [repo-dir]",
	Short: "Run dossier gardening (broken-anchor cleanup, duplicate merge, cap split, and doc freshness)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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

		epStore := episode.NewStore(dataDir, repoID)
		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)

		res, err := exec.RunGardening(cmd.Context(), tick.GardeningOptions{
			DryRun:           gardenDryRun,
			MaxDossierTokens: gardenMaxTokens,
		})
		if err != nil {
			return err
		}

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
