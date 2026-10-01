package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "repo-memory",
	Short: "Repository-level memory for coding agents",
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	commands := []struct {
		use     string
		aliases []string
		short   string
	}{
		{use: "hook", short: "Execute harness hook event path"},
		{use: "tick", aliases: []string{"run"}, short: "Advance asynchronous background jobs (time path)"},
		{use: "mcp", short: "Stdio MCP server for memory pull tools"},
		{use: "install", short: "Install repo-memory plugin and harness configurations"},
		{use: "uninstall", short: "Uninstall repo-memory plugin and harness configurations"},
		{use: "search", short: "Search captured episodes, claims, and memory"},
		{use: "report", short: "Generate or view memory metrics and status report"},
		{use: "status", short: "Display current repository memory status"},
		{use: "reindex", short: "Rebuild repository index"},
		{use: "doctor", short: "Verify repository memory installation and health"},
	}

	for _, c := range commands {
		cmdName := c.use
		cmd := &cobra.Command{
			Use:     c.use,
			Aliases: c.aliases,
			Short:   c.short,
			Args:    cobra.ArbitraryArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Printf("%s subcommand stub\n", cmdName)
				return nil
			},
		}
		rootCmd.AddCommand(cmd)
	}
}
