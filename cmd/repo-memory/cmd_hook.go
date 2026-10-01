package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var hookCmd = &cobra.Command{
	Use:   "hook <harness> <event>",
	Short: "Execute harness hook event path",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) >= 2 {
			fmt.Printf("hook subcommand stub: harness=%s event=%s\n", args[0], args[1])
		} else {
			fmt.Println("hook subcommand stub")
		}
		return nil
	},
}
