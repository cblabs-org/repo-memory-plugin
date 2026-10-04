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

var usageJSON bool

var usageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Check available model quota and remaining capacity",
	RunE: func(cmd *cobra.Command, args []string) error {
		status, err := telemetry.CheckQuota(cmd.Context())
		if err != nil {
			return fmt.Errorf("checking quota: %w", err)
		}

		if usageJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(status)
		}

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
