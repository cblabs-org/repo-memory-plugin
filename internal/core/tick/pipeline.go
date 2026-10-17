/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/pipeline.go
 * Description: Core PipelineExecutor coordinating end-to-end repository background
 *              processing: capture-to-episode conversion, zero-token checkpoint/artifact
 *              harvest, sidecar fallback, doc freshness checks, warm consolidation,
 *              closes_when evaluation, skill graduation sync, and index rebuilds.
 *
 * Exported Types / Functions:
 * - PipelineExecutor:                  Executes the multi-stage memory pipeline for a repository.
 *   - NewPipelineExecutor():           Constructs a PipelineExecutor for a repository.
 *   - SetHousekeeper():                Sets the Housekeeper LLM harness on the executor.
 *   - SetConfig():                     Sets the repository configuration on the executor.
 *   - ConvertCapturesToEvidence():     Converts queued capture jobs into normalized episodes.
 *   - HarvestFreeSources():            Runs deterministic zero-token harvest over checkpoints and artifacts.
 *   - Run():                           Executes all pipeline stages in sequence.
 * - Ticker.ProcessPipeline():          Initializes stores and runs PipelineExecutor for a repository.
 */

package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/skills"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/telemetry"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

// PipelineExecutor coordinates the multi-stage background pipeline for a single enrolled repository.
type PipelineExecutor struct {
	hk          harness.Housekeeper // Background LLM harness for distillation and consolidation.
	dataDir     string              // Base memory data directory path.
	repoRoot    string              // Absolute path to the enrolled repository root.
	repoID      string              // Registry slug identifying the repository under <data-dir>/<repo-id>.
	store       *jobs.Store         // SQLite job queue store for the repository.
	epStore     *episode.Store      // Episode store for reading and writing normalized episodes.
	idx         *index.Index        // Multi-tier SQLite FTS5 search index handle.
	cfg         config.RepoConfig   // Parsed .memory/config.yaml settings governing budgets, spend guards, and scopes.
	budgetLimit int                 // Maximum daily token budget allowed for this run.
	budgetUsed  int                 // Cumulative tokens consumed during this run.
	mu          sync.Mutex          // Guards budgetUsed and spend guard updates.

	FallbackIdleThreshold time.Duration                                         // Idle threshold before sidecar fallback runs (defaults to 2h when <= 0).
	QuotaChecker          func(context.Context) (*telemetry.QuotaStatus, error) // Optional quota probe override for testing and spend guards.
	Now                   func() time.Time                                      // Optional clock override for deterministic testing.
}

// ProcessPipeline initializes the episode store, search index, and PipelineExecutor for repoID and runs all stages.
func (t *Ticker) ProcessPipeline(ctx context.Context, store *jobs.Store, repoRoot, repoID string) error {
	// --- Step 1: Verify housekeeper availability and load repository config ---
	if t.hk == nil {
		return nil
	}

	cfg, err := config.LoadRepoConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("loading repo config: %w", err)
	}

	// --- Step 2: Open episode store and SQLite search index ---
	epStore := episode.NewStore(t.dataDir, repoID)
	idxPath := index.IndexPath(t.dataDir, repoID)
	if err := os.MkdirAll(filepath.Dir(idxPath), 0755); err != nil {
		return fmt.Errorf("creating index directory: %w", err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		return fmt.Errorf("opening index: %w", err)
	}
	defer idx.Close()

	// --- Step 3: Construct PipelineExecutor and run the pipeline ---
	exec := &PipelineExecutor{
		hk:          t.hk,
		dataDir:     t.dataDir,
		repoRoot:    repoRoot,
		repoID:      repoID,
		store:       store,
		epStore:     epStore,
		idx:         idx,
		cfg:         cfg,
		budgetLimit: cfg.Budgets.DailyTokens,
	}

	return exec.Run(ctx)
}

// checkBudget accumulates token usage from cost (if non-nil) and returns an error when budgetLimit is exceeded.
func (p *PipelineExecutor) checkBudget(cost *jobs.Cost) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cost != nil {
		p.budgetUsed += cost.InputTokens + cost.OutputTokens
	}
	if p.budgetLimit > 0 && p.budgetUsed >= p.budgetLimit {
		return fmt.Errorf("daily token budget exceeded")
	}
	return nil
}

// NewPipelineExecutor creates a PipelineExecutor for pipeline processing.
func NewPipelineExecutor(dataDir, repoRoot, repoID string, store *jobs.Store, epStore *episode.Store) *PipelineExecutor {
	var cfg config.RepoConfig
	if repoRoot != "" {
		if loaded, err := config.LoadRepoConfig(repoRoot); err == nil {
			cfg = loaded
		}
	}
	return &PipelineExecutor{
		dataDir:     dataDir,
		repoRoot:    repoRoot,
		repoID:      repoID,
		store:       store,
		epStore:     epStore,
		cfg:         cfg,
		budgetLimit: cfg.Budgets.DailyTokens,
	}
}

// SetHousekeeper sets the housekeeper harness for the executor.
func (p *PipelineExecutor) SetHousekeeper(hk harness.Housekeeper) {
	p.hk = hk
}

// SetConfig sets the repository configuration on the executor.
func (p *PipelineExecutor) SetConfig(cfg config.RepoConfig) {
	p.cfg = cfg
	if cfg.Budgets.DailyTokens > 0 {
		p.budgetLimit = cfg.Budgets.DailyTokens
	}
}

// usageTracker wraps a Housekeeper to accumulate token usage across multiple invocations.
type usageTracker struct {
	hk    harness.Housekeeper // Wrapped Housekeeper delegate invoked for each pipeline stage.
	mu    sync.Mutex          // Guards cumulative usage counters.
	usage harness.Usage       // Accumulated token usage across calls.
}

// newUsageTracker constructs a usageTracker wrapping hk.
func newUsageTracker(hk harness.Housekeeper) *usageTracker {
	return &usageTracker{hk: hk}
}

// Run invokes the underlying Housekeeper for stage and accumulates its token usage.
func (u *usageTracker) Run(ctx context.Context, stage harness.Stage, input any) (harness.Result, error) {
	if u.hk == nil {
		return harness.Result{}, fmt.Errorf("no housekeeper configured")
	}
	res, err := u.hk.Run(ctx, stage, input)
	u.mu.Lock()
	u.usage.InputTokens += res.Usage.InputTokens
	u.usage.OutputTokens += res.Usage.OutputTokens
	u.usage.ThinkingTokens += res.Usage.ThinkingTokens
	u.usage.CacheReadTokens += res.Usage.CacheReadTokens
	u.mu.Unlock()
	return res, err
}

// Cost returns the accumulated token usage converted into a jobs.Cost record.
func (u *usageTracker) Cost() jobs.Cost {
	u.mu.Lock()
	defer u.mu.Unlock()
	return jobs.FromUsage(u.usage)
}

// ConvertCapturesToEvidence exposes convertCapturesToEvidence for testing and direct invocation.
func (p *PipelineExecutor) ConvertCapturesToEvidence(ctx context.Context) error {
	return p.convertCapturesToEvidence(ctx)
}

// HarvestFreeSources runs Stage 1b deterministic zero-token harvest over episode checkpoints
// and conversation artifacts (`source: "checkpoint"` and `source: "artifact"`).
func (p *PipelineExecutor) HarvestFreeSources(ctx context.Context) (int, error) {
	return inbox.HarvestCheckpointsAndArtifacts(ctx, p.dataDir, p.repoID, p.repoRoot, p.epStore, inbox.NewStore(p.dataDir, p.repoID))
}

// Run executes all sequential stages of the repository memory pipeline.
func (p *PipelineExecutor) Run(ctx context.Context) error {
	// --- Step 1: Convert queued transcript captures into normalized episodes ---
	if err := p.convertCapturesToEvidence(ctx); err != nil {
		return err
	}

	// --- Step 2: Harvest zero-token checkpoints and conversation artifacts ---
	if _, err := p.HarvestFreeSources(ctx); err != nil {
		return err
	}

	// --- Step 3: Run Stage 1c sidecar fallback distillation for eligible idle episodes ---
	if _, err := p.RunSidecarFallback(ctx); err != nil {
		return err
	}

	// --- Step 4: Reconcile deterministic doc-seed freshness ---
	if err := p.checkDocFreshness(ctx); err != nil {
		return err
	}

	// --- Step 5: Run Stage 2 warm batch consolidation ---
	if _, err := p.RunConsolidation(ctx, false); err != nil {
		return err
	}

	// --- Step 6: Evaluate deterministic closes_when predicates on open issues ---
	if _, err := p.EvaluateClosesWhen(ctx); err != nil {
		return fmt.Errorf("evaluating closes_when conditions: %w", err)
	}

	// --- Step 7: Check build manifest freshness, rebuild index, and sync graduated skills ---
	if p.repoRoot != "" {
		if _, err := skills.CheckBuildManifestFreshness(ctx, p.repoRoot, p.dataDir, p.repoID, p.idx); err != nil {
			return fmt.Errorf("checking build manifest freshness: %w", err)
		}
	}

	if p.idx != nil {
		var currentBranch string
		if p.repoRoot != "" {
			currentBranch, _ = gitops.GetCurrentBranchWithContext(ctx, p.repoRoot)
		}
		if err := consolidate.RebuildMultiTierIndex(ctx, p.idx, p.dataDir, p.repoID, p.repoRoot, currentBranch); err != nil {
			return fmt.Errorf("rebuilding multi-tier index: %w", err)
		}
	}

	if p.repoRoot != "" {
		if _, err := skills.SyncGraduatedSkills(ctx, p.repoRoot, p.dataDir, p.repoID, false); err != nil {
			return fmt.Errorf("syncing graduated skills: %w", err)
		}
	}

	// --- Step 8: Persist updated A6 freshness state ---
	p.refreshFreshnessState(ctx, false)

	return nil
}

// convertCapturesToEvidence drains queued KindCapture jobs, parses transcripts, and writes episodes.
func (p *PipelineExecutor) convertCapturesToEvidence(ctx context.Context) error {
	// --- Step 1: Ensure the episode store is initialized ---
	if p.epStore == nil {
		p.epStore = episode.NewStore(p.dataDir, p.repoID)
	}

	// --- Step 2: Acquire and process each capture job ---
	for {
		job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindCapture)
		if err != nil {
			if errors.Is(err, jobs.ErrNoJobAvailable) {
				break
			}
			return err
		}

		var entry spool.Entry
		if err := json.Unmarshal([]byte(job.Payload), &entry); err != nil || entry.ConversationID == "" {
			_ = p.store.Complete(ctx, job.ID, nil)
			continue
		}

		// --- Step 3: Resolve transcript path from queue copy or original path ---
		transcriptPath := entry.TranscriptPath
		queueCandidate := filepath.Join(p.dataDir, p.repoID, "queue", entry.ConversationID+".jsonl")
		if _, err := os.Stat(queueCandidate); err == nil {
			transcriptPath = queueCandidate
		} else if transcriptPath != "" {
			if _, err := os.Stat(transcriptPath); err != nil {
				transcriptPath = ""
			}
		}

		if transcriptPath == "" {
			p.store.Fail(ctx, job.ID, fmt.Sprintf("no transcript path for conversation %s", entry.ConversationID), 5*time.Minute)
			continue
		}

		// --- Step 4: Parse transcript and persist episode ---
		parsed, err := evidence.ParseTranscript(transcriptPath)
		if err != nil {
			p.store.Fail(ctx, job.ID, fmt.Sprintf("failed to parse transcript %s: %v", transcriptPath, err), 5*time.Minute)
			continue
		}

		entry.TranscriptPath = transcriptPath
		ep := evidence.BuildEpisode(entry, parsed, p.repoID)

		if _, err := p.epStore.WriteEpisode(ep); err != nil {
			p.store.Fail(ctx, job.ID, fmt.Sprintf("failed to write episode for %s: %v", entry.ConversationID, err), 5*time.Minute)
			continue
		}

		_ = p.store.Complete(ctx, job.ID, nil)
	}
	return nil
}
