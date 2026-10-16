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
	"github.com/cblabs-org/repo-memory-plugin/internal/core/coverage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

var (
	statusRepoDir string
	statusJSON    bool
)

// StatusOutput represents the structured output of `repo-memory status`.
type StatusOutput struct {
	RepoRoot        string   `json:"repo_root"`
	RepoID          string   `json:"repo_id,omitempty"`
	Enrolled        bool     `json:"enrolled"`
	Freshness       string   `json:"freshness"`
	ActiveDossiers  int      `json:"active_dossiers"`
	ActiveEntries   int      `json:"active_entries"`
	GraduatedSkills []string `json:"graduated_skills"`
}

var statusCmd = &cobra.Command{
	Use:   "status [repo-dir]",
	Short: "Display current repository memory status",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if strings.TrimSpace(statusRepoDir) != "" {
			targetDir = statusRepoDir
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
			targetDir = args[0]
		}
		return RunStatus(cmd, targetDir, statusJSON)
	},
}

func init() {
	statusCmd.Flags().StringVar(&statusRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output structured JSON result")
}

// RunStatus inspects repository memory status and writes human-readable or JSON output to cmd.OutOrStdout().
func RunStatus(cmd *cobra.Command, targetDir string, asJSON bool) error {
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

	var graduated []string
	if gradRes, err := skills.SyncGraduatedSkills(ctx, repoRoot, dataDir, repoID, true); err == nil && gradRes != nil {
		graduated = gradRes.Graduated
	}
	if graduated == nil {
		graduated = []string{}
	}

	out := StatusOutput{
		RepoRoot:        repoRoot,
		RepoID:          repoID,
		Enrolled:        enrolled,
		Freshness:       freshness,
		ActiveDossiers:  activeDossiers,
		ActiveEntries:   len(activeEntries),
		GraduatedSkills: graduated,
	}

	if asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	w := cmd.OutOrStdout()
	fmt.Fprintln(w, out.Freshness)
	fmt.Fprintf(w, "Repository: %s\n", out.RepoRoot)
	fmt.Fprintf(w, "Enrolled:   %v\n", out.Enrolled)
	fmt.Fprintf(w, "Dossiers:   %d active (%d active entries)\n", out.ActiveDossiers, out.ActiveEntries)
	fmt.Fprintf(w, "Skills:     %d graduated\n", len(out.GraduatedSkills))
	return nil
}

func loadProjectedActiveEntries(ctx context.Context, repoRoot string) (int, []dossiers.Entry) {
	if strings.TrimSpace(repoRoot) == "" {
		return 0, nil
	}
	opsDir := filepath.Join(repoRoot, ".memory", "ops")
	info, err := os.Stat(opsDir)
	if err != nil || !info.IsDir() {
		return 0, nil
	}
	projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
	if err != nil {
		return 0, nil
	}
	activeDossiers := 0
	var activeEntries []dossiers.Entry
	for _, d := range projected {
		if d == nil {
			continue
		}
		hasActive := false
		for _, e := range d.Entries {
			if !e.Retired && strings.TrimSpace(e.SupersededBy) == "" {
				hasActive = true
				activeEntries = append(activeEntries, e)
			}
		}
		if hasActive {
			activeDossiers++
		}
	}
	return activeDossiers, activeEntries
}
