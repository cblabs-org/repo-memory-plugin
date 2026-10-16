package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
)

var (
	promoteSkillsRepoDir string
	promoteSkillsDryRun  bool
	promoteSkillsJSON    bool
)

var promoteSkillsCmd = &cobra.Command{
	Use:   "promote-skills [repo-dir]",
	Short: "Deterministically graduate qualified procedural recipes to .agents/skills/rm-<slug>/SKILL.md",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if strings.TrimSpace(promoteSkillsRepoDir) != "" {
			targetDir = promoteSkillsRepoDir
		} else if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
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
			repoID, _ = gitops.GenerateRepoID(repoRoot)
		}

		if !promoteSkillsDryRun {
			_, _ = skills.CheckBuildManifestFreshness(cmd.Context(), repoRoot, dataDir, repoID, nil)
		}

		res, err := skills.SyncGraduatedSkills(cmd.Context(), repoRoot, dataDir, repoID, promoteSkillsDryRun)
		if err != nil {
			return err
		}

		if promoteSkillsJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		prefix := ""
		if res.DryRun {
			prefix = "DRY RUN: "
		}
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"%sGraduated skills: %d qualified (%d written, %d unchanged, %d stale removed)\n",
			prefix,
			len(res.Graduated),
			len(res.Written),
			len(res.Unchanged),
			len(res.Removed),
		)
		for _, slug := range res.Graduated {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", slug)
		}
		return nil
	},
}

func init() {
	promoteSkillsCmd.Flags().StringVar(&promoteSkillsRepoDir, "repo", "", "Repository directory (defaults to current working directory)")
	promoteSkillsCmd.Flags().BoolVar(&promoteSkillsDryRun, "dry-run", false, "Preview skills that would be written or removed without modifying files")
	promoteSkillsCmd.Flags().BoolVar(&promoteSkillsJSON, "json", false, "Output structured JSON result")
}
