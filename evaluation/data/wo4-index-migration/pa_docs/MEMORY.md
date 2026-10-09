# Principal Architect Memory
<!-- Last updated: 2026-10-06T14:47:00+01:00 -->

## Active Programs
| Program | Plan | Status |
|---|---|---|
| repo-memory | [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md) | **v2.1 plan.** SPEC v0.10 + v2 design rev 3. 19 PRs total. **10/19 complete** (PR-01 through PR-06). X-gate **PASSED**. Next: PR-07-v2-storage. Early kill gate X6-lite at PR-09. Confirmatory X6 at PR-12-v2-pilot. |

## Active Projects
| Project | Status | Modules | Context |
|---|---|---|---|
| pr-07-v2-storage | WO1/6 — op-log active | Ops, Dossiers, Index, Scopes, Seeding | [PROJECT.md](../pr-07-v2-storage/PROJECT.md) |

## Recently Completed (last 5)
| Project | Completed | Modules | Key Outcome |
|---|---|---|---|
| pr-06-v2-spike | 2026-10-06 | Experiments X1, X9, X5, X8, X3 | All 5 experiments passed. X-gate criteria met. 3 rounds of corrections applied (injectSteps, agent frontmatter, hooks discovery). Key: no split hook placement needed, PreInvocation fires before subagent prompt. GitHub PR #6. |
| pr-06b-v2-plugin-split | 2026-10-05 | Install, Enroll, Packaging | Split global plugin into daemon (sidecar only) + workspace bundle (hooks/MCP per-repo). init→enroll, leave→unenroll. Phase 0 validated. 2 review rounds. GitHub PR #5. |
| pr-05-v2-foundation | 2026-10-04 | Evidence, Hooks, Telemetry, Snapshot | Close PR-05b, fix capture→evidence, artifact capture+snapshot, hooks restored (13μs fast path), 2.4 GB probe logs cleaned, cost telemetry with gen_metadata/quota guard. |
| pr-05b-dogfood-pilot | 2026-10-04 | Pipeline, Views, Evaluation | Dogfood run on 71 episodes → 125 claims. WO4 superseded by `dogfood-evaluation-01.md`. Led to v2 pivot. Branch closed onto `main` by PR-05-v2-foundation wo1. |
| pr-05a-pipeline-code | 2026-10-02 | Executors, Triage, Extract, Reconcile, Verify, Publish | v1 pipeline implemented. 7/7 WOs. Merged to `main`. Stages superseded by v2; executor, job chaining and local publish retained. |

## Older Projects
6 completed projects.
Full project history: [PROJECT_INDEX.md](PROJECT_INDEX.md)
Cross-cutting module index: [MODULE_INDEX.md](MODULE_INDEX.md)
