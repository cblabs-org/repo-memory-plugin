package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

var hookWorkspaceFlag string

var hookCmd = &cobra.Command{
	Use:   "hook <harness> <event>",
	Short: "Execute harness hook event path",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		valid, err := harness.FastPathValidation(hookWorkspaceFlag, "")
		if err != nil || !valid {
			// Sub-5ms fast-path: immediately return harness no-op output
			fmt.Fprintln(cmd.OutOrStdout(), "{}")
			return nil
		}

		if len(args) >= 2 {
			fmt.Fprintf(cmd.OutOrStdout(), "hook subcommand stub: harness=%s event=%s\n", args[0], args[1])
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "hook subcommand stub")
		}
		return nil
	},
}

func init() {
	hookCmd.Flags().StringVar(&hookWorkspaceFlag, "workspace", "", "Workspace directory override for hook evaluation")
}
