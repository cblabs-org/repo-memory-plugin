# Principal Architect Memory
<!-- Last updated: 2026-10-04T12:17:00+01:00 -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.1 plan.** SPEC v0.10 + v2 design rev 3. 19 PRs total. **9/19 complete** (PR-01 through PR-05-v2-foundation). PR-06-v2-spike next. X-gate at PR-06-v2-spike exit. Early kill gate X6-lite at PR-09. Confirmatory X6 at PR-12-v2-pilot. |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-06-v2-spike | WO2/5 (resumes — WO3 next) | Experiments X1, X9, X5, X8, X3 | [PROJECT.md](../pr-06-v2-spike/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-06b-v2-plugin-split | 2026-10-05 | Install, Enroll, Packaging | Split global plugin into daemon (sidecar only) + workspace bundle (hooks/MCP per-repo). init→enroll, leave→unenroll. Phase 0 validated. 2 review rounds. GitHub PR #5. |
| pr-05-v2-foundation | 2026-10-04 | Evidence, Hooks, Telemetry, Snapshot | Close PR-05b, fix capture→evidence, artifact capture+snapshot, hooks restored (13μs fast path), 2.4 GB probe logs cleaned, cost telemetry with gen_metadata/quota guard. |
| pr-05b-dogfood-pilot | 2026-10-04 | Pipeline, Views, Evaluation | Dogfood run on 71 episodes → 125 claims. WO4 superseded by `dogfood-evaluation-01.md`. Led to v2 pivot. Branch closed onto `main` by PR-05-v2-foundation wo1. |
| pr-05a-pipeline-code | 2026-10-02 | Executors, Triage, Extract, Reconcile, Verify, Publish | v1 pipeline implemented. 7/7 WOs. Merged to `main`. Stages superseded by v2; executor, job chaining and local publish retained. |
| pr-04-entities-claims | 2026-10-02 | Entities, Claims, Index, Events | Data model layer. 7/7 WOs. Entities and index carry forward. Events store retired in v2 (PD26). |
| pr-03-episode-search | 2026-10-02 | FTS Index, Search CLI, PA Import, Eval Harness | FTS5 search, memory.ask fallback, eval harness with B1-B5 datasets. 6/6 WOs. |
| pr-02b-capture-runtime | 2026-10-02 | Capture Runtime, Spool, Stop Hook, Tick, Sweep | Spool, tick, hooks, and job table. 7/7 WOs. |

## Older Projects
4 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
