# Principal Architect Memory
<!-- Last updated: 2026-10-15T19:26:00Z -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.2 plan approved (2026-10-15).** **PR-10a-v2-2-docs-and-specs complete (5/5 WOs)** (`SPEC.md` v1.0, Design v2 Rev 4, Addendum A Rev 3, Hooks Reference, `README.md`, v1 views removed). Next active project: **PR-10b-delivery-and-storage-fixes** (`UserMessage` persistence fix, `DetectRewind`, issues `#10`–`#13`, `#16`, PR #18 review remediation) before resuming **PR-10-eval-v2** Stage 2 (`WO4`–`WO7`). |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-10b-delivery-and-storage-fixes | **Active (6/6 WOs complete — pending Chief Architect PR #18 re-verification)** — WO1 (`89595e4`), WO2 (`a808fa5`), WO3 (`bed436e`), WO4 (`2b97c02`), WO5 (`60fd200`), WO6 (`bcdfae0`) complete | Delivery (`UserMessage`, `DetectRewind`, `InjectedSet`), Index (`#13`), Projector (`#11`), Seeder (`#10`, `#12`), Ops (`#16`) | [PROJECT.md](../pr-10b-delivery-and-storage-fixes/PROJECT.md) |
| pr-10-eval-v2 | **Paused at WO3/7 (Stage 1 complete)** — Resumes at WO4 (`wo4-dry-run`) after `pr-10b` lands | Evaluation, esp32 template, harness, seeded index | [PROJECT.md](../pr-10-eval-v2/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-10a-v2-2-docs-and-specs | 2026-10-15 | SPEC v1.0, Design v2 Rev 4, Addendum A Rev 3, Hooks Reference, README | Upgraded `SPEC.md` to v1.0 (`v2.2`), `design-v2-distillation-retrieval.md` to Rev 4, `addendum-a-provenance-and-hardening.md` to Rev 3, updated `antigravity-hooks-reference.md`, created `README.md`, removed stale v1 `GEMINI.md` and `.memory/views/`, and scrubbed legacy path hygiene across `docs/`. |
| pr-09b-product-fixes | 2026-10-11 | Delivery, Index, Ranker, Seeder | Stage 0: index wiring consolidated (canonical IndexPath), ranker improvements (bm25 relevance, per-session dedup, relevance threshold, FTS error surfacing), binary rebuilt. Evaluation v2 prerequisite complete. |
| pr-09-v2-x6-lite | 2026-10-11 | Evaluation, Replay Harness, X6 Protocol, Dossier Build | X6-lite STOP: 84% no-memory baseline, no headroom on spec-style tasks. 92 runs, ~22.1M tokens (3× budget). 3 product fixes found (injection channel, tool parser, placeholder filter). Harness validated and reusable. PA retrieval study: 1.3% MEMORY.md uptake. Evaluation v2 pivot approved (esp32, 2×2 factorial, 7.5M cap). |
| pr-08-v2-delivery | 2026-10-07 | Delivery, Ledger, Injection, Gates, Compaction, MCP Tools | Delivery engine: systemMessage/ephemeralMessage channels, deny-once gates, cross-process ledger with flock, compaction re-injection. Pull tools (MCP): memory_brief, memory_search, etc. 5 review rounds, 34 fixes. GitHub PR #14. |
| pr-07-v2-storage | 2026-10-06 | Ops, Dossiers, Index, Scopes, Seeding | Full v2 storage layer. Append-only op log (6 types, month-sharded). Dossier schema (8 entry types), deterministic renderer, atlas, history, cap linting, hand-edit detection. Scope resolver (declared + derived + aliases). SQLite index migration (entries, FTS5, anchors, error patterns, aliases). v1 pipeline removed (~2500 lines). Seeding: 174 entries from 49 PA workorders. GitHub PR #9. |

## Older Projects
9 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
