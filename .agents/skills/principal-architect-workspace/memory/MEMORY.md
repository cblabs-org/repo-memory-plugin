# Principal Architect Memory
<!-- Last updated: 2026-10-07T07:20:00+01:00 -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.1 plan.** SPEC v0.10 + v2 design rev 3. 19 PRs total. **11/19 complete** (PR-01 through PR-07). X-gate **PASSED**. Next: PR-08-v2-delivery. Early kill gate X6-lite at PR-09. Confirmatory X6 at PR-12-v2-pilot. |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-08-v2-delivery | ☐ Planned — WO0/6 | Delivery, Ledger, Injection, Gates, Compaction, MCP Tools | [PROJECT.md](../pr-08-v2-delivery/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-07-v2-storage | 2026-10-06 | Ops, Dossiers, Index, Scopes, Seeding | Full v2 storage layer. Append-only op log (6 types, month-sharded). Dossier schema (8 entry types), deterministic renderer, atlas, history, cap linting, hand-edit detection. Scope resolver (declared + derived + aliases). SQLite index migration (entries, FTS5, anchors, error patterns, aliases). v1 pipeline removed (~2500 lines). Seeding: 174 entries from 49 PA workorders. GitHub PR #9. |
| pr-06-v2-spike | 2026-10-06 | Experiments X1, X9, X5, X8, X3 | All 5 experiments passed. X-gate criteria met. 3 rounds of corrections applied (injectSteps, agent frontmatter, hooks discovery). Key: no split hook placement needed, PreInvocation fires before subagent prompt. GitHub PR #6. |
| pr-06b-v2-plugin-split | 2026-10-05 | Install, Enroll, Packaging | Split global plugin into daemon (sidecar only) + workspace bundle (hooks/MCP per-repo). init→enroll, leave→unenroll. Phase 0 validated. 2 review rounds. GitHub PR #5. |
| pr-05-v2-foundation | 2026-10-04 | Evidence, Hooks, Telemetry, Snapshot | Close PR-05b, fix capture→evidence, artifact capture+snapshot, hooks restored (13μs fast path), 2.4 GB probe logs cleaned, cost telemetry with gen_metadata/quota guard. |
| pr-05b-dogfood-pilot | 2026-10-04 | Pipeline, Views, Evaluation | Dogfood run on 71 episodes → 125 claims. WO4 superseded by `dogfood-evaluation-01.md`. Led to v2 pivot. Branch closed onto `main` by PR-05-v2-foundation wo1. |

## Older Projects
6 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
