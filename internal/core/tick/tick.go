package tick

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
)

var ErrLocked = errors.New("tick is already running")

type Ticker struct {
	dataDir     string
	budget      time.Duration
	attributors []SessionAttributor
	copiers     []TranscriptCopier
	hk          harness.Housekeeper
}

// TickerOption allows configuring optional ticker parameters.
type TickerOption func(*Ticker)

// WithAttributor registers a session attributor for sweeping enrolled repositories.
func WithAttributor(a SessionAttributor) TickerOption {
	return func(t *Ticker) {
		if a != nil {
			t.attributors = append(t.attributors, a)
		}
	}
}

// WithCopier registers a transcript copier for sweeping enrolled repositories.
func WithCopier(c TranscriptCopier) TickerOption {
	return func(t *Ticker) {
		if c != nil {
			t.copiers = append(t.copiers, c)
		}
	}
}


// WithHousekeeper registers the LLM harness for pipeline processing.
func WithHousekeeper(hk harness.Housekeeper) TickerOption {
	return func(t *Ticker) {
		t.hk = hk
	}
}

func NewTicker(dataDir string, budget time.Duration, opts ...TickerOption) *Ticker {
	t := &Ticker{
		dataDir: dataDir,
		budget:  budget,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Ticker) Run(ctx context.Context) error {
	deadline := time.Now().Add(t.budget)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	lockPath := filepath.Join(t.dataDir, "tick.lock")
	if err := os.MkdirAll(t.dataDir, 0755); err != nil {
		return err
	}

	l, err := TryLock(lockPath)
	if err != nil {
		if errors.Is(err, ErrLocked) {
			return nil // overlap behavior: just exit
		}
		return err
	}
	defer l.Unlock()

	reg, err := config.LoadRegistry(t.dataDir)
	if err != nil {
		return fmt.Errorf("loading registry: %w", err)
	}

	// Group registry by repoRoot
	enrolledRepos := make(map[string]config.RegistryEntry)
	for _, entry := range reg {
		enrolledRepos[entry.RootPath] = entry
	}

	for repoRoot, entry := range enrolledRepos {
		if err := t.processRepo(ctx, repoRoot, entry.RepoID); err != nil {
			fmt.Fprintf(os.Stderr, "tick error for repo %s: %v\n", repoRoot, err)
		}
	}

	// Sweep enrolled repos
	if err := Sweep(ctx, t.dataDir, enrolledRepos, t.attributors...); err != nil {
		fmt.Fprintf(os.Stderr, "sweep error: %v\n", err)
	}

	return nil
}

func (t *Ticker) processRepo(ctx context.Context, repoRoot, repoID string) error {
	repoDataDir := filepath.Join(t.dataDir, repoID)
	spoolDir := filepath.Join(repoDataDir, "spool")
	if err := os.MkdirAll(spoolDir, 0755); err != nil {
		return err
	}

	store, err := jobs.Open(filepath.Join(repoDataDir, "jobs.sqlite"))
	if err != nil {
		return err
	}
	defer store.Close()

	// 2. Ingest spool files as jobs
	reader := spool.NewReader(spoolDir)
	_, err = reader.Ingest(func(item spool.Item) error {
		job := &jobs.Job{
			Kind:           jobs.Kind(item.Entry.Kind),
			IdempotencyKey: item.Entry.ID,
			Priority:       10,
			Payload:        item.Entry.ID,
		}
		if job.IdempotencyKey == "" {
			job.IdempotencyKey = fmt.Sprintf("ingest-%s-%s", item.Entry.Kind, item.Entry.ConversationID)
		}
		_, err := store.Enqueue(ctx, job)
		if err != nil {
			// ignore idempotency conflict
			if err.Error() == "job exists" || err.Error() == "UNIQUE constraint failed" {
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 3. Reclaim expired leases
	_, err = store.ReclaimExpiredLeases(ctx, time.Now())
	if err != nil {
		return err
	}

	// 4. Run pipeline
	if err := t.ProcessPipeline(ctx, store, repoRoot, repoID); err != nil {
		fmt.Fprintf(os.Stderr, "pipeline error for repo %s: %v\n", repoID, err)
	}

	return nil
}
