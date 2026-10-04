
package tick

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
	claimstore "github.com/cblabs-org/repo-memory-plugin/internal/core/claims/store"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/evidence"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/extract"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/publish"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/reconcile"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/triage"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/pipeline/verify"
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
	claimStore  *claimstore.Store
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

	repoDataDir := filepath.Join(t.dataDir, repoID)
	epStore := episode.NewStore(t.dataDir, repoID)
	claimStore, err := claimstore.New(filepath.Join(repoDataDir, "claims"))
	if err != nil {
		return fmt.Errorf("opening claim store: %w", err)
	}
	idx, err := index.NewIndex(filepath.Join(repoDataDir, "index.sqlite"))
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
		claimStore:  claimStore,
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

// ConvertCapturesToEvidence exposes convertCapturesToEvidence for testing and direct invocation.
func (p *PipelineExecutor) ConvertCapturesToEvidence(ctx context.Context) error {
	return p.convertCapturesToEvidence(ctx)
}

func (p *PipelineExecutor) Run(ctx context.Context) error {
	if err := p.convertCapturesToEvidence(ctx); err != nil {
		return err
	}

	if err := p.processTriage(ctx); err != nil {
		return err
	}

	if err := p.processExtract(ctx); err != nil {
		return err
	}

	if err := p.processReconcile(ctx); err != nil {
		return err
	}

	if err := p.processVerify(ctx); err != nil {
		return err
	}

	if err := p.processPublish(ctx); err != nil {
		return err
	}

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
		if transcriptPath == "" {
			queueCandidate := filepath.Join(p.dataDir, p.repoID, "queue", entry.ConversationID+".jsonl")
			if _, err := os.Stat(queueCandidate); err == nil {
				transcriptPath = queueCandidate
			}
		} else {
			if _, err := os.Stat(transcriptPath); err != nil {
				queueCandidate := filepath.Join(p.dataDir, p.repoID, "queue", entry.ConversationID+".jsonl")
				if _, qErr := os.Stat(queueCandidate); qErr == nil {
					transcriptPath = queueCandidate
				}
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

type extractPayload struct {
	EpisodeID string `json:"episode_id"`
	Result    triage.TriageResult `json:"result"`
}

func (p *PipelineExecutor) processTriage(ctx context.Context) error {
	for {
		if err := p.checkBudget(nil); err != nil {
			return err
		}

		job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindTriage)
		if err != nil {
			if errors.Is(err, jobs.ErrNoJobAvailable) {
				break
			}
			return err
		}

		epID := job.Payload
		ep, err := p.epStore.Read(epID)
		if err != nil {
			if errors.Is(err, harness.ErrQuotaExhausted) {
				p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
				return err
			}
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		tcfg := triage.Config{Subsystems: []string{}}
		for _, s := range p.cfg.Subsystems {
			tcfg.Subsystems = append(tcfg.Subsystems, s.Title)
		}

		triager := triage.NewTriager(p.hk, tcfg, triage.DefaultWindowsConfig())
		res, err := triager.Process(ctx, []*episode.Episode{ep})
		if err != nil {
			if errors.Is(err, harness.ErrQuotaExhausted) {
				p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
				return err
			}
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		if tRes, ok := res[ep.Header.EpisodeID]; ok && tRes.Keep {
			payload := extractPayload{EpisodeID: epID, Result: tRes}
			b, _ := json.Marshal(payload)
			extractJob := &jobs.Job{
				Kind:           jobs.KindExtract,
				Priority:       jobs.PriorityNormal,
				IdempotencyKey: "extract-" + epID,
				Payload:        string(b),
			}
			_, _ = p.store.Enqueue(ctx, extractJob)
		}

		_ = p.store.Complete(ctx, job.ID, &jobs.Cost{}) // Dummy cost since Triager doesn't export usage natively
	}
	return nil
}

func (p *PipelineExecutor) processExtract(ctx context.Context) error {
	var wg sync.WaitGroup
	errCh := make(chan error, 1)

	// Concurrency rule: extraction must be parallel. Spawn 3 workers.
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if err := p.checkBudget(nil); err != nil {
					return
				}

				job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindExtract)
				if err != nil {
					if !errors.Is(err, jobs.ErrNoJobAvailable) {
						select {
						case errCh <- err:
						default:
						}
					}
					return
				}

				var payload extractPayload
				if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
					p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
					continue
				}

				ep, err := p.epStore.Read(payload.EpisodeID)
				if err != nil {
					p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
					continue
				}

				extractor := extract.NewExtractor(p.hk, 16000)
				triageMap := map[string]triage.TriageResult{ep.Header.EpisodeID: payload.Result}
				
				results, err := extractor.Extract(ctx, []*episode.Episode{ep}, triageMap)
				if err != nil {
					p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
					continue
				}

				// Enqueue Reconcile jobs
				for _, res := range results {
					for i := range res.Candidates {
						cand := res.Candidates[i]
						cand.EpisodeID = payload.EpisodeID // Force it to be the SessionID
						cb, _ := json.Marshal(cand)
						reconcileJob := &jobs.Job{
							Kind:           jobs.KindReconcile,
							Priority:       jobs.PriorityNormal,
							IdempotencyKey: "reconcile-" + payload.EpisodeID + "-" + cand.Statement,
							Payload:        string(cb),
						}
						_, _ = p.store.Enqueue(ctx, reconcileJob)
					}
				}

				_ = p.store.Complete(ctx, job.ID, &jobs.Cost{})
			}
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (p *PipelineExecutor) processReconcile(ctx context.Context) error {
	for {
		if err := p.checkBudget(nil); err != nil {
			return err
		}

		job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindReconcile)
		if err != nil {
			if errors.Is(err, jobs.ErrNoJobAvailable) {
				break
			}
			return err
		}

		var cand extract.Candidate
		if err := json.Unmarshal([]byte(job.Payload), &cand); err != nil {
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		sessionID := cand.EpisodeID
		if sessionID == "" || strings.HasPrefix(sessionID, "ep-") {
			// Extract from idempotency key: reconcile-pa-filename-ULID-statement
			// Find the ULID which is 26 uppercase chars and digits
			parts := strings.Split(job.IdempotencyKey, "-")
			for i, pPart := range parts {
				if len(pPart) == 26 && i >= 2 {
					sessionID = strings.Join(parts[1:i+1], "-")
					cand.EpisodeID = sessionID
					break
				}
			}
		}

		ep, err := p.epStore.Read(sessionID)
		if err != nil {
			p.store.Fail(ctx, job.ID, fmt.Sprintf("failed to read session %q: %v", sessionID, err), 5*time.Minute)
			continue
		}

		reconciler := reconcile.NewReconciler(p.hk, p.claimStore)
		actions, err := reconciler.Reconcile(ctx, ep, []extract.Candidate{cand})
		if err != nil {
			if errors.Is(err, harness.ErrQuotaExhausted) {
				p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
				return err
			}
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		for _, action := range actions {
			if action.Op == reconcile.OpCreate || action.Op == reconcile.OpRefine || action.Op == reconcile.OpPromote {
				verifyJob := &jobs.Job{
					Kind:           jobs.KindVerify,
					Priority:       jobs.PriorityNormal,
					IdempotencyKey: "verify-" + action.Candidate.Statement,
					Payload:        action.Candidate.Statement, // Just need something to link to claim
				}
				
				// Reconciler already modifies claimStore in applyAction!
				// But we need the actual claim ID.
				// For WO7 simplicity, if the action is applied, we assume it's verifiable.
				// We need the claim ID from somewhere. We might need to pull it.
				// Since Reconciler applies action and saves the claim, we can just trigger Verify on all claims.
				// Wait, Reconcile applies actions which update claims.
				// For verify, we just need to pass the target claim IDs.
				
				_, _ = p.store.Enqueue(ctx, verifyJob)
			}
		}

		_ = p.store.Complete(ctx, job.ID, &jobs.Cost{})
	}
	return nil
}

func (p *PipelineExecutor) processVerify(ctx context.Context) error {
	for {
		if err := p.checkBudget(nil); err != nil {
			return err
		}

		job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindVerify)
		if err != nil {
			if errors.Is(err, jobs.ErrNoJobAvailable) {
				break
			}
			return err
		}
		
		// In a real implementation we would fetch the specific claim.
		// Since Reconcile just proposes claims, Verify can verify all unverified claims.
		claimsList, err := p.claimStore.ListAll()
		var targetClaims []*claims.Claim
		for _, c := range claimsList {
			if c.Status == claims.StatusHypothesis || c.Status == claims.StatusObserved {
				targetClaims = append(targetClaims, c)
			}
		}

		verifier := verify.NewVerifier(p.hk, p.claimStore, p.repoRoot)
		if err := verifier.VerifyClaims(ctx, targetClaims); err != nil {
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		for _, c := range targetClaims {
			if c.Status == claims.StatusVerified {
				publishJob := &jobs.Job{
					Kind:           jobs.KindPublish,
					Priority:       jobs.PriorityNormal,
					IdempotencyKey: "publish-" + c.ID,
					Payload:        c.ID,
				}
				_, _ = p.store.Enqueue(ctx, publishJob)
			}
		}

		_ = p.store.Complete(ctx, job.ID, &jobs.Cost{})
	}
	return nil
}

func (p *PipelineExecutor) processPublish(ctx context.Context) error {
	for {
		if err := p.checkBudget(nil); err != nil {
			return err
		}

		job, err := p.store.AcquireNext(ctx, 5*time.Minute, jobs.KindPublish)
		if err != nil {
			if errors.Is(err, jobs.ErrNoJobAvailable) {
				break
			}
			return err
		}

		claimID := job.Payload
		claim, err := p.claimStore.Read(claimID)
		if err != nil {
			if errors.Is(err, harness.ErrQuotaExhausted) {
				p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
				return err
			}
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		publisher := publish.NewLocalPublisher(p.dataDir, p.repoID, p.idx)
		if err := publisher.Publish(ctx, claim); err != nil {
			// If not verified, publisher might error. We can fail the job.
			p.store.Fail(ctx, job.ID, err.Error(), 5*time.Minute)
			continue
		}

		_ = p.store.Complete(ctx, job.ID, &jobs.Cost{})
	}
	return nil
}
