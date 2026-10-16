# Project: `pr-13-v2-consolidation` — Stage 2 Warm Batch Consolidation, Echo Filter, Corroboration Gate & Five-Tier Scope Hierarchy

> **Programme:** [PROGRAM.md](../programme-repo-memory/PROGRAM.md) · [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)
> **Status:** Active (`WO3/6` in progress — `WO1`–`WO2` complete)
> **Base Branch:** `pr-12-v2-distillation/wo6-managed-instruction-block-and-verification` (`e6ee499`)
> **Specifications:** [SPEC.md](../../../../SPEC.md) (§5.3–§5.5, §6.4–§6.5) · [v2.2 Design Rev 4](../../../../docs/design-v2-distillation-retrieval.md) (§5.6, §6, §7.1) · [Addendum A Rev 3](../../../../docs/addendum-a-provenance-and-hardening.md) (§5 A4, §7 A6, §11.2)

---

## 1. Objective & Architectural Scope

Implement **Stage 2 Warm Batch Consolidation & Five-Tier Publish (`v2.2`)**, turning Stage 1 inbox notes (`turn-note`, `mcp-note`, `checkpoint`, `artifact`, `sidecar`, `pa`) into validated op-log batches (`.memory/ops/<yyyy-mm>/<ulid>.json`), projected dossiers (`.memory/dossiers/`), `.memory/ATLAS.md`, and `index.db`:

1. **Deterministic Echo Filter (`P13`) & Pre-Consolidation Corroboration Gate (`VD18`):**
   - **Echo Filter (`P13`):** Drop any candidate note item that repeats an entry in its note's `ExposureList` (or repeats an existing entry when `ExposureList` is unknown) unless the source episode both modified one of the entry's anchored files **and** subsequently passed a build or test step.
   - **Corroboration Gate (`VD18`):** Require any candidate `semantic` or `procedural` rule (`invariant`, `contract`, `convention`, `recipe`, `procedure`, or `gate: edit`) arriving with `verified: null`/empty to be corroborated by (a) a passing `PostToolUse` build/test step after an anchored edit in the episode, (b) a verified static code anchor present in `HEAD`, or (c) explicit `USER_INPUT` confirmation in the episode. Uncorroborated assertions are automatically downgraded to `open-issue` (with a `closes_when` condition).
2. **Deterministic Routing, Exact-Duplicate Merging & Existing-Entry Anchor Checks:**
   - Route candidate items to dossiers via `config.ScopeResolver` and `DeriveModuleScopes`, and classify each item into the **Five-Tier State Scope Hierarchy (`VD17`)**: `user:` (`<data-dir>/<repo-id>/user-dossiers/`), `branch:` (`.memory/local/ops/<branch-slug>/` & `.memory/local/dossiers/<branch-slug>/` for unmerged branch-only anchors), or `repo:` (`.memory/ops/` & `.memory/dossiers/`).
   - Merge exact duplicates deterministically into `OpAmend` (adding evidence IDs) without an LLM call, and flag existing dossier entries whose anchors no longer resolve in `HEAD`.
3. **Warm Multi-Turn `stream-json` Consolidation (`VD15`):**
   - Run `repo-memory-distiller` across target dossiers (supporting multi-turn `--input-format stream-json --output-format stream-json` warm-cache batching across dossiers in a single process per tick, plus `StageConsolidate` via `harness.Housekeeper`) with `<consolidation-batch-TOKEN>` data framing (§5.6).
   - Emit typed ops (`add`, `amend`, `supersede`, `retire`, `move`, `split`) — never rewriting whole markdown pages.
4. **Deterministic Op Validation, Recording, Projection & Cap Linting:**
   - Validate every op against existing entry IDs, required fields, line/token caps, and the 4-layer secret scrubber (`scrub.Scrub`), append one ULID batch file per tier, re-render `.memory/dossiers/` (with `{section?}` resolution), `.history.md`, `.memory/ATLAS.md`, rebuild `index.db`, and mark consolidated inbox notes (`Consolidated = true`).
5. **Five-Tier Storage, Branch-Merge Promotion & Two-Lane Worktree Publish:**
   - Isolate `user:` dossiers in `<data-dir>/<repo-id>/user-dossiers/` and unmerged `branch:` ops in gitignored `.memory/local/`.
   - Promote `.memory/local/ops/<branch-slug>/` batches to shared `.memory/ops/` when a feature branch merges into the default branch (`PromoteMergedBranchOps`).
   - Publish shared `repo:` changes into a dedicated git worktree (`<data-dir>/<repo-id>/worktrees/<clone-id>/`, branch `memory/pending`) without touching the user's working tree or checkout.
6. **Tick Orchestration & `repo-memory consolidate` CLI:**
   - Wire Stage 2 consolidation into `PipelineExecutor.Run` (running when $\ge 10$ unconsolidated notes are waiting or every 24h, gated by `/usage` $\ge 25\%$ headroom and daily token budgets), record freshness stamps in `<data-dir>/<repo-id>/state/freshness.json`, and expose `repo-memory consolidate` (`--force`, `--dry-run`, `--json`).

---

## 2. Work Order Decomposition (Approved)

| # | Work Order | Branch | Scope & Deliverables | Status |
|---|---|---|---|---|
| **WO1** | [wo1-echo-filter-and-corroboration-gate](wo1-echo-filter-and-corroboration-gate/WORKORDER.md) | `pr-13-v2-consolidation/wo1-echo-filter-and-corroboration-gate` | Create `internal/core/consolidate/filter.go` & `filter_test.go` implementing the deterministic P13 Echo Filter (`FilterEchoes`) and the `VD18` Pre-Consolidation Corroboration Gate (`EnforceCorroborationGate` downgrading uncorroborated `semantic`/`procedural` rules to `open-issue`). | ✅ Complete (`0454981`) |
| **WO2** | [wo2-routing-dedup-and-anchor-checks](wo2-routing-dedup-and-anchor-checks/WORKORDER.md) | `pr-13-v2-consolidation/wo2-routing-dedup-and-anchor-checks` | Create `internal/core/consolidate/route.go` & `route_test.go` implementing Five-Tier scope classification (`user:`, `branch:`, `repo:`), dossier routing (`config.ScopeResolver` + `DeriveModuleScopes`), deterministic exact-duplicate merging (`MergeExactDuplicates` -> `OpAmend`), and broken-anchor detection on existing dossier entries (`CheckDossierAnchors`). | ✅ Complete (`3adc140`) |
| **WO3** | [wo3-multi-turn-stream-json-consolidator](wo3-multi-turn-stream-json-consolidator/WORKORDER.md) | `pr-13-v2-consolidation/wo3-multi-turn-stream-json-consolidator` | Create `internal/core/consolidate/llm.go` & `llm_test.go` (and multi-turn `stream-json` support in `internal/harness/antigravity/executor.go`): `<consolidation-batch-TOKEN>` data framing, `echo_token` verification, tool-call/page-rewrite rejection, multi-dossier warm-cache consolidation, and deterministic op synthesis fallback. | 🔄 In Progress |
| **WO4** | [wo4-validate-record-and-render](wo4-validate-record-and-render/WORKORDER.md) | `pr-13-v2-consolidation/wo4-validate-record-and-render` | Create `internal/core/consolidate/record.go` & `record_test.go`: state-aware op validation (existing target IDs, required fields, ≤6-line cap, `scrub.Scrub` + quarantine), typed entry ID generation, op-batch writing, dossier/atlas/history projection, `dossiers.LintDossier` cap checks, `index.db` rebuild, and `inboxStore.MarkConsolidated`. | ☐ Planned |
| **WO5** | [wo5-five-tier-scope-and-two-lane-publish](wo5-five-tier-scope-and-two-lane-publish/WORKORDER.md) | `pr-13-v2-consolidation/wo5-five-tier-scope-and-two-lane-publish` | Create `internal/core/consolidate/publish.go` and `internal/core/gitops/worktree.go` (plus tests): `.memory/.gitignore` (`local/`, `.cache/`), `user-dossiers/` projection, `.memory/local/` branch-tier storage, `PromoteMergedBranchOps` on branch merge, branch-local index inclusion, and dedicated git worktree publishing on `memory/pending`. | ☐ Planned |
| **WO6** | [wo6-tick-orchestration-and-verification](wo6-tick-orchestration-and-verification/WORKORDER.md) | `pr-13-v2-consolidation/wo6-tick-orchestration-and-verification` | Create `internal/core/tick/consolidate.go` & `cmd/repo-memory/cmd_consolidate.go`, wire into `PipelineExecutor.Run` ($\ge 10$ notes or 24h cadence, quota guard, `state/freshness.json`), add end-to-end consolidation tests, and rebuild `~/.local/bin/repo-memory`. | ☐ Planned |

---

## 3. Approval & Execution Ledger

- **Plan Approved:** 2026-10-16 (Chief Architect `v2.2` Programme Plan)
- **Execution Mode:** Strict 3-Tier Delegation (PA spawns 1 dedicated Tier 3 Execution Worker per Work Order; Cold-Firewall PR Audit Sub-Agent reviews the stacked GitHub PR at Step 6A before Step 6B graduation).

## 4. Session Log
- `2026-10-16`: Initialized `pr-13-v2-consolidation` on base `pr-12-v2-distillation/wo6-managed-instruction-block-and-verification` (`e6ee499`).
- `2026-10-16`: Completed **WO1 (`wo1-echo-filter-and-corroboration-gate`)** (`0454981`): implemented `FilterEchoes` (P13 echo filter with edit+test reconfirmation and Addendum A §11.2 unknown-exposure handling), `EnforceCorroborationGate` (`VD18` corroboration gate via edit+test pass, static file/symbol anchor verification, or explicit human `USER_INPUT` confirmation, downgrading uncorroborated rules to `open-issue` with deterministic `closes_when`), and shared title/anchor normalization helpers in `internal/core/consolidate/filter.go`.
- `2026-10-16`: Completed **WO2 (`wo2-routing-dedup-and-anchor-checks`)** (`3adc140`): implemented `ClassifyTier` (`user:`, `branch:`, `repo:`), `RouteItem`/`RouteItems` (`ScopeResolver` + `DeriveModuleScopes` + keyword routing), `MergeExactDuplicates` (coalesced `OpAmend` preserving existing evidence + batch-internal candidate deduplication across alias-equivalent types), and `CheckDossierAnchors` in `internal/core/consolidate/route.go`.
