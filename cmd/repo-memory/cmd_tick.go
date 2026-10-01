package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tickCmd = &cobra.Command{
	Use:     "tick",
	Aliases: []string{"run"},
	Short:   "Advance asynchronous background jobs (time path)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("tick subcommand stub")
		return nil
	},
}
