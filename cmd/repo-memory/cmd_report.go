package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate or view memory metrics and status report",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("report subcommand stub")
		return nil
	},
}
