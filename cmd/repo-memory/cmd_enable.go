package main

import (
	"fmt"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/spf13/cobra"
)

var enableRepoFlag bool

var enableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable repository memory globally or for the current repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunEnable(cmd, enableRepoFlag)
	},
}

func init() {
	enableCmd.Flags().BoolVar(&enableRepoFlag, "repo", false, "Enable repository memory for the current repository only")
}

// RunEnable enables repository memory globally or for the current repository.
func RunEnable(cmd *cobra.Command, repoOnly bool) error {
	if repoOnly {
		repoRoot, err := gitops.FindRepoRoot(".")
		if err != nil {
			return fmt.Errorf("finding repository root: %w", err)
		}
		if err := config.SetRepoEnabled(repoRoot, true); err != nil {
			return fmt.Errorf("enabling repository memory: %w", err)
		}
		if err := EnsureWorkspacePluginConfigured(repoRoot); err != nil {
			return fmt.Errorf("configuring workspace plugin: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Enabled repository memory for %s\n", repoRoot)
		return nil
	}

	dataDir := config.DataDir()
	if err := config.SetMachineEnabled(dataDir, true); err != nil {
		return fmt.Errorf("enabling machine memory: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Enabled repository memory globally")
	return nil
}
