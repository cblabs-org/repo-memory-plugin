# Principal Architect Memory
<!-- Last updated: 2026-10-15T16:36:00Z -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.2 plan approved (2026-10-15).** Upgrading SPEC to v1.0 + v2 design rev 4 + Addendum A rev 3 in **PR-10a-v2-2-docs-and-specs** (active next), followed by **PR-10b-delivery-and-storage-fixes** (`UserMessage` persistence fix, `DetectRewind`, issues `#10`–`#13`, `#16`) before resuming **PR-10-eval-v2** Stage 2 (`WO4`–`WO7`). |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-10a-v2-2-docs-and-specs | **Active (1/5 WOs)** — WO1 (`design-v2-distillation-retrieval.md` Rev 4) complete; WO2 (`addendum-a-provenance-and-hardening.md` Rev 3) active next | SPEC v1.0, Design v2 Rev 4, Addendum A Rev 3, Hooks Reference, README | [PROJECT.md](../pr-10a-v2-2-docs-and-specs/PROJECT.md) |
| pr-10-eval-v2 | **Paused at WO3/7 (Stage 1 complete)** — Resumes at WO4 (`wo4-dry-run`) after `pr-10a` and `pr-10b` land | Evaluation, esp32 template, harness, seeded index | [PROJECT.md](../pr-10-eval-v2/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-09b-product-fixes | 2026-10-11 | Delivery, Index, Ranker, Seeder | Stage 0: index wiring consolidated (canonical IndexPath), ranker improvements (bm25 relevance, per-session dedup, relevance threshold, FTS error surfacing), binary rebuilt. Evaluation v2 prerequisite complete. |
| pr-09-v2-x6-lite | 2026-10-11 | Evaluation, Replay Harness, X6 Protocol, Dossier Build | X6-lite STOP: 84% no-memory baseline, no headroom on spec-style tasks. 92 runs, ~22.1M tokens (3× budget). 3 product fixes found (injection channel, tool parser, placeholder filter). Harness validated and reusable. PA retrieval study: 1.3% MEMORY.md uptake. Evaluation v2 pivot approved (esp32, 2×2 factorial, 7.5M cap). |
| pr-08-v2-delivery | 2026-10-07 | Delivery, Ledger, Injection, Gates, Compaction, MCP Tools | Delivery engine: systemMessage/ephemeralMessage channels, deny-once gates, cross-process ledger with flock, compaction re-injection. Pull tools (MCP): memory_brief, memory_search, etc. 5 review rounds, 34 fixes. GitHub PR #14. |
| pr-07-v2-storage | 2026-10-06 | Ops, Dossiers, Index, Scopes, Seeding | Full v2 storage layer. Append-only op log (6 types, month-sharded). Dossier schema (8 entry types), deterministic renderer, atlas, history, cap linting, hand-edit detection. Scope resolver (declared + derived + aliases). SQLite index migration (entries, FTS5, anchors, error patterns, aliases). v1 pipeline removed (~2500 lines). Seeding: 174 entries from 49 PA workorders. GitHub PR #9. |
| pr-06-v2-spike | 2026-10-06 | Experiments X1, X9, X5, X8, X3 | All 5 experiments passed. X-gate criteria met. 3 rounds of corrections applied (injectSteps, agent frontmatter, hooks discovery). Key: no split hook placement needed, PreInvocation fires before subagent prompt. GitHub PR #6. |

## Older Projects
8 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
