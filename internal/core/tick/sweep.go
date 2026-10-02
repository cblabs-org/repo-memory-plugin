package tick

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

func Sweep(ctx context.Context, memoryDataDir string, enrolledRepos map[string]config.RegistryEntry) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	appBrainDir := filepath.Join(home, ".gemini", "antigravity", "brain")
	cliBrainDir := filepath.Join(home, ".gemini", "antigravity-cli", "brain")
	// TODO: raw claude code

	if err := sweepDir(ctx, appBrainDir, memoryDataDir, enrolledRepos); err != nil {
		fmt.Fprintf(os.Stderr, "sweep app dir err: %v\n", err)
	}
	if err := sweepDir(ctx, cliBrainDir, memoryDataDir, enrolledRepos); err != nil {
		fmt.Fprintf(os.Stderr, "sweep cli dir err: %v\n", err)
	}

	return nil
}

func sweepDir(ctx context.Context, brainDir, memoryDataDir string, enrolledRepos map[string]config.RegistryEntry) error {
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		convID := entry.Name()
		if convID == "scratch" { // Not a brain
			continue
		}

		convDir := filepath.Join(brainDir, convID)
		
		// check if idle for 15 mins
		info, err := os.Stat(convDir)
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) < 15*time.Minute {
			continue
		}

		// determine workspace by looking at the registry mappings
		// for now, we just find the workspace by parsing the system prompt in the transcript
		workspace, err := extractWorkspaceFromTranscript(convDir)
		if err != nil || workspace == "" {
			continue
		}
		
		var matchingEntry config.RegistryEntry
		var found bool
		for root, en := range enrolledRepos {
			if strings.HasPrefix(workspace, root) {
				matchingEntry = en
				found = true
				break
			}
		}
		
		if !found {
			continue
		}

		repoDataDir := filepath.Join(memoryDataDir, matchingEntry.RepoID)
		queueDir := filepath.Join(repoDataDir, "episodes", "queue")
		stateDir := filepath.Join(repoDataDir, "state")
		os.MkdirAll(queueDir, 0755)
		os.MkdirAll(stateDir, 0755)

		_, err = spool.CopyTranscriptIncremental(convID, convDir, queueDir, stateDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "incremental copy err for %s: %v\n", convID, err)
			continue
		}

		// Enqueue capture job
		store, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
		if err == nil {
			store.Enqueue(ctx, &jobs.Job{
				Kind:           "capture",
				IdempotencyKey: "sweep-" + convID,
				Priority:       10,
			})
			store.Close()
		}
	}
	return nil
}

func extractWorkspaceFromTranscript(convDir string) (string, error) {
	// this is a stub logic to extract workspace
	transcriptFile := filepath.Join(convDir, ".system_generated", "logs", "transcript.jsonl")
	data, err := os.ReadFile(transcriptFile)
	if err != nil {
		return "", err
	}

	content := string(data)
	idx := strings.Index(content, "->")
	if idx > 0 {
		start := strings.LastIndex(content[:idx], "\n")
		if start == -1 { start = 0 }
		line := content[start:idx]
		line = strings.TrimSpace(line)
		return line, nil
	}
	
	return "", nil
}
