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
	reportRepoDir string
	reportJSON    bool
)

// ReportOutput represents the structured output of `repo-memory report`.
type ReportOutput struct {
	RepoRoot          string                   `json:"repo_root"`
	RepoID            string                   `json:"repo_id,omitempty"`
	Enrolled          bool                     `json:"enrolled"`
	Freshness         string                   `json:"freshness"`
	ActiveDossiers    int                      `json:"active_dossiers"`
	ActiveEntries     int                      `json:"active_entries"`
	SemanticEntries   int                      `json:"semantic_entries"`
	ProceduralEntries int                      `json:"procedural_entries"`
	EpisodicEntries   int                      `json:"episodic_entries"`
	GraduatedSkills   []string                 `json:"graduated_skills"`
	Coverage          *coverage.CoverageReport `json:"coverage,omitempty"`
	Gaps              []coverage.ScopeCoverage `json:"gaps,omitempty"`
	GapLines          []string                 `json:"gap_lines,omitempty"`
}

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

	activeDossiers, activeEntries := loadProjectedActiveEntries(ctx, repoRoot)

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
