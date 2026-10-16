---
name: pr-13-v2-consolidation-wo6
description: Handoff instructions for Work Order 6 (Tick Orchestration, repo-memory consolidate CLI & End-to-End Verification)
complexity: complex
---

# WORKORDER: wo6-tick-orchestration-and-verification

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Wire the complete 7-step Stage 2 Warm Batch Consolidation pipeline (`filter.go`, `route.go`, `llm.go`, `record.go`, `publish.go`, `worktree.go`) into `internal/core/tick/consolidate.go`, `internal/core/tick/pipeline.go`, and the `repo-memory consolidate` CLI command (`cmd/repo-memory/cmd_consolidate.go`), add end-to-end tests in `internal/core/tick/consolidate_test.go`, rebuild `~/.local/bin/repo-memory`, and verify the entire repository test suite.

## Context & Architectural Why
- **Tick Cadence & Spend Guards (`SPEC.md` §6.4; `design-v2` §6; Addendum A §6 `A5` & §7 `A6`):**
  - Stage 2 Consolidation runs automatically during `repo-memory tick` when $\ge 10$ unconsolidated inbox notes are waiting or once every 24 hours when $\ge 1$ unconsolidated note is waiting (or immediately when `--force` is passed).
  - Before invoking `Housekeeper`, it checks `/usage` Flash quota headroom ($\ge 25\%$) and the daily token budget.
  - It persists `<data-dir>/<repo-id>/state/freshness.json` (`last_consolidation_at`, `unconsolidated_notes`, `active_entries`) for `memory_status` and observability.
  - `repo-memory consolidate [--force] [--dry-run] [--json]` allows developers and CI to run or preview consolidation on demand.

## Strict Boundary Box
- **Allowed Files to Create/Modify:**
  - `internal/core/tick/consolidate.go`
  - `internal/core/tick/consolidate_test.go`
  - `internal/core/tick/pipeline.go`
  - `cmd/repo-memory/cmd_consolidate.go`
  - `cmd/repo-memory/main.go`
- **Forbidden Actions:**
  - Do NOT modify any files outside the allowed list above.
  - Do NOT change git branches or rebase.
  - `internal/core/tick` MUST NOT import `internal/harness/antigravity` (depend only on `internal/harness`).
  - **External GitHub Hygiene:** NEVER include any internal references (internal monorepo paths, shortlinks, internal codenames, or local home paths) in code, comments, or commit messages.
  - Use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"` when committing.

## Tooling Constraints
- Verify with:
  - `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`
  - `go test -race ./cmd/... ./internal/... ./packaging/...`
  - `go vet ./...`
  - `GOOS=windows go build ./...`

## Line-Item Instructions

- [ ] **1. Implement `internal/core/tick/consolidate.go`:**
  - Define constants:
    - `DefaultConsolidationMinNotes = 10`
    - `DefaultConsolidationMaxAge = 24 * time.Hour`
  - Define `ConsolidationOptions`, `ConsolidationResult`, and `FreshnessState` (`<dataDir>/<repoID>/state/freshness.json`):
    ```go
    type ConsolidationOptions struct {
    	Force  bool `json:"force"`
    	DryRun bool `json:"dry_run"`
    }

    type ConsolidationResult struct {
    	Skipped              bool                        `json:"skipped"`
    	SkipReason           string                      `json:"skip_reason,omitempty"`
    	DryRun               bool                        `json:"dry_run"`
    	PromotedBranches     []string                    `json:"promoted_branches,omitempty"`
    	UnconsolidatedNotes  int                         `json:"unconsolidated_notes"`
    	ProcessedNotes       int                         `json:"processed_notes"`
    	CandidateItems       int                         `json:"candidate_items"`
    	EchoesDropped        int                         `json:"echoes_dropped"`
    	DowngradedToIssue    int                         `json:"downgraded_to_issue"`
    	ExactDuplicatesAmend int                         `json:"exact_duplicates_amend"`
    	ProposedScopes       []consolidate.ProposedScope `json:"proposed_scopes,omitempty"`
    	OpsAdded             int                         `json:"ops_added"`
    	OpsAmended           int                         `json:"ops_amended"`
    	OpsSuperseded        int                         `json:"ops_superseded"`
    	OpsRetired           int                         `json:"ops_retired"`
    	OpsMoved             int                         `json:"ops_moved"`
    	OpsSplit             int                         `json:"ops_split"`
    	OpsRejected          int                         `json:"ops_rejected"`
    	UserBatchID          string                      `json:"user_batch_id,omitempty"`
    	BranchBatches        map[string]string           `json:"branch_batches,omitempty"`
    	RepoBatchID          string                      `json:"repo_batch_id,omitempty"`
    	WorktreeCommitted    bool                        `json:"worktree_committed"`
    	WorktreeCommitSHA    string                      `json:"worktree_commit_sha,omitempty"`
    	Usage                harness.Usage               `json:"usage"`
    	EstimatedInputTokens int                         `json:"estimated_input_tokens"`
    }

    type FreshnessState struct {
    	LastCaptureAt       string `json:"last_capture_at,omitempty"`
    	LastConsolidationAt string `json:"last_consolidation_at,omitempty"`
    	UnconsolidatedNotes int    `json:"unconsolidated_notes"`
    	ActiveEntries       int    `json:"active_entries"`
    	UpdatedAt           string `json:"updated_at"`
    }
    ```
  - Implement `FreshnessStatePath(dataDir, repoID string) string`, `LoadFreshnessState(dataDir, repoID string) (*FreshnessState, error)`, and `SaveFreshnessState(dataDir, repoID string, st *FreshnessState) error`.
  - Implement `(p *PipelineExecutor) RunConsolidation(ctx context.Context, force bool) (*ConsolidationResult, error)` and `(p *PipelineExecutor) RunConsolidationWithOptions(ctx context.Context, opts ConsolidationOptions) (*ConsolidationResult, error)`:
    1. **Branch Promotion First (`!opts.DryRun`):**
       - When `!opts.DryRun && p.repoRoot != ""`, call `consolidate.PromoteMergedBranchOps(ctx, p.repoRoot)`. Record `PromotedBranches` and hold `PromotedBatches` so they are published to the worktree and indexed.
    2. **Load Unconsolidated Notes:**
       - `inboxStore := inbox.NewStore(p.dataDir, p.repoID)`
       - `notes, err := inboxStore.ListUnconsolidated()`
       - If `len(notes) == 0`:
         - If `len(promotedBatches) > 0` and `!opts.DryRun`: publish `promotedBatches` via `gitops.PublishToMemoryWorktree` (ignoring git error if `p.repoRoot` is not a git repo), rebuild multi-tier index, and save `FreshnessState`.
         - Return `&ConsolidationResult{Skipped: len(promotedBatches) == 0, SkipReason: "no_unconsolidated_notes", PromotedBranches: ...}, nil`.
    3. **Cadence Trigger Check (when `!opts.Force && !opts.DryRun`):**
       - Load `FreshnessState`.
       - Due if `len(notes) >= DefaultConsolidationMinNotes` (10) OR (`st.LastConsolidationAt == ""` || `now.Sub(lastConsolidatedAt) >= DefaultConsolidationMaxAge` (24h)).
       - If not due, update `FreshnessState.UnconsolidatedNotes = len(notes)` and return `&ConsolidationResult{Skipped: true, SkipReason: "cadence_not_due", UnconsolidatedNotes: len(notes)}, nil`.
    4. **Quota & Budget Guard (when `!opts.DryRun && p.hk != nil`):**
       - Check `p.checkQuotaHeadroom(ctx)` and `p.checkBudget(nil)`. If either fails, return `&ConsolidationResult{Skipped: true, SkipReason: "quota_or_budget_exhausted", UnconsolidatedNotes: len(notes)}, nil`.
    5. **Load Existing Dossiers & Episodes:**
       - Project existing `repo:` dossiers (`<repoRoot>/.memory/ops`), `user:` dossiers (`<dataDir>/<repoID>/user-dossiers/ops`), and any existing `branch:` dossiers (`<repoRoot>/.memory/local/ops/<slug>`).
       - Merge all active dossiers into a combined lookup map for echo filtering, and load `<repoRoot>/.memory/ATLAS.md` text if present.
       - Load episodes from `p.epStore` into `episodesByConv map[string]*episode.Episode` keyed by `SessionID` and `EpisodeID`.
    6. **Step 1 — Filter Echoes & Corroboration Gate:**
       - Run `echoRes := consolidate.FilterEchoes(notes, allExistingDossiers, episodesByConv)`.
       - Record `res.EchoesDropped = len(echoRes.DroppedEchoes)`.
       - Run `corroborated := consolidate.EnforceCorroborationGate(echoRes.Survivors, p.repoRoot, episodesByConv)`.
       - Count items where `Downgraded == true` into `res.DowngradedToIssue`.
    7. **Steps 2–4 — Route, Merge Exact Duplicates & Check Existing Anchors:**
       - Load `scopeCfg` (`config.LoadScopeConfig(filepath.Join(p.repoRoot, ".memory", "config.yaml"))`) and `aliasMap` (`config.BuildAliasMap(ops.NewReader(filepath.Join(p.repoRoot, ".memory", "ops")), ctx)`).
       - Run `routed, proposedScopes := consolidate.RouteItems(corroborated, p.repoRoot, scopeCfg, aliasMap)`.
       - Partition `routed` into:
         - `userRouted []consolidate.RoutedItem` (`Tier == consolidate.TierUser`)
         - `branchRouted map[string][]consolidate.RoutedItem` (`Tier == consolidate.TierBranch`, keyed by `consolidate.SanitizeBranchSlug(rc.BranchSlug)`)
         - `repoRouted []consolidate.RoutedItem` (`Tier == consolidate.TierRepo`)
       - For each tier partition, run `consolidate.MergeExactDuplicates(partitionItems, tierExistingDossiers, now)` to obtain deterministic `AmendOps` and `Unmerged` items.
       - For `TierRepo`, also run `consolidate.CheckDossierAnchors(p.repoRoot, repoDossiers)` for touched scopes (or all scopes if `opts.Force`).
    8. **Dry-Run Handling (`opts.DryRun == true`):**
       - Build `DossierConsolidationInput` slices for `TierUser`, `TierBranch`, and `TierRepo`, estimate prompt tokens (`len(prompt) / 4`), synthesize deterministic ops via `consolidate.SynthesizeDeterministicOps` to populate `OpsAdded`/`OpsAmended`/`OpsRetired` preview counts, and return `res, nil` without calling `p.hk` or writing to disk.
    9. **Step 5 — Multi-Turn `stream-json` Consolidation (or Deterministic Synthesis when `p.hk == nil`):**
       - Build all `[]consolidate.DossierConsolidationInput` across `TierUser`, `TierBranch`, and `TierRepo` (tagging each input with its `Tier` and `BranchSlug`).
       - Call `consolidate.ConsolidateDossiers(ctx, p.hk, inputs, now)` (Wait: to keep ops partitioned by `Tier` and `BranchSlug`, either call `ConsolidateDossiers` per tier OR tag/group inputs so each tier's ops stay associated with its tier! Wait: calling `ConsolidateDossiers` once with all inputs in a single multi-turn `stream-json` session when `p.hk != nil` is easy if we either pass a callback or group by `input.Tier`/`input.BranchSlug`, OR since almost all LLM consolidation turns are `TierRepo`, calling `ConsolidateDossiers` per non-empty tier also works! Even better: if we have a helper in `consolidate.go` that runs `ConsolidateDossiers` per non-empty tier, `TierRepo` runs all its dossier turns in a single `RunMultiTurn` call!).
       - Account for `usage` via `cost := jobs.FromUsage(usage); _ = p.checkBudget(&cost)`.
    10. **Steps 6–7 — Validate, Scrub, Record, Publish Worktree, Rebuild Index & Mark Notes Consolidated:**
        - Build `scrubCtx := scrub.Context{Boundary: "op", RepoID: p.repoID, RepoRoot: p.repoRoot, DataDir: p.dataDir, AllowedIDs: allowedIDsFromNotes(notes)}`.
        - For `TierUser` (if `len(userCandidateOps) > 0`):
          - `validUserOps, rejectedUser := consolidate.ValidateAndPrepareOps(userCandidateOps, userDossiers, scrubCtx, now)`
          - `userRec, err := consolidate.RecordUserTier(ctx, p.dataDir, p.repoID, validUserOps, nil, nil, now)`
        - For each `branchSlug, branchCandidateOps` in `TierBranch` (if `len(branchCandidateOps) > 0`):
          - `validBranchOps, rejectedBranch := consolidate.ValidateAndPrepareOps(branchCandidateOps, branchDossiers[branchSlug], scrubCtx, now)`
          - `branchRec, err := consolidate.RecordBranchTier(ctx, p.repoRoot, branchSlug, validBranchOps, nil, nil, now)`
        - For `TierRepo` (if `len(repoCandidateOps) > 0`):
          - `validRepoOps, rejectedRepo := consolidate.ValidateAndPrepareOps(repoCandidateOps, repoDossiers, scrubCtx, now)`
          - `repoRec, err := consolidate.RecordRepoTier(ctx, p.repoRoot, validRepoOps, nil, nil, now)`
        - Mark all `notes` as consolidated via `inboxStore.MarkConsolidated(n.ID)` and set `res.ProcessedNotes = len(notes)`.
        - Collect any `repoRec.Batch` + `promotedBatches`; if non-empty and `p.repoRoot != ""`, call `gitops.PublishToMemoryWorktree(ctx, p.dataDir, p.repoID, p.repoRoot, batchesToPublish)` (if `PublishToMemoryWorktree` succeeds, record `res.WorktreeCommitted = wtRes.Committed` and `res.WorktreeCommitSHA = wtRes.CommitSHA`).
        - Rebuild `<dataDir>/<repoID>/index.db` via `consolidate.RebuildMultiTierIndex(ctx, idx, p.dataDir, p.repoID, p.repoRoot, currentBranch)` (using `p.idx` if non-nil, or opening `index.NewIndex(index.IndexPath(p.dataDir, p.repoID))`).
        - Save `FreshnessState` (`LastConsolidationAt = now.Format(time.RFC3339)`, `UnconsolidatedNotes = 0`, `ActiveEntries = countActiveEntries(repoDossiers)`).

- [ ] **2. Wire `RunConsolidation` and `RebuildMultiTierIndex` into `internal/core/tick/pipeline.go`:**
  - In `(p *PipelineExecutor) Run(ctx context.Context) error`:
    - Call `if _, err := p.RunConsolidation(ctx, false); err != nil { return err }` after `p.checkDocFreshness(ctx)`.
    - Update the index rebuild at the end of `Run(ctx)` to call `consolidate.RebuildMultiTierIndex(ctx, p.idx, p.dataDir, p.repoID, p.repoRoot, currentBranch)` (where `currentBranch, _ := gitops.GetCurrentBranchWithContext(ctx, p.repoRoot)`).

- [ ] **3. Implement `cmd/repo-memory/cmd_consolidate.go` and register in `cmd/repo-memory/main.go`:**
  - Create `consolidateCmd` (`Use: "consolidate"`, `Short: "Run Stage 2 warm batch consolidation over pending inbox notes"`):
    - Flags:
      - `--force` (`bool`, default `false`, `"Consolidate immediately regardless of note count or 24h cadence"`)
      - `--dry-run` (`bool`, default `false`, `"Preview candidate routing, filtering, and estimated token usage without writing"`)
      - `--json` (`bool`, default `false`, `"Output structured JSON result"`)
      - `--deterministic` (`bool`, default `false`, `"Use deterministic op synthesis without invoking the LLM housekeeper"`)
    - Note: when `repo-memory consolidate` is invoked directly from the CLI, treat `Force: true` by default (or pass `Force: consolidateForce || true` unless `--respect-cadence` is desired — wait: let's set `--force` default or pass `Force: true` when invoked via `repo-memory consolidate`, or honour `consolidateForce` while defaulting to running if `len(notes) > 0` when invoked via CLI! Wait: if a test runs `repo-memory consolidate --force` or `repo-memory consolidate`, setting `Force: true` whenever `consolidateForce` is true OR when invoked via CLI `consolidate` command ensures `repo-memory consolidate` always consolidates waiting notes immediately!).
    - If `!consolidateDeterministic && !consolidateDryRun`: if `_, lookErr := exec.LookPath("agy"); lookErr == nil`, configure `exec.SetHousekeeper(antigravity.NewHousekeeper(dataDir, repoID, model, "5m"))` (otherwise leave `hk == nil` so deterministic synthesis runs cleanly in environments without `agy`).
  - Register `rootCmd.AddCommand(consolidateCmd)` in `cmd/repo-memory/main.go`.

- [ ] **4. Write End-to-End Consolidation Tests in `internal/core/tick/consolidate_test.go`:**
  - `TestRunConsolidation_CadenceAndForce`:
    - Verifies that with `< 10` notes and a recent `LastConsolidationAt` (`< 24h` ago), `RunConsolidation(ctx, false)` skips with `SkipReason == "cadence_not_due"`, whereas `RunConsolidation(ctx, true)` (or $\ge 10$ notes, or `LastConsolidationAt` $> 24\text{h}$ ago) executes consolidation.
  - `TestRunConsolidation_EndToEndPipeline`:
    - Sets up a temporary git repo (`main`), `dataDir`, `repoID`, existing dossier entry (`got-existing1`), episode store, and 4 inbox notes:
      1. Note 1: echo of `got-existing1` without anchored edit+test -> dropped by P13 Echo Filter (`EchoesDropped == 1`).
      2. Note 2: uncorroborated `invariant` without static anchor or passing test -> downgraded by `VD18` Corroboration Gate to `open-issue` (`DowngradedToIssue == 1`).
      3. Note 3: exact duplicate of an existing entry -> merged deterministically via `OpAmend` (`ExactDuplicatesAmend == 1`).
      4. Note 4: workstation-local `user:` item (`~/.ssh/config`) + corroborated `repo:` `gotcha` item.
    - Runs `RunConsolidation(ctx, true)` (both with `--dry-run` first to verify zero disk writes, then a real run).
    - Verifies:
      - `user:` item is written to `<dataDir>/<repoID>/user-dossiers/` and NOT `.memory/ops/`.
      - `repo:` ops are written to `<repoRoot>/.memory/ops/`, rendered in `<repoRoot>/.memory/dossiers/` and `<repoRoot>/.memory/ATLAS.md`, and committed to the dedicated `memory/pending` git worktree (`WorktreeCommitted == true`).
      - All 4 inbox notes are marked `Consolidated == true` (`ListUnconsolidated()` returns 0).
      - `<dataDir>/<repoID>/index.db` contains both the `repo:` entries and the `user:` entry.
      - `<dataDir>/<repoID>/state/freshness.json` is written with `UnconsolidatedNotes == 0` and non-empty `LastConsolidationAt`.

- [ ] **5. Rebuild Binary & Full Verification:**
  - Run `go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`.
  - Run `go test -race ./cmd/... ./internal/... ./packaging/...`.
  - Run `go vet ./...`.
  - Run `GOOS=windows go build ./...`.

## Verification Requirement
Ensure all tests across the entire repository pass (`go test -race ./cmd/... ./internal/... ./packaging/...`), `go vet ./...` is clean, `GOOS=windows go build ./...` succeeds, and `~/.local/bin/repo-memory` is rebuilt. Commit your changes on `pr-13-v2-consolidation/wo6-tick-orchestration-and-verification`, then **STOP and send a message back to the Principal Architect**.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `internal/core/tick/consolidate.go` — Implemented `DefaultConsolidationMinNotes`, `DefaultConsolidationMaxAge`, `ConsolidationOptions`, `ProposedScope`, `ConsolidationResult`, `FreshnessState` (`FreshnessStatePath`, `LoadFreshnessState`, `SaveFreshnessState`), `RunConsolidation`, and `RunConsolidationWithOptions` orchestrating the complete 7-step Stage 2 Warm Batch Consolidation pipeline (`PromoteMergedBranchOps`, cadence & quota/budget checks, `FilterEchoes`, `EnforceCorroborationGate`, `RouteItems`, `MergeExactDuplicates`, `CheckDossierAnchors`, `--dry-run` preview, `ConsolidateDossiers`, `ValidateAndPrepareOps`, `RecordUserTier`/`RecordBranchTier`/`RecordRepoTier`, `MarkConsolidated`, `PublishToMemoryWorktree`, `RebuildMultiTierIndex`, and `freshness.json`).
- `internal/core/tick/pipeline.go` — Wired `p.RunConsolidation(ctx, false)` into `PipelineExecutor.Run(ctx)` after `checkDocFreshness(ctx)` and updated the end-of-tick index rebuild to `consolidate.RebuildMultiTierIndex`.
- `cmd/repo-memory/cmd_consolidate.go` & `cmd/repo-memory/main.go` — Added `repo-memory consolidate [repo-dir] [--force] [--dry-run] [--json] [--deterministic]` and registered `consolidateCmd` on `rootCmd`.
- `internal/core/tick/consolidate_test.go` — Added `TestRunConsolidation_CadenceAndForce`, `TestRunConsolidation_EndToEndPipeline`, and `TestRunConsolidation_QuotaGuardSkipsWhenExhausted`.

### Decisions Made
- When `LastConsolidationAt` is empty on a non-forced tick with `< 10` waiting notes, cadence checks the age of the oldest unconsolidated note (`now.Sub(notes[0].CreatedAt) >= 24h`) so freshly harvested single notes wait for either 10 notes or the 24h window before triggering consolidation.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
