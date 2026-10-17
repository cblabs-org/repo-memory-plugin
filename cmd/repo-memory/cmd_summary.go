/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_summary.go
 * Description: Implements the `repo-memory summary` CLI command, generating a
 *              token-bounded structured summary of a worker session from a
 *              session ID or transcript path.
 *
 * Exported Types / Functions:
 * - RunSummary(): Executes `repo-memory summary` and writes markdown or JSON to cmd.OutOrStdout().
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
)

var (
	summarySession   string // Session ID or transcript path passed via --session.
	summaryRepoDir   string // Optional --repo directory override for `repo-memory summary`.
	summaryDataDir   string // Optional --data-dir override for private storage.
	summaryMaxTokens int    // Maximum estimated tokens for the generated markdown summary.
	summaryJSON      bool   // When true, outputs the session summary as structured JSON.
)

// summaryCmd defines the `repo-memory summary` subcommand.
var summaryCmd = &cobra.Command{
	Use:   "summary [session-id-or-path]",
	Short: "Generate a bounded structured summary of a worker session",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := strings.TrimSpace(summarySession)
		if target == "" && len(args) > 0 {
			target = strings.TrimSpace(args[0])
		}
		if target == "" {
			return fmt.Errorf("usage: repo-memory summary <session-id-or-path> [--json] [--max-tokens N]")
		}
		return RunSummary(cmd, summaryRepoDir, summaryDataDir, target, summaryMaxTokens, summaryJSON)
	},
}

func init() {
	summaryCmd.Flags().StringVar(&summarySession, "session", "", "Session ID or transcript file/directory path")
	summaryCmd.Flags().StringVar(&summaryRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	summaryCmd.Flags().StringVar(&summaryDataDir, "data-dir", "", "Data directory override")
	summaryCmd.Flags().IntVar(&summaryMaxTokens, "max-tokens", episode.DefaultSummaryMaxTokens, "Maximum estimated tokens for summary markdown")
	summaryCmd.Flags().BoolVar(&summaryJSON, "json", false, "Output structured JSON result")
}

// RunSummary executes `repo-memory summary` and writes human-readable markdown or JSON to cmd.OutOrStdout().
func RunSummary(cmd *cobra.Command, targetRepoDir, overrideDataDir, target string, maxTokens int, asJSON bool) error {
	// --- Step 1: Resolve repository root, data directory, and repo ID ---
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx

	repoDir := "."
	if strings.TrimSpace(targetRepoDir) != "" {
		repoDir = strings.TrimSpace(targetRepoDir)
	}
	repoRoot, err := gitops.FindRepoRoot(repoDir)
	if err != nil {
		if absDir, absErr := filepath.Abs(repoDir); absErr == nil {
			repoRoot = absDir
		} else {
			repoRoot = repoDir
		}
	}

	dataDir := strings.TrimSpace(overrideDataDir)
	if dataDir == "" {
		dataDir = config.DataDir()
	}

	repoID := ""
	if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil {
		repoID = entry.RepoID
	}
	if repoID == "" {
		repoID, _ = gitops.GenerateRepoID(repoRoot)
	}

	// --- Step 2: Summarize the target session input within the token budget ---
	res, err := episode.SummarizeSessionInput(dataDir, repoID, repoRoot, target, maxTokens)
	if err != nil {
		return err
	}

	// --- Step 3: Emit JSON or markdown summary ---
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprint(cmd.OutOrStdout(), res.SummaryMarkdown)
	return nil
}
