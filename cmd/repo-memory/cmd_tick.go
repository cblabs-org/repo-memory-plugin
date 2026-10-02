package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
)

var tickCmd = &cobra.Command{
	Use:     "tick",
	Aliases: []string{"run"},
	Short:   "Advance asynchronous background jobs (time path)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ticker := tick.NewTicker(config.DataDir(), 55*time.Second) // 1 minute tick minus 5s buffer
		return ticker.Run(context.Background())
	},
}
