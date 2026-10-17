/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_usage.go
 * Description: Implements the `repo-memory usage` CLI command, querying model
 *              quota buckets and displaying remaining capacity and reset windows.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/spf13/cobra"
)

// usageJSON controls whether `repo-memory usage` outputs JSON instead of a table.
var usageJSON bool

// usageCmd defines the `repo-memory usage` subcommand.
var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Check available model quota and remaining capacity",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Query model quota status ---
		status, err := telemetry.CheckQuota(cmd.Context())
		if err != nil {
			return fmt.Errorf("checking quota: %w", err)
		}

		// --- Step 2: Emit JSON when --json is enabled ---
		if usageJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(status)
		}

		// --- Step 3: Format quota buckets and reset countdowns as a table ---
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MODEL / BUCKET\tREMAINING\tRESETS IN\tRESET TIME")
		fmt.Fprintln(w, "--------------\t---------\t---------\t----------")

		now := time.Now()
		for _, b := range status.Buckets {
			pct := fmt.Sprintf("%.1f%%", b.RemainingFraction*100)
			resetsIn := "-"
			resetStr := "-"
			if b.ResetTime != nil {
				resetStr = b.ResetTime.Local().Format(time.RFC3339)
				dur := b.ResetTime.Sub(now)
				if dur > 0 {
					hours := int(dur.Hours())
					mins := int(dur.Minutes()) % 60
					resetsIn = fmt.Sprintf("%dh %dm", hours, mins)
				} else {
					resetsIn = "resetting soon"
				}
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.Name, pct, resetsIn, resetStr)
		}

		return w.Flush()
	},
}

func init() {
	usageCmd.Flags().BoolVar(&usageJSON, "json", false, "Output quota status as JSON")
}
