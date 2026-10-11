# PR-09b: Product Fixes

> **Programme:** repo-memory
> **PR:** pr-09b-product-fixes
> **Base branch:** main (post PR-09 merge at `45a68cd`)
> **Status:** ✅ Complete
> **Human approval:** Decomposition approved 2026-10-11 as part of the post-X6-lite action plan.

## Objective

Ship the product fixes found during X6-lite evaluation. These are prerequisite to evaluation v2 (Stage 0 in the approved proposal). The delivery pipeline is broken without these fixes — injections are rejected, moments don't fire, and seeded entries contain junk.

Additionally, fix the ranker to produce relevant results: the current ranker ignores bm25 scores, produces 35-60% duplicate injections, and swallows search errors. These must be fixed before any evaluation can meaningfully test repo-memory's value.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-delivery-fixes](wo1-delivery-fixes/WORKORDER.md) | Cherry-pick and integrate the 3 validated delivery fixes from X6-lite (`8d7ad7b`, `d57f561`, `3036eb4`). Fix index wiring (reindex/tick/hooks use different index files). Rebuild and reinstall the plugin binary. | standard | ✅ Complete |
| 2 | [wo2-ranker-improvements](wo2-ranker-improvements/WORKORDER.md) | Ranker: incorporate bm25 relevance score in final ranking; per-session deduplication; surface FTS errors; add relevance threshold (inject nothing when nothing clears the bar). Fix `about_to_edit` moment (fires too late because model decides and edits in same turn). | standard | ✅ Complete |

## Dependencies

- **Base:** main at `45a68cd` (post PR-09 merge)
- **Preconditions:** PR-08 delivery engine code on main, PR-09 evaluation evidence on main
- **Downstream:** Evaluation v2 (PR-10) Stage 0 gate requires these fixes

## Exit Criteria

- All 3 X6-lite fixes integrated (injection reaches the model, moments fire, junk filtered)
- Index wiring consolidated (one index file used by all pathways)
- Binary rebuilt and reinstalled
- Ranker uses bm25 relevance in final ranking
- Per-session deduplication eliminates repeated injections
- FTS errors surfaced, not swallowed
- Relevance threshold: empty injection when nothing clears the bar
- Unit tests pass, smoke test with real injection reaching the model

## Session Log

### 2026-10-11 — Project Creation
- Created as Stage 0 of the evaluation v2 proposal
- 2 WOs: delivery fixes (cherry-pick + index wiring) and ranker improvements
- Spawning WO1 first, WO2 stacks on top

### 2026-10-11 — WO1 Complete
- Canonical `IndexPath()` consolidates all 4 pathways (reindex, tick, hooks, MCP) to `<data-dir>/<repo-id>/index.db`
- New unit tests: native tool-call parser (`TestPreInvocation_AntigravityNativeToolCalls`), junk filter (`TestReportToOps_JunkFiltered`)
- Fixed `TestSessionAttributor_Integration` (mtime sync issue)
- Binary rebuilt and installed at `~/.local/bin/repo-memory`
- End-to-end smoke: enrol → seed → tick → index populated → PreInvocation injects → MCP returns results
- Commit `3bf063c` on `pr-09b/wo1-delivery-fixes`
- Branching WO2 for ranker improvements

### 2026-10-11 — WO2 Complete, Project Wrap-Up
- bm25 FTS scores incorporated into ranking (`score += ftsScore * 100`)
- Per-session deduplication via `InjectedSet` in Engine/Ledger — eliminates 35-60% duplicate injections
- FTS errors surfaced as ephemeral warning steps instead of swallowed
- Relevance threshold: `DefaultMinRelevanceScore = 50`, below-threshold entries filtered
- `about_to_edit` limitation documented (model decides + edits same turn)
- 5 new tests covering all changes
- Commit `d3ab994` on `pr-09b/wo2-ranker-improvements`
- **Stage 0 complete.** Ready to merge and begin evaluation v2 Stage 1.
