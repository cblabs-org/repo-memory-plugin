/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/tick/consolidate.go
 * Description: Stage 2 warm batch consolidation pipeline (SPEC §6.3.2) and A6
 *              freshness state persistence/formatting (Addendum A §7). Promotes
 *              merged branch ops, filters echoes, enforces corroboration gates,
 *              routes items across user/branch/repo tiers, deduplicates and
 *              consolidates dossiers, publishes to the memory worktree, and
 *              updates the multi-tier SQLite search index.
 *
 * Exported Types / Functions:
 * - DefaultConsolidationMinNotes:                     Minimum unconsolidated notes triggering immediate consolidation.
 * - DefaultConsolidationMaxAge:                       Maximum note wait age before daily consolidation triggers.
 * - ConsolidationOptions:                             Options for a Stage 2 consolidation run.
 * - ProposedScope:                                    Describes a newly derived dossier scope proposed during routing.
 * - ConsolidationResult:                              Summary metrics and batch IDs from a Stage 2 consolidation run.
 * - FreshnessState:                                   Tracks capture, consolidation, and index freshness on disk.
 * - FreshnessStatePath():                             Returns the filesystem path to freshness.json.
 * - LoadFreshnessState():                             Reads freshness.json from disk.
 * - SaveFreshnessState():                             Atomically writes freshness.json to disk.
 * - PipelineExecutor.RunConsolidation():              Executes Stage 2 consolidation with a boolean force flag.
 * - PipelineExecutor.RunConsolidationWithOptions():   Executes the 7-step Stage 2 warm batch consolidation pipeline.
 * - PipelineExecutor.RefreshFreshnessState():         Computes and persists the A6 FreshnessState on disk.
 * - FormatFreshnessLine():                            Formats the 1-line A6 freshness status header.
 */

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
	Force  bool `json:"force"`   // When true, bypasses cadence and spend guard interval checks.
	DryRun bool `json:"dry_run"` // When true, previews deterministic operations without mutating disk or calling the LLM.
}

// ProposedScope describes a newly derived dossier scope proposed during routing.
type ProposedScope struct {
	Scope string               `json:"scope"`          // Slash-separated module or topic scope slug (e.g. "core/consolidate").
	Kind  dossiers.DossierKind `json:"kind,omitempty"` // Dossier kind (e.g. module, topic).
}

// ConsolidationResult summarizes the outcome of a Stage 2 warm batch consolidation run.
type ConsolidationResult struct {
	Skipped              bool              `json:"skipped"`                       // Whether consolidation was skipped due to empty inbox, cadence, or quota.
	SkipReason           string            `json:"skip_reason,omitempty"`         // Machine-readable reason consolidation was skipped.
	DryRun               bool              `json:"dry_run"`                       // Whether the run executed in dry-run preview mode.
	PromotedBranches     []string          `json:"promoted_branches,omitempty"`   // Branch slugs whose local ops were promoted to the repo tier.
	UnconsolidatedNotes  int               `json:"unconsolidated_notes"`          // Total unconsolidated notes found in the inbox before processing.
	ProcessedNotes       int               `json:"processed_notes"`               // Number of inbox notes marked consolidated during the run.
	CandidateItems       int               `json:"candidate_items"`               // Total note items inspected across the batch.
	EchoesDropped        int               `json:"echoes_dropped"`                // Number of note items dropped by echo suppression.
	DowngradedToIssue    int               `json:"downgraded_to_issue"`           // Number of uncorroborated items downgraded to open-issue.
	ExactDuplicatesAmend int               `json:"exact_duplicates_amend"`        // Number of exact duplicate items merged via deterministic OpAmend.
	ProposedScopes       []ProposedScope   `json:"proposed_scopes,omitempty"`     // Newly derived scopes encountered during routing.
	OpsAdded             int               `json:"ops_added"`                     // Count of OpAdd operations recorded.
	OpsAmended           int               `json:"ops_amended"`                   // Count of OpAmend operations recorded.
	OpsSuperseded        int               `json:"ops_superseded"`                // Count of OpSupersede operations recorded.
	OpsRetired           int               `json:"ops_retired"`                   // Count of OpRetire operations recorded.
	OpsMoved             int               `json:"ops_moved"`                     // Count of OpMove operations recorded.
	OpsSplit             int               `json:"ops_split"`                     // Count of OpSplit operations recorded.
	OpsRejected          int               `json:"ops_rejected"`                  // Count of candidate operations rejected by validation or scrubbing.
	UserBatchID          string            `json:"user_batch_id,omitempty"`       // Recorded op-batch ID for the user tier, if any.
	BranchBatches        map[string]string `json:"branch_batches,omitempty"`      // Recorded op-batch IDs keyed by branch slug.
	RepoBatchID          string            `json:"repo_batch_id,omitempty"`       // Recorded op-batch ID for the shared repo tier, if any.
	WorktreeCommitted    bool              `json:"worktree_committed"`            // Whether batches were committed to the isolated memory worktree.
	WorktreeCommitSHA    string            `json:"worktree_commit_sha,omitempty"` // Git commit SHA created in the memory worktree.
	Usage                harness.Usage     `json:"usage"`                         // Cumulative LLM token usage across consolidation turns.
	EstimatedInputTokens int               `json:"estimated_input_tokens"`        // Calibrated pre-run input token estimate.
}

// FreshnessState tracks capture, consolidation, and index freshness at <dataDir>/<repoID>/state/freshness.json (Addendum A §7).
type FreshnessState struct {
	LastCaptureAt       string `json:"last_capture_at,omitempty"`       // RFC3339 timestamp of the last episode capture.
	LastConsolidationAt string `json:"last_consolidation_at,omitempty"` // RFC3339 timestamp of the last completed consolidation run.
	UnconsolidatedNotes int    `json:"unconsolidated_notes"`            // Number of inbox notes awaiting consolidation.
	ActiveEntries       int    `json:"active_entries"`                  // Total active (non-retired, non-superseded) dossier entries.
	IndexOpLogBatchID   string `json:"index_oplog_batch_id,omitempty"`  // Latest op-log batch ID incorporated into the SQLite index.
	IndexCurrentToOpLog bool   `json:"index_current_to_oplog"`          // Whether the SQLite index matches the latest op-log batch on disk.
	IndexHeadSHA        string `json:"index_head_sha,omitempty"`        // Git HEAD commit SHA at the time the index was last built.
	HeadMovedSinceBuild bool   `json:"head_moved_since_build"`          // Whether git HEAD has moved since IndexHeadSHA was recorded.
	SuspectEntries      int    `json:"suspect_entries"`                 // Count of active entries flagged as suspect/stale.
	UpdatedAt           string `json:"updated_at"`                      // RFC3339 timestamp when this state record was last updated.
}

// FreshnessStatePath returns the path to <dataDir>/<repoID>/state/freshness.json.
func FreshnessStatePath(dataDir, repoID string) string {
	return filepath.Join(dataDir, repoID, "state", "freshness.json")
}

// LoadFreshnessState reads <dataDir>/<repoID>/state/freshness.json.
// If the file does not exist, it returns an empty FreshnessState without error.
func LoadFreshnessState(dataDir, repoID string) (*FreshnessState, error) {
	// --- Step 1: Validate inputs and read freshness.json from disk ---
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

	// --- Step 2: Unmarshal JSON payload ---
	var st FreshnessState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("unmarshaling freshness state %s: %w", path, err)
	}
	return &st, nil
}

// SaveFreshnessState atomically writes st to <dataDir>/<repoID>/state/freshness.json.
func SaveFreshnessState(dataDir, repoID string, st *FreshnessState) error {
	// --- Step 1: Validate inputs and ensure parent directory exists ---
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

	// --- Step 2: Serialize state to formatted JSON ---
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling freshness state: %w", err)
	}

	// --- Step 3: Write to a temporary file and atomically rename into place ---
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

	// --- Step 1: Promote merged branch operations to repo tier (!opts.DryRun) ---
	var promotedBatches []ops.OpBatch
	if !opts.DryRun && strings.TrimSpace(p.repoRoot) != "" {
		if promoRes, promoErr := consolidate.PromoteMergedBranchOps(ctx, p.repoRoot); promoErr == nil && promoRes != nil {
			res.PromotedBranches = promoRes.PromotedBranches
			promotedBatches = promoRes.PromotedBatches
		}
	}

	// --- Step 2: Load unconsolidated inbox notes ---
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

	// --- Step 3: Evaluate cadence and A5 spend guards (when !opts.Force && !opts.DryRun) ---
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

	// --- Step 4: Verify quota headroom and daily token budget ---
	if !opts.DryRun && p.hk != nil {
		if !p.checkQuotaHeadroom(ctx) || p.checkBudget(nil) != nil {
			res.Skipped = true
			res.SkipReason = "quota_or_budget_exhausted"
			return res, nil
		}
	}

	// --- Step 5: Load existing dossiers across tiers and episodes by conversation ---
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

	// --- Step 6: Filter echoes and enforce corroboration gate ---
	type noteSurvivors struct {
		note  *inbox.Note      // Source inbox note.
		items []inbox.NoteItem // Surviving note items after echo and corroboration filtering.
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

	// --- Step 7: Route items across tiers, merge exact duplicates, and check anchors ---
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

	// --- Step 8: Return deterministic preview when opts.DryRun is enabled ---
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

	// --- Step 9: Run multi-turn stream-json LLM consolidation (or deterministic synthesis when p.hk == nil) ---
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

	// --- Step 10: Validate, scrub, record ops, publish worktree, rebuild index, and mark notes consolidated ---
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

// recordUsage accumulates LLM token usage into res.Usage and updates the executor's budget tracker.
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

// rebuildMultiTierIndex rebuilds the SQLite FTS5 index across repo, branch, and user tiers.
func (p *PipelineExecutor) rebuildMultiTierIndex(ctx context.Context) error {
	// --- Step 1: Resolve current git branch and reuse p.idx if already open ---
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

	// --- Step 2: Open index on disk and rebuild across all tiers ---
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

// loadEpisodesByConv loads all episodes from the episode store indexed by both SessionID and EpisodeID.
func (p *PipelineExecutor) loadEpisodesByConv() map[string]*episode.Episode {
	// --- Step 1: Resolve episode store and list all episodes ---
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

	// --- Step 2: Index episodes by SessionID and EpisodeID ---
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

// projectOpsDir projects dossiers from opsDir, returning an empty map when opsDir does not exist.
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

// loadAllBranchDossiers projects branch-tier dossiers for every branch subdirectory under .memory/local/ops/.
func loadAllBranchDossiers(ctx context.Context, repoRoot string) map[string]map[string]*dossiers.Dossier {
	// --- Step 1: Read branch subdirectories under .memory/local/ops/ ---
	out := make(map[string]map[string]*dossiers.Dossier)
	if strings.TrimSpace(repoRoot) == "" {
		return out
	}
	localOpsRoot := filepath.Join(repoRoot, ".memory", "local", "ops")
	entries, err := os.ReadDir(localOpsRoot)
	if err != nil {
		return out
	}

	// --- Step 2: Project each branch ops directory ---
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		slug := e.Name()
		out[slug] = projectOpsDir(ctx, filepath.Join(localOpsRoot, slug))
	}
	return out
}

// combineAllActiveEntries flattens and deduplicates active entries across repo, user, and branch dossiers.
func combineAllActiveEntries(
	repoDossiers map[string]*dossiers.Dossier,
	userDossiers map[string]*dossiers.Dossier,
	branchDossiers map[string]map[string]*dossiers.Dossier,
) []dossiers.Entry {
	// --- Step 1: Set up deduplicating entry collector ---
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

	// --- Step 2: Append active entries from repo, user, and sorted branch tiers ---
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

// buildExistingModulesFromDossiers constructs ModuleScope descriptors from active anchors in repoDossiers.
func buildExistingModulesFromDossiers(repoDossiers map[string]*dossiers.Dossier) []config.ModuleScope {
	// --- Step 1: Collect and sort dossier scopes ---
	if len(repoDossiers) == 0 {
		return nil
	}
	scopes := make([]string, 0, len(repoDossiers))
	for scope := range repoDossiers {
		scopes = append(scopes, scope)
	}
	sort.Strings(scopes)

	// --- Step 2: Extract unique anchor file paths for each module dossier ---
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

// buildBranchMergedChecker returns a predicate reporting whether a branch is merged into HEAD or is the default branch.
func buildBranchMergedChecker(ctx context.Context, repoRoot string) func(string) bool {
	// --- Step 1: Query git branch --merged HEAD for merged branch names ---
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

	// --- Step 2: Return closure checking default branch or sanitized merged set ---
	return func(branch string) bool {
		if consolidate.IsDefaultBranch(branch) {
			return true
		}
		return mergedSlugs[consolidate.SanitizeBranchSlug(branch)]
	}
}

// mergeTierExactDuplicates merges exact duplicate routed items against tierDossiers while preserving tier assignment.
func mergeTierExactDuplicates(
	items []consolidate.RoutedItem,
	tierDossiers map[string]*dossiers.Dossier,
	tier consolidate.ScopeTier,
	now time.Time,
) consolidate.DedupResult {
	// --- Step 1: Temporarily normalize item tier to TierRepo for MergeExactDuplicates ---
	if len(items) == 0 {
		return consolidate.DedupResult{}
	}
	normalized := make([]consolidate.RoutedItem, len(items))
	for i, it := range items {
		normalized[i] = it
		normalized[i].Tier = consolidate.TierRepo
	}

	// --- Step 2: Run exact deduplication and restore original tier on remaining items ---
	dRes := consolidate.MergeExactDuplicates(normalized, tierDossiers, now)
	for i := range dRes.Remaining {
		dRes.Remaining[i].Tier = tier
	}
	return dRes
}

// buildDossierInputs groups remaining candidate items and broken anchor reports by scope into DossierConsolidationInput payloads.
func buildDossierInputs(
	tier consolidate.ScopeTier,
	branchSlug string,
	remaining []consolidate.RoutedItem,
	existingDossiers map[string]*dossiers.Dossier,
	brokenByScope map[string][]consolidate.BrokenAnchorReport,
	atlasSummary string,
) []consolidate.DossierConsolidationInput {
	// --- Step 1: Group candidate items and broken anchor scopes ---
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

	// --- Step 2: Sort scopes and assemble DossierConsolidationInput structs ---
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

// estimateConsolidationTokens estimates raw input tokens across all consolidation prompts using len(prompt)/4.
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

// tallyOps increments operation counters on res by op type.
func tallyOps(res *ConsolidationResult, validOps []ops.Op) {
	// --- Step 1: Increment per-type operation counters on res ---
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

// countActiveEntries returns the number of active entries across dossiersMap.
func countActiveEntries(dossiersMap map[string]*dossiers.Dossier) int {
	return len(consolidate.FlattenActiveEntries(dossiersMap))
}

// normalizeDossiersForMatching clones src and strips markdown bold markers from the first line of each entry's text.
func normalizeDossiersForMatching(src map[string]*dossiers.Dossier) map[string]*dossiers.Dossier {
	// --- Step 1: Short-circuit empty dossier maps ---
	if len(src) == 0 {
		return src
	}

	// --- Step 2: Deep-copy entries and strip first-line bold formatting ---
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

// normalizeBranchDossiersForMatching applies normalizeDossiersForMatching across all branch dossier maps.
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

// stripFirstLineBold removes "**" markdown bold delimiters from the first non-empty line of text.
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

// allowedIDsFromNotesAndEpisodes collects known note, conversation, exposure, episode, and commit IDs exempt from scrubbing.
func allowedIDsFromNotesAndEpisodes(notes []*inbox.Note, episodesByConv map[string]*episode.Episode) map[string]bool {
	// --- Step 1: Collect IDs from inbox notes and exposures ---
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

	// --- Step 2: Collect IDs and commit SHAs from episodes ---
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

// RefreshFreshnessState computes and persists the full A6 FreshnessState at <dataDir>/<repoID>/state/freshness.json.
func (p *PipelineExecutor) RefreshFreshnessState(ctx context.Context, updateLastConsolidation bool) {
	p.refreshFreshnessState(ctx, updateLastConsolidation)
}

// refreshFreshnessState computes and persists the full A6 FreshnessState at <dataDir>/<repoID>/state/freshness.json.
func (p *PipelineExecutor) refreshFreshnessState(ctx context.Context, updateLastConsolidation bool) {
	// --- Step 1: Load existing freshness state and update consolidation timestamp if requested ---
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

	// --- Step 2: Recompute unconsolidated note count, active/suspect entry counts, op-log batch ID, and HEAD SHA ---
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

	// --- Step 3: Persist updated FreshnessState to disk ---
	st.UpdatedAt = now.Format(time.RFC3339)
	_ = SaveFreshnessState(p.dataDir, p.repoID, st)
}

// countActiveAndSuspectEntries counts active and suspect entries from idx if available, or by projecting op-log directories.
func countActiveAndSuspectEntries(ctx context.Context, dataDir, repoID, repoRoot string, idx *index.Index) (active int, suspect int) {
	// --- Step 1: Query SQLite index directly when available ---
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

	// --- Step 2: Fall back to projecting repo, user, and branch op logs ---
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

// latestOpLogBatchID scans repo, active branch, and user op-log directories for the newest batch ID.
func latestOpLogBatchID(ctx context.Context, dataDir, repoID, repoRoot string) string {
	// --- Step 1: Collect candidate op-log directories across tiers ---
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

	// --- Step 2: Iterate batches and track the most recent BatchID ---
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

// currentGitHeadSHA returns the trimmed HEAD commit SHA of repoRoot, or "" on error.
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

// formatCompactAge formats duration d as a compact human-readable unit string ("0s", "5m", "3h", "2d").
func formatCompactAge(d time.Duration) string {
	// --- Step 1: Clamp negative durations and format by unit threshold ---
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
	// --- Step 1: Load persisted freshness state and compare index against latest op-log batch ---
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

	// --- Step 2: Compare live git HEAD SHA against IndexHeadSHA ---
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

	// --- Step 3: Count unconsolidated inbox notes and suspect entries ---
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

	// --- Step 4: Format the canonical 1-line A6 freshness status header ---
	return fmt.Sprintf("memory: %s · %s · %d notes not yet consolidated · %d entries suspect",
		indexStatus, headStatus, unconsolidatedCount, suspectCount)
}
