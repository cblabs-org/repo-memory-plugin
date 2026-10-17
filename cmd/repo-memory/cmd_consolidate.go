/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_consolidate.go
 * Description: CLI command for running Stage 2 warm batch consolidation over
 *              pending inbox notes into .memory/ops/ and multi-tier dossiers.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"encoding/json"
	"fmt"
	osExec "os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/antigravity"
)

var (
	consolidateRepoDir       string // Optional --repo directory override.
	consolidateForce         bool   // Whether to bypass cadence/count thresholds.
	consolidateDryRun        bool   // Whether to preview consolidation without writing.
	consolidateJSON          bool   // Whether to emit structured JSON output.
	consolidateDeterministic bool   // Whether to use deterministic op synthesis without LLM.
)

// consolidateCmd defines the `repo-memory consolidate` subcommand.
var consolidateCmd = &cobra.Command{
	Use:   "consolidate [repo-dir]",
	Short: "Run Stage 2 warm batch consolidation over pending inbox notes",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve target repository root and repo ID ---
		targetDir := "."
		if strings.TrimSpace(consolidateRepoDir) != "" {
			targetDir = strings.TrimSpace(consolidateRepoDir)
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetDir = strings.TrimSpace(args[0])
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

		// --- Step 2: Configure pipeline executor and run Stage 2 consolidation ---
		epStore := episode.NewStore(dataDir, repoID)
		exec := tick.NewPipelineExecutor(dataDir, repoRoot, repoID, nil, epStore)

		if !consolidateDeterministic && !consolidateDryRun {
			if _, lookErr := osExec.LookPath("agy"); lookErr == nil {
				exec.SetHousekeeper(antigravity.NewHousekeeper(dataDir, repoID, "", "5m"))
			}
		}

		force := true
		if cmd.Flags().Changed("force") {
			force = consolidateForce
		}

		res, err := exec.RunConsolidationWithOptions(cmd.Context(), tick.ConsolidationOptions{
			Force:  force,
			DryRun: consolidateDryRun,
		})
		if err != nil {
			return err
		}

		// --- Step 3: Format and emit consolidation results ---
		if consolidateJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		if res.DryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: ")
		}
		if res.Skipped {
			fmt.Fprintf(
				cmd.OutOrStdout(),
				"Consolidation skipped (%s): %d unconsolidated notes waiting\n",
				res.SkipReason,
				res.UnconsolidatedNotes,
			)
			return nil
		}

		fmt.Fprintf(
			cmd.OutOrStdout(),
			"Consolidated %d/%d notes (%d items, %d echoes dropped, %d downgraded, %d exact-dup amends) -> ops: +%d ~%d ^%d -%d (est tokens: %d)\n",
			res.ProcessedNotes,
			res.UnconsolidatedNotes,
			res.CandidateItems,
			res.EchoesDropped,
			res.DowngradedToIssue,
			res.ExactDuplicatesAmend,
			res.OpsAdded,
			res.OpsAmended,
			res.OpsSuperseded,
			res.OpsRetired,
			res.EstimatedInputTokens,
		)
		return nil
	},
}

func init() {
	consolidateCmd.Flags().StringVar(&consolidateRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	consolidateCmd.Flags().BoolVar(&consolidateForce, "force", false, "Consolidate immediately regardless of note count or 24h cadence")
	consolidateCmd.Flags().BoolVar(&consolidateDryRun, "dry-run", false, "Preview candidate routing, filtering, and estimated token usage without writing")
	consolidateCmd.Flags().BoolVar(&consolidateJSON, "json", false, "Output structured JSON result")
	consolidateCmd.Flags().BoolVar(&consolidateDeterministic, "deterministic", false, "Use deterministic op synthesis without invoking the LLM housekeeper")
}
