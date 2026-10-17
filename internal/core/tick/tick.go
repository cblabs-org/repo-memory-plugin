/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/tick.go
 * Description: Top-level background Ticker orchestration. Acquires a non-blocking
 *              lockfile, iterates over enrolled repositories to ingest spool items
 *              and run the multi-stage pipeline, and sweeps idle sessions.
 *
 * Exported Types / Functions:
 * - ErrLocked:             Sentinel error returned when another tick process holds the lock.
 * - Ticker:                Coordinates periodic background processing across enrolled repositories.
 *   - NewTicker():         Constructs a Ticker with a data directory, time budget, and options.
 *   - Run():               Executes one bounded tick pass across all enrolled repositories.
 *   - ProcessPipeline():   Runs the repository pipeline (defined in pipeline.go).
 * - TickerOption:          Functional option for configuring a Ticker.
 * - WithAttributor():      Registers a SessionAttributor on a Ticker.
 * - WithCopier():          Registers a TranscriptCopier on a Ticker.
 * - WithHousekeeper():     Registers the Housekeeper LLM harness on a Ticker.
 */

package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// ErrLocked is returned by TryLock when another tick instance already holds the advisory lock.
var ErrLocked = errors.New("tick is already running")

// Ticker coordinates bounded background job ingestion, pipeline execution, and session sweeping.
type Ticker struct {
	dataDir     string              // Base memory data directory containing registry and per-repo state.
	budget      time.Duration       // Maximum wall-clock duration for a single Run invocation.
	attributors []SessionAttributor // Custom session attributors configured on this ticker.
	copiers     []TranscriptCopier  // Custom transcript copiers configured on this ticker.
	hk          harness.Housekeeper // Background LLM harness used for distillation and consolidation.
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

// NewTicker constructs a Ticker configured with dataDir, wall-clock budget, and optional functional options.
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

// Run executes a single bounded tick cycle across all enrolled repositories.
// If another tick process holds tick.lock, Run returns nil immediately.
func (t *Ticker) Run(ctx context.Context) error {
	// --- Step 1: Apply wall-clock budget deadline and acquire non-blocking tick lock ---
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

	// --- Step 2: Load enrolled repository registry and process each repository ---
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
		if entry.Enabled == nil || *entry.Enabled {
			_ = config.CreateEnrolledMarker(repoRoot)
		}
		if err := t.processRepo(ctx, repoRoot, entry.RepoID); err != nil {
			fmt.Fprintf(os.Stderr, "tick error for repo %s: %v\n", repoRoot, err)
		}
	}

	// --- Step 3: Sweep idle sessions across enrolled repositories ---
	if err := Sweep(ctx, t.dataDir, enrolledRepos, t.attributors...); err != nil {
		fmt.Fprintf(os.Stderr, "sweep error: %v\n", err)
	}

	return nil
}

// processRepo ingests queued spool files into the SQLite job store, reclaims expired leases,
// and executes the multi-stage pipeline for a single repository.
func (t *Ticker) processRepo(ctx context.Context, repoRoot, repoID string) error {
	// --- Step 1: Ensure spool directory exists and open the repository job store ---
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

	// --- Step 2: Ingest spool files as jobs ---
	reader := spool.NewReader(spoolDir)
	_, err = reader.Ingest(func(item spool.Item) error {
		payloadBytes, _ := json.Marshal(item.Entry)
		job := &jobs.Job{
			Kind:           jobs.Kind(item.Entry.Kind),
			IdempotencyKey: item.Entry.ID,
			Priority:       10,
			Payload:        string(payloadBytes),
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

	// --- Step 3: Reclaim expired leases ---
	_, err = store.ReclaimExpiredLeases(ctx, time.Now())
	if err != nil {
		return err
	}

	// --- Step 4: Run pipeline ---
	if err := t.ProcessPipeline(ctx, store, repoRoot, repoID); err != nil {
		fmt.Fprintf(os.Stderr, "pipeline error for repo %s: %v\n", repoID, err)
	}

	return nil
}
