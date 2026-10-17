/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_tick.go
 * Description: Implements the `repo-memory tick` (alias `run`) CLI command,
 *              advancing asynchronous background jobs with harness session
 *              attribution, transcript copying, and housekeeping.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

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

// tickCmd defines the `repo-memory tick` subcommand for advancing background jobs.
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
			tick.WithHousekeeper(antigravity.NewHousekeeper(config.DataDir(), "default", "", "")),
		)
		return ticker.Run(context.Background())
	},
}
