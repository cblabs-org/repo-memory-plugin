# Principal Architect Memory
<!-- Last updated: 2026-10-15T19:31:00Z -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.2 plan approved (2026-10-15).** **PR-10a-v2-2-docs-and-specs complete (5/5 WOs, PR #17)** and **PR-10b-delivery-and-storage-fixes complete (6/6 WOs, PR #18 APPROVED)**. Next active work: resume **PR-10-eval-v2** Stage 2 (`WO4`–`WO7`) and **PR-11-v2-doc-seeding-and-lifecycles**. |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-11-v2-doc-seeding-and-lifecycles | **Active (WO1/5)** — WO1 complete (`c9640fc`), WO2 in progress | Dossiers (`CognitiveKind`, `{section?}`), Ops (`doc-seed`), Index, Seeder (`DocSeedSource`, `seed-docs`), Tick (`dream-freshness`) | [PROJECT.md](../pr-11-v2-doc-seeding-and-lifecycles/PROJECT.md) |
| pr-10-eval-v2 | **Paused at WO3/7 (Stage 1 complete)** — Ready to resume at WO4 (`wo4-dry-run`) now that `pr-10b` (PR #18) is complete | Evaluation, esp32 template, harness, seeded index | [PROJECT.md](../pr-10-eval-v2/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-10b-delivery-and-storage-fixes | 2026-10-15 | Delivery (`UserMessage`, `DetectRewind`, `InjectedSet`), Index (`#13`), Projector (`#11`), Seeder (`#10`, `#12`), Ops (`#16`) | Fixed "One-Invocation Vanishing Memory" (`UserMessage` for `task-start`, `first-touch`, `first-entry`, and compaction re-injection; `EphemeralMessage` for 1-step `about-to-edit`, `error-match`, and FTS warnings), wired `DetectRewind` and cross-process `InjectedSet` persistence, resolved GitHub Issues `#10`, `#11`, `#12`, `#13`, and `#16` (`repo-memory ops gc`), cleaned `.memory/` and `evaluation/data/*`, and rebuilt `~/.local/bin/repo-memory` + evaluation indexes. 1 review round, 5 findings resolved. GitHub PR #18 APPROVED. |
| pr-10a-v2-2-docs-and-specs | 2026-10-15 | SPEC v1.0, Design v2 Rev 4, Addendum A Rev 3, Hooks Reference, README | Upgraded `SPEC.md` to v1.0 (`v2.2`), `design-v2-distillation-retrieval.md` to Rev 4, `addendum-a-provenance-and-hardening.md` to Rev 3, updated `antigravity-hooks-reference.md`, created `README.md`, removed stale v1 `GEMINI.md` and `.memory/views/`, and scrubbed legacy path hygiene across `docs/`. GitHub PR #17. |
| pr-09b-product-fixes | 2026-10-11 | Delivery, Index, Ranker, Seeder | Stage 0: index wiring consolidated (canonical IndexPath), ranker improvements (bm25 relevance, per-session dedup, relevance threshold, FTS error surfacing), binary rebuilt. Evaluation v2 prerequisite complete. |
| pr-09-v2-x6-lite | 2026-10-11 | Evaluation, Replay Harness, X6 Protocol, Dossier Build | X6-lite STOP: 84% no-memory baseline, no headroom on spec-style tasks. 92 runs, ~22.1M tokens (3× budget). 3 product fixes found (injection channel, tool parser, placeholder filter). Harness validated and reusable. PA retrieval study: 1.3% MEMORY.md uptake. Evaluation v2 pivot approved (esp32, 2×2 factorial, 7.5M cap). |
| pr-08-v2-delivery | 2026-10-07 | Delivery, Ledger, Injection, Gates, Compaction, MCP Tools | Delivery engine: systemMessage/ephemeralMessage channels, deny-once gates, cross-process ledger with flock, compaction re-injection. Pull tools (MCP): memory_brief, memory_search, etc. 5 review rounds, 34 fixes. GitHub PR #14. |

## Older Projects
10 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
