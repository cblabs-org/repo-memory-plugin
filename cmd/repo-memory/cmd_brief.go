/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_brief.go
 * Description: CLI subcommand and runner for generating a bounded memory brief
 *              from repository dossiers and the SQLite index for a Principal
 *              Architect workorder or explicit module scope.
 *
 * Exported Types / Functions:
 * - RunBrief(): Generates a bounded workorder or scope brief in Markdown or JSON.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/delivery"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
)

var (
	briefWorkorderPath string // Path to a WORKORDER.md file (--workorder).
	briefScope         string // Comma-separated module scope(s) to brief (--scope).
	briefRepoDir       string // Target repository root directory (--repo).
	briefDataDir       string // Optional data directory override (--data-dir).
	briefMaxTokens     int    // Maximum estimated tokens for brief markdown (--max-tokens).
	briefJSON          bool   // Whether to output structured JSON (--json).
)

// briefCmd defines the `repo-memory brief` subcommand.
var briefCmd = &cobra.Command{
	Use:   "brief [workorder-path]",
	Short: "Generate a bounded memory brief for a Principal Architect workorder or scope",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		woPath := strings.TrimSpace(briefWorkorderPath)
		if woPath == "" && len(args) > 0 {
			woPath = strings.TrimSpace(args[0])
		}
		scope := strings.TrimSpace(briefScope)
		if woPath == "" && scope == "" {
			return fmt.Errorf("usage: repo-memory brief [--workorder <path>] [--scope <scope>] [--max-tokens <n>] [--json]")
		}
		return RunBrief(cmd, briefRepoDir, briefDataDir, woPath, scope, briefMaxTokens, briefJSON)
	},
}

func init() {
	briefCmd.Flags().StringVar(&briefWorkorderPath, "workorder", "", "Path to WORKORDER.md file")
	briefCmd.Flags().StringVar(&briefScope, "scope", "", "Comma-separated scope(s) to brief")
	briefCmd.Flags().StringVar(&briefRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	briefCmd.Flags().StringVar(&briefDataDir, "data-dir", "", "Data directory override")
	briefCmd.Flags().IntVar(&briefMaxTokens, "max-tokens", delivery.DefaultBriefMaxTokens, "Maximum estimated tokens for brief markdown")
	briefCmd.Flags().BoolVar(&briefJSON, "json", false, "Output structured JSON result")
}

// RunBrief executes `repo-memory brief` and writes human-readable markdown or JSON to cmd.OutOrStdout().
func RunBrief(cmd *cobra.Command, targetRepoDir, overrideDataDir, workorderPath, explicitScope string, maxTokens int, asJSON bool) error {
	// --- Step 1: Resolve repository root, data directory, and repository ID ---
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

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

	// --- Step 2: Open read-only SQLite index when available ---
	var idx *index.Index
	if dataDir != "" && repoID != "" {
		idxCandidates := []string{
			index.IndexPath(dataDir, repoID),
			filepath.Join(dataDir, "repos", repoID, "index.db"),
		}
		for _, idxPath := range idxCandidates {
			if fi, statErr := os.Stat(idxPath); statErr == nil && !fi.IsDir() {
				if opened, openErr := index.NewReadOnlyIndex(idxPath); openErr == nil {
					idx = opened
					defer idx.Close()
					break
				}
			}
		}
	}

	// --- Step 3: Build workorder brief and render output ---
	res, err := delivery.BuildWorkorderBrief(ctx, idx, repoRoot, dataDir, repoID, workorderPath, explicitScope, maxTokens)
	if err != nil {
		return err
	}

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	fmt.Fprint(cmd.OutOrStdout(), res.BriefMarkdown)
	return nil
}
