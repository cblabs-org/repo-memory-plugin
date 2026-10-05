package main

import (
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

var disableRepoFlag bool

var disableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable repository memory globally or for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunDisable(cmd, disableRepoFlag)
	},
}

func init() {
	disableCmd.Flags().BoolVar(&disableRepoFlag, "repo", false, "Disable repository memory for the current repository only")
}

// RunDisable disables repository memory globally or for the current repository.
func RunDisable(cmd *cobra.Command, repoOnly bool) error {
	if repoOnly {
		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repository root: %w", err)
		}
		if err := config.SetRepoEnabled(repoRoot, false); err != nil {
			return fmt.Errorf("disabling repository memory: %w", err)
		}
		if err := RemoveWorkspacePluginConfigured(repoRoot); err != nil {
			return fmt.Errorf("removing workspace plugin configuration: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Disabled repository memory for %s\n", repoRoot)
		return nil
	}

	dataDir := config.DataDir()
	if err := config.SetMachineEnabled(dataDir, false); err != nil {
		return fmt.Errorf("disabling machine memory: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Disabled repository memory globally")
	return nil
}
