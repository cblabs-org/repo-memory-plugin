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

	if failedCount > 0 {
		return fmt.Errorf("snapshot failed for %d of %d conversation(s)", failedCount, len(args))
	}

	return nil
}

func resolveBrainDir(convID string) (brainDir string, actualConvID string, err error) {
	// If convID is a direct directory path on disk
	if fi, statErr := os.Stat(convID); statErr == nil && fi.IsDir() {
		clean := filepath.Clean(convID)
		return clean, filepath.Base(clean), nil
	}

	// Environment variable override
	if envDir := os.Getenv("ANTIGRAVITY_BRAIN_DIR"); envDir != "" {
		cand := filepath.Join(envDir, convID)
		if fi, statErr := os.Stat(cand); statErr == nil && fi.IsDir() {
			return cand, convID, nil
		}
	}

	// User home standard locations
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
