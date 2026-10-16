package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/seed"
)

var (
	opsGCRepoDir     string
	opsGCOpsDir      string
	opsGCDossiersDir string
	opsGCConfigPath  string
	opsGCPruneIDs    []string
	opsGCDryRun      bool
)

var opsCmd = &cobra.Command{
	Use:   "ops",
	Short: "Manage repository memory operation logs",
}

var opsGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Prune junk/stale ops, sanitize anchors, repair scopes, and optionally re-render dossiers",
	RunE: func(cmd *cobra.Command, args []string) error {
		repoRoot, _ := gitops.FindRepoRoot(opsGCRepoDir)
		resolvePath := func(p string, flagChanged bool) string {
			if p == "" || filepath.IsAbs(p) {
				return p
			}
			if repoRoot != "" {
				inRepo := filepath.Join(repoRoot, p)
				if !flagChanged {
					return inRepo
				}
				if _, err := os.Stat(inRepo); err == nil {
					return inRepo
				}
			}
			if _, err := os.Stat(p); err == nil {
				if abs, err := filepath.Abs(p); err == nil {
					return abs
				}
				return p
			}
			if _, err := os.Stat(filepath.Dir(filepath.Clean(p))); err == nil {
				if abs, err := filepath.Abs(p); err == nil {
					return abs
				}
				return p
			}
			if repoRoot != "" {
				return filepath.Join(repoRoot, p)
			}
			return p
		}

		opts := seed.GCOptions{
			OpsDir:      resolvePath(opsGCOpsDir, cmd.Flags().Changed("ops-dir")),
			DossiersDir: resolvePath(opsGCDossiersDir, cmd.Flags().Changed("dossiers-dir")),
			ConfigPath:  resolvePath(opsGCConfigPath, cmd.Flags().Changed("config")),
			PruneIDs:    opsGCPruneIDs,
			DryRun:      opsGCDryRun,
		}

		stats, err := seed.RunOpsGC(context.Background(), opts)
		if err != nil {
			return err
		}

		prefix := ""
		if opts.DryRun {
			prefix = "[DRY RUN] "
		}
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"%sops gc complete: scanned=%d modified=%d deleted=%d ops_removed=%d anchors_removed=%d scopes_repaired=%d\n",
			prefix,
			stats.BatchesScanned,
			stats.BatchesModified,
			stats.BatchesDeleted,
			stats.OpsRemoved,
			stats.AnchorsRemoved,
			stats.ScopesRepaired,
		)
		return nil
	},
}

func init() {
	opsGCCmd.Flags().StringVar(&opsGCRepoDir, "repo", ".", "Path to the target repository root")
	opsGCCmd.Flags().StringVar(&opsGCOpsDir, "ops-dir", ".memory/ops", "Path to the ops directory")
	opsGCCmd.Flags().StringVar(&opsGCDossiersDir, "dossiers-dir", "", "Optional directory in which to re-render dossiers/ and ATLAS.md")
	opsGCCmd.Flags().StringVar(&opsGCConfigPath, "config", ".memory/config.yaml", "Path to config.yaml")
	opsGCCmd.Flags().StringSliceVar(&opsGCPruneIDs, "prune-id", nil, "Additional entry IDs to prune")
	opsGCCmd.Flags().BoolVar(&opsGCDryRun, "dry-run", false, "Report changes without modifying files")
	opsCmd.AddCommand(opsGCCmd)
}
