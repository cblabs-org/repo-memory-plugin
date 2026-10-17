/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/sweep.go
 * Description: Session discovery and sweeping across enrolled repositories. Attributes
 *              idle agent sessions, incrementally copies changed transcripts and
 *              conversation artifacts into the local spool queue, and enqueues capture jobs.
 *
 * Exported Types / Functions:
 * - AttributedSession:   Represents an agent session attributed to an enrolled repository.
 * - SessionAttributor:   Interface for discovering and attributing sessions to enrolled repositories.
 * - TranscriptCopier:    Callback signature for copying raw harness transcripts across enrolled repositories.
 * - RegisterAttributor(): Registers a default SessionAttributor for Sweep.
 * - DefaultAttributors(): Returns a copy of registered default SessionAttributors.
 * - RegisterCopier():     Registers a default TranscriptCopier for Sweep.
 * - DefaultCopiers():     Returns a copy of registered default TranscriptCopiers.
 * - Sweep():              Scans idle sessions across enrolled repositories and enqueues capture jobs.
 */

package tick

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

// AttributedSession represents an agent session attributed to an enrolled repository.
type AttributedSession struct {
	ConversationID       string    `json:"conversation_id"`                  // Unique conversation/session identifier.
	ParentConversationID string    `json:"parent_conversation_id,omitempty"` // Parent conversation ID if spawned as a subagent.
	RepoID               string    `json:"repo_id"`                          // Registry slug of the enrolled repository matched via workspace URI.
	RepoRoot             string    `json:"repo_root"`                        // Absolute filesystem path to the repository root.
	TranscriptPath       string    `json:"transcript_path"`                  // Absolute path to the raw transcript file or directory.
	LastModified         time.Time `json:"last_modified"`                    // Last modification timestamp of the transcript.
	Idle                 bool      `json:"idle"`                             // Whether the session has been idle past the sweep threshold.
	Source               string    `json:"source,omitempty"`                 // Attribution source label (e.g. "pb", "jsonl").
	IsHousekeeping       bool      `json:"is_housekeeping,omitempty"`        // True when the session belongs to a background housekeeper run.
}

// SessionAttributor discovers and attributes sessions to enrolled repositories.
type SessionAttributor interface {
	// AttributedSessions returns all discovered sessions mapped to enrolled repositories.
	AttributedSessions(ctx context.Context, enrolledRepos map[string]config.RegistryEntry) ([]AttributedSession, error)
}

var (
	defaultAttributorsMu sync.RWMutex        // Guards defaultAttributors slice.
	defaultAttributors   []SessionAttributor // Registered default session attributors.

	defaultCopiersMu sync.RWMutex       // Guards defaultCopiers slice.
	defaultCopiers   []TranscriptCopier // Registered default transcript copiers.
)

// TranscriptCopier defines a function type for copying raw harness transcripts across enrolled repositories during sweep.
type TranscriptCopier func(ctx context.Context, memoryDataDir string, enrolledRepos map[string]config.RegistryEntry) error

// RegisterAttributor adds a default session attributor used by Sweep when no explicit attributors are provided.
func RegisterAttributor(a SessionAttributor) {
	if a == nil {
		return
	}
	defaultAttributorsMu.Lock()
	defer defaultAttributorsMu.Unlock()
	defaultAttributors = append(defaultAttributors, a)
}

// DefaultAttributors returns a copy of registered default attributors.
func DefaultAttributors() []SessionAttributor {
	defaultAttributorsMu.RLock()
	defer defaultAttributorsMu.RUnlock()
	copied := make([]SessionAttributor, len(defaultAttributors))
	copy(copied, defaultAttributors)
	return copied
}

// RegisterCopier adds a default transcript copier used by Sweep when no explicit copiers are provided.
func RegisterCopier(c TranscriptCopier) {
	if c == nil {
		return
	}
	defaultCopiersMu.Lock()
	defer defaultCopiersMu.Unlock()
	defaultCopiers = append(defaultCopiers, c)
}

// DefaultCopiers returns a copy of registered default copiers.
func DefaultCopiers() []TranscriptCopier {
	defaultCopiersMu.RLock()
	defer defaultCopiersMu.RUnlock()
	copied := make([]TranscriptCopier, len(defaultCopiers))
	copy(copied, defaultCopiers)
	return copied
}

// Sweep scans for idle sessions across enrolled repositories using registered or provided attributors,
// incrementally copies changed transcripts into queue/, enqueues capture jobs, and runs transcript copiers.
func Sweep(ctx context.Context, memoryDataDir string, enrolledRepos map[string]config.RegistryEntry, attributors ...SessionAttributor) error {
	// --- Step 1: Execute registered transcript copiers ---
	copiers := DefaultCopiers()
	for _, copier := range copiers {
		if err := copier(ctx, memoryDataDir, enrolledRepos); err != nil {
			fmt.Fprintf(os.Stderr, "transcript copier error: %v\n", err)
		}
	}

	// --- Step 2: Resolve session attributors to run ---
	if len(attributors) == 0 {
		attributors = DefaultAttributors()
	}
	if len(attributors) == 0 && len(copiers) == 0 {
		return nil
	}

	// --- Step 3: Attribute idle sessions, copy transcripts/artifacts, and enqueue capture jobs ---
	for _, attr := range attributors {
		sessions, err := attr.AttributedSessions(ctx, enrolledRepos)
		if err != nil {
			fmt.Fprintf(os.Stderr, "attributing sessions error: %v\n", err)
			continue
		}

		for _, s := range sessions {
			if !s.Idle {
				continue
			}
			if s.TranscriptPath == "" {
				continue
			}
			if s.IsHousekeeping {
				continue
			}

			repoDataDir := filepath.Join(memoryDataDir, s.RepoID)
			queueDir := filepath.Join(repoDataDir, "queue")
			stateDir := filepath.Join(repoDataDir, "state")
			if err := os.MkdirAll(queueDir, 0755); err != nil {
				continue
			}
			if err := os.MkdirAll(stateDir, 0755); err != nil {
				continue
			}

			res, err := spool.CopyTranscriptIncremental(s.ConversationID, s.TranscriptPath, queueDir, stateDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "incremental copy err for %s: %v\n", s.ConversationID, err)
				continue
			}

			// Copy artifacts from the brain directory
			brainDir := deriveBrainDir(s.ConversationID, s.TranscriptPath)
			if brainDir != "" {
				artDst := evidence.ArtifactDir(memoryDataDir, s.RepoID, s.ConversationID)
				if _, err := evidence.CopyArtifacts(brainDir, artDst); err != nil {
					fmt.Fprintf(os.Stderr, "copying artifacts err for %s: %v\n", s.ConversationID, err)
				}
			}

			// Enqueue capture job
			store, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
			if err == nil {
				payloadMap := map[string]any{
					"conversation_id":        s.ConversationID,
					"parent_conversation_id": s.ParentConversationID,
					"repo_id":                s.RepoID,
					"repo_root":              s.RepoRoot,
					"transcript_path":        s.TranscriptPath,
					"source":                 s.Source,
				}
				payloadBytes, _ := json.Marshal(payloadMap)

				_, _ = store.Enqueue(ctx, &jobs.Job{
					Kind:           jobs.KindCapture,
					IdempotencyKey: fmt.Sprintf("sweep-%s-%d", s.ConversationID, res.TotalBytes),
					Priority:       10,
					Payload:        string(payloadBytes),
				})
				_ = store.Close()
			}
		}
	}

	return nil
}

// deriveBrainDir locates the conversation's brain artifact directory from transcriptPath
// or standard Antigravity user-home fallback directories.
func deriveBrainDir(convID, transcriptPath string) string {
	// --- Step 1: Walk upward from transcriptPath to locate a directory matching convID ---
	if transcriptPath != "" {
		if fi, err := os.Stat(transcriptPath); err == nil && fi.IsDir() {
			return transcriptPath
		}
		// If transcriptPath is a file, walk up parent directories to find directory named convID
		dir := filepath.Dir(transcriptPath)
		for dir != "" && dir != "." && dir != string(filepath.Separator) {
			if filepath.Base(dir) == convID {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	// --- Step 2: Check ANTIGRAVITY_BRAIN_DIR environment override ---
	if envDir := os.Getenv("ANTIGRAVITY_BRAIN_DIR"); envDir != "" {
		cand := filepath.Join(envDir, convID)
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			return cand
		}
	}

	// --- Step 3: Probe standard home directory brain paths ---
	home, err := os.UserHomeDir()
	if err == nil {
		candidates := []string{
			filepath.Join(home, ".gemini", "antigravity", "brain", convID),
			filepath.Join(home, ".gemini", "antigravity-cli", "brain", convID),
		}
		for _, cand := range candidates {
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				return cand
			}
		}
	}

	return ""
}
