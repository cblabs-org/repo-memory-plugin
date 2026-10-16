package tick

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/config"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/consolidate"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/dossiers"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/episode"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/gitops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/inbox"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/index"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/jobs"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/ops"
	"github.com/cblabs-org/repo-memory-plugin/internal/core/scrub"
	"github.com/cblabs-org/repo-memory-plugin/internal/harness"
)

const (
	// DefaultConsolidationMinNotes is the minimum number of unconsolidated inbox notes
	// that triggers an immediate Stage 2 consolidation run during tick.
	DefaultConsolidationMinNotes = 10
	// DefaultConsolidationMaxAge is the maximum age before a waiting unconsolidated inbox note
	// triggers a daily Stage 2 consolidation run during tick.
	DefaultConsolidationMaxAge = 24 * time.Hour
)

// ConsolidationOptions configures a Stage 2 warm batch consolidation run.
type ConsolidationOptions struct {
	Force  bool `json:"force"`
	DryRun bool `json:"dry_run"`
}

// ProposedScope describes a newly derived dossier scope proposed during routing.
type ProposedScope struct {
	Scope string               `json:"scope"`
	Kind  dossiers.DossierKind `json:"kind,omitempty"`
}

// ConsolidationResult summarizes the outcome of a Stage 2 warm batch consolidation run.
type ConsolidationResult struct {
	Skipped              bool              `json:"skipped"`
	SkipReason           string            `json:"skip_reason,omitempty"`
	DryRun               bool              `json:"dry_run"`
	PromotedBranches     []string          `json:"promoted_branches,omitempty"`
	UnconsolidatedNotes  int               `json:"unconsolidated_notes"`
	ProcessedNotes       int               `json:"processed_notes"`
	CandidateItems       int               `json:"candidate_items"`
	EchoesDropped        int               `json:"echoes_dropped"`
	DowngradedToIssue    int               `json:"downgraded_to_issue"`
	ExactDuplicatesAmend int               `json:"exact_duplicates_amend"`
	ProposedScopes       []ProposedScope   `json:"proposed_scopes,omitempty"`
	OpsAdded             int               `json:"ops_added"`
	OpsAmended           int               `json:"ops_amended"`
	OpsSuperseded        int               `json:"ops_superseded"`
	OpsRetired           int               `json:"ops_retired"`
	OpsMoved             int               `json:"ops_moved"`
	OpsSplit             int               `json:"ops_split"`
	OpsRejected          int               `json:"ops_rejected"`
	UserBatchID          string            `json:"user_batch_id,omitempty"`
	BranchBatches        map[string]string `json:"branch_batches,omitempty"`
	RepoBatchID          string            `json:"repo_batch_id,omitempty"`
	WorktreeCommitted    bool              `json:"worktree_committed"`
	WorktreeCommitSHA    string            `json:"worktree_commit_sha,omitempty"`
	Usage                harness.Usage     `json:"usage"`
	EstimatedInputTokens int               `json:"estimated_input_tokens"`
}

// FreshnessState tracks capture, consolidation, and index freshness at <dataDir>/<repoID>/state/freshness.json (Addendum A §7).
type FreshnessState struct {
	LastCaptureAt       string `json:"last_capture_at,omitempty"`
	LastConsolidationAt string `json:"last_consolidation_at,omitempty"`
	UnconsolidatedNotes int    `json:"unconsolidated_notes"`
	ActiveEntries       int    `json:"active_entries"`
	IndexOpLogBatchID   string `json:"index_oplog_batch_id,omitempty"`
	IndexCurrentToOpLog bool   `json:"index_current_to_oplog"`
	IndexHeadSHA        string `json:"index_head_sha,omitempty"`
	HeadMovedSinceBuild bool   `json:"head_moved_since_build"`
	SuspectEntries      int    `json:"suspect_entries"`
	UpdatedAt           string `json:"updated_at"`
}

// FreshnessStatePath returns the path to <dataDir>/<repoID>/state/freshness.json.
func FreshnessStatePath(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "state", "freshness.json")
}

// LoadFreshnessState reads <dataDir>/<repoID>/state/freshness.json.
// If the file does not exist, it returns an empty FreshnessState without error.
func LoadFreshnessState(dataDir, repoID string) (*FreshnessState, error) {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(repoID) == "" {
		return &FreshnessState{}, nil
	}
	path := FreshnessStatePath(dataDir, repoID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FreshnessState{}, nil
		}
		return nil, fmt.Errorf("reading freshness state %s: %w", path, err)
	}
	var st FreshnessState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("unmarshaling freshness state %s: %w", path, err)
	}
	return &st, nil
}

// SaveFreshnessState atomically writes st to <dataDir>/<repoID>/state/freshness.json.
func SaveFreshnessState(dataDir, repoID string, st *FreshnessState) error {
	if strings.TrimSpace(dataDir) == "" || strings.TrimSpace(repoID) == "" || st == nil {
		return nil
	}
	if strings.TrimSpace(st.UpdatedAt) == "" {
		st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	path := FreshnessStatePath(dataDir, repoID)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating freshness state directory %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling freshness state: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".freshness-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp freshness file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp freshness file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp freshness file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp freshness file to %s: %w", path, err)
	}
	return nil
}

// RunConsolidation executes Stage 2 warm batch consolidation over pending inbox notes.
func (p *PipelineExecutor) RunConsolidation(ctx context.Context, force bool) (*ConsolidationResult, error) {
	return p.RunConsolidationWithOptions(ctx, ConsolidationOptions{Force: force})
}

// RunConsolidationWithOptions executes the 7-step Stage 2 warm batch consolidation pipeline.
func (p *PipelineExecutor) RunConsolidationWithOptions(ctx context.Context, opts ConsolidationOptions) (*ConsolidationResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := p.now()
	res := &ConsolidationResult{
		DryRun: opts.DryRun,
	}

	// 1. Branch Promotion First (!opts.DryRun).
	var promotedBatches []ops.OpBatch
	if !opts.DryRun && strings.TrimSpace(p.repoRoot) != "" {
		if promoRes, promoErr := consolidate.PromoteMergedBranchOps(ctx, p.repoRoot); promoErr == nil && promoRes != nil {
			res.PromotedBranches = promoRes.PromotedBranches
			promotedBatches = promoRes.PromotedBatches
		}
	}

	// 2. Load Unconsolidated Notes.
	inboxStore := inbox.NewStore(p.dataDir, p.repoID)
	notes, err := inboxStore.ListUnconsolidated()
	if err != nil {
		return nil, fmt.Errorf("listing unconsolidated notes: %w", err)
	}
	res.UnconsolidatedNotes = len(notes)

	if len(notes) == 0 {
		if len(promotedBatches) > 0 && !opts.DryRun {
			if strings.TrimSpace(p.repoRoot) != "" && strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
				if wtRes, wtErr := gitops.PublishToMemoryWorktree(ctx, p.dataDir, p.repoID, p.repoRoot, promotedBatches); wtErr == nil && wtRes != nil {
					res.WorktreeCommitted = wtRes.Committed
					res.WorktreeCommitSHA = wtRes.CommitSHA
				}
			}
			_ = p.rebuildMultiTierIndex(ctx)
			p.refreshFreshnessState(ctx, false)
		}
		res.Skipped = len(promotedBatches) == 0
		res.SkipReason = "no_unconsolidated_notes"
		return res, nil
	}

	// 3. Cadence & A5 Spend Guard Check (when !opts.Force && !opts.DryRun).
	st, _ := LoadFreshnessState(p.dataDir, p.repoID)
	if st == nil {
		st = &FreshnessState{}
	}
	effSpend := p.effectiveSpendConfig()
	consSpendCfg := effSpend.Jobs["consolidation"]
	minInterval := DefaultConsolidationMaxAge
	if consSpendCfg.MinInterval != "" {
		if parsedDur, parseErr := time.ParseDuration(consSpendCfg.MinInterval); parseErr == nil && parsedDur > 0 {
			minInterval = parsedDur
		}
	}
	if !opts.Force && !opts.DryRun {
		due := len(notes) >= DefaultConsolidationMinNotes
		if !due {
			if lastStr := strings.TrimSpace(st.LastConsolidationAt); lastStr != "" {
				if lastTS, parseErr := time.Parse(time.RFC3339, lastStr); parseErr != nil || now.Sub(lastTS) >= minInterval {
					due = true
				}
			} else if guardSt, gErr := LoadSpendGuardState(p.dataDir, p.repoID); gErr == nil && strings.TrimSpace(guardSt.LastRunAt["consolidation"]) != "" {
				if lastTS, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(guardSt.LastRunAt["consolidation"])); parseErr != nil || now.Sub(lastTS) >= minInterval {
					due = true
				}
			} else {
				oldestCreated := notes[0].CreatedAt
				if oldestCreated.IsZero() || now.Sub(oldestCreated) >= minInterval {
					due = true
				}
			}
		}
		if !due {
			st.UnconsolidatedNotes = len(notes)
			st.UpdatedAt = now.Format(time.RFC3339)
			_ = SaveFreshnessState(p.dataDir, p.repoID, st)
			res.Skipped = true
			res.SkipReason = "cadence_not_due"
			return res, nil
		}
	}

	// Enforce A5 max_items batch cap per consolidation run.
	_, _, maxItems := p.CheckSpendGuard("consolidation", opts.Force)
	if maxItems > 0 && len(notes) > maxItems {
		notes = notes[:maxItems]
	}

	// 4. Quota & Budget Guard (when !opts.DryRun && p.hk != nil).
	if !opts.DryRun && p.hk != nil {
		if !p.checkQuotaHeadroom(ctx) || p.checkBudget(nil) != nil {
			res.Skipped = true
			res.SkipReason = "quota_or_budget_exhausted"
			return res, nil
		}
	}

	// 5. Load Existing Dossiers & Episodes.
	var repoOpsDir string
	if strings.TrimSpace(p.repoRoot) != "" {
		repoOpsDir = filepath.Join(p.repoRoot, ".memory", "ops")
	}
	repoDossiers := projectOpsDir(ctx, repoOpsDir)

	var userOpsDir string
	if strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
		userOpsDir = filepath.Join(consolidate.UserTierBaseDir(p.dataDir, p.repoID), "ops")
	}
	userDossiers := projectOpsDir(ctx, userOpsDir)

	branchDossiers := loadAllBranchDossiers(ctx, p.repoRoot)
	allActiveEntries := combineAllActiveEntries(
		normalizeDossiersForMatching(repoDossiers),
		normalizeDossiersForMatching(userDossiers),
		normalizeBranchDossiersForMatching(branchDossiers),
	)

	var atlasSummary string
	if strings.TrimSpace(p.repoRoot) != "" {
		if data, readErr := os.ReadFile(filepath.Join(p.repoRoot, ".memory", "ATLAS.md")); readErr == nil {
			atlasSummary = strings.TrimSpace(string(data))
		}
	}

	episodesByConv := p.loadEpisodesByConv()

	// 6. Step 1 — Filter Echoes & Corroboration Gate.
	type noteSurvivors struct {
		note  *inbox.Note
		items []inbox.NoteItem
	}
	var survivingNotes []noteSurvivors
	for _, note := range notes {
		if note == nil {
			continue
		}
		res.CandidateItems += len(note.Items)
		ep := episodesByConv[strings.TrimSpace(note.ConversationID)]
		echoRes := consolidate.FilterEchoes(note, allActiveEntries, ep)
		res.EchoesDropped += echoRes.EchoSuppressionCount

		corrRes := consolidate.EnforceCorroborationGate(note, echoRes.Kept, ep, p.repoRoot)
		res.DowngradedToIssue += corrRes.CorroborationDowngradeCount
		if len(corrRes.Items) > 0 {
			survivingNotes = append(survivingNotes, noteSurvivors{
				note:  note,
				items: corrRes.Items,
			})
		}
	}

	// 7. Steps 2–4 — Route, Merge Exact Duplicates & Check Existing Anchors.
	var scopeCfg *config.ScopeConfig
	if strings.TrimSpace(p.repoRoot) != "" {
		scopeCfg, _ = config.LoadScopeConfig(filepath.Join(p.repoRoot, ".memory", "config.yaml"))
	}
	if scopeCfg == nil {
		scopeCfg = &config.ScopeConfig{}
	}

	var aliasMap *config.AliasMap
	if repoOpsDir != "" {
		if fi, statErr := os.Stat(repoOpsDir); statErr == nil && fi.IsDir() {
			aliasMap, _ = config.BuildAliasMap(ops.NewReader(repoOpsDir), ctx)
		}
	}
	if aliasMap == nil {
		aliasMap = config.NewAliasMap()
	}

	existingModules := buildExistingModulesFromDossiers(repoDossiers)
	resolver := config.NewScopeResolver(scopeCfg, existingModules, aliasMap)
	isBranchMerged := buildBranchMergedChecker(ctx, p.repoRoot)

	var (
		userRouted   []consolidate.RoutedItem
		branchRouted = make(map[string][]consolidate.RoutedItem)
		repoRouted   []consolidate.RoutedItem
	)
	seenProposed := make(map[string]bool)

	for _, ns := range survivingNotes {
		routed := consolidate.RouteItems(ns.note, ns.items, resolver, p.repoRoot, isBranchMerged)
		for _, ri := range routed {
			if ri.DerivedNew && ri.DossierScope != "" && !seenProposed[ri.DossierScope] {
				seenProposed[ri.DossierScope] = true
				res.ProposedScopes = append(res.ProposedScopes, ProposedScope{
					Scope: ri.DossierScope,
					Kind:  ri.DossierKind,
				})
			}
			switch ri.Tier {
			case consolidate.TierUser:
				userRouted = append(userRouted, ri)
			case consolidate.TierBranch:
				slug := consolidate.SanitizeBranchSlug(ri.Branch)
				branchRouted[slug] = append(branchRouted[slug], ri)
			case consolidate.TierTemp, consolidate.TierSession:
				// Transient session/temp items are never persisted to user/branch/repo dossiers.
			default:
				ri.Tier = consolidate.TierRepo
				repoRouted = append(repoRouted, ri)
			}
		}
	}

	userDedup := mergeTierExactDuplicates(userRouted, normalizeDossiersForMatching(userDossiers), consolidate.TierUser, now)
	res.ExactDuplicatesAmend += len(userDedup.AmendOps)

	branchSlugs := make([]string, 0, len(branchRouted))
	for slug := range branchRouted {
		branchSlugs = append(branchSlugs, slug)
	}
	sort.Strings(branchSlugs)

	branchDedup := make(map[string]consolidate.DedupResult, len(branchSlugs))
	for _, slug := range branchSlugs {
		dRes := mergeTierExactDuplicates(branchRouted[slug], normalizeDossiersForMatching(branchDossiers[slug]), consolidate.TierBranch, now)
		branchDedup[slug] = dRes
		res.ExactDuplicatesAmend += len(dRes.AmendOps)
	}

	repoDedup := consolidate.MergeExactDuplicates(repoRouted, normalizeDossiersForMatching(repoDossiers), now)
	res.ExactDuplicatesAmend += len(repoDedup.AmendOps)

	brokenByScope := make(map[string][]consolidate.BrokenAnchorReport)
	if strings.TrimSpace(p.repoRoot) != "" {
		touchedRepoScopes := make(map[string]bool)
		for _, ri := range repoDedup.Remaining {
			touchedRepoScopes[ri.DossierScope] = true
		}
		for scope, d := range repoDossiers {
			if opts.Force || touchedRepoScopes[scope] {
				if reports := consolidate.CheckDossierAnchors(p.repoRoot, d); len(reports) > 0 {
					brokenByScope[scope] = reports
				}
			}
		}
	}

	userInputs := buildDossierInputs(consolidate.TierUser, "", userDedup.Remaining, userDossiers, nil, "")
	branchInputs := make(map[string][]consolidate.DossierConsolidationInput, len(branchSlugs))
	for _, slug := range branchSlugs {
		if branchDossiers[slug] == nil {
			branchDossiers[slug] = make(map[string]*dossiers.Dossier)
		}
		branchInputs[slug] = buildDossierInputs(consolidate.TierBranch, slug, branchDedup[slug].Remaining, branchDossiers[slug], nil, "")
	}
	repoInputs := buildDossierInputs(consolidate.TierRepo, "", repoDedup.Remaining, repoDossiers, brokenByScope, atlasSummary)

	rawEst := estimateConsolidationTokens(userInputs)
	for _, slug := range branchSlugs {
		rawEst += estimateConsolidationTokens(branchInputs[slug])
	}
	rawEst += estimateConsolidationTokens(repoInputs)
	if guardSt, gErr := LoadSpendGuardState(p.dataDir, p.repoID); gErr == nil {
		res.EstimatedInputTokens = int(float64(rawEst)*guardSt.CalibrationMultiplier() + 0.5)
	} else {
		res.EstimatedInputTokens = rawEst
	}

	// 8. Dry-Run Handling (opts.DryRun == true).
	if opts.DryRun {
		var previewOps []ops.Op
		previewOps = append(previewOps, userDedup.AmendOps...)
		for _, in := range userInputs {
			previewOps = append(previewOps, consolidate.SynthesizeDeterministicOps(in, now)...)
		}
		for _, slug := range branchSlugs {
			previewOps = append(previewOps, branchDedup[slug].AmendOps...)
			for _, in := range branchInputs[slug] {
				previewOps = append(previewOps, consolidate.SynthesizeDeterministicOps(in, now)...)
			}
		}
		previewOps = append(previewOps, repoDedup.AmendOps...)
		for _, in := range repoInputs {
			previewOps = append(previewOps, consolidate.SynthesizeDeterministicOps(in, now)...)
		}
		tallyOps(res, previewOps)
		return res, nil
	}

	// 9. Step 5 — Multi-Turn stream-json Consolidation (or Deterministic Synthesis when p.hk == nil).
	userLLMOps, uUsage, err := consolidate.ConsolidateDossiers(ctx, p.hk, userInputs, now)
	p.recordUsage(res, uUsage)
	if err != nil {
		return nil, err
	}

	branchLLMOps := make(map[string][]ops.Op, len(branchSlugs))
	for _, slug := range branchSlugs {
		bOps, bUsage, bErr := consolidate.ConsolidateDossiers(ctx, p.hk, branchInputs[slug], now)
		p.recordUsage(res, bUsage)
		if bErr != nil {
			return nil, bErr
		}
		branchLLMOps[slug] = bOps
	}

	repoLLMOps, rUsage, err := consolidate.ConsolidateDossiers(ctx, p.hk, repoInputs, now)
	p.recordUsage(res, rUsage)
	if err != nil {
		return nil, err
	}

	actualTokens := res.Usage.InputTokens + res.Usage.OutputTokens
	p.RecordSpendRun("consolidation", res.EstimatedInputTokens, actualTokens)

	userCandidateOps := append(append([]ops.Op(nil), userDedup.AmendOps...), userLLMOps...)
	repoCandidateOps := append(append([]ops.Op(nil), repoDedup.AmendOps...), repoLLMOps...)

	// 10. Steps 6–7 — Validate, Scrub, Record, Publish Worktree, Rebuild Index & Mark Notes Consolidated.
	scrubCtx := scrub.Context{
		Boundary:   "op",
		RepoID:     p.repoID,
		RepoRoot:   p.repoRoot,
		DataDir:    p.dataDir,
		AllowedIDs: allowedIDsFromNotesAndEpisodes(notes, episodesByConv),
	}

	if len(userCandidateOps) > 0 {
		validUserOps, rejectedUser := consolidate.ValidateAndPrepareOps(userCandidateOps, userDossiers, scrubCtx, now)
		res.OpsRejected += len(rejectedUser)
		tallyOps(res, validUserOps)
		if len(validUserOps) > 0 && strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
			userRec, recErr := consolidate.RecordUserTier(ctx, p.dataDir, p.repoID, validUserOps, nil, nil, now)
			if recErr != nil {
				return nil, recErr
			}
			if userRec != nil && userRec.Batch != nil {
				res.UserBatchID = userRec.Batch.BatchID
			}
		}
	}

	for _, slug := range branchSlugs {
		branchCandidateOps := append(append([]ops.Op(nil), branchDedup[slug].AmendOps...), branchLLMOps[slug]...)
		if len(branchCandidateOps) == 0 {
			continue
		}
		validBranchOps, rejectedBranch := consolidate.ValidateAndPrepareOps(branchCandidateOps, branchDossiers[slug], scrubCtx, now)
		res.OpsRejected += len(rejectedBranch)
		tallyOps(res, validBranchOps)
		if len(validBranchOps) > 0 && strings.TrimSpace(p.repoRoot) != "" {
			branchRec, recErr := consolidate.RecordBranchTier(ctx, p.repoRoot, slug, validBranchOps, nil, nil, now)
			if recErr != nil {
				return nil, recErr
			}
			if branchRec != nil && branchRec.Batch != nil {
				if res.BranchBatches == nil {
					res.BranchBatches = make(map[string]string)
				}
				res.BranchBatches[slug] = branchRec.Batch.BatchID
			}
		}
	}

	batchesToPublish := append([]ops.OpBatch(nil), promotedBatches...)
	if len(repoCandidateOps) > 0 {
		validRepoOps, rejectedRepo := consolidate.ValidateAndPrepareOps(repoCandidateOps, repoDossiers, scrubCtx, now)
		res.OpsRejected += len(rejectedRepo)
		tallyOps(res, validRepoOps)
		if len(validRepoOps) > 0 && strings.TrimSpace(p.repoRoot) != "" {
			repoRec, recErr := consolidate.RecordRepoTier(ctx, p.repoRoot, validRepoOps, nil, nil, now)
			if recErr != nil {
				return nil, recErr
			}
			if repoRec != nil {
				repoDossiers = repoRec.Dossiers
				if repoRec.Batch != nil {
					res.RepoBatchID = repoRec.Batch.BatchID
					batchesToPublish = append(batchesToPublish, *repoRec.Batch)
				}
			}
		}
	}

	for _, n := range notes {
		if n == nil || strings.TrimSpace(n.ID) == "" {
			continue
		}
		if err := inboxStore.MarkConsolidated(n.ID); err != nil {
			return nil, fmt.Errorf("marking note %s consolidated: %w", n.ID, err)
		}
		res.ProcessedNotes++
	}

	if len(batchesToPublish) > 0 && strings.TrimSpace(p.repoRoot) != "" && strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
		if wtRes, wtErr := gitops.PublishToMemoryWorktree(ctx, p.dataDir, p.repoID, p.repoRoot, batchesToPublish); wtErr == nil && wtRes != nil {
			res.WorktreeCommitted = wtRes.Committed
			res.WorktreeCommitSHA = wtRes.CommitSHA
		}
	}

	if err := p.rebuildMultiTierIndex(ctx); err != nil {
		return nil, fmt.Errorf("rebuilding multi-tier index: %w", err)
	}

	p.refreshFreshnessState(ctx, true)

	return res, nil
}

func (p *PipelineExecutor) recordUsage(res *ConsolidationResult, u harness.Usage) {
	res.Usage.InputTokens += u.InputTokens
	res.Usage.OutputTokens += u.OutputTokens
	res.Usage.ThinkingTokens += u.ThinkingTokens
	res.Usage.CacheReadTokens += u.CacheReadTokens
	if u.InputTokens > 0 || u.OutputTokens > 0 {
		cost := jobs.FromUsage(u)
		_ = p.checkBudget(&cost)
	}
}

func (p *PipelineExecutor) rebuildMultiTierIndex(ctx context.Context) error {
	var currentBranch string
	if strings.TrimSpace(p.repoRoot) != "" {
		currentBranch, _ = gitops.GetCurrentBranchWithContext(ctx, p.repoRoot)
	}
	if p.idx != nil {
		return consolidate.RebuildMultiTierIndex(ctx, p.idx, p.dataDir, p.repoID, p.repoRoot, currentBranch)
	}
	if strings.TrimSpace(p.dataDir) == "" || strings.TrimSpace(p.repoID) == "" {
		return nil
	}
	idxPath := index.IndexPath(p.dataDir, p.repoID)
	if err := os.MkdirAll(filepath.Dir(idxPath), 0o755); err != nil {
		return fmt.Errorf("creating index directory: %w", err)
	}
	idx, err := index.NewIndex(idxPath)
	if err != nil {
		return fmt.Errorf("opening index: %w", err)
	}
	defer idx.Close()
	return consolidate.RebuildMultiTierIndex(ctx, idx, p.dataDir, p.repoID, p.repoRoot, currentBranch)
}

func (p *PipelineExecutor) loadEpisodesByConv() map[string]*episode.Episode {
	out := make(map[string]*episode.Episode)
	epStore := p.epStore
	if epStore == nil && strings.TrimSpace(p.dataDir) != "" && strings.TrimSpace(p.repoID) != "" {
		epStore = episode.NewStore(p.dataDir, p.repoID)
	}
	if epStore == nil {
		return out
	}
	eps, err := epStore.ListEpisodes()
	if err != nil {
		return out
	}
	for _, ep := range eps {
		if ep == nil {
			continue
		}
		if sid := strings.TrimSpace(ep.Header.SessionID); sid != "" {
			out[sid] = ep
		}
		if eid := strings.TrimSpace(ep.Header.EpisodeID); eid != "" {
			out[eid] = ep
		}
	}
	return out
}

func projectOpsDir(ctx context.Context, opsDir string) map[string]*dossiers.Dossier {
	if strings.TrimSpace(opsDir) == "" {
		return make(map[string]*dossiers.Dossier)
	}
	info, err := os.Stat(opsDir)
	if err != nil || !info.IsDir() {
		return make(map[string]*dossiers.Dossier)
	}
	projected, err := dossiers.NewProjector(ops.NewReader(opsDir)).Project(ctx)
	if err != nil || projected == nil {
		return make(map[string]*dossiers.Dossier)
	}
	return projected
}

func loadAllBranchDossiers(ctx context.Context, repoRoot string) map[string]map[string]*dossiers.Dossier {
	out := make(map[string]map[string]*dossiers.Dossier)
	if strings.TrimSpace(repoRoot) == "" {
		return out
	}
	localOpsRoot := filepath.Join(repoRoot, ".memory", "local", "ops")
	entries, err := os.ReadDir(localOpsRoot)
	if err != nil {
		return out
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		out[slug] = projectOpsDir(ctx, filepath.Join(localOpsRoot, slug))
	}
	return out
}

func combineAllActiveEntries(
	repoDossiers map[string]*dossiers.Dossier,
	userDossiers map[string]*dossiers.Dossier,
	branchDossiers map[string]map[string]*dossiers.Dossier,
) []dossiers.Entry {
	seen := make(map[string]bool)
	var out []dossiers.Entry
	appendEntries := func(entries []dossiers.Entry) {
		for _, e := range entries {
			if e.ID != "" && seen[e.ID] {
				continue
			}
			if e.ID != "" {
				seen[e.ID] = true
			}
			out = append(out, e)
		}
	}
	appendEntries(consolidate.FlattenActiveEntries(repoDossiers))
	appendEntries(consolidate.FlattenActiveEntries(userDossiers))

	slugs := make([]string, 0, len(branchDossiers))
	for s := range branchDossiers {
		slugs = append(slugs, s)
	}
	sort.Strings(slugs)
	for _, s := range slugs {
		appendEntries(consolidate.FlattenActiveEntries(branchDossiers[s]))
	}
	return out
}

func buildExistingModulesFromDossiers(repoDossiers map[string]*dossiers.Dossier) []config.ModuleScope {
	if len(repoDossiers) == 0 {
		return nil
	}
	scopes := make([]string, 0, len(repoDossiers))
	for scope := range repoDossiers {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	var mods []config.ModuleScope
	for _, scope := range scopes {
		d := repoDossiers[scope]
		if d == nil {
			continue
		}
		if d.Kind != "" && d.Kind != dossiers.DossierKindModule {
			continue
		}
		seenAnchor := make(map[string]bool)
		var pathGlobs []string
		for _, e := range d.Entries {
			if e.Retired || e.SupersededBy != "" {
				continue
			}
			for _, a := range e.Anchors {
				p := consolidate.StripAnchorSymbol(a)
				if p != "" && !seenAnchor[p] {
					seenAnchor[p] = true
					pathGlobs = append(pathGlobs, p)
				}
			}
		}
		mods = append(mods, config.ModuleScope{
			Name:      scope,
			Kind:      dossiers.DossierKindModule,
			PathGlobs: pathGlobs,
		})
	}
	return mods
}

func buildBranchMergedChecker(ctx context.Context, repoRoot string) func(string) bool {
	if strings.TrimSpace(repoRoot) == "" {
		return nil
	}
	mergedSlugs := make(map[string]bool)
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "branch", "--merged", "HEAD")
	if out, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(strings.TrimLeft(line, "*+ "))
			if name != "" {
				mergedSlugs[consolidate.SanitizeBranchSlug(name)] = true
			}
		}
	}
	return func(branch string) bool {
		if consolidate.IsDefaultBranch(branch) {
			return true
		}
		return mergedSlugs[consolidate.SanitizeBranchSlug(branch)]
	}
}

func mergeTierExactDuplicates(
	items []consolidate.RoutedItem,
	tierDossiers map[string]*dossiers.Dossier,
	tier consolidate.ScopeTier,
	now time.Time,
) consolidate.DedupResult {
	if len(items) == 0 {
		return consolidate.DedupResult{}
	}
	normalized := make([]consolidate.RoutedItem, len(items))
	for i, it := range items {
		normalized[i] = it
		normalized[i].Tier = consolidate.TierRepo
	}
	dRes := consolidate.MergeExactDuplicates(normalized, tierDossiers, now)
	for i := range dRes.Remaining {
		dRes.Remaining[i].Tier = tier
	}
	return dRes
}

func buildDossierInputs(
	tier consolidate.ScopeTier,
	branchSlug string,
	remaining []consolidate.RoutedItem,
	existingDossiers map[string]*dossiers.Dossier,
	brokenByScope map[string][]consolidate.BrokenAnchorReport,
	atlasSummary string,
) []consolidate.DossierConsolidationInput {
	candidatesByScope := make(map[string][]consolidate.RoutedItem)
	scopeSet := make(map[string]bool)

	for _, ri := range remaining {
		scope := strings.TrimSpace(ri.DossierScope)
		if scope == "" {
			scope = "general"
			ri.DossierScope = scope
		}
		scopeSet[scope] = true
		candidatesByScope[scope] = append(candidatesByScope[scope], ri)
	}
	for scope, broken := range brokenByScope {
		if len(broken) > 0 {
			scopeSet[scope] = true
		}
	}

	if len(scopeSet) == 0 {
		return nil
	}

	scopes := make([]string, 0, len(scopeSet))
	for scope := range scopeSet {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	inputs := make([]consolidate.DossierConsolidationInput, 0, len(scopes))
	for _, scope := range scopes {
		var existingEntries []dossiers.Entry
		var kind dossiers.DossierKind
		if d := existingDossiers[scope]; d != nil {
			kind = d.Kind
			for _, e := range d.Entries {
				if !e.Retired && e.SupersededBy == "" {
					existingEntries = append(existingEntries, e)
				}
			}
		}
		cands := candidatesByScope[scope]
		if kind == "" && len(cands) > 0 {
			kind = cands[0].DossierKind
		}
		if kind == "" {
			kind = dossiers.DossierKindModule
		}
		inputs = append(inputs, consolidate.DossierConsolidationInput{
			Scope:           scope,
			Kind:            kind,
			Tier:            tier,
			BranchSlug:      branchSlug,
			ExistingEntries: existingEntries,
			CandidateItems:  cands,
			BrokenAnchors:   brokenByScope[scope],
			AtlasSummary:    atlasSummary,
		})
	}
	return inputs
}

func estimateConsolidationTokens(inputs []consolidate.DossierConsolidationInput) int {
	total := 0
	for _, in := range inputs {
		if len(in.CandidateItems) == 0 && len(in.BrokenAnchors) == 0 {
			continue
		}
		if prompt, err := consolidate.BuildConsolidationPrompt(in, "0000000000000000"); err == nil {
			total += len(prompt) / 4
		}
	}
	return total
}

func tallyOps(res *ConsolidationResult, validOps []ops.Op) {
	for _, op := range validOps {
		switch op.Type {
		case ops.OpAdd:
			res.OpsAdded++
		case ops.OpAmend:
			res.OpsAmended++
		case ops.OpSupersede:
			res.OpsSuperseded++
		case ops.OpRetire:
			res.OpsRetired++
		case ops.OpMove:
			res.OpsMoved++
		case ops.OpSplit:
			res.OpsSplit++
		}
	}
}

func countActiveEntries(dossiersMap map[string]*dossiers.Dossier) int {
	return len(consolidate.FlattenActiveEntries(dossiersMap))
}

func normalizeDossiersForMatching(src map[string]*dossiers.Dossier) map[string]*dossiers.Dossier {
	if len(src) == 0 {
		return src
	}
	out := make(map[string]*dossiers.Dossier, len(src))
	for scope, d := range src {
		if d == nil {
			continue
		}
		cp := *d
		cp.Entries = make([]dossiers.Entry, len(d.Entries))
		for i, e := range d.Entries {
			e.Text = stripFirstLineBold(e.Text)
			cp.Entries[i] = e
		}
		out[scope] = &cp
	}
	return out
}

func normalizeBranchDossiersForMatching(src map[string]map[string]*dossiers.Dossier) map[string]map[string]*dossiers.Dossier {
	if len(src) == 0 {
		return src
	}
	out := make(map[string]map[string]*dossiers.Dossier, len(src))
	for slug, m := range src {
		out[slug] = normalizeDossiersForMatching(m)
	}
	return out
}

func stripFirstLineBold(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		lines[i] = strings.ReplaceAll(line, "**", "")
		break
	}
	return strings.Join(lines, "\n")
}

func allowedIDsFromNotesAndEpisodes(notes []*inbox.Note, episodesByConv map[string]*episode.Episode) map[string]bool {
	allowed := make(map[string]bool)
	add := func(id string) {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			allowed[trimmed] = true
		}
	}
	for _, n := range notes {
		if n == nil {
			continue
		}
		add(n.ID)
		add(n.ConversationID)
		add(n.HeadCommit)
		for _, exp := range n.ExposureList {
			add(exp.EntryID)
		}
	}
	for k, ep := range episodesByConv {
		add(k)
		if ep == nil {
			continue
		}
		add(ep.Header.EpisodeID)
		add(ep.Header.SessionID)
		add(ep.Header.HeadStart)
		add(ep.Header.HeadEnd)
		for _, c := range ep.Header.Commits {
			add(c)
		}
		for _, s := range ep.Steps {
			add(s.Commit)
		}
	}
	return allowed
}

// refreshFreshnessState computes and persists the full A6 FreshnessState at <dataDir>/<repoID>/state/freshness.json.
func (p *PipelineExecutor) refreshFreshnessState(ctx context.Context, updateLastConsolidation bool) {
	if strings.TrimSpace(p.dataDir) == "" || strings.TrimSpace(p.repoID) == "" {
		return
	}
	now := p.now()
	st, _ := LoadFreshnessState(p.dataDir, p.repoID)
	if st == nil {
		st = &FreshnessState{}
	}
	if updateLastConsolidation {
		st.LastConsolidationAt = now.Format(time.RFC3339)
	}

	if unconsolidated, err := inbox.NewStore(p.dataDir, p.repoID).ListUnconsolidated(); err == nil {
		st.UnconsolidatedNotes = len(unconsolidated)
	}

	activeCount, suspectCount := countActiveAndSuspectEntries(ctx, p.dataDir, p.repoID, p.repoRoot, p.idx)
	st.ActiveEntries = activeCount
	st.SuspectEntries = suspectCount

	latestBatch := latestOpLogBatchID(ctx, p.dataDir, p.repoID, p.repoRoot)
	st.IndexOpLogBatchID = latestBatch
	st.IndexCurrentToOpLog = true

	headSHA := currentGitHeadSHA(ctx, p.repoRoot)
	st.IndexHeadSHA = headSHA
	st.HeadMovedSinceBuild = false

	st.UpdatedAt = now.Format(time.RFC3339)
	_ = SaveFreshnessState(p.dataDir, p.repoID, st)
}

func countActiveAndSuspectEntries(ctx context.Context, dataDir, repoID, repoRoot string, idx *index.Index) (active int, suspect int) {
	if idx != nil {
		if entries, err := idx.ListAllEntries(ctx); err == nil {
			for _, e := range entries {
				if e.Retired || e.SupersededBy != "" {
					continue
				}
				active++
				if e.StaleSuspect || strings.EqualFold(strings.TrimSpace(e.Verified), "suspect") {
					suspect++
				}
			}
			return active, suspect
		}
	}

	var repoOpsDir string
	if strings.TrimSpace(repoRoot) != "" {
		repoOpsDir = filepath.Join(repoRoot, ".memory", "ops")
	}
	repoDossiers := projectOpsDir(ctx, repoOpsDir)

	var userOpsDir string
	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		userOpsDir = filepath.Join(consolidate.UserTierBaseDir(dataDir, repoID), "ops")
	}
	userDossiers := projectOpsDir(ctx, userOpsDir)

	branchDossiers := loadAllBranchDossiers(ctx, repoRoot)
	allActive := combineAllActiveEntries(repoDossiers, userDossiers, branchDossiers)
	for _, e := range allActive {
		active++
		if e.StaleSuspect || strings.EqualFold(strings.TrimSpace(e.Verified), "suspect") {
			suspect++
		}
	}
	return active, suspect
}

func latestOpLogBatchID(ctx context.Context, dataDir, repoID, repoRoot string) string {
	if ctx == nil {
		ctx = context.Background()
	}
	var dirs []string
	if strings.TrimSpace(repoRoot) != "" {
		dirs = append(dirs, filepath.Join(repoRoot, ".memory", "ops"))
		if currentBranch, err := gitops.GetCurrentBranchWithContext(ctx, repoRoot); err == nil && strings.TrimSpace(currentBranch) != "" {
			dirs = append(dirs, consolidate.BranchTierOpsDir(repoRoot, currentBranch))
		}
	}
	if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		dirs = append(dirs, filepath.Join(consolidate.UserTierBaseDir(dataDir, repoID), "ops"))
	}

	var latestID string
	var latestTS time.Time
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		reader := ops.NewReader(dir)
		for batch, err := range reader.Iterate(ctx) {
			if err != nil || batch == nil {
				continue
			}
			if latestID == "" || batch.CreatedAt.After(latestTS) || (batch.CreatedAt.Equal(latestTS) && batch.BatchID > latestID) {
				latestID = batch.BatchID
				latestTS = batch.CreatedAt
			}
		}
	}
	return latestID
}

func currentGitHeadSHA(ctx context.Context, repoRoot string) string {
	if strings.TrimSpace(repoRoot) == "" {
		return ""
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sha, err := gitops.GetHeadCommitWithContext(ctx, repoRoot)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(sha)
}

func formatCompactAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		secs := int(d.Seconds())
		if secs <= 0 {
			return "0s"
		}
		return fmt.Sprintf("%ds", secs)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// FormatFreshnessLine computes or reads the live A6 freshness state and formats the exact 1-line header
// from Addendum A §7.2:
//
//	memory: <index status> · <HEAD status> · <N> notes not yet consolidated · <M> entries suspect
func FormatFreshnessLine(ctx context.Context, dataDir, repoID, repoRoot string, idx *index.Index, inboxStore *inbox.Store) string {
	if ctx == nil {
		ctx = context.Background()
	}
	st, _ := LoadFreshnessState(dataDir, repoID)
	diskBatchID := latestOpLogBatchID(ctx, dataDir, repoID, repoRoot)

	indexCurrent := true
	if st != nil && strings.TrimSpace(st.UpdatedAt) != "" {
		if !st.IndexCurrentToOpLog {
			indexCurrent = false
		} else if diskBatchID != "" && st.IndexOpLogBatchID != diskBatchID {
			indexCurrent = false
		}
	} else if diskBatchID != "" {
		indexCurrent = false
	}

	indexStatus := "index current to op-log"
	if !indexCurrent {
		ageStr := "unknown"
		refTS := ""
		if st != nil {
			refTS = strings.TrimSpace(st.LastConsolidationAt)
			if refTS == "" {
				refTS = strings.TrimSpace(st.UpdatedAt)
			}
		}
		if refTS != "" {
			if parsed, err := time.Parse(time.RFC3339, refTS); err == nil {
				ageStr = formatCompactAge(time.Since(parsed))
			}
		}
		indexStatus = fmt.Sprintf("index behind op-log (last built %s ago)", ageStr)
	}

	liveHead := currentGitHeadSHA(ctx, repoRoot)
	headMoved := false
	if st != nil && st.HeadMovedSinceBuild {
		headMoved = true
	} else if liveHead != "" && st != nil && strings.TrimSpace(st.IndexHeadSHA) != "" && liveHead != strings.TrimSpace(st.IndexHeadSHA) {
		headMoved = true
	}
	headStatus := "HEAD matches last build"
	if headMoved {
		headStatus = "HEAD moved since last build"
	}

	unconsolidatedCount := 0
	if inboxStore != nil {
		if notes, err := inboxStore.ListUnconsolidated(); err == nil {
			unconsolidatedCount = len(notes)
		}
	} else if strings.TrimSpace(dataDir) != "" && strings.TrimSpace(repoID) != "" {
		if notes, err := inbox.NewStore(dataDir, repoID).ListUnconsolidated(); err == nil {
			unconsolidatedCount = len(notes)
		}
	} else if st != nil {
		unconsolidatedCount = st.UnconsolidatedNotes
	}

	_, suspectCount := countActiveAndSuspectEntries(ctx, dataDir, repoID, repoRoot, idx)
	if suspectCount == 0 && idx == nil && st != nil && st.SuspectEntries > 0 {
		suspectCount = st.SuspectEntries
	}

	return fmt.Sprintf("memory: %s · %s · %d notes not yet consolidated · %d entries suspect",
		indexStatus, headStatus, unconsolidatedCount, suspectCount)
}

