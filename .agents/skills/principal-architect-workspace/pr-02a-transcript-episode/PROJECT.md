# PR-02a: Transcript → Episode

> **Programme:** repo-memory
> **PR:** pr-02a-transcript-episode
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Turn an Antigravity conversation's transcript file set into a compacted, scrubbed, attributed episode. This is pure transformation, testable entirely on fixtures before any runtime capture exists.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | `wo1-transcript-parser` | `pr-02a-transcript-episode/wo1-transcript-parser` | ☐ Pending | architect | — |
| 2 | `wo2-compaction` | `pr-02a-transcript-episode/wo2-compaction` | ☐ Pending | architect | — |
| 3 | `wo3-episode-store` | `pr-02a-transcript-episode/wo3-episode-store` | ☐ Pending | standard | — |
| 4 | `wo4-commit-linkage` | `pr-02a-transcript-episode/wo4-commit-linkage` | ☐ Pending | standard | — |
| 5 | `wo5-metrics-skeleton` | `pr-02a-transcript-episode/wo5-metrics-skeleton` | ☐ Pending | standard | — |

## Exit Criteria
- Fixture conversations produce episodes with no unresolved `truncated_fields`; the chunk-only fallback yields byte-identical results to the monolithic file
- Compaction ratio ≥ 4× measured on full transcripts; §4.4.1 sizes re-measured
- Zero secrets in a scan of 50 fixture episodes
- Core tests pass on the `fake` adapter; `internal/core` imports no concrete adapter

## Session Log
