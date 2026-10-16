# Principal Architect Memory
<!-- Last updated: 2026-10-16T09:23:00Z -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.2 plan executing (19/22 complete).** **PR-10a** (PR #17), **PR-10b** (PR #18 APPROVED), **PR-11-v2-doc-seeding-and-lifecycles** (PR #19 APPROVED), **PR-12-v2-distillation** (6/6 WOs, PR #20 APPROVED), and **PR-13-v2-consolidation** (6/6 WOs, PR #21 APPROVED) complete. Next active work: **PR-14-v2-pilot** and **PR-10-eval-v2** Stage 2 (`WO4`–`WO7`). |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-14-v2-micro-probes-and-pilot | **Active (WO1/4 ready to start)** — Layer 2 Read-Side & Write-Side Micro-Probes and `esp32` 1-Task Pilot | Evaluation (`eval-v2`), Seed (`pa` + `doc-seed`), Delivery Probe, Distillation/Consolidation Probe | [PROJECT.md](../pr-14-v2-micro-probes-and-pilot/PROJECT.md) |
| pr-10-eval-v2 | **Paused at WO3/7 (Stage 1 complete)** — Superseded/integrated into `pr-14-v2-micro-probes-and-pilot` for Layer 2 probes | Evaluation, esp32 template, harness, seeded index | [PROJECT.md](../pr-10-eval-v2/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-13-v2-consolidation | 2026-10-16 | Consolidate (`filter`, `route`, `llm`, `record`, `publish`), GitOps (`worktree`), Tick (`consolidate`), Harness (`MultiTurnHousekeeper`) | Implemented Stage 2 warm batch consolidation: deterministic `P13` Echo Filter (`FilterEchoes`), `VD18` Pre-Consolidation Corroboration Gate (`EnforceCorroborationGate` downgrading uncorroborated `semantic`/`procedural` rules to `open-issue`), Five-Tier scope classification (`user:`, `branch:`, `repo:`) & routing, exact-duplicate `OpAmend` merging, broken-anchor detection (`CheckDossierAnchors`), multi-turn `stream-json` `repo-memory-distiller` (`RunMultiTurn`), state-aware op validation + 4-layer scrubber (`ValidateAndPrepareOps`), branch-merge promotion (`PromoteMergedBranchOps`), dedicated `memory/pending` git worktree publish, `RebuildMultiTierIndex`, `PipelineExecutor.Run` wiring ($\ge 10$ notes or 24h cadence + 25% Flash quota guard + `freshness.json`), and `repo-memory consolidate` CLI. 1 review round, 0 blocking/0 major/2 minor findings. GitHub PR #21 APPROVED. |
| pr-12-v2-distillation | 2026-10-16 | State (`marks`, `subagents`), Inbox (`notes`, `free_harvest`), Stop Hook (`stop_distill`), Prepare, Scrub, Tick (`fallback`), Config (`managed_block`) | Implemented Stage 1 warm-cache distillation: per-conversation marks (`ClampIfRewound`), inbox notes & subagent registry, milestone-gated `Stop` (`NO_TOOL_CALL`) continuation & harvester, zero-token `CHECKPOINT` + `brain/` artifact harvest, 4-layer scrubber (preserving UUIDs/ULIDs/git SHAs + quarantine), `<session-log-TOKEN>` preparation layer & output validator, Stage 1c sidecar fallback (`repo-memory-distiller` with 25% Flash quota guard), and Addendum A8 `v2` managed block in `enroll`/`unenroll`/`doctor`. 1 review round, 3 minor findings resolved. GitHub PR #20 APPROVED. |
| pr-11-v2-doc-seeding-and-lifecycles | 2026-10-15 | Dossiers (`CognitiveKind`, `{section?}`), Ops (`doc-seed`), Index, Seeder (`DocSeedSource`, `seed-docs`), Tick (`dream-freshness`) | Implemented the 3 `v2.2` cognitive lifecycles (`semantic`, `procedural`, `episodic`), `{section?}` optional section rendering and `kind:`/`source_doc:` metadata lines, deterministic `DocSeedSource` and `repo-memory seed-docs` CLI, async git-blob-SHA doc freshness ("Dreaming" in `tick`), and dogfooded `seed-docs` across `repo-memory-plugin` (mutually idempotent with `ops gc`). 1 review round, 3 findings resolved. GitHub PR #19 APPROVED. |
| pr-10b-delivery-and-storage-fixes | 2026-10-15 | Delivery (`UserMessage`, `DetectRewind`, `InjectedSet`), Index (`#13`), Projector (`#11`), Seeder (`#10`, `#12`), Ops (`#16`) | Fixed "One-Invocation Vanishing Memory" (`UserMessage` for `task-start`, `first-touch`, `first-entry`, and compaction re-injection; `EphemeralMessage` for 1-step `about-to-edit`, `error-match`, and FTS warnings), wired `DetectRewind` and cross-process `InjectedSet` persistence, resolved GitHub Issues `#10`, `#11`, `#12`, `#13`, and `#16` (`repo-memory ops gc`), cleaned `.memory/` and `evaluation/data/*`, and rebuilt `~/.local/bin/repo-memory` + evaluation indexes. 1 review round, 5 findings resolved. GitHub PR #18 APPROVED. |
| pr-10a-v2-2-docs-and-specs | 2026-10-15 | SPEC v1.0, Design v2 Rev 4, Addendum A Rev 3, Hooks Reference, README | Upgraded `SPEC.md` to v1.0 (`v2.2`), `design-v2-distillation-retrieval.md` to Rev 4, `addendum-a-provenance-and-hardening.md` to Rev 3, updated `docs/antigravity-hooks-reference.md`, created `README.md`, removed stale v1 `GEMINI.md` and `.memory/views/`, and scrubbed legacy path hygiene across `docs/`. GitHub PR #17. |

## Older Projects
13 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
