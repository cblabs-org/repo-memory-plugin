/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: cmd/repo-memory/cmd_snapshot.go
 * Description: Implements the `repo-memory snapshot` CLI command, copying
 *              conversation transcripts and artifacts from the agent brain
 *              directory into the enrolled repository's private evidence store.
 *
 * Exported Types / Functions:
 * - RunSnapshot(): Copies transcripts and artifacts for conversation IDs into the evidence store.
 */

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/spf13/cobra"
)

// snapshotCmd defines the `repo-memory snapshot` subcommand.
var snapshotCmd = &cobra.Command{
	Use:   "snapshot <conversation-id> [conversation-id...]",
	Short: "Snapshot conversation transcripts and artifacts into the evidence store",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunSnapshot(cmd, args)
	},
}

// RunSnapshot copies transcripts and artifacts for the specified conversation IDs into the evidence store.
func RunSnapshot(cmd *cobra.Command, args []string) error {
	// --- Step 1: Verify enrolled repository and initialize data directory layout ---
	repoRoot, err := gitops.FindRepoRoot(".")
	if err != nil {
		return fmt.Errorf("finding repository root: %w", err)
	}

	dataDir := config.DataDir()
	entry, found, err := config.FindRegistryEntry(dataDir, repoRoot)
	if err != nil {
		return fmt.Errorf("checking registry: %w", err)
	}
	if !found || entry == nil {
		return fmt.Errorf("repository %s is not enrolled in repo-memory", repoRoot)
	}

	repoID := entry.RepoID
	if repoID == "" {
		repoID, err = gitops.GenerateRepoID(entry.RootPath)
		if err != nil {
			return fmt.Errorf("generating repo id: %w", err)
		}
	}

	if err := config.EnsureDataDirLayout(dataDir, repoID); err != nil {
		return fmt.Errorf("ensuring data dir layout: %w", err)
	}

	queueDir := filepath.Join(dataDir, repoID, "queue")
	stateDir := filepath.Join(dataDir, repoID, "state")
	spoolDir := filepath.Join(dataDir, repoID, "spool")

	branch, _ := gitops.GetCurrentBranch(repoRoot)
	head, _ := gitops.GetHeadCommit(repoRoot)

	// --- Step 2: Copy transcripts, record spool entries, and copy artifacts per conversation ---
	var failedCount int
	for _, rawArg := range args {
		brainDir, convID, rErr := resolveBrainDir(rawArg)
		if rErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error resolving conversation %s: %v\n", rawArg, rErr)
			failedCount++
			continue
		}

		// 1. Copy transcript using spool incremental copy
		copyResult, tErr := spool.CopyTranscriptIncremental(convID, brainDir, queueDir, stateDir)
		if tErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error copying transcript for %s: %v\n", convID, tErr)
			failedCount++
			continue
		}

		var bytesCopied int64
		if copyResult != nil {
			bytesCopied = copyResult.BytesCopied
		}

		// 2. Record capture entry in spool
		captureEntry := spool.NewCaptureEntry("antigravity", convID, brainDir, repoRoot, branch, head)
		_, _ = spool.Write(spoolDir, captureEntry)

		// 3. Copy artifacts into evidence store
		artDst := evidence.ArtifactDir(dataDir, repoID, convID)
		artifactsCopied, aErr := evidence.CopyArtifacts(brainDir, artDst)
		if aErr != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "error copying artifacts for %s: %v\n", convID, aErr)
			failedCount++
			continue
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Snapshotted %s: transcript (%d bytes), %d artifact(s) copied\n",
			convID, bytesCopied, artifactsCopied)
	}

	// --- Step 3: Report aggregate failure status if any conversation failed ---
	if failedCount > 0 {
		return fmt.Errorf("snapshot failed for %d of %d conversation(s)", failedCount, len(args))
	}

	return nil
}

// resolveBrainDir locates the on-disk brain directory and canonical conversation ID
// for a direct path or conversation UUID across environment and home candidates.
func resolveBrainDir(convID string) (brainDir string, actualConvID string, err error) {
	// --- Step 1: Check whether convID is already an existing directory path ---
	if fi, statErr := os.Stat(convID); statErr == nil && fi.IsDir() {
		clean := filepath.Clean(convID)
		return clean, filepath.Base(clean), nil
	}

	// --- Step 2: Check ANTIGRAVITY_BRAIN_DIR environment variable override ---
	if envDir := os.Getenv("ANTIGRAVITY_BRAIN_DIR"); envDir != "" {
		cand := filepath.Join(envDir, convID)
		if fi, statErr := os.Stat(cand); statErr == nil && fi.IsDir() {
			return cand, convID, nil
		}
	}

	// --- Step 3: Probe standard Antigravity brain directories under user home ---
	home, homeErr := os.UserHomeDir()
	if homeErr == nil {
		candidates := []string{
			filepath.Join(home, ".gemini", "antigravity", "brain", convID),
			filepath.Join(home, ".gemini", "antigravity-cli", "brain", convID),
		}
		for _, cand := range candidates {
			if fi, statErr := os.Stat(cand); statErr == nil && fi.IsDir() {
				return cand, convID, nil
			}
		}
	}

	return "", convID, fmt.Errorf("brain directory not found for %s", convID)
}
