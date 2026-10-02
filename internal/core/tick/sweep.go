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
}

// SessionAttributor discovers and attributes sessions to enrolled repositories.
type SessionAttributor interface {
	AttributedSessions(ctx context.Context, enrolledRepos map[string]config.RegistryEntry) ([]AttributedSession, error)
}

var (
	defaultAttributorsMu sync.RWMutex
	defaultAttributors   []SessionAttributor
)

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

// Sweep scans for idle sessions across enrolled repositories using registered or provided attributors,
// incrementally copies changed transcripts into queue/, and enqueues capture jobs.
func Sweep(ctx context.Context, memoryDataDir string, enrolledRepos map[string]config.RegistryEntry, attributors ...SessionAttributor) error {
	if len(attributors) == 0 {
		attributors = DefaultAttributors()
	}
	if len(attributors) == 0 {
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

			repoDataDir := filepath.Join(memoryDataDir, s.RepoID)
			queueDir := filepath.Join(repoDataDir, "queue")
			stateDir := filepath.Join(repoDataDir, "state")
			if err := os.MkdirAll(queueDir, 0755); err != nil {
				continue
			}
			if err := os.MkdirAll(stateDir, 0755); err != nil {
				continue
			}

			_, err = spool.CopyTranscriptIncremental(s.ConversationID, s.TranscriptPath, queueDir, stateDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "incremental copy err for %s: %v\n", s.ConversationID, err)
				continue
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
					IdempotencyKey: "sweep-" + s.ConversationID,
					Priority:       10,
					Payload:        string(payloadBytes),
				})
				_ = store.Close()
			}
		}
	}

	return nil
}
