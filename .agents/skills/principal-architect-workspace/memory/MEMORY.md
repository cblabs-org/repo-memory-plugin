# Principal Architect Memory
<!-- Last updated: 2026-10-15T20:58:00Z -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.2 plan executing (17/22 complete).** **PR-10a** (PR #17), **PR-10b** (PR #18 APPROVED), and **PR-11-v2-doc-seeding-and-lifecycles** (6/6 WOs, PR #19 APPROVED) complete. Next active work: **PR-12-v2-distillation** and **PR-10-eval-v2** Stage 2 (`WO4`–`WO7`). |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-12-v2-distillation | **Active (1/6 WOs)** — Starting WO2 (`wo2-stop-milestone-gate-and-harvest`) | State (`marks`, `subagents`), Inbox (`notes`, `free_harvest`), Stop Hook (`stop_distill`), Prepare, Scrub, Tick (`fallback`), Config (`managed_block`) | [PROJECT.md](../pr-12-v2-distillation/PROJECT.md) |
| pr-10-eval-v2 | **Paused at WO3/7 (Stage 1 complete)** — Ready to resume at WO4 (`wo4-dry-run`) | Evaluation, esp32 template, harness, seeded index | [PROJECT.md](../pr-10-eval-v2/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-11-v2-doc-seeding-and-lifecycles | 2026-10-15 | Dossiers (`CognitiveKind`, `{section?}`), Ops (`doc-seed`), Index, Seeder (`DocSeedSource`, `seed-docs`), Tick (`dream-freshness`) | Implemented the 3 `v2.2` cognitive lifecycles (`semantic`, `procedural`, `episodic`), `{section?}` optional section rendering and `kind:`/`source_doc:` metadata lines, deterministic `DocSeedSource` and `repo-memory seed-docs` CLI, async git-blob-SHA doc freshness ("Dreaming" in `tick`), and dogfooded `seed-docs` across `repo-memory-plugin` (mutually idempotent with `ops gc`). 1 review round, 3 findings resolved. GitHub PR #19 APPROVED. |
| pr-10b-delivery-and-storage-fixes | 2026-10-15 | Delivery (`UserMessage`, `DetectRewind`, `InjectedSet`), Index (`#13`), Projector (`#11`), Seeder (`#10`, `#12`), Ops (`#16`) | Fixed "One-Invocation Vanishing Memory" (`UserMessage` for `task-start`, `first-touch`, `first-entry`, and compaction re-injection; `EphemeralMessage` for 1-step `about-to-edit`, `error-match`, and FTS warnings), wired `DetectRewind` and cross-process `InjectedSet` persistence, resolved GitHub Issues `#10`, `#11`, `#12`, `#13`, and `#16` (`repo-memory ops gc`), cleaned `.memory/` and `evaluation/data/*`, and rebuilt `~/.local/bin/repo-memory` + evaluation indexes. 1 review round, 5 findings resolved. GitHub PR #18 APPROVED. |
| pr-10a-v2-2-docs-and-specs | 2026-10-15 | SPEC v1.0, Design v2 Rev 4, Addendum A Rev 3, Hooks Reference, README | Upgraded `SPEC.md` to v1.0 (`v2.2`), `design-v2-distillation-retrieval.md` to Rev 4, `addendum-a-provenance-and-hardening.md` to Rev 3, updated `antigravity-hooks-reference.md`, created `README.md`, removed stale v1 `GEMINI.md` and `.memory/views/`, and scrubbed legacy path hygiene across `docs/`. GitHub PR #17. |
| pr-09b-product-fixes | 2026-10-11 | Delivery, Index, Ranker, Seeder | Stage 0: index wiring consolidated (canonical IndexPath), ranker improvements (bm25 relevance, per-session dedup, relevance threshold, FTS error surfacing), binary rebuilt. Evaluation v2 prerequisite complete. |
| pr-09-v2-x6-lite | 2026-10-11 | Evaluation, Replay Harness, X6 Protocol, Dossier Build | X6-lite STOP: 84% no-memory baseline, no headroom on spec-style tasks. 92 runs, ~22.1M tokens (3× budget). 3 product fixes found (injection channel, tool parser, placeholder filter). Harness validated and reusable. PA retrieval study: 1.3% MEMORY.md uptake. Evaluation v2 pivot approved (esp32, 2×2 factorial, 7.5M cap). |

## Older Projects
11 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
