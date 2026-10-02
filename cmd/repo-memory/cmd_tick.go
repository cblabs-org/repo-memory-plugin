package main

import (
	"context"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/antigravity"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness/claudecode"
	"github.com/spf13/cobra"
)

func init() {
	tick.RegisterAttributor(antigravity.NewSessionAttributor())
}

var tickCmd = &cobra.Command{
	Use:     "tick",
	Aliases: []string{"run"},
	Short:   "Advance asynchronous background jobs (time path)",
	Args:    cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ticker := tick.NewTicker(
			config.DataDir(),
			55*time.Second, // 1 minute tick minus 5s buffer
			tick.WithAttributor(antigravity.NewSessionAttributor()),
			tick.WithCopier(claudecode.CopyTranscripts),
		)
		return ticker.Run(context.Background())
	},
}
