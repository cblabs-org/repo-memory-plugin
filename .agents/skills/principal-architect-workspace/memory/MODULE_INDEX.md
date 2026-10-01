# Module Index
<!-- Cross-cutting reference. Read when you need to find what
     work has touched a specific domain or module.
     Domains are architect-defined based on the codebase architecture.
     Rows marked (planned) come from PROGRAM-PLAN.md and SPEC §18.2; replace
     the Context link with the project's PROJECT.md once it exists. -->

## Core Infrastructure
| Project | Scope | Context |
|---|---|---|
| pr-01-go-skeleton (planned) | `cmd/repo-memory`, `internal/harness` (interfaces, normalised types), `internal/harness/fake`, `internal/core/config`, enrolment and registry, kill switch, install/uninstall, CI | [PROGRAM-PLAN.md § PR-01](../programme-repo-memory/PROGRAM-PLAN.md#pr-01-go-skeleton-interfaces-enrolment-ci--probe-hook) |
| pr-04-entities-claims-events (planned) | `internal/core/entities`, `internal/core/claims`, `internal/core/index`, `.memory/claims/`, `.memory/events/` | [PROGRAM-PLAN.md § PR-04](../programme-repo-memory/PROGRAM-PLAN.md#pr-04-entities-claims-index--events-store) |

## Harness Adapters
| Project | Scope | Context |
|---|---|---|
| pr-01-go-skeleton (planned) | Probe hook (`hook antigravity <event> --probe`) | [PROGRAM-PLAN.md § PR-01](../programme-repo-memory/PROGRAM-PLAN.md#pr-01-go-skeleton-interfaces-enrolment-ci--probe-hook) |
| pr-01b-experiment-spike (planned) | E1–E12 reports in `docs/experiments/` | [PROGRAM-PLAN.md § PR-01b](../programme-repo-memory/PROGRAM-PLAN.md#pr-01b-experiment-spike) |
| pr-02a-transcript-to-episode (planned) | `internal/harness/antigravity/transcript.go` (transcript file-set merge, `truncated_fields`) | [PROGRAM-PLAN.md § PR-02a](../programme-repo-memory/PROGRAM-PLAN.md#pr-02a-transcript--episode) |
| pr-02b-capture-runtime (planned) | `internal/harness/antigravity/hooks.go` (`Stop`), session attribution, `packaging/antigravity/sidecars/tick` | [PROGRAM-PLAN.md § PR-02b](../programme-repo-memory/PROGRAM-PLAN.md#pr-02b-capture-runtime) |
| pr-05a-pipeline (planned) | `internal/harness/antigravity/executor.go` (`agy -p`), recorded executor | [PROGRAM-PLAN.md § PR-05a](../programme-repo-memory/PROGRAM-PLAN.md#pr-05a-pipeline-code) |
| pr-07-delivery (planned) | Antigravity hook codec (`PreInvocation`, `PreToolUse`, `PostToolUse`), hook-payload fixtures | [PROGRAM-PLAN.md § PR-07](../programme-repo-memory/PROGRAM-PLAN.md#pr-07-delivery) |

## Pipeline (Capture → Publish)
| Project | Scope | Context |
|---|---|---|
| pr-02a-transcript-to-episode (planned) | `internal/core/pipeline/compact`, `internal/core/episode`, `internal/core/scrub`, commit linkage | [PROGRAM-PLAN.md § PR-02a](../programme-repo-memory/PROGRAM-PLAN.md#pr-02a-transcript--episode) |
| pr-02b-capture-runtime (planned) | `internal/core/jobs`, `internal/core/tick`, spool and incremental copy | [PROGRAM-PLAN.md § PR-02b](../programme-repo-memory/PROGRAM-PLAN.md#pr-02b-capture-runtime) |
| pr-05a-pipeline (planned) | `internal/core/pipeline/{triage,extract,reconcile,verify}`, `publish/local.go` | [PROGRAM-PLAN.md § PR-05a](../programme-repo-memory/PROGRAM-PLAN.md#pr-05a-pipeline-code) |
| pr-06-publish-review-brief (planned) | `publish/shared.go`, `internal/core/views` | [PROGRAM-PLAN.md § PR-06](../programme-repo-memory/PROGRAM-PLAN.md#pr-06-shared-publish-views-review--briefask-core) |
| pr-10-backfill (planned) | Imports, inferred rationale, `reproject` | [PROGRAM-PLAN.md § PR-10](../programme-repo-memory/PROGRAM-PLAN.md#pr-10-full-backfill) |

## Delivery (Hooks & Pull Tools)
| Project | Scope | Context |
|---|---|---|
| pr-03-episode-search-eval (planned) | `search` CLI, `memory.ask` episode fallback | [PROGRAM-PLAN.md § PR-03](../programme-repo-memory/PROGRAM-PLAN.md#pr-03-episode-search--evaluation-harness) |
| pr-06-publish-review-brief (planned) | `internal/core/delivery/brief`, `brief`/`ask` CLI | [PROGRAM-PLAN.md § PR-06](../programme-repo-memory/PROGRAM-PLAN.md#pr-06-shared-publish-views-review--briefask-core) |
| pr-07-delivery (planned) | `internal/core/delivery`, `internal/mcpserver`, map include, `model_decision` rules, degraded modes | [PROGRAM-PLAN.md § PR-07](../programme-repo-memory/PROGRAM-PLAN.md#pr-07-delivery) |

## Observability & Measurement
| Project | Scope | Context |
|---|---|---|
| pr-02a-transcript-to-episode (planned) | `internal/core/metrics` (skeleton) | [PROGRAM-PLAN.md § PR-02a](../programme-repo-memory/PROGRAM-PLAN.md#pr-02a-transcript--episode) |
| pr-03-episode-search-eval (planned) | `eval` harness, B1/B2/B3/B5 datasets | [PROGRAM-PLAN.md § PR-03](../programme-repo-memory/PROGRAM-PLAN.md#pr-03-episode-search--evaluation-harness) |
| pr-05b-pilot-b5 (planned) | B5 evaluation, real pipeline cost | [PROGRAM-PLAN.md § PR-05b](../programme-repo-memory/PROGRAM-PLAN.md#pr-05b-pilot--b5-gate-milestone) |
| pr-08-learning-loop (planned) | `internal/core/metrics/coverage.go`, session metrics, replay tuning, `report` | [PROGRAM-PLAN.md § PR-08](../programme-repo-memory/PROGRAM-PLAN.md#pr-08-learning-loop) |

## Review & Governance
| Project | Scope | Context |
|---|---|---|
| pr-04-entities-claims-events (planned) | Events store (`.memory/events/reviews`, `answers`) | [PROGRAM-PLAN.md § PR-04](../programme-repo-memory/PROGRAM-PLAN.md#pr-04-entities-claims-index--events-store) |
| pr-06-publish-review-brief (planned) | Review profiles, weekly review report, reply parsing, lint gate | [PROGRAM-PLAN.md § PR-06](../programme-repo-memory/PROGRAM-PLAN.md#pr-06-shared-publish-views-review--briefask-core) |
| pr-09-pa-integration (planned) | Episode summaries, WORKORDER briefs, PA compatibility mode (cross-repo) | [PROGRAM-PLAN.md § PR-09](../programme-repo-memory/PROGRAM-PLAN.md#pr-09-pa-integration-cross-repo) |
