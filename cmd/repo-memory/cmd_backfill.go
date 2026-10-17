/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_backfill.go
 * Description: CLI subcommand for backfilling repository memory from historical
 *              transcripts, Principal Architect workspace artifacts, repository
 *              documentation, and git revert commits.
 *
 * Exported Types / Functions:
 * - (none — package main CLI command registration)
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/tick"
	"github.com/spf13/cobra"
)

var (
	backfillRepo        string // Target repository root path (--repo).
	backfillDataDir     string // Optional data directory override (--data-dir).
	backfillDryRun      bool   // Whether to preview backfill actions without writing (--dry-run).
	backfillJSON        bool   // Whether to emit structured JSON output (--json).
	backfillForce       bool   // Whether to bypass A5 spend guard interval/quota limits (--force).
	backfillTranscripts bool   // Whether to backfill PA workspace artifacts and historical transcripts (--transcripts).
	backfillDocs        bool   // Whether to backfill repository docs via incremental doc seeding (--docs).
	backfillGit         bool   // Whether to backfill refutation/gotcha notes from git revert commits (--git).
	backfillMaxItems    int    // Maximum number of items to process (--max-items; 0 uses spend guard default).
)

// backfillCmd defines the `repo-memory backfill` subcommand.
var backfillCmd = &cobra.Command{
	Use:     "backfill [repo-dir]",
	Aliases: []string{"backfill-captures"},
	Short:   "Backfill repository memory from historical transcripts/PA artifacts, docs, and git reverts",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// --- Step 1: Resolve execution context, data directory, and repository root ---
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}

		dataDir := strings.TrimSpace(backfillDataDir)
		if dataDir == "" {
			dataDir = config.DataDir()
		}

		targetRepo := strings.TrimSpace(backfillRepo)
		if targetRepo == "" && len(args) > 0 {
			targetRepo = strings.TrimSpace(args[0])
		}

		repoRoot := ""
		if targetRepo != "" {
			if gr, err := gitops.FindRepoRoot(targetRepo); err == nil && gr != "" {
				repoRoot = gr
			} else if abs, absErr := filepath.Abs(targetRepo); absErr == nil {
				repoRoot = abs
			}
		} else if cwd, err := os.Getwd(); err == nil {
			if gr, err := gitops.FindRepoRoot(cwd); err == nil && gr != "" {
				repoRoot = gr
			}
		}

		// --- Step 2: Resolve repository ID and execute backfill ---
		repoID := ""
		if repoRoot != "" {
			if entry, found, _ := config.FindRegistryEntry(dataDir, repoRoot); found && entry != nil && entry.RepoID != "" {
				repoID = entry.RepoID
			}
			if repoID == "" {
				id, err := gitops.GenerateRepoID(repoRoot)
				if err != nil || id == "" {
					id = filepath.Base(repoRoot)
				}
				repoID = id
			}
		}

		var res *tick.BackfillResult
		if repoRoot != "" {
			var err error
			res, err = tick.RunBackfill(ctx, tick.BackfillOptions{
				RepoRoot:           repoRoot,
				DataDir:            dataDir,
				RepoID:             repoID,
				DryRun:             backfillDryRun,
				Force:              backfillForce,
				IncludeTranscripts: backfillTranscripts,
				IncludeDocs:        backfillDocs,
				IncludeGit:         backfillGit,
				MaxItems:           backfillMaxItems,
			})
			if err != nil {
				return err
			}
		} else {
			res = &tick.BackfillResult{
				DryRun:            backfillDryRun,
				SpendGuardAllowed: true,
			}
		}

		// --- Step 3: Requeue capture jobs across registered repositories when invoked globally ---
		// Fallback / supplementary support for registered repos when invoked without --repo
		// or via the legacy backfill-captures alias.
		if targetRepo == "" && (cmd.CalledAs() == "backfill-captures" || repoRoot == "") {
			if reg, err := config.LoadRegistry(dataDir); err == nil {
				for _, entry := range reg {
					if entry.RepoID == "" || entry.RepoID == repoID {
						continue
					}
					dbPaths := []string{
						filepath.Join(dataDir, entry.RepoID, "jobs.sqlite"),
						filepath.Join(dataDir, "repos", entry.RepoID, "jobs.sqlite"),
					}
					for _, dbPath := range dbPaths {
						if _, err := os.Stat(dbPath); err != nil {
							continue
						}
						store, err := jobs.Open(dbPath)
						if err != nil {
							continue
						}
						if backfillDryRun {
							if list, err := store.List(ctx, jobs.Filter{Kind: jobs.KindCapture}); err == nil {
								for _, j := range list {
									if j.State == jobs.StateDone || j.State == jobs.StateDead || j.State == jobs.StateFailed {
										res.CapturesRequeued++
									}
								}
							}
						} else {
							if n, err := store.RequeueKind(ctx, jobs.KindCapture); err == nil {
								res.CapturesRequeued += n
							}
						}
						_ = store.Close()
					}
				}
			}
		}

		// --- Step 4: Format and write backfill output ---
		if backfillJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(res)
		}

		out := cmd.OutOrStdout()
		if !res.SpendGuardAllowed && !backfillForce && !backfillDryRun {
			fmt.Fprintf(out, "Backfill skipped (spend guard: %s)\n", res.SpendGuardReason)
			return nil
		}
		if cmd.CalledAs() == "backfill-captures" || res.CapturesRequeued > 0 {
			fmt.Fprintf(out, "Requeued %d captures\n", res.CapturesRequeued)
		}
		fmt.Fprintf(out, "Backfill complete (dry_run=%v, items=%d/%d): pa_imported=%d, episodes_queued=%d, docs_seeded=%d (entries_mutated=%d), git_notes=%d (reverts_scanned=%d)\n",
			res.DryRun,
			res.ItemsProcessed,
			res.EffectiveMaxItems,
			res.PAFilesImported,
			res.EpisodesQueued,
			res.DocsSeeded,
			res.DocEntriesMutated,
			res.GitNotesCreated,
			res.GitRevertsScanned,
		)
		return nil
	},
}

func init() {
	backfillCmd.Flags().StringVar(&backfillRepo, "repo", "", "Path to repository root")
	backfillCmd.Flags().StringVar(&backfillDataDir, "data-dir", "", "Override data directory")
	backfillCmd.Flags().BoolVar(&backfillDryRun, "dry-run", false, "Preview backfill actions without mutating disk")
	backfillCmd.Flags().BoolVar(&backfillJSON, "json", false, "Output JSON result")
	backfillCmd.Flags().BoolVar(&backfillForce, "force", false, "Bypass A5 spend guard interval/quota limits")
	backfillCmd.Flags().BoolVar(&backfillTranscripts, "transcripts", false, "Backfill PA workspace artifacts and historical transcripts/episodes")
	backfillCmd.Flags().BoolVar(&backfillDocs, "docs", false, "Backfill repository docs via incremental doc seeding")
	backfillCmd.Flags().BoolVar(&backfillGit, "git", false, "Backfill refutation/gotcha notes from git revert commits")
	backfillCmd.Flags().IntVar(&backfillMaxItems, "max-items", 0, "Maximum number of items to process (0 uses spend guard default)")
}
