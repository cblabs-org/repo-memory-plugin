package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "repo-memory",
	Short:   "Repository-level memory for coding agents",
	Version: Version,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(hookCmd)
	rootCmd.AddCommand(enrollCmd)
	rootCmd.AddCommand(unenrollCmd)
	rootCmd.AddCommand(enableCmd)
	rootCmd.AddCommand(disableCmd)
	rootCmd.AddCommand(tickCmd)
	rootCmd.AddCommand(mcpCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(searchCmd)
	rootCmd.AddCommand(reportCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(reindexCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(importPACmd)
	rootCmd.AddCommand(evalCmd)
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(usageCmd)
	rootCmd.AddCommand(seedCmd)
	rootCmd.AddCommand(seedDocsCmd)
	rootCmd.AddCommand(opsCmd)
	rootCmd.AddCommand(consolidateCmd)
	rootCmd.AddCommand(promoteSkillsCmd)
}

