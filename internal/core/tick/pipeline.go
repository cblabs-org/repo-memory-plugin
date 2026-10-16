package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/spool"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

type PipelineExecutor struct {
	hk          harness.Housekeeper
	dataDir     string
	repoRoot    string
	repoID      string
	store       *jobs.Store
	epStore     *episode.Store
	idx         *index.Index
	cfg         config.RepoConfig
	budgetLimit int
	budgetUsed  int
	mu          sync.Mutex
}

func (t *Ticker) ProcessPipeline(ctx context.Context, store *jobs.Store, repoRoot, repoID string) error {
	if t.hk == nil {
		return nil
	}

	cfg, err := config.LoadRepoConfig(repoRoot)
	if err != nil {
		return fmt.Errorf("loading repo config: %w", err)
	}

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
	return &PipelineExecutor{
		dataDir:  dataDir,
		repoRoot: repoRoot,
		repoID:   repoID,
		store:    store,
		epStore:  epStore,
	}
}

// SetHousekeeper sets the housekeeper harness for the executor.
func (p *PipelineExecutor) SetHousekeeper(hk harness.Housekeeper) {
	p.hk = hk
}

type usageTracker struct {
	hk    harness.Housekeeper
	mu    sync.Mutex
	usage harness.Usage
}

func newUsageTracker(hk harness.Housekeeper) *usageTracker {
	return &usageTracker{hk: hk}
}

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

func (p *PipelineExecutor) Run(ctx context.Context) error {
	if err := p.convertCapturesToEvidence(ctx); err != nil {
		return err
	}

	if _, err := p.HarvestFreeSources(ctx); err != nil {
		return err
	}

	if err := p.checkDocFreshness(ctx); err != nil {
		return err
	}

	if p.idx != nil && p.repoRoot != "" {
		opsDir := filepath.Join(p.repoRoot, ".memory", "ops")
		if fi, err := os.Stat(opsDir); err == nil && fi.IsDir() {
			opsReader := ops.NewReader(opsDir)
			if err := p.idx.RebuildFromOpLog(ctx, opsReader); err != nil {
				return fmt.Errorf("rebuilding index from op log: %w", err)
			}
		}
	}

	log.Println("v1 pipeline stages removed; v2 distillation not yet implemented")

	return nil
}

func (p *PipelineExecutor) convertCapturesToEvidence(ctx context.Context) error {
	if p.epStore == nil {
		p.epStore = episode.NewStore(p.dataDir, p.repoID)
	}

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

