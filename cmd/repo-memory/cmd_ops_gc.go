/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_ops_gc.go
 * Description: CLI subcommands (`repo-memory ops` and `repo-memory ops gc`) for
 *              pruning junk or stale operation log entries, sanitizing anchors,
 *              repairing scopes, and optionally re-rendering dossiers.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

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
	opsGCRepoDir     string   // Target repository root directory (--repo).
	opsGCOpsDir      string   // Path to the .memory/ops directory (--ops-dir).
	opsGCDossiersDir string   // Optional directory in which to re-render dossiers and ATLAS.md (--dossiers-dir).
	opsGCConfigPath  string   // Path to .memory/config.yaml (--config).
	opsGCPruneIDs    []string // Additional entry IDs to prune explicitly (--prune-id).
	opsGCDryRun      bool     // Whether to report GC changes without modifying files (--dry-run).
)

// opsCmd defines the parent `repo-memory ops` command group.
var opsCmd = &cobra.Command{
	Use:   "ops",
	Short: "Manage repository memory operation logs",
}

// opsGCCmd defines the `repo-memory ops gc` subcommand.
var opsGCCmd = &cobra.Command{
	Use:   "gc",
	Short: "Prune junk/stale ops, sanitize anchors, repair scopes, and optionally re-render dossiers",
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve repository root and relative path helper ---
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

		// --- Step 2: Build GC options and execute operation log garbage collection ---
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

		// --- Step 3: Format and print GC statistics ---
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
