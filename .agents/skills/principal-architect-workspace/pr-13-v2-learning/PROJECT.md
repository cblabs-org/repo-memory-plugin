# PR-13: Close Conditions, Coverage & Metrics

> **Programme:** repo-memory
> **PR name:** pr-13-v2-learning
> **Base branch:** `PR-12-v2-pilot`
> **Status:** ☐ Planned
> **SPEC ref:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan ref:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Establish the continuous learning loop to maintain memory quality over time: evaluate machine-checkable `closes_when` conditions in the tick to retire completed or invalidated entries, run weekly gardening to propose splits and merge near-duplicates, calculate coverage and gap scores per dossier, elicit human knowledge on high gaps, collect session-level push and retrieval metrics, and optimize delivery policy via offline replay.

## Design & SPEC Sections

- **v2 Design Document:**
  - [§5.2 (What qualifies as a note: close conditions)](../../../docs/design-v2-distillation-retrieval.md#52-what-qualifies-as-a-note): Machine-checkable `closes_when` rules (issue closed, branch merged, work order completed, command passes, path or symbol exists/absent, file contains/lacks pattern). Entries without conditions are pull-only.
  - [§9 / §11 (Learning Loop & Maintenance)](../../../docs/design-v2-distillation-retrieval.md#11-what-changes-in-the-current-build): Adapted learning loop, gardening jobs, metrics collection in `metrics/*.jsonl`, and offline delivery policy replay tuning.
- **SPEC v0.10:**
  - §9 (Maintenance & Feedback Loops): Gardening, coverage scoring, elicitation, and metrics.

## Dependencies & Gate Preconditions

- **Base branch:** `PR-12-v2-pilot`
- **Gate Precondition:** **Confirmatory X6 must have passed** in PR-12 (distilled dossiers beat no-memory and match PA docs at fewer tokens).

## Work Order Ledger

| # | WO name | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-close-conditions` | Evaluate `closes_when` in the tick: issue closed, branch merged, work order completed, command passes, path or symbol exists or is absent, file contains or lacks a pattern. Retire the entry with the reason. Entries without a condition are pull-only. | standard | ☐ Planned |
| 2 | `wo2-gardening` | Weekly, gated job with every op reviewed in the memory PR: propose splits near the cap, merge near-duplicates, retire entries with gone anchors. | standard | ☐ Planned |
| 3 | `wo3-coverage` | Coverage and gap scores per dossier and per type | standard | ☐ Planned |
| 4 | `wo4-elicitation` | 1–3 questions from the highest gaps, written to the weekly review and `QUESTIONS.md`. Answers become notes tagged `source: human`. | standard | ☐ Planned |
| 5 | `wo5-session-metrics` | Push precision (pushed entries referenced later), tokens pushed, hit rate, fallback rate. Stored in `metrics/*.jsonl`. | standard | ☐ Planned |
| 6 | `wo6-replay-tuning` | Offline replay of delivery policy on episode prefixes, labelled from suffixes. **Exit:** one policy change that improves push precision on the replay set without lowering hit rate. | architect | ☐ Planned |

## Exit Criteria

- Close conditions fire correctly; gardening proposals are sensible; gap scores are meaningful.
- Questions appear in the review; metrics are collected; one policy change is accepted per wo6.

## Human Tasks

- Review memory PRs containing weekly gardening proposals (every op reviewed).
- Answer weekly review reports and gap questions in `QUESTIONS.md` (answers become notes tagged `source: human`).

## Risks

- **Close conditions misfire:** Conditions retire valid entries prematurely or fail to retire stale entries. *Mitigation:* Explicit deterministic evaluation in the tick with logged reasons; unconditioned entries remain pull-only. (Owner: PR-13)
- **Gardening degrades dossiers:** Automated split or merge proposals reduce dossier quality. *Mitigation:* Gated weekly job where every op is reviewed in the memory PR by a human before applying. (Owner: PR-13)
- **Elicitation burden:** Excessive questions overburden developers. *Mitigation:* Capped at 1–3 targeted questions from the highest gaps, routed to weekly reviews. (Owner: PR-13)

## Session Log

<!-- Empty, ready for use -->
