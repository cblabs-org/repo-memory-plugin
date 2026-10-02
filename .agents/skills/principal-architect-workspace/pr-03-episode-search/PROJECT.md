# PR-03: Episode Search & Evaluation Harness

> **Programme:** repo-memory
> **PR:** pr-03-episode-search
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Make captured history searchable before the knowledge pipeline exists, and import PA files as searchable episodes. Build the evaluation harness and datasets (B1–B5) that later PRs are gated on, and measure the episode-search baseline for B2.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | `wo1-fts-index` | `pr-03-episode-search/wo1-fts-index` | ☐ Pending | standard | — |
| 2 | `wo2-search-cli` | `pr-03-episode-search/wo2-search-cli` | ☐ Pending | standard | — |
| 3 | `wo3-pa-import` | `pr-03-episode-search/wo3-pa-import` | ☐ Pending | standard | — |
| 4 | `wo4-ask-fallback` | `pr-03-episode-search/wo4-ask-fallback` | ☐ Pending | standard | — |
| 5 | `wo5-eval-harness` | `pr-03-episode-search/wo5-eval-harness` | ☐ Pending | standard | — |
| 6 | `wo6-eval-datasets` | `pr-03-episode-search/wo6-eval-datasets` | ☐ Pending | standard | — |

## Exit Criteria
- Can find what was tried, and when, across all captured and imported history
- PA workspace files searchable
- `memory.ask` returns relevant episodes with "unverified" marking
- B1, B2, B3 and B5 datasets exist and have been reviewed by the user
- B2 baseline (episode search) measured

## Session Log

- **2026-10-02**: Human approved PR-03 Work Order decomposition (WO1-WO6). Initiating WO1.
