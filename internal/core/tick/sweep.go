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
	ConversationID       string    `json:"conversation_id"`
	ParentConversationID string    `json:"parent_conversation_id,omitempty"`
	RepoID               string    `json:"repo_id"`
	RepoRoot             string    `json:"repo_root"`
	TranscriptPath       string    `json:"transcript_path"`
	LastModified         time.Time `json:"last_modified"`
	Idle                 bool      `json:"idle"`
	Source               string    `json:"source,omitempty"`
	IsHousekeeping       bool      `json:"is_housekeeping,omitempty"`
}

// SessionAttributor discovers and attributes sessions to enrolled repositories.
type SessionAttributor interface {
	AttributedSessions(ctx context.Context, enrolledRepos map[string]config.RegistryEntry) ([]AttributedSession, error)
}

var (
	defaultAttributorsMu sync.RWMutex
	defaultAttributors   []SessionAttributor

	defaultCopiersMu     sync.RWMutex
	defaultCopiers       []TranscriptCopier
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
	copiers := DefaultCopiers()
	for _, copier := range copiers {
		if err := copier(ctx, memoryDataDir, enrolledRepos); err != nil {
			fmt.Fprintf(os.Stderr, "transcript copier error: %v\n", err)
		}
	}

	if len(attributors) == 0 {
		attributors = DefaultAttributors()
	}
	if len(attributors) == 0 && len(copiers) == 0 {
		return nil
	}

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

func deriveBrainDir(convID, transcriptPath string) string {
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

	// Fallback to environment variable or standard user home brain directories
	if envDir := os.Getenv("ANTIGRAVITY_BRAIN_DIR"); envDir != "" {
		cand := filepath.Join(envDir, convID)
		if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
			return cand
		}
	}

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

