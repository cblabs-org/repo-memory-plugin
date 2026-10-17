/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_report.go
 * Description: Implements the `repo-memory report` CLI command, generating
 *              repository memory metrics, cognitive kind counts, graduated
 *              skill listings, and scope coverage / gap analysis.
 *
 * Exported Types / Functions:
 * - ReportOutput: Structured summary of repository memory metrics and scope coverage.
 * - RunReport():  Generates and prints a repository memory metrics and coverage report.
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
	"github.com/cblabs-org/repo-memory-plugin/internal/core/coverage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

var (
	reportRepoDir string // Optional --repo directory override for `repo-memory report`.
	reportJSON    bool   // When true, outputs the report as structured JSON.
)

// ReportOutput represents the structured output of `repo-memory report`.
type ReportOutput struct {
	RepoRoot          string                   `json:"repo_root"`          // Absolute path to the repository root.
	RepoID            string                   `json:"repo_id,omitempty"`  // Stable registry slug derived from remote origin URL or root path hash.
	Enrolled          bool                     `json:"enrolled"`           // Whether the repository is actively enrolled.
	Freshness         string                   `json:"freshness"`          // Single-line freshness status header.
	ActiveDossiers    int                      `json:"active_dossiers"`    // Number of scopes with active dossier entries.
	ActiveEntries     int                      `json:"active_entries"`     // Total active (non-retired) dossier entries.
	SemanticEntries   int                      `json:"semantic_entries"`   // Count of semantic-kind active entries.
	ProceduralEntries int                      `json:"procedural_entries"` // Count of procedural-kind active entries.
	EpisodicEntries   int                      `json:"episodic_entries"`   // Count of episodic-kind active entries.
	GraduatedSkills   []string                 `json:"graduated_skills"`   // Slugs of graduated procedural skills.
	Coverage          *coverage.CoverageReport `json:"coverage,omitempty"` // Per-scope coverage scores and metrics.
	Gaps              []coverage.ScopeCoverage `json:"gaps,omitempty"`     // Scopes falling below the coverage threshold.
	GapLines          []string                 `json:"gap_lines,omitempty"`// Formatted human-readable coverage gap lines.
}

// reportCmd defines the `repo-memory report` subcommand.
var reportCmd = &cobra.Command{
	Use:   "report [repo-dir]",
	Short: "Generate or view memory metrics and status report",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if strings.TrimSpace(reportRepoDir) != "" {
			targetDir = reportRepoDir
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetDir = args[0]
		}
		return RunReport(cmd, targetDir, reportJSON)
	},
}

func init() {
	reportCmd.Flags().StringVar(&reportRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	reportCmd.Flags().BoolVar(&reportJSON, "json", false, "Output structured JSON result")
}

// RunReport generates a v2.2 repository memory metrics and coverage report.
func RunReport(cmd *cobra.Command, targetDir string, asJSON bool) error {
	// --- Step 1: Resolve repository root, registry ID, and freshness header ---
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	repoRoot, err := gitops.FindRepoRoot(targetDir)
	if err != nil {
		if absDir, absErr := filepath.Abs(targetDir); absErr == nil {
			repoRoot = absDir
		} else {
			repoRoot = targetDir
		}
	}

	dataDir := config.DataDir()
	repoID := ""
	if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil {
		repoID = entry.RepoID
	}
	if repoID == "" {
		repoID, _ = gitops.GenerateRepoID(repoRoot)
	}

	enrolled := config.IsEnrolledFast(repoRoot)
	freshness := coverage.FormatFreshnessHeader(repoRoot, dataDir, repoID)

	// --- Step 2: Project active dossier entries and classify cognitive kinds ---
	activeDossiers, activeEntries := loadProjectedActiveEntries(ctx, repoRoot, dataDir, repoID)

	semanticCount := 0
	proceduralCount := 0
	episodicCount := 0
	for _, e := range activeEntries {
		switch e.EffectiveKind() {
		case dossiers.CognitiveKindProcedural:
			proceduralCount++
		case dossiers.CognitiveKindEpisodic:
			episodicCount++
		default:
			semanticCount++
		}
	}

	// --- Step 3: Evaluate graduated skills and scope coverage gaps ---
	var graduated []string
	if gradRes, err := skills.SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, true); err == nil && gradRes != nil {
		graduated = gradRes.Graduated
	}
	if graduated == nil {
		graduated = []string{}
	}

	threshold := coverage.DefaultGapThreshold
	if rCfg, err := config.LoadRepoConfig(repoRoot); err == nil && rCfg.Gaps.Threshold > 0 {
		threshold = rCfg.Gaps.Threshold
	}
	covReport, _ := coverage.ComputeCoverage(ctx, repoRoot, activeEntries, threshold)
	gaps := coverage.IdentifyCoverageGaps(covReport)
	gapLines := coverage.FormatGapLines(covReport)

	out := ReportOutput{
		RepoRoot:          repoRoot,
		RepoID:            repoID,
		Enrolled:          enrolled,
		Freshness:         freshness,
		ActiveDossiers:    activeDossiers,
		ActiveEntries:     len(activeEntries),
		SemanticEntries:   semanticCount,
		ProceduralEntries: proceduralCount,
		EpisodicEntries:   episodicCount,
		GraduatedSkills:   graduated,
		Coverage:          covReport,
		Gaps:              gaps,
		GapLines:          gapLines,
	}

	// --- Step 4: Render report as JSON or formatted text ---
	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintln(w, out.Freshness)
	fmt.Fprintf(w, "Repository: %s (enrolled: %v)\n", out.RepoRoot, out.Enrolled)
	fmt.Fprintf(w, "Dossiers:   %d active\n", out.ActiveDossiers)
	fmt.Fprintf(w, "Entries:    %d active (semantic: %d, procedural: %d, episodic: %d)\n",
		out.ActiveEntries, out.SemanticEntries, out.ProceduralEntries, out.EpisodicEntries)
	fmt.Fprintf(w, "Skills:     %d graduated\n", len(out.GraduatedSkills))
	for _, s := range out.GraduatedSkills {
		fmt.Fprintf(w, "  - %s\n", s)
	}
	if covReport != nil && len(covReport.Scopes) > 0 {
		fmt.Fprintln(w, "Scope Coverage:")
		for _, sc := range covReport.Scopes {
			fmt.Fprintf(w, "  - %s: score %.2f (%d active, %d suspect, %d open issues)\n",
				sc.Scope, sc.Score, sc.ActiveEntries, sc.SuspectEntries, sc.OpenIssues)
		}
	}
	if len(out.GapLines) > 0 {
		fmt.Fprintln(w, "Coverage Gaps:")
		for _, gl := range out.GapLines {
			fmt.Fprintf(w, "  - %s\n", gl)
		}
	}
	return nil
}
