# Empirical Evaluation Report: `v2.2` Write-Side Distillation & Consolidation Micro-Probe

**Date:** 2026-10-16  
**Project:** `pr-14-v2-micro-probes-and-pilot` / `WO3` (`wo3-write-side-distillation-consolidation-probe`)  
**Executable Probe:** `./evaluation/eval-v2/scripts/run_write_probe.sh` (`internal/core/tick/eval_v2_write_probe_test.go`)  
**Verdict:** **PASS (`GO` TO STAGE 2a LIVE PILOT)**

---

## 1. Executive Summary & Verdict

The `v2.2` Write-Side Distillation & Consolidation Micro-Probe empirically validates the complete `v2.2` write-side pipeline (`PR-12` + `PR-13`) across real repository transcripts (`evaluation/repo-memory-transcripts/`, 25 full sessions) and deterministic multi-note consolidation batches, without spending paid external LLM tokens during automated test execution.

All four architectural criteria (`X2`, `X10`, `X4`, and end-to-end Five-Tier publishing) pass with zero defects:

1. **Stage 1a (`Stop` Continuation) & Stage 1b (Free Zero-Token Harvest) Selectivity (`X2` — PASS):**
   - Across **25 real development transcripts** (`83` assistant turns), the milestone gate (`evaluateMilestoneGate`) triggered on **18 milestone turns (`21.69%`)** and stayed completely silent (`{}`) on **65 routine turns (`78.31%`)**, confirming that `Stop` continuations fire strictly at high-signal boundaries (`git commit`, `gh pr create`, multi-file edits followed by build/test, error recovery after $\ge 15$ steps, and `WORKORDER.md` completion reports).
   - Subagent turns (`state/subagents.json`) and `executionNum >= 1` continuation turns are suppressed **100% of the time**, eliminating recursive continuation loops.
   - Stage 1b (`inbox.HarvestCheckpointsAndArtifacts`) extracted **6 structured items** across **3 zero-token notes** (`checkpoint-note` and `artifact-note`) with **0 LLM calls (`0` tokens)** and **100% idempotency** on repeat invocations (`0` duplicate notes written).
2. **Stage 1c Preparation Layer (`X10`), Distiller Validation & 4-Layer Scrubber (PASS):**
   - Across all **25 real transcripts** (`3,518,270` raw bytes), `prepare.PrepareTranscript` reduced byte volume to **`1,326,524` bytes (`62.30%` overall reduction)**, reaching **`74.20%` reduction** (`458,329` $\to$ `118,270` bytes, `55` stubbed instruction reads) on instruction-heavy real sessions and **`93.96%` reduction** (`24,589` $\to$ `1,484` bytes) on the multi-step Principal Architect benchmark transcript.
   - `100%` of `[repo-memory ...]` injected blocks and `<!-- repo-memory:begin/end -->` spans were stripped; `100%` of prior `Stop` continuation prompts were stripped; and `100%` of reads of `SKILL.md`, `WORKORDER.md`, `CLAUDE.md`, `AGENTS.md`, and `.memory/dossiers/` were replaced with 1-line `[instruction read: <path> (<N> lines) — omitted]` stubs.
   - `prepare.ValidateDistillerOutput` accepted grounded `<<<OP_BATCH_JSON>>>` outputs and rejected **100% (4/4)** of adversarial failure modes (missing framing tokens, prompt-hijacked task continuations, citations of stubbed instruction steps, and parroted `InstructionSnippets`).
   - `scrub.Scrub` redacted **100% (4/4)** of injected credential classes (`«secret:aws»`, `«secret:github»`, `«secret:token»`, `«secret:private_key»`) while preserving **100% (4/4)** of allow-listed entry ULIDs and git commit SHAs.
   - `tick.RunSidecarFallback` cleanly extracted structured `sidecar` notes for stale transcripts (`> 5m` past last `Stop` mark) while respecting the `80%` quota threshold (`78%` active window).
3. **Stage 2 Warm Batch Consolidation Funnel (`X4`) & Five-Tier Publishing (PASS):**
   - Across **6 unconsolidated notes (`13` candidate items)**, `consolidate.RunConsolidationWithOptions` dropped **3 pure echoes (`P13`)**, preserved **1 reconfirmed echo** with an updated `verified: <date> (<sha>, reconfirmed)` stamp, downgraded **3 uncorroborated semantic/procedural claims (`VD18`)** to `open-issue`, promoted **3 corroborated claims** (`test-pass`, `anchor-verified`, `user-confirmed`), and merged **1 exact duplicate** into `OpAmend`.
   - Five-tier routing (`ClassifyTier`) partitioned validated ops cleanly into **1 `user:` batch** (`user-dossiers/ops/`), **1 `branch:` batch** (`.memory/local/ops/feature-ds18b20-probe/`), and **8 `repo:` ops** (`.memory/ops/`), while `ValidateAndPrepareOps` rejected **3 invalid/fabricated ops** into `quarantine/<batch-id>.json`.
   - `Dossiers.RenderMarkdown` and `dossiers.LintDossier` verified **0 lint errors** across all 5 rendered dossiers (`repo:`, `branch:`, and `user:`), confirmed `PublishRepoTierViaWorktree` committed cleanly to `memory/pending` (`dddd127e2bd608b6caca8790b098d76fdddb6f1a`), and verified multi-tier FTS5 search (`11` active entries across `repo:` + `branch:` + `user:` tiers) with `UnconsolidatedNotes == 0`.

---

## 2. Stage 1a (`Stop` Continuation) & Stage 1b (Free Harvest) Selectivity (`X2`)

### 2.1 Real Transcript Corpus Selectivity (`evaluation/repo-memory-transcripts/`)

The probe scanned all **25 real session transcripts** in `evaluation/repo-memory-transcripts/`, slicing each transcript at every completed assistant turn boundary (`83` total turn boundaries) and evaluating `antigravity.EvaluateMilestoneTurn`:

| Metric | Value | Notes |
| :--- | ---: | :--- |
| **Real Transcripts Evaluated** | **25** | Full `transcript.jsonl` corpus in `evaluation/repo-memory-transcripts/` |
| **Total Assistant Turn Boundaries** | **83** | Every completed assistant response boundary across all 25 sessions |
| **Milestone Turns Triggered (`Matched: true`)** | **18 (`21.69%`)** | Continuation fires only when high-value engineering work concludes |
| **Routine Turns Skipped (`Matched: false`)** | **65 (`78.31%`)** | Silent `{}` response on exploratory reads, Q&A, and single-file tweaks |

#### Breakdown of Triggered Milestone Turns by Category

Note: A single milestone turn can satisfy more than one trigger condition (e.g., editing $\ge 2$ files, running `go test`, and committing in the same turn window).

| Milestone Trigger Category | Turns Matched | Trigger Rule Verified in `evaluateMilestoneGate` |
| :--- | ---: | :--- |
| **`commit`** (`git commit`) | **11** | Successful `run_command` containing `git commit` within the last 10 steps |
| **`multi_file_edit_test`** ($\ge 2$ file edits + build/test) | **6** | Edits to $\ge 2$ distinct files followed by a passing `go test` / `go build` / `pytest` / `cargo test` |
| **`pa_workorder_done`** (`WORKORDER.md` completion) | **5** | Write/edit to `WORKORDER.md` containing `## Completion Report` |
| **`pr`** (`gh pr create`) | **1** | Successful `run_command` containing `gh pr create` |

### 2.2 End-to-End `Stop` Hook Execution (`handleStopDistill`)

| Test Case | Input State | `Stop` Hook Decision | Empirical Outcome |
| :--- | :--- | :--- | :--- |
| **Case 1: Routine Turn** | Read-only `view_file` + status reply (`executionNum: 0`) | `""` (allow stop) | Silent `{}` output; `0` notes written; `0` continuation turns spent. |
| **Case 2: Subagent Turn** | Milestone `git commit`, but `conversationId` registered in `state/subagents.json` | `""` (allow stop) | Subagent suppression gate blocked continuation (`P1`); parent harvests subagent work. |
| **Case 3: Milestone Turn (`executionNum == 0`)** | Edits to 2 files + passing `go test ./...` + `git commit` | `"continue"` | Returned `BuildTurnNoteRequestPrompt` requesting a fenced ```` ```repo-memory-note ```` YAML block; recorded `PendingRequestStep` in `state/stop-marks.json`. |
| **Case 4: Continuation Turn (`executionNum == 1`)** | Model replies with `<!-- repo-memory:begin -->` YAML block containing 1 valid `invariant` (with inline AWS key `AKIAIOSFODNN7EXAMPLE`) and 1 incomplete item (missing `do`) | `""` (allow stop) | Hard single-continuation guard allowed termination; `LintNoteItems` kept `1` valid item and dropped `1` incomplete item; `scrub.Scrub` redacted `AKIAIOSFODNN7EXAMPLE` $\to$ `«secret:aws»`; attached `ExposureList` (`[inv-01J00000000000000000000001]`) and `AnchorChecks` (`internal/core/index/db.go: true`, `internal/core/index/missing_file.go: false`); advanced `LastStepIndex = 7`. |

### 2.3 Stage 1b Free Zero-Token Harvest (`inbox.HarvestCheckpointsAndArtifacts`)

| Metric | Run 1 (Initial Harvest) | Run 2 (Idempotency Re-Run) | Verification |
| :--- | ---: | ---: | :--- |
| **External LLM Calls / Tokens Spent** | **0 calls / 0 tokens** | **0 calls / 0 tokens** | 100% deterministic Go extraction |
| **`checkpoint-note` Items Extracted** | **3 items** (1 `checkpoint-note`) | **0 items** (skipped via `state/checkpoint-marks.json`) | Extracted Decisions, Failed Approaches, and Rules from `CHECKPOINT` summary step |
| **`artifact-note` Items Extracted** | **3 items** (2 `artifact-note` files) | **0 items** (skipped via SHA-256 hash in `state/artifact-marks.json`) | Extracted from `implementation_plan.md` (`2` items) and `walkthrough.md` (`1` item) |
| **Total Notes Created in `inbox/unconsolidated/`** | **3 notes** | **0 new notes** (`3` total on disk) | Zero duplicate notes on repeated invocations |

---

## 3. Stage 1c Preparation Layer (`X10`), Validation & Scrubber Results

### 3.1 Corpus-Wide Transcript Preparation Reduction (`prepare.PrepareTranscript`)

Running `prepare.PrepareTranscript` across all **25 real transcripts** in `evaluation/repo-memory-transcripts/` plus the multi-step Principal Architect benchmark transcript demonstrated substantial byte and token savings before any sidecar fallback call:

| Scope / Transcript Cohort | Transcripts | Raw JSONL Bytes | Prepared Bytes | Byte Reduction (%) |
| :--- | ---: | ---: | ---: | ---: |
| **Full Real Transcript Corpus (`evaluation/repo-memory-transcripts/`)** | **25** | `3,518,270` | `1,326,524` | **`62.30%`** |
| **Top 5 Instruction-Heavy Real Sessions** | **5** | `1,768,081` | `561,895` | **`68.22%`** (up to **`74.20%`**) |
| **Multi-Step PA Benchmark Session (with `SKILL.md` + `WORKORDER.md` + exposures)** | **1** | `24,589` | `1,484` | **`93.96%`** |

#### Top 5 Instruction-Heavy Real Transcripts in `evaluation/repo-memory-transcripts/`

| Conversation ID | Raw Bytes | Prepared Bytes | Reduction (%) | Stubbed Instruction Reads | Subagent Spawns Stubbed | `InstructionSnippets` Collected |
| :--- | ---: | ---: | ---: | ---: | ---: | ---: |
| `323730fe-d5d9-47d8-aecd-997c6a82ca6a` | `458,329` | `118,270` | **`74.20%`** | 55 | 0 | 55 |
| `00259bf0-c1c7-422f-a32e-fd039792d869` | `435,215` | `117,007` | **`73.12%`** | 51 | 0 | 51 |
| `2faf69fe-d19a-4dc5-ae89-89285d5d6295` | `236,309` | `78,271` | **`66.88%`** | 16 | 0 | 16 |
| `191995fd-31a5-4882-8660-b96a00c9fd70` | `369,087` | `132,120` | **`64.20%`** | 34 | 0 | 34 |
| `1778ca2b-4150-461f-97cb-72d0586554e8` | `269,141` | `116,227` | **`56.82%`** | 3 | 2 | 5 |

#### Stripping & Stubbing Completeness on Benchmark Session

| Sanitization Rule | Target Pattern | Stripping / Stubbing Rate |
| :--- | :--- | ---: |
| **Injected `[repo-memory ...]` Block Stripping** | `[repo-memory — active context]` and `<!-- repo-memory:begin -->...<!-- repo-memory:end -->` | **`100.0%`** (0 bytes leaked into prepared text; `2` exposure IDs captured in `PreparedTranscript.Exposures`) |
| **Prior `Stop` Continuation Prompt Stripping** | `"repo-memory: before you stop, write a memory note..."` | **`100.0%`** (`1` continuation prompt stripped completely) |
| **Instruction & Skill File Read Stubbing** | `.agents/skills/principal-architect/SKILL.md`, `WORKORDER.md`, `CLAUDE.md`, `.memory/dossiers/*.md` | **`100.0%`** (`4` multi-KB file reads replaced with 1-line `[instruction read: <path> (<N> lines) — omitted]` stubs) |
| **Subagent Spawn Prompt Stubbing** | `invoke_subagent` (`Prompt` payload) | **`100.0%`** (`1` subagent prompt replaced with `[subagent spawn: ... — omitted]`) |

### 3.2 `prepare.ValidateDistillerOutput` Anti-Hijack & Grounding Gate

| Test Case | Simulated Distiller Output | Validator Verdict | Rejection Reason |
| :--- | :--- | :--- | :--- |
| **1. Grounded Op Batch** | `<<<OP_BATCH_JSON>>>` with valid `add` op citing `evidence: ["step 6"]` | **ACCEPTED** (`1` op) | — |
| **2. Missing Framing Tokens** | Raw JSON without `<<<OP_BATCH_JSON>>> ... <<<END_OP_BATCH_JSON>>>` | **REJECTED** | `missing <<<OP_BATCH_JSON>>> ... <<<END_OP_BATCH_JSON>>> framing tokens` |
| **3. Task Execution Continuation** | Hijacked output beginning with `"I will now implement Task 1..."` before `<<<OP_BATCH_JSON>>>` | **REJECTED** | `unexpected text outside framing tokens` / task execution prose blocked |
| **4. Citation of Stubbed Instruction Step** | Op citing `evidence: ["step 2"]` (where step 2 was a stubbed `SKILL.md` read) | **REJECTED** | `op[0] cites stubbed instruction read step 2` |
| **5. Parroted `InstructionSnippets` Text** | Op whose `why` / `do` text overlaps $> 60\%$ 5-grams with a stubbed `SKILL.md` snippet | **REJECTED** | `op[0] parrots instruction file text (>60% 5-gram overlap)` |

### 3.3 4-Layer `scrub.Scrub` Secret & High-Entropy Redaction

| Input Pattern | Category | Output Marker / Disposition | Verified |
| :--- | :--- | :--- | :---: |
| `AKIAIOSFODNN7EXAMPLE` | AWS Access Key ID (`AKIA[0-9A-Z]{16}`) | Redacted $\to$ `«secret:aws»` | PASS |
| `ghp_1234567890abcdefghijklmnopqrstuvwxyz` | GitHub Personal Access Token (`ghp_...`) | Redacted $\to$ `«secret:github»` | PASS |
| `Authorization: Bearer eyJhbGciOiJIUzI1NiIs...` | Bearer JWT / API Token | Redacted $\to$ `Authorization: Bearer «secret:token»` | PASS |
| `-----BEGIN OPENSSH PRIVATE KEY----- ...` | PEM / OpenSSH Private Key Block | Redacted $\to$ `«secret:private_key»` | PASS |
| `inv-01J00000000000000000000001`, `got-01J00000000000000000000002` | Allow-listed Entry ULIDs (`AllowedIDs`) | **Preserved verbatim** (0 false-positive redactions) | PASS |
| `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`, `83edd25` | Allow-listed Git Commit SHAs | **Preserved verbatim** (0 false-positive redactions) | PASS |

### 3.4 Quota-Gated Sidecar Fallback (`tick.RunSidecarFallback`)

With a stale transcript (`ModTime = now - 10m`, last `Stop` mark at step 1 vs. final step 4) and quota usage at **`78%`** (below the `80%` hard cutoff), `RunSidecarFallback` invoked the injected `DistillerFunc` once on the prepared transcript slice and wrote **`1` `sidecar` note** (`source: "sidecar"`) into `inbox/unconsolidated/`.

---

## 4. Stage 2 Warm Batch Consolidation Funnel (`X4`)

`SubProbe4` seeded a temporary Git repository with `4` existing repo-tier entries (`inv-01J00000000000000000000001`, `got-01J00000000000000000000002`, `con-01J00000000000000000000003`, `rec-01J00000000000000000000004`) and fed **6 unconsolidated notes containing 13 candidate items** into `consolidate.RunConsolidationWithOptions`:

### 4.1 End-to-End Stage 2 Consolidation Funnel Table

| Funnel Stage | Mechanism / Function | Input Count | Filtered / Transformed | Output Count | Details |
| :--- | :--- | ---: | ---: | ---: | :--- |
| **0. Unconsolidated Inbox Notes** | `inboxStore.ListUnconsolidated()` | **6 notes** | — | **13 items** | Notes 1–6 spanning echoes, uncorroborated claims, corroborated claims, exact duplicates, and tier-scoped items |
| **1. Exposure-Aware De-Echoing (`P13`)** | `FilterEchoes` (`internal/core/consolidate/deecho.go`) | **13 items** | **3 dropped**, **1 reconfirmed** | **10 items** | Dropped 3 items echoing exposed entries (`inv-...01`, `got-...02`, `con-...03`) without new anchor/commit; kept 1 reconfirmed item (`[inv-...01]`) at new commit `696a1b8` with `verified: 2026-10-16 (696a1b8, reconfirmed)` |
| **2. Corroboration Gate (`VD18`)** | `EnforceCorroboration` (`internal/core/consolidate/corroborate.go`) | **10 items** | **3 downgraded**, **3 corroborated** | **10 items** | Downgraded 3 uncorroborated `invariant`/`contract`/`recipe` items to `open-issue`; promoted 3 corroborated items (`test-pass`, `anchor-verified`, `user-confirmed`) |
| **3. Exact-Duplicate Coalescing** | `MergeExactDuplicates` (`internal/core/consolidate/dedup.go`) | **10 items** | **1 merged into `OpAmend`** | **10 ops** (`9` `OpAdd` + `1` `OpAmend`) | Coalesced duplicate recipe `"Rebuild SQLite index after manual dossier inspection"` into `OpAmend` on `rec-01J00000000000000000000004` |
| **4. Five-Tier Classification** | `ClassifyTier` (`internal/core/consolidate/tiers.go`) | **10 ops** | Partitioned across `user:` / `branch:` / `repo:` | **10 ops** | **1 `user:` op** (`workstation/gh`), **1 `branch:` op** (`feature/ds18b20-probe`), **8 `repo:` ops** (`core`, `core/index`, `general`) |
| **5. Op Validation & Quarantine** | `ValidateAndPrepareOps` (`internal/core/consolidate/validate.go`) | **13 ops** (incl. 3 adversarial ops) | **3 rejected & quarantined** | **10 valid ops**, **1 quarantine file** | Rejected: (1) `supersede` targeting nonexistent `inv-01J99999999999999999999999`, (2) `add` citing fabricated anchor `internal/core/nonexistent_fabricated_path.go`, (3) `add` containing unredacted key `AKIAIOSFODNN7EXAMPLE` |
| **6. Dry-Run Isolation Check** | `RunConsolidationWithOptions(..., DryRun: true)` | **6 notes** | `0` disk mutations | **0 files written** | Verified `DryRun: true` leaves `.memory/ops/`, `.memory/dossiers/`, and `inbox/unconsolidated/` untouched |
| **7. Live Multi-Tier Projection & Worktree Publish** | `PublishRepoTierViaWorktree` + `RebuildMultiTierIndex` | **10 valid ops** | Projected **5 dossiers** (`0` lint errors) | **11 indexed entries**, **0 unconsolidated notes** | Committed repo-tier ops & dossiers to `memory/pending` (`dddd127e2bd608b6caca8790b098d76fdddb6f1a`); indexed all 3 tiers (`repo:` + `branch:` + `user:`) into `index.db` |

---

## 5. Sample Projected Dossiers & Diffs

All 5 rendered dossiers passed `dossiers.LintDossier` with **0 lint errors**, demonstrating template compliance (`{section?}` empty-section omission, `verified:` stamps with corroboration reasons, `OpAmend` merged evidence trails, and `open-issue` downgrades).

### 5.1 Repo-Tier Dossier: `.memory/dossiers/core/index.md`

Shows the seeded entries alongside the **reconfirmed echo (`[inv-01M52AEFR5EMCSY3D5MBQDMGCW]`)** stamped with `verified 2026-10-16 (696a1b8, reconfirmed)` and the **`OpAmend` merged recipe (`[rec-01J00000000000000000000004]`)** combining `evidence: note-seed-4, note-01M52AEFPS6B8ZGYCGT9JMNYEK`:

```markdown
# core/index

## Purpose and boundaries
SQLite FTS5 index and query layer.

## Invariants & contracts

### [con-01J00000000000000000000003] **Close read-only SQLite handles after hook query**
`contract · kind: semantic · anchors: internal/core/index/db.go · verified 2026-10-15 (83edd25) · evidence: note-seed-3`
Why: Prevents descriptor leaks in CLI hooks.
Do: Defer idx.Close() immediately after NewReadOnlyIndex.

### [inv-01J00000000000000000000001] **Use pointer-file swap on reindex**
`invariant · kind: semantic · anchors: internal/core/index/db.go · verified 2026-10-15 (83edd25) · evidence: note-seed-1`
Why: Prevents SQLite lock contention.
Do: Write to a new SQLite file and swap index-pointer.

### [inv-01M52AEFR5EMCSY3D5MBQDMGCW] **Pointer-file swap reconfirmed under concurrent load [inv-01J00000000000000000000001]**
`invariant · kind: semantic · anchors: internal/core/index/db.go · verified 2026-10-16 (696a1b8, reconfirmed) · evidence: note-01M52AEFP0XZSASYBY8AZ18X6C`
Why: Re-verified during concurrent stress test.
Do: Write to a new SQLite file and swap index-pointer atomically.

## Ruled out, failure signatures and gotchas

### [got-01J00000000000000000000002] **Escape FTS5 double quotes in user queries**
`gotcha · kind: semantic · anchors: internal/core/index/db.go · verified 2026-10-15 (83edd25) · evidence: note-seed-2`
Why: Unbalanced quotes cause FTS5 syntax errors.
Do: Sanitize FTS5 query tokens before MATCH.

## Procedures / Recipes

### [rec-01J00000000000000000000004] **Rebuild SQLite index after manual dossier inspection**
`recipe · kind: procedural · anchors: internal/core/index/db.go · verified 2026-10-15 (83edd25) · evidence: note-seed-4, note-01M52AEFPS6B8ZGYCGT9JMNYEK`
Why: Ensures FTS5 tables reflect latest projections.
Do: Run repo-memory reindex.
```

### 5.2 Repo-Tier Dossier: `.memory/dossiers/core.md`

Shows the **3 corroborated semantic rules (`VD18`)** promoted with `verified 2026-10-16 (696a1b8, anchor-verified)` and `verified 2026-10-16 (696a1b8, test-pass)`, alongside the **uncorroborated invariant downgraded to `## Open issues` (`[issue-01M52AEFR5Y1CYNY8SCMK5RBA5]`)** while omitting empty sections (`## Decisions`, `## Ruled out, failure signatures and gotchas`, `## Procedures / Recipes`):

```markdown
# core

## Purpose and boundaries
Curated repository memory for core.

## Invariants & contracts

### [con-01M52AEFR7QJJZYZE14353Y52Q] **VerifyContractSymbol must remain exported in consolidate**
`contract · kind: semantic · anchors: internal/core/consolidate/run.go#VerifyContractSymbol · verified 2026-10-16 (696a1b8, anchor-verified) · evidence: note-01M52AEFPKM1YAWTHK98TD2ZGF`
Why: External callers rely on VerifyContractSymbol.
Do: Keep VerifyContractSymbol exported in internal/core/consolidate/run.go.

### [con-01M52AEFR846SZTMTXA20FA941] **Preserve ULIDs and commit SHAs in scrubber allow-lists**
`convention · kind: semantic · anchors: internal/core/scrub/scrubber.go · verified 2026-10-16 (696a1b8, anchor-verified) · evidence: note-01M52AEFPKM1YAWTHK98TD2ZGF`
Why: Confirmed by user so op references remain intact.
Do: Populate KnownIDs and KnownGitSHAs before calling scrub.Scrub.

### [inv-01M52AEFR68PEVJ0MBT2Z1YK54] **Validate batch ULIDs before writing ops JSON**
`invariant · kind: semantic · anchors: internal/core/ops/writer.go · verified 2026-10-16 (696a1b8, test-pass) · evidence: note-01M52AEFPKM1YAWTHK98TD2ZGF`
Why: Prevents partial batch files in .memory/ops.
Do: Validate every op in WriteBatch before atomic rename.

## Open issues

### [issue-01M52AEFR5Y1CYNY8SCMK5RBA5] **Hypothetical distributed cache lock invariant**
`open-issue · kind: semantic · anchors: internal/core/cache/nonexistent_lock.go · evidence: note-01M52AEFP9KQPDV50YB2MA39GC`
Why: Speculated during planning without code or test.
Do: Acquire distributed lock before flushing cache.
```

### 5.3 Branch-Tier Dossier: `.memory/local/dossiers/feature-ds18b20-probe/dossiers/main/modules.md`

Routed automatically to the git-ignored branch tier (`.memory/local/dossiers/feature-ds18b20-probe/`) because `note.Branch == "feature/ds18b20-probe"` and `main/modules/ds18b20_sensor/driver.c` does not yet exist on `main`:

```markdown
# main/modules

## Purpose and boundaries
Curated repository memory for main/modules.

## Decisions

### [dec-01M52AEFR5Y1CYNY8SCJNR2HRG] **Use 1-Wire CRC8 table in DS18B20 sensor driver**
`decision · kind: episodic · anchors: main/modules/ds18b20_sensor/driver.c · evidence: note-01M52AEFPTJQG7Z8X7C2P14XC5`
Why: Avoids bit-banging loop overhead on feature/ds18b20-probe.
Do: Keep lookup table in main/modules/ds18b20_sensor/driver.c.
```

### 5.4 User-Tier Dossier: `<dataDir>/<repo-id>/user-dossiers/dossiers/workstation/gh.md`

Routed automatically to the personal user tier (`user-dossiers/`) because the item carries `scope: "user:workstation/gh"` and workstation-local `~/` / `localhost:` paths:

```markdown
# workstation/gh

## Purpose and boundaries
Curated repository memory for workstation/gh.

## Ruled out, failure signatures and gotchas

### [got-01M52AEFR5Y1CYNY8SCFC6K42A] **Local workstation keyring socket path for gh CLI**
`gotcha · kind: semantic · anchors: ~/.config/gh/hosts.yml · evidence: note-01M52AEFPTJQG7Z8X7C2P14XC5`
Why: Headless shells need ~/.config/gh and localhost:8080 proxy configured.
Do: Set REPO_MEMORY_DATA_DIR and check ~/.config/gh/hosts.yml on workstation.
```

---

## 6. Reproducibility

Run the standalone write-side micro-probe script from the repository root:

```bash
./evaluation/eval-v2/scripts/run_write_probe.sh
```

Or invoke the Go test suite directly:

```bash
go test -v -run TestEvalV2WriteSideMicroProbe ./internal/core/tick/...
```
