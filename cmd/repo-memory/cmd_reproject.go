/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_reproject.go
 * Description: Implements the `repo-memory reproject` CLI command, which
 *              deterministically replays the operation log to regenerate
 *              dossiers, ATLAS.md, QUESTIONS.md, and graduated skills.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/coverage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/spf13/cobra"
)

var (
	reprojectRepo         string // Optional --repo path to the target repository root.
	reprojectDataDir      string // Optional --data-dir override for private storage.
	reprojectOpsDir       string // Path to the primary ops directory (--ops-dir).
	reprojectDryRun       bool   // When true, previews reprojection changes without writing to disk.
	reprojectJSON         bool   // When true, emits the reprojection result as indented JSON.
	reprojectAllowLoss    bool   // When true, permits overwriting unrecorded entries or decisions missing from ops.
	reprojectIncludeLocal bool   // When true, overlays branch-local and user ops directories from dataDir.
)

// reprojectCmd defines the `repo-memory reproject` subcommand.
var reprojectCmd = &cobra.Command{
	Use:   "reproject [repo-dir]",
	Short: "Deterministically replay the op-log to regenerate dossiers, ATLAS.md, QUESTIONS.md, and graduated skills",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		// --- Step 1: Resolve target repository root, data directory, and repo ID ---
		targetRepo := strings.TrimSpace(reprojectRepo)
		if targetRepo == "" && len(args) > 0 {
			targetRepo = strings.TrimSpace(args[0])
		}
		if targetRepo == "" {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			targetRepo = cwd
		}

		repoRoot, err := gitops.FindRepoRoot(targetRepo)
		if err != nil || repoRoot == "" {
			abs, absErr := filepath.Abs(targetRepo)
			if absErr != nil {
				return fmt.Errorf("finding repo root: %w", err)
			}
			repoRoot = abs
		}

		dataDir := strings.TrimSpace(reprojectDataDir)
		if dataDir == "" {
			dataDir = config.DataDir()
		}

		repoID := ""
		if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil && entry.RepoID != "" {
			repoID = entry.RepoID
		}
		if repoID == "" {
			id, err := gitops.GenerateRepoID(repoRoot)
			if err != nil || id == "" {
				id = filepath.Base(repoRoot)
			}
			repoID = id
		}

		// --- Step 2: Assemble primary and optional local operation log directories ---
		primaryOpsDir := strings.TrimSpace(reprojectOpsDir)
		if primaryOpsDir == "" {
			primaryOpsDir = filepath.Join(repoRoot, ".memory", "ops")
		} else if !filepath.IsAbs(primaryOpsDir) {
			primaryOpsDir = filepath.Join(repoRoot, primaryOpsDir)
		}

		opsDirs := []string{primaryOpsDir}
		if reprojectIncludeLocal && dataDir != "" {
			if repoID != "" {
				opsDirs = append(opsDirs,
					filepath.Join(dataDir, "repos", repoID, "ops-local"),
					filepath.Join(dataDir, repoID, "ops-local"),
				)
			}
			opsDirs = append(opsDirs, filepath.Join(dataDir, "user", "ops"))
		}

		// --- Step 3: Replay operation logs and reproject dossier files ---
		memDir := filepath.Join(repoRoot, ".memory")
		res, dossiersMap, reprojErr := dossiers.Reproject(ctx, dossiers.ReprojectOptions{
			MemoryDir: memDir,
			OpsDirs:   opsDirs,
			DryRun:    reprojectDryRun,
			AllowLoss: reprojectAllowLoss,
		})
		if reprojErr != nil {
			if reprojectJSON && res != nil {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				_ = enc.Encode(res)
			}
			return reprojErr
		}

		// --- Step 4: Sync graduated skills and regenerate QUESTIONS.md ---
		repoCfg, err := config.LoadRepoConfig(repoRoot)
		if err != nil {
			repoCfg = config.DefaultRepoConfig()
		}

		if skillRes, sErr := skills.SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, reprojectDryRun); sErr == nil && skillRes != nil {
			res.SkillsGraduated = len(skillRes.Graduated)
		}

		var activeEntries []dossiers.Entry
		for _, d := range dossiersMap {
			activeEntries = append(activeEntries, d.Entries...)
		}

		if covReport, covErr := coverage.ComputeCoverage(ctx, repoRoot, activeEntries, repoCfg.Gaps.Threshold); covErr == nil && covReport != nil {
			if reprojectDryRun {
				expectedQ := coverage.GenerateQuestionsMarkdown(covReport, activeEntries)
				qPath := filepath.Join(memDir, "QUESTIONS.md")
				existingQ, readErr := os.ReadFile(qPath)
				res.QuestionsUpdated = readErr != nil || string(existingQ) != expectedQ
			} else {
				if wErr := coverage.WriteQuestionsFile(repoRoot, covReport, activeEntries); wErr == nil {
					res.QuestionsUpdated = true
				}
			}
		}

		// --- Step 5: Emit JSON or human-readable reprojection summary ---
		if reprojectJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		fmt.Fprintf(cmd.OutOrStdout(),
			"Reprojected %d dossiers (%d active, %d history entries; %d verified preserved, %d human decisions preserved): added=%d, updated=%d, unchanged=%d, removed=%d, skills=%d (dry_run=%v)\n",
			res.DossiersProjected,
			res.ActiveEntries,
			res.HistoryEntries,
			res.VerifiedPreserved,
			res.HumanDecisionsPreserved,
			len(res.FilesAdded),
			len(res.FilesUpdated),
			len(res.FilesUnchanged),
			len(res.FilesRemoved),
			res.SkillsGraduated,
			res.DryRun,
		)
		return nil
	},
}

func init() {
	reprojectCmd.Flags().StringVar(&reprojectRepo, "repo", "", "Path to repository root")
	reprojectCmd.Flags().StringVar(&reprojectDataDir, "data-dir", "", "Override data directory")
	reprojectCmd.Flags().StringVar(&reprojectOpsDir, "ops-dir", ".memory/ops", "Path to primary ops directory")
	reprojectCmd.Flags().BoolVar(&reprojectDryRun, "dry-run", false, "Preview reprojection changes without mutating disk")
	reprojectCmd.Flags().BoolVar(&reprojectJSON, "json", false, "Output JSON result")
	reprojectCmd.Flags().BoolVar(&reprojectAllowLoss, "allow-loss", false, "Allow overwriting unrecorded entries or decisions missing from ops")
	reprojectCmd.Flags().BoolVar(&reprojectIncludeLocal, "include-local", false, "Overlay local branch ops-local and user ops directories from dataDir")
}
