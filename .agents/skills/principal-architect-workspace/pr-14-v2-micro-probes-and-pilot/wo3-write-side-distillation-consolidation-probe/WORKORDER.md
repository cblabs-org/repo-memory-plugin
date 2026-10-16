# Work Order: `wo3-write-side-distillation-consolidation-probe`

> **Project:** [PROJECT.md](../PROJECT.md) (`pr-14-v2-micro-probes-and-pilot`)
> **Branch:** `pr-14-v2-micro-probes-and-pilot/wo3-write-side-distillation-consolidation-probe`
> **Base Branch:** `pr-14-v2-micro-probes-and-pilot/wo2-read-side-offline-retrieval-probe`
> **Status:** Complete

---

## 1. Objective & Context

Build and execute the repeatable **Layer 2 Write-Side Distillation & Consolidation Micro-Probe** (`X2`/`X4`/`X10`) for `repo-memory v2.2` against real captured sessions in `evaluation/repo-memory-transcripts/` (with a deterministic CI fallback when gitignored/external files are absent), verifying all 4 stages of the `v2.2` write pipeline end-to-end before live multi-replicate evaluation:

1. **Stage 1b Free Zero-Token Harvest (`inbox.HarvestCheckpointsAndArtifacts`):**
   - Harvest `CHECKPOINT` summaries (`Model Knowledge`, `User Knowledge`, `Work Accomplished`) and conversation `brain/` markdown artifacts (`<dataDir>/<repoID>/evidence/artifacts/<convID>/**/*.md`) anchored to valid repository paths with **zero LLM tokens**, verifying idempotency across repeated calls (`dedup_key`).
2. **Stage 1a `Stop` Milestone Gate & Harvest (`antigravity.HandleStop` / `DetectMilestones` / `BuildTurnNoteRequestPrompt` / `ParseRepoMemoryNoteBlock`):**
   - Scan real captured session transcripts from `evaluation/repo-memory-transcripts/` (and representative milestone turns: `git commit`, `gh pr create`, multi-file edit + build/test, error resolved after $\ge 15$ steps, PA `WORKORDER.md` `## Completion Report`), verifying:
     - Milestone detection accuracy (`DetectMilestones`) vs non-milestone turns;
     - Subagent and `/goal` exclusion;
     - `executionNum == 0` continuation prompt generation (`decision: "continue"` with `BuildTurnNoteRequestPrompt`);
     - `executionNum >= 1` YAML block parsing (`ParseRepoMemoryNoteBlock`), `LintNoteItems`, 4-layer scrubbing (`scrub.Scrub`), `AnchorChecks` attachment (`inbox.CheckAnchors`), and `ExposureList` attachment from the session's `delivery.Ledger`.
3. **Stage 1c Preparation Layer, Sidecar Fallback & 4-Layer Scrubber (`prepare.PrepareSession`, `prepare.ValidateDistillerOutput`, `scrub.Scrub`, `tick.RunSidecarFallback`):**
   - Run `PrepareSession` over real instruction-heavy Principal Architect / worker sessions from `evaluation/repo-memory-transcripts/` (`X10` regression check) and measure:
     - Raw vs. prepared character/token footprint and **payload reduction percentage** (`> 70%` reduction via collapsing `SKILL.md`, `WORKORDER.md`, `PROJECT.md`, and `docs/` file reads into 1-line `[viewed <path>, <N> lines]` stubs and summarizing subagent invocations);
     - Complete stripping of `[repo-memory ...]` injection blocks and `repo-memory: before you stop...` continuation prompts (`shouldDropStep`);
     - Random delimiter wrapping (`<session-log-<token>>` ... `</session-log-<token>>`) and role-marker escaping;
     - Post-call validation (`ValidateDistillerOutput`): accepting grounded items with valid `echo_token` and step citations while deterministically rejecting (a) missing/wrong `echo_token` (`ErrMissingEchoToken`), (b) tool calls or task-doing prose (`ErrTaskExecutionDetected`), (c) citations to stubbed file-read steps or non-existent steps (`ErrInvalidStepCitation`), and (d) copied instruction text from viewed `SKILL.md`/`WORKORDER.md` files (`ErrCopiedInstructionText`);
     - 4-layer scrubber (`scrub.Scrub`): redacting/rejecting secrets (AWS keys, GitHub PATs, bearer tokens, private keys) and quarantining payloads while preserving valid UUIDs, ULIDs, and git commit SHAs.
4. **Stage 2 Warm Batch Consolidation (`tick.PipelineExecutor.RunConsolidationWithOptions` / `consolidate.*`):**
   - Execute `RunConsolidationWithOptions` (both `DryRun: true` and `Force: true`) over the harvested Stage 1a/1b/1c inbox notes and seeded dossiers:
     - **Step 1 (`FilterEchoes` — `P13` Echo Filter):** Verify that note items repeating an entry in `ExposureList` (or active entries when exposure is unknown) without an anchored edit + passing build/test in the source episode are suppressed (`100%` un-reconfirmed echo suppression), whereas items backed by an anchored file edit followed by a passing build/test step in the episode are preserved and stamped `reconfirmed`;
     - **Step 2 (`EnforceCorroborationGate` — `VD18` Corroboration Gate):** Verify that uncorroborated `semantic`/`procedural` rules (`invariant`, `contract`, `convention`, `recipe`, `procedure`, or `gate: edit`) with `verified: null`/empty and no passing test, verified static anchor, or explicit user confirmation are downgraded to `open-issue` with a deterministic `closes_when` condition (`100%` uncorroborated rule downgrade), whereas rules corroborated by `test-pass`, `anchor-verified`, or `user-confirmed` are promoted;
     - **Step 3 (`RouteItems` & `MergeExactDuplicates`):** Verify Five-Tier routing (`TierUser` for `~/.ssh/config` or `user:` scopes $\to$ `<dataDir>/<repoID>/user-dossiers/`, `TierBranch` for unmerged feature branches $\to$ `.memory/local/ops/<branch_slug>/`, `TierRepo` for default/merged branches $\to$ `.memory/ops/`) and deterministic `OpAmend` coalescing of exact duplicates;
     - **Step 4 (`ValidateAndPrepareOps`, `RecordRepoTier`, `PublishToMemoryWorktree`, `RebuildMultiTierIndex`):** Verify $\le 6$-line entry text cap enforcement, `0` `LintDossier` errors, `0` broken anchors on newly written entries, `memory/pending` worktree commit, multi-tier `index.db` rebuild, and `state/freshness.json` update.

---

## 2. Files in Scope

| File | Action | Description |
|---|---|---|
| `internal/core/tick/eval_v2_write_probe_test.go` | **Create** | Comprehensive Go test `TestEvalV2WriteSideMicroProbe` exercising Stage 1b Free Harvest, Stage 1a `Stop` Milestone Gate & Harvest, Stage 1c Preparation Layer (`X10`) + `ValidateDistillerOutput` + `scrub.Scrub` + `RunSidecarFallback`, and Stage 2 `RunConsolidationWithOptions` (`P13` Echo Filter, `VD18` Corroboration Gate, Five-Tier routing, `OpAmend` dedup, worktree publish, and `index.db` rebuild), logging detailed Markdown metrics tables. |
| `evaluation/eval-v2/scripts/run_write_probe.sh` | **Create** | Executable shell script (`chmod +x`) that runs `go test -v -run TestEvalV2WriteSideMicroProbe ./internal/core/tick/...` for one-command execution. |
| `docs/evaluations/v2-2-write-side-micro-probe.md` | **Create** | Comprehensive empirical evaluation report documenting all Stage 1a/1b/1c and Stage 2 metrics (`X2`/`X4`/`X10`), real transcript corpus analysis from `evaluation/repo-memory-transcripts/`, payload reduction tables, security/grounding gate verification, consolidation funnel metrics, and sample projected dossier diffs. |

> **Strict Hygiene Constraint:** Never include any proprietary internal identifiers, internal issue/changelist prefixes, internal domain emails, or absolute local machine home paths in any repository file, test, script, report, or commit message.

---

## 3. Detailed Implementation Checklist

- [x] **Task 1: Create `internal/core/tick/eval_v2_write_probe_test.go` (`TestEvalV2WriteSideMicroProbe`)**
  - Inspect real sessions under `evaluation/repo-memory-transcripts/` when present on disk (and provide a deterministic fallback fixture when absent so `go test ./...` passes in any clean CI checkout).
  - **Sub-probe 1 — Real Corpus Milestone Scan & Stage 1a `Stop` Hook Probe (`antigravity.ParseTranscript`, `antigravity.DetectMilestones`, `antigravity.HandleStop`):**
    - Scan up to 25 real conversation transcripts in `evaluation/repo-memory-transcripts/` using `antigravity.ParseTranscript` and `antigravity.DetectMilestones` to measure real-world milestone trigger rates across commit, PR, multi-file edit+test, error resolution, and PA `WORKORDER.md` completion turns.
    - Execute `antigravity.HandleStop` across both `executionNum == 0` (verifying `decision: "continue"` and `BuildTurnNoteRequestPrompt` emission when `turn_notes: on` and a milestone occurred, and `decision: ""` for subagents or non-milestone turns) and `executionNum == 1` (verifying `repo-memory-note` YAML block extraction, `ExposureList` attachment from `delivery.Ledger`, `AnchorChecks` computation, 4-layer scrubbing, and `mark` advancement).
  - **Sub-probe 2 — Stage 1b Free Zero-Token Harvest (`inbox.HarvestCheckpointsAndArtifacts`):**
    - Populate an `episode.Store` with real/representative `CHECKPOINT` summaries (`Model Knowledge`, `User Knowledge`, `Work Accomplished`, `Files and Code`) and `<dataDir>/<repoID>/evidence/artifacts/<convID>/` markdown artifacts referencing real repository paths.
    - Run `inbox.HarvestCheckpointsAndArtifacts` twice in succession; verify that the first run harvests valid `source: "checkpoint"` and `source: "artifact"` notes with verified `AnchorChecks` and **0 LLM tokens**, and the second run creates **0 duplicate notes** (`HasDedupKey` idempotency).
  - **Sub-probe 3 — Stage 1c Preparation Layer (`X10`), 4-Layer Scrubber & Sidecar Fallback (`prepare.PrepareSession`, `prepare.ValidateDistillerOutput`, `scrub.Scrub`, `PipelineExecutor.RunSidecarFallback`):**
    - Convert real instruction-heavy PA/worker transcripts from `evaluation/repo-memory-transcripts/` into `[]harness.Step` (including large `SKILL.md` / `WORKORDER.md` / `PROJECT.md` / `docs/` read steps, `[repo-memory ...]` injection steps, and `invoke_subagent` steps) and run `prepare.PrepareSession`.
    - Measure and assert:
      - Raw input bytes/tokens vs. `PreparedSession.FormattedContainer` bytes/tokens (**assert `>= 70%` payload reduction** on instruction-heavy sessions);
      - `100%` of `[repo-memory ...]` exposure steps and `repo-memory: before you stop...` prompts are dropped;
      - `100%` of instruction file reads (`SKILL.md`, `WORKORDER.md`, `PROJECT.md`, `docs/`) are collapsed to 1-line `[viewed <path>, <N> lines]` stubs and recorded in `StubbedSteps` and `InstructionSnippets`;
      - `ValidateDistillerOutput` deterministically accepts grounded items and rejects all 4 failure modes (`ErrMissingEchoToken`, `ErrTaskExecutionDetected`, `ErrInvalidStepCitation` on stubbed/non-existent steps, and `ErrCopiedInstructionText` on copied `SKILL.md`/`WORKORDER.md` text);
      - `scrub.Scrub` redacts/blocks AWS keys, GitHub PATs, bearer tokens, and private keys while preserving valid UUIDs, ULIDs, and 40-hex git commit SHAs;
      - `PipelineExecutor.RunSidecarFallback` processes an eligible subagent/idle episode via a deterministic mock `Housekeeper`, writes a `source: "sidecar"` inbox note with `ExposureList` and `AnchorChecks`, and advances `state/marks/<convID>.json`.
  - **Sub-probe 4 — Stage 2 Warm Batch Consolidation (`PipelineExecutor.RunConsolidationWithOptions`):**
    - Seed an isolated git repository (`main` + unmerged `feature/ds18b20-probe` branch) with initial dossiers and feed a comprehensive batch of inbox notes (from Stage 1a `turn-note`, Stage 1b `checkpoint`/`artifact`, and Stage 1c `sidecar`) covering:
      1. **Un-reconfirmed Echoes (3 items):** Repeat exposed entries without anchored edit + passing build/test $\to$ verify `100%` (`3/3`) suppressed by `FilterEchoes` (`EchoesDropped == 3`);
      2. **Reconfirmed Echo (1 item):** Repeats an exposed entry where the episode edited the anchored file and subsequently passed `go test` / `pio test` $\to$ verify kept and stamped `reconfirmed`;
      3. **Uncorroborated Semantic/Procedural Rules (3 items):** `invariant`, `contract`, and `recipe` (`verified: ""` / `"null"`) with nonexistent anchors and no passing test or user confirmation $\to$ verify `100%` (`3/3`) downgraded by `EnforceCorroborationGate` (`DowngradedToIssue == 3`) to `open-issue` with deterministic `closes_when`;
      4. **Corroborated Rules (3 items):** Corroborated via (a) edit + passing test (`test-pass`), (b) verified static file#symbol anchor in `repoRoot` (`anchor-verified`), and (c) explicit human `USER_INPUT` confirmation (`user-confirmed`) $\to$ verify `100%` (`3/3`) promoted with verified stamps;
      5. **Exact Duplicate from Unexposed Session (1 item):** Matches an existing active entry's title/do and overlapping anchor $\to$ verify coalesced via deterministic `OpAmend` (`ExactDuplicatesAmend == 1`);
      6. **Five-Tier Routing (3 items):** Workstation-local `user:` preference (`~/.ssh/config`), unmerged branch `branch:` finding (`feature/ds18b20-probe`), and shared `repo:` architectural decision (`main`) $\to$ verify strict tier isolation (`user-dossiers/` vs `.memory/local/ops/feature__ds18b20-probe/` vs `.memory/ops/`);
      7. **Validation & Guardrails (2 malformed/secret items):** One candidate op exceeding the 6-line entry cap and one containing a raw secret token $\to$ verify rejected by `ValidateAndPrepareOps` (`OpsRejected`) and quarantined without aborting valid ops;
      8. **Worktree Publish, Multi-Tier Index & Freshness:** Verify `PublishToMemoryWorktree` commits to `memory/pending`, `RebuildMultiTierIndex` indexes `repo:`, `branch:`, and `user:` entries into `<dataDir>/<repoID>/index.db`, `dossiers.LintDossier` reports `0` cap violations, and `state/freshness.json` records `UnconsolidatedNotes == 0`.
- [x] **Task 2: Create `evaluation/eval-v2/scripts/run_write_probe.sh`**
  - Create an executable script (`chmod +x`) that runs `go test -v -run TestEvalV2WriteSideMicroProbe ./internal/core/tick/...` and prints a clear PASS summary.
- [x] **Task 3: Write `docs/evaluations/v2-2-write-side-micro-probe.md`**
  - Document the complete empirical findings from `TestEvalV2WriteSideMicroProbe` and `./evaluation/eval-v2/scripts/run_write_probe.sh`, including:
    1. Executive Summary & Comparison against `v2.1` Failure Modes (`X2` cold synthesis, `X4` uncorroborated rule promotion & echo amplification, `X10` PA `SKILL.md`/`WORKORDER.md` instruction ingestion);
    2. Real Transcript Corpus Scan (`evaluation/repo-memory-transcripts/` milestone trigger distribution & Stage 1a `Stop` hook latency/yield);
    3. Stage 1b Free Zero-Token Harvest (`CHECKPOINT` + `brain/` artifact yield and deduplication);
    4. Stage 1c Preparation Layer (`X10`) & 4-Layer Security/Grounding Gate Metrics (raw vs. prepared bytes/tokens, `>70%` payload reduction table, `ValidateDistillerOutput` 4-gate rejection matrix, and `scrub.Scrub` secret/identifier results);
    5. Stage 2 Warm Batch Consolidation Funnel Metrics (`P13` Echo Filter suppression vs. reconfirmation, `VD18` Corroboration Gate downgrade vs. promotion, `OpAmend` duplicate coalescing, Five-Tier routing isolation, `ValidateAndPrepareOps` cap/scrubber rejections, `memory/pending` worktree commit, and sample rendered dossier entries);
    6. Readiness Verdict for `WO4` (Live `esp32` Pilot & Final Synthesis).
- [x] **Task 4: Verify Full Test Suite & Append Completion Report**
  - Run `./evaluation/eval-v2/scripts/run_write_probe.sh` and `go test -race ./cmd/... ./internal/... ./packaging/...`.
  - Append a `## Completion Report` section to this `WORKORDER.md`, commit all changes on branch `pr-14-v2-micro-probes-and-pilot/wo3-write-side-distillation-consolidation-probe` with author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`, and message the Principal Architect.

---

## Completion Report

### Summary of Changes

1. **Created `internal/core/tick/eval_v2_write_probe_test.go` (`TestEvalV2WriteSideMicroProbe`):**
   - Implemented the 4-stage `v2.2` Write-Side Distillation & Consolidation Micro-Probe in `package tick` (with zero imports of `internal/harness/*`, preserving `TestCoreImportBoundaries`):
     - **`SubProbe1_Stage1a_StopHookAndMilestoneDetection`:** Scanned all 25 real transcripts in `evaluation/repo-memory-transcripts/` across 83 assistant turn boundaries (`18` milestone turns triggered = `21.69%`, `65` routine turns skipped = `78.31%`; categories: `commit: 11`, `multi_file_edit_test: 6`, `pa_workorder_done: 5`, `pr: 1`). Exercised the compiled `repo-memory hook antigravity Stop` binary end-to-end across routine turns (`decision: ""`), subagent turns (`decision: ""`), milestone `executionNum == 0` (`decision: "continue"` with `BuildTurnNoteRequestPrompt`), and continuation `executionNum == 1` (harvesting 1 valid item, dropping 1 incomplete item via `LintNoteItems`, redacting inline AWS secret `AKIAIOSFODNN7EXAMPLE` $\to$ `«secret:aws»`, attaching `ExposureList` and `AnchorChecks`, and advancing `state/stop-marks.json`).
     - **`SubProbe2_Stage1b_FreeZeroTokenHarvest`:** Exercised `inbox.HarvestCheckpointsAndArtifacts` twice over `CHECKPOINT` summaries and `brain/` artifacts (`implementation_plan.md`, `walkthrough.md`), verifying 0 LLM calls (`0` tokens), 3 notes created on Run 1 (`3` checkpoint items + `3` artifact items), and 0 duplicate notes on Run 2 (`100%` idempotency).
     - **`SubProbe3_Stage1c_PreparationScrubberAndFallback`:** Measured `prepare.PrepareTranscript` across all 25 real transcripts (`3,518,270` $\to$ `1,326,524` bytes = `62.30%` overall reduction; up to `74.20%` reduction on top instruction-heavy session `323730fe-d5d9-47d8-aecd-997c6a82ca6a` with `55` stubbed reads; and `93.96%` reduction on the PA benchmark session), `100%` stripping of `[repo-memory ...]` exposures and prior continuation prompts, `100%` stubbing of instruction file reads, 5-case verification of `prepare.ValidateDistillerOutput`, 4-layer `scrub.Scrub` redaction/allow-list preservation, and quota-gated `tick.RunSidecarFallback` (`78%` quota $\to$ `1` `sidecar` note).
     - **`SubProbe4_Stage2_ConsolidationFunnelAndPublish`:** Executed `consolidate.RunConsolidationWithOptions` (both `DryRun: true` and live publish) across 6 notes (`13` candidate items), verifying `EchoesDropped == 3` (`P13`), `EchoesReconfirmed == 1` (`reconfirmed`), `DowngradedToIssue == 3` (`VD18`), `CorroboratedPromoted == 3` (`test-pass`, `anchor-verified`, `user-confirmed`), `ExactDuplicatesAmend == 1` (`OpAmend`), Five-Tier routing (`1` `user:`, `1` `branch:`, `8` `repo:`), `OpsRejected == 3` (`1` quarantine batch), `0` `LintDossier` errors across 5 rendered dossiers, `memory/pending` worktree commit, and multi-tier `index.db` FTS5 search (`11` entries across `repo:` + `branch:` + `user:` tiers).
2. **Created `evaluation/eval-v2/scripts/run_write_probe.sh`:**
   - Added executable one-command runner (`chmod +x`) executing `go test -v -run TestEvalV2WriteSideMicroProbe ./internal/core/tick/...`.
3. **Created `docs/evaluations/v2-2-write-side-micro-probe.md`:**
   - Authored the empirical evaluation report covering all Stage 1a/1b/1c and Stage 2 metrics (`X2`, `X10`, `X4`), real transcript corpus breakdown, consolidation funnel table, and verbatim projected dossier samples across `repo:`, `branch:`, and `user:` tiers.

### Verification Commands & Results

- `./evaluation/eval-v2/scripts/run_write_probe.sh` $\to$ **PASS** (`TestEvalV2WriteSideMicroProbe` all 4 sub-probes passed).
- `go test -v -run TestCoreImportBoundaries ./internal/core/...` $\to$ **PASS** (`162` Go files scanned, `0` boundary violations).
- `go test -race ./cmd/... ./internal/... ./packaging/...` $\to$ **PASS**.

