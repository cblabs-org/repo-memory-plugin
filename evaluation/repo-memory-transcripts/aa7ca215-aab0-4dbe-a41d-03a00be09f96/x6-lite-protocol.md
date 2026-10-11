# X6-lite Pre-Registered Protocol

> **Experiment:** X6-lite — Early Kill Gate  
> **PR:** pr-09-v2-x6-lite  
> **Protocol Design:** PD23  
> **Status:** DRAFT — awaiting user approval  
> **Date:** 2026-10-09  

---

## 1. Objective

Determine whether **delivered dossiers make agents better at completing work orders on this repository**. This is the early kill gate: if time-cut seeded dossiers (condition c) do not beat no-memory (condition a) on the primary metric, the project stops or is redirected before investing in distillation (PR-10) and consolidation (PR-11).

---

## 2. Selected Work Orders (7)

Work orders are chosen where earlier knowledge plausibly matters — they depend on architectural decisions, dead ends, API quirks, or cross-cutting conventions discovered in earlier work. At least two are expected to be difficult without memory.

| # | WO ID | PR | Description | Parent Commit | Why Memory Matters |
|---|---|---|---|---|---|
| **1** | `wo1-op-log` | PR-07 | Append-only op log: 6 op types, JSON batch files (ULID-named), month-sharded layout, writer/reader/iterator, `IterateBefore` time cut | `42deb64` | **Low (negative control).** Self-contained package with clean boundaries. Requires the v2 design spec and ULID conventions but minimal accumulated project knowledge. Useful as a calibration point: we expect all 3 conditions to perform similarly. |
| **2** | `wo2-dossier-schema-renderer` | PR-07 | Dossier schema (8 entry types), deterministic renderer, atlas, history, cap linting, hand-edit detection | `4004825` | **High.** Must implement the exact schema from the v2 design (8 entry types, required fields, ~3k token cap). Depends on op-log conventions from WO1 and the v2 design decisions (PD22–PD26). Hand-edit detection is a novel pattern not obvious without history. |
| **3** | `wo3-scopes-aliases` | PR-07 | Scope resolver: declared, derived (entity extractor), alias (split ops) | `3177bfe` | **High.** Three-source scope combiner depends on the entity extractor from PR-04, the config layout from PR-01, and the split op type from WO1. Alias resolution from `memory.brief` needs the exact brief design. |
| **4** | `wo4-index-migration` | PR-07 | SQLite index migration: entries/anchors/FTS5/aliases/error patterns tables; rebuild from op log | `501d05d` | **High.** Must replace v1 claims/relations tables with v2 schema. Depends on knowing the exact op-log structure (WO1), entry schema (WO2), and anchor/alias conventions (WO3). FTS5 decisions from PR-03 carry forward. |
| **5** | `wo6-seeding` | PR-07 | PA report parser, lean-agent seeding pass, `--before <commit>` time cut | `ada9476` | **Critical — expected to struggle without memory.** The seeding pass must produce well-formed ops for the WO1 op-log, emit entries matching the WO2 schema, and implement time cuts for X6. Depends on dead ends from the v1 pipeline evaluation (B5 found "accurate but shallow"). The `--before` time cut is a novel requirement from PD23. |
| **6** | `wo1-ledger-engine` | PR-08 | Injection ledger, anchor verification, ranking, budgets, exposure tracking | `7321774` | **High.** Builds directly on the PR-07 dossier/index/anchor systems. Must implement the ranking formula (exact anchor > directory; invariant > failure-sig > ruled-out; verified > unverified) from the v2 design. Cross-process safety via `flock` was a late design decision. |
| **7** | `wo4-gates` | PR-08 | PreToolUse deny-once gates, force_ask for invariants | `e4acaa2` | **Critical — expected to struggle without memory.** Must follow the X9 hook contract (bimodal: allow/deny with message) discovered in PR-06 spike. Deny-once lifecycle (PendingGates handshake between PreToolUse→PostToolUse) is a novel pattern. Gate entries must be tagged correctly per the WO2 entry schema. |
| **8** | `wo6-pull-tools` | PR-08 | MCP tools: memory_brief, memory_why, memory_history, memory_search, memory_note, memory_stats | `6a840b3` | **Medium-High.** Must use underscore naming (not dots), resolve aliases (WO3 scopes), query the FTS5 index (WO4), record in exposure lists (WO1 ledger). Depends on the MCP server architecture established in PR-06b. |

### Selection Rationale

- **8 work orders** across 2 PRs (PR-07: 5 WOs, PR-08: 3 WOs) — all are late-programme WOs where accumulated knowledge is richest.
- **≥ 2 expected to struggle without memory:** WO6-seeding (novel time-cut requirement, depends on v1 dead ends) and WO4-gates (depends on X9 hook contract, novel deny-once lifecycle pattern).
- **1 negative control:** WO1-op-log is self-contained with low prior-knowledge dependency — we expect all conditions to perform similarly, providing a calibration point.
- All 8 have **executable exit tests** (Go test suites, specific verification commands).
- The WOs form a **dependency chain** — each later WO depends on conventions and data structures introduced by earlier ones, making earlier knowledge genuinely useful.

---

## 3. Time Cuts

For each work order, dossiers and PA documents are built **only from sessions, reports, and commits before that work order's parent commit**. Nothing from the target work order's own sessions or later may appear.

| WO | Parent Commit | Time Cut |
|---|---|---|
| PR-07/wo1 | `42deb64` (PA: PR-07 project setup) | All material before this commit. Includes: PR-01 through PR-06b sessions, all PA reports up to this point. No PR-07 sessions yet. |
| PR-07/wo2 | `4004825` (PA: WO1 complete, WO2 active) | Adds: PR-07/wo1 sessions and completion report. |
| PR-07/wo3 | `3177bfe` (PA: WO2 complete, WO3 active) | Adds: PR-07/wo2 sessions and completion report. |
| PR-07/wo4 | `501d05d` (PA: WO3 complete, WO4 active) | Adds: PR-07/wo3 sessions and completion report. |
| PR-07/wo6 | `ada9476` (PA: WO5 complete, WO6 active) | Adds: PR-07/wo3–wo5 sessions and completion reports. |
| PR-08/wo1 | `7321774` (PA: PR-08 project setup) | All of PR-07 complete. Includes all PR-07 sessions, completion reports, merged code. |
| PR-08/wo4 | `e4acaa2` (PA: WO3 complete, WO4 WORKORDER) | Adds: PR-08/wo1–wo3 sessions and completion reports. |
| PR-08/wo6 | `6a840b3` (PA: WO4 complete, WO5+WO6 WORKORDERs) | Adds: PR-08/wo1–wo4 sessions and completion reports. |

### Leakage Guard

- Dossiers built with `repo-memory seed --before <parent-commit>`.
- PA documents (PROJECT.md, WORKORDER.md, MEMORY.md) taken from git at the parent commit.
- User reviews dossiers for leakage before runs (WO2 of PR-09).

---

## 4. Conditions

Each work order is run under 3 conditions:

| Condition | Label | Description |
|---|---|---|
| **(a)** No Memory | `baseline` | The agent receives only the WORKORDER.md text and the codebase at the parent commit. No PA documents, no dossiers, no memory of any kind. |
| **(b)** PA Documents | `pa-docs` | The agent receives the WORKORDER.md text **plus** all PA documents (PROJECT.md, PROGRAM.md, MEMORY.md, MODULE_INDEX.md, relevant earlier WORKORDERs with completion reports) as they existed at the parent commit. No dossiers. |
| **(c)** Time-Cut Dossiers | `dossiers` | The agent receives the WORKORDER.md text **plus** time-cut dossiers delivered through the PR-08 delivery mechanism (systemMessage persistent pushes, ephemeral hints, deny-once gates, MCP pull tools). No PA documents. |

### Notes on Conditions

- Condition (b) is **reported but not gating** for X6-lite. "Match PA at fewer tokens" is informative only.
- The comparison that **gates** the project is: **(c) vs (a)** on the primary metric.
- All conditions use the same checkout (parent commit) and the same WORKORDER.md text.

---

## 5. Seeds

| Seeds per WO × Condition | Count |
|---|---|
| Minimum seeds | 2 |
| Tiebreaker seed (on disagreement) | +1 |

- **2 seeds** per work order per condition = 2 × 8 × 3 = **48 baseline runs**.
- A **3rd seed** is added for any (WO, condition) pair where the 2 seeds disagree on the primary metric (one passes all exit tests, the other fails at least one). Worst case: +24 tiebreaker runs = **72 total runs**.
- Seeds are deterministic random integers, pre-assigned:

| Seed | Value |
|---|---|
| Seed 1 | `42` |
| Seed 2 | `137` |
| Seed 3 (tiebreaker) | `256` |

Seeds control the model temperature/sampling and are passed as configuration to the replay harness.

---

## 6. Metrics

### Primary Metric (Gating)

**Share of exit tests passing per work order, aggregated across work orders.**

For each (WO, condition, seed) run:
1. Execute the work order's exit tests against the agent's produced code.
2. Record pass/fail for each individual test.
3. Compute the **exit test pass rate** = (tests passed / total tests).

Aggregation:
- For each (WO, condition), take the **mean** exit test pass rate across seeds.
- The **aggregate score** for a condition = mean of per-WO pass rates across all 8 WOs.

### Secondary Metrics (Reported, Not Gating)

| Metric | Description |
|---|---|
| **Steps** | Total agent steps (tool calls) per run. |
| **Total tokens** | Input + output tokens consumed per run. |
| **Wall-clock time** | Elapsed time per run. |
| **Per-WO breakdown** | Exit test results by individual work order. |

### Pass Threshold (Pre-Registered)

> **The project continues if condition (c) strictly beats condition (a) on the aggregate primary metric.**

Formally: `mean_pass_rate(c) > mean_pass_rate(a)`

Additionally reported (informative, not gating):
- Whether (c) matches or beats (b) on primary metric.
- Whether (c) achieves comparable results to (b) at fewer tokens.
- Per-WO breakdown showing which WOs benefited most from dossiers.

---

## 7. Exit Tests per Work Order

These are the executable tests that determine pass/fail:

### PR-07/wo1 — Op Log
```
go test ./internal/core/ops/... -count=1
```
Tests cover: op type validation (add, amend, supersede, retire, move, split), batch file round-trip (write → read), chronological iteration, `IterateBefore` time-cut via git commit, ULID ordering.

### PR-07/wo2 — Dossier Schema & Renderer
```
go test ./internal/core/dossiers/... -count=1
```
Tests cover: entry schema validation, deterministic rendering, atlas generation, history files, cap linting (~3k token limit), hand-edit detection.

### PR-07/wo3 — Scopes & Aliases
```
go test ./internal/core/config/... -count=1
```
Tests cover: declared scope loading, derived scope computation (entity extractor), alias resolution from split ops, `memory.brief` scope lookup.

### PR-07/wo4 — Index Migration
```
go test ./internal/core/index/... -count=1
repo-memory reindex   # must complete without error
```
Tests cover: v2 schema creation (entries, anchors, aliases, error patterns), FTS5 over entries, rebuild from op log, pointer-file swap.

### PR-07/wo6 — Seeding
```
go test ./internal/core/seed/... -count=1
repo-memory seed --before <test-commit>  # must produce valid ops
```
Tests cover: PA report parser, op validation, time-cut filtering, `source: seed` tagging, idempotent re-seeding (content-addressed IDs).

### PR-08/wo1 — Ledger Engine
```
go test ./internal/core/delivery/... -count=1
```
Tests cover: ledger CRUD, anchor verification (suspect marking), ranking formula, budget enforcement, exposure list recording, cross-process flock safety.

### PR-08/wo4 — Gates
```
go test ./internal/harness/antigravity/... -count=1
echo '{}' | go run ./cmd/repo-memory hook antigravity PreToolUse  # must output {"decision":"allow"}
```
Tests cover: deny-once lifecycle (PreToolUse deny → PostToolUse ack → PreToolUse allow), gate entry tagging, force_ask for invariants, X9 contract compliance (never return `{}`).

### PR-08/wo6 — Pull Tools
```
go test ./internal/mcpserver/... -count=1
```
Tests cover: all 6 MCP tools (brief, why, history, search, note, stats), alias resolution, FTS5 queries, exposure list recording, underscore naming.

---

## 8. Quota Budget & Schedule

### Per-Run Estimate

| Component | Estimate |
|---|---|
| Agent input tokens (codebase + instructions) | ~50k |
| Agent reasoning/output tokens | ~30k |
| Dossier injection overhead (condition c only) | ~5k |
| PA doc overhead (condition b only) | ~10k |
| **Estimated total per run** | **~80k–90k tokens** |
| Estimated steps per run | 15–40 |
| Estimated wall-clock per run | 3–8 minutes |

### Total Budget

| Scenario | Runs | Estimated Tokens |
|---|---|---|
| Base (2 seeds × 8 WOs × 3 conditions) | 48 | ~4.0M |
| Tiebreakers (worst case +24) | 72 | ~6.1M |
| **Budget request** | **≤ 72 runs** | **≤ 7M tokens** |

### Schedule

Runs are executed across the 5-hour and weekly quota pools:

| Batch | Runs | Conditions | Estimated Time |
|---|---|---|---|
| Batch 1 | 16 | (a) no-memory, all 8 WOs × seed 1+2 | ~1.5 hours |
| Batch 2 | 16 | (b) pa-docs, all 8 WOs × seed 1+2 | ~1.5 hours |
| Batch 3 | 16 | (c) dossiers, all 8 WOs × seed 1+2 | ~1.5 hours |
| Batch 4 | 0–24 | Tiebreaker runs (seed 3) | ~0–2.5 hours |

**Total estimated wall-clock: 4.5–7 hours** (across 1–2 days, depending on quota availability).

---

## 9. Snapshot (PD17 Compliance)

Before any run begins:
1. Copy all candidate work order transcripts into the evidence store.
2. Copy all PA files (PROJECT.md, WORKORDER.md, MEMORY.md, MODULE_INDEX.md, PROGRAM-PLAN.md) at each parent commit into the evidence store.
3. Verify no pruning has removed candidate materials.

This ensures the evaluation inputs survive Antigravity's ~19-day pruning window.

---

## 10. Reporting

Results will be recorded in `docs/experiments/X6-lite.md` with:

1. **Summary table:** aggregate pass rates for (a), (b), (c).
2. **Per-WO breakdown:** pass rates by work order and condition.
3. **Per-run detail:** exit test results, steps, tokens, wall-clock time.
4. **Token efficiency:** (c) vs (b) token comparison.
5. **Recommendation:** continue, redirect (e.g. PA-sourced dossiers only), or stop.
6. **Gate decision:** recorded with user's explicit approval.

---

## Approval

> [!IMPORTANT]
> This protocol must be approved by the user before any runs begin.
> No modifications to the protocol are permitted after approval without re-approval.

- [ ] Work order selection approved
- [ ] Time cuts approved
- [ ] Conditions approved
- [ ] Metrics and threshold approved
- [ ] Budget approved
- [ ] Schedule approved

**Approved by:** _______________  
**Date:** _______________
