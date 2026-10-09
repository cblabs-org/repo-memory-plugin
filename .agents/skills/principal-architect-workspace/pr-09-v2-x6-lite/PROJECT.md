# PR-09: X6-lite Early Kill Gate

> **Programme:** repo-memory
> **PR:** pr-09-v2-x6-lite
> **Base branch:** PR-08-v2-delivery
> **Status:** ☐ Planned
> **SPEC ref:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan ref:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Answer the project's central question before building distillation and consolidation: **do delivered dossiers make agents better on this repository's work?** It uses time-cut dossiers from PR-07 seeding, delivered by PR-08, under the X6 protocol.

## Design & SPEC Sections

- **v2 Design Sections:** §12 (X6 B1 Replay Evaluation), §2.3
- **v2 Decisions:** PD21 (Read side first), PD22 (Early kill gate), PD23 (X6 protocol)

### X6 Protocol (shared by X6-lite and X6, PD23)

The protocol is written down and approved by the user before any run.

1. **Work orders.**
   - 6–8 completed work orders that have executable exit tests.
   - Chosen where earlier knowledge plausibly matters: they depend on decisions, dead ends or external behaviour found in earlier work orders.
   - Include at least two where a no-memory run is expected to struggle.
2. **Time cut.** For each work order, dossiers are built only from sessions, PA reports and commits **before that work order's parent commit**. PA documents are also taken as of the parent commit. Nothing from the target work order's own sessions, or anything later, may appear in any condition.
3. **Conditions:**
   - (a) no memory;
   - (b) PA documents as of the parent commit;
   - (c) time-cut dossiers delivered through the PR-08 mechanism.
4. **Seeds.** At least 2 per work order and condition. A third is added where results disagree.
5. **Metrics:**
   - Primary: the share of exit tests passing.
   - Secondary: steps and total tokens.
   - The pass threshold is fixed in advance: (c) beats (a) on the primary metric, aggregated over work orders. "Match PA at fewer tokens" is reported, not gating, for X6-lite.
6. **Budget.** Estimate the quota per run, and schedule runs across the 5-hour and weekly pools. The user approves the budget.
7. **Snapshot.** Copy the candidate work orders' transcripts and PA files into the evidence store immediately, before pruning removes them (PD17).

## Dependencies & Gate Preconditions

- **Base branch:** `PR-08-v2-delivery`
- **Preconditions:**
  - PR-08-v2-delivery complete (delivery engine, hooks, ledger, MCP tools working against dossiers).
  - PR-07-v2-storage complete (`--before <commit>` time-cut seeding ready).
  - Candidate work order transcripts and PA files snapshotted into evidence store before pruning removes them (PD17).
- **Mandatory Gate Requirement:** The protocol, selected work orders, and quota budget **MUST be approved by the user before any run begins**.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-protocol` | Write the pre-registered protocol: work-order selection with reasons, time cuts, conditions, seeds, primary and secondary metrics, threshold, and quota budget and schedule. The user approves it before any run. | interactive | ✅ Complete |
| 2 | `wo2-dossier-build` | For each selected work order, build time-cut dossiers with `--before <parent commit>`, and time-cut PA documents. The user reviews the dossiers for leakage and accuracy. | standard | ✅ Complete |
| 3 | `wo3-replay-harness` | A harness that checks out the parent commit, runs a worker on the work order's text under each condition, and collects test results, steps and tokens. Reuse the PR-03 eval harness. | standard | ✅ Complete |
| 4 | `wo4-runs` | Execute the runs to the schedule and budget. | interactive | ☐ Planned |
| 5 | `wo5-report-decision` | Report the results against the pre-registered threshold. Recommend continue, redirect (for example, delivery-only with PA-sourced dossiers) or stop. The user decides. | standard | ☐ Planned |

## Exit Criteria

- **Early kill gate:** time-cut dossiers beat no-memory on the primary metric. If not, the project stops or is redirected by user decision.
- Results, including the PA comparison and token costs, recorded in `docs/experiments/X6-lite.md`.

## Human Tasks

- **Protocol & Budget Approval:** Approve the pre-registered X6 protocol, work-order selections, and quota budget before any run (wo1).
- **Leakage Review:** Review the time-cut dossiers for leakage and accuracy (wo2).
- **Gate Decision:** Decide continue, redirect (e.g. delivery-only with PA-sourced dossiers), or stop based on the evaluation report (wo5).

## Risks

- **X6 leakage:** Dossiers contain knowledge from the target work order or later. Mitigation: Enforce strict time cut by parent commit for dossiers and PA documents; user leakage review before runs.
- **X6 too small to detect effects:** Sample noise obscures true impact. Mitigation: Pre-registered metric; 6–8 work orders chosen with headroom; ≥ 2 seeds per condition (adding a 3rd on disagreement).
- **Conversation pruning destroys inputs:** Antigravity pruning (~500 conversations kept, ~19 days) removes candidate transcripts. Mitigation: Snapshot candidate work order transcripts and PA files into the evidence store in advance (PD17 / PR-05 wo3).
- **Dossiers don't improve outcomes:** Time-cut dossiers fail to beat no-memory. Mitigation: The early kill gate stops or redirects the programme before investing in distillation (PR-10) and consolidation (PR-11).

## Session Log

### 2026-10-07 — Initialization & WO1 Handoff
- Programme housekeeping: marked PR-08 complete, updated MEMORY.md and PROGRAM.md.
- Created branch `pr-09-v2-x6-lite/wo1-protocol` based on `main`.
### 2026-10-09 — WO1 Complete & Starting WO2/WO3
- WO1 `wo1-protocol` completed interactively. Protocol approved by user, selecting 8 WOs, setting threshold to condition (c) strictly beating condition (a).
- Prepared to branch and spawn WO2 `wo2-dossier-build` and WO3 `wo3-replay-harness` in parallel.

### 2026-10-09 — WO2/WO3 Complete & WO4 Handoff
- WO2 (Dossier Build) successfully generated time-cut `ops/`, `dossiers/`, and `pa_docs/` in `evaluation/data/` for all 8 WOs. Snapshotted PA docs.
- WO3 (Replay Harness) successfully built `internal/eval/x6_replay.go` containing the dry-run tested evaluation loops.
- Created branch `pr-09-v2-x6-lite/wo4-runs` based on `wo3-replay-harness`.
- Prepared WO4 WORKORDER.md (interactive complexity). Handoff ready.
