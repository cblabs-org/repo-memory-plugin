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
	reprojectRepo         string
	reprojectDataDir      string
	reprojectOpsDir       string
	reprojectDryRun       bool
	reprojectJSON         bool
	reprojectAllowLoss    bool
	reprojectIncludeLocal bool
)

var reprojectCmd = &cobra.Command{
	Use:   "reproject [repo-dir]",
	Short: "Deterministically replay the op-log to regenerate dossiers, ATLAS.md, QUESTIONS.md, and graduated skills",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

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
