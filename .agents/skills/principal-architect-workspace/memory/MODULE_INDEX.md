# Module Index
<!-- Cross-cutting reference. Read when you need to find what
     work has touched a specific domain or module.
     Domains are architect-defined based on the codebase architecture.
     Rows marked ✅ link to the completed project; (planned) link to the plan. -->

## Core Infrastructure
| Project | Scope | Context |
|---|---|---|
| pr-01-go-skeleton ✅ | `cmd/repo-memory`, `internal/harness` (interfaces, normalised types), `internal/harness/fake`, `internal/core/config`, enrolment and registry, kill switch, install/uninstall, CI | [PROJECT.md](../pr-01-go-skeleton/PROJECT.md) |
| pr-04-entities-claims ✅ | `internal/core/entities`, `internal/core/claims`, `internal/core/index`, `.memory/claims/`, `.memory/events/` | [PROJECT.md](../pr-04-entities-claims/PROJECT.md) |
| pr-05-v2-foundation (planned) | Close PR-05b, fix capture→episode, hooks restore, cost telemetry, artifact capture | [PROJECT.md](../pr-05-v2-foundation/PROJECT.md) |

## Harness Adapters
| Project | Scope | Context |
|---|---|---|
| pr-01-go-skeleton ✅ | Probe hook (`hook antigravity <event> --probe`) | [PROJECT.md](../pr-01-go-skeleton/PROJECT.md) |
| pr-01b-experiment-spike ✅ | E1–E12 reports in `docs/experiments/` | [PROJECT.md](../pr-01b-experiment-spike/PROJECT.md) |
| pr-02a-transcript-episode ✅ | `internal/harness/antigravity/transcript.go` (transcript file-set, `truncated_fields`) | [PROJECT.md](../pr-02a-transcript-episode/PROJECT.md) |
| pr-02b-capture-runtime ✅ | `internal/harness/antigravity/hooks.go` (`Stop`), session attribution, `packaging/antigravity/sidecars/tick` | [PROJECT.md](../pr-02b-capture-runtime/PROJECT.md) |
| pr-05a-pipeline-code ✅ | `internal/harness/antigravity/executor.go` (`agy -p`), recorded executor | [PROJECT.md](../pr-05a-pipeline-code/PROJECT.md) |
| pr-06-v2-spike (planned) | Prototype `Stop` continue, `systemMessage` injection, lean agent | [PROJECT.md](../pr-06-v2-spike/PROJECT.md) |
| pr-08-v2-delivery (planned) | Antigravity hook codec (`PreInvocation`, `PreToolUse`, `PostToolUse`), delivery moments | [PROJECT.md](../pr-08-v2-delivery/PROJECT.md) |
| pr-10-v2-distillation (planned) | Production `Stop` gate and harvest, turn-note settings, milestone signals | [PROJECT.md](../pr-10-v2-distillation/PROJECT.md) |

## Pipeline (Capture → Publish)

### Retained from v1
| Project | Scope | Status |
|---|---|---|
| pr-02a-transcript-episode ✅ | `internal/core/pipeline/compact`, `internal/core/episode`, `internal/core/scrub`, commit linkage | Retained |
| pr-02b-capture-runtime ✅ | `internal/core/jobs`, `internal/core/tick`, spool and incremental copy | Retained |
| pr-05a-pipeline-code ✅ | Headless executor, job chaining, local-publish worktree, 429 handling | Retained |

### Removed from v1 (PR-07-v2-storage wo5)
| Package | Was | Removed because |
|---|---|---|
| `internal/core/pipeline/triage` | Salience-window triage | Replaced by turn notes + fallback |
| `internal/core/pipeline/extract` | LLM claim extraction | Replaced by lean-agent seeding and consolidation |
| `internal/core/pipeline/reconcile` | LLM reconciliation | Replaced by consolidation ops |
| `internal/core/pipeline/verify` | LLM verification | Replaced by anchor checks at collection and delivery |
| `internal/core/views` | MAP.md, subsystem pages | Replaced by dossier renderer + ATLAS.md |
| `.memory/events/` | v1 events store | Retired in favour of `.memory/ops/` (PD26) |
| `.memory/claims/` | Per-file claims | Archived to `.memory/archive/v1-claims/` |

### Added in v2
| Project | Scope | Context |
|---|---|---|
| pr-07-v2-storage (planned) | `internal/core/ops/` (op log), `internal/core/dossiers/` (schema, renderer), `.memory/ops/`, `.memory/dossiers/`, `ATLAS.md`, `.memory/config.yaml` scopes | [PROJECT.md](../pr-07-v2-storage/PROJECT.md) |
| pr-10-v2-distillation (planned) | `internal/core/state/` (marks, exposure lists, subagent registry), `internal/core/pipeline/prepare/` (preparation layer), `internal/core/pipeline/fallback/` (sidecar fallback) | [PROJECT.md](../pr-10-v2-distillation/PROJECT.md) |
| pr-11-v2-consolidation (planned) | `internal/core/consolidate/` (echo filter, routing, dedup, LLM decide, validate, publish) | [PROJECT.md](../pr-11-v2-consolidation/PROJECT.md) |
| pr-15-v2-backfill (planned) | Full imports, `repo-memory reproject` | [PROJECT.md](../pr-15-v2-backfill/PROJECT.md) |

## Delivery (Hooks & Pull Tools)
| Project | Scope | Context |
|---|---|---|
| pr-03-episode-search ✅ | `search` CLI, `memory.ask` episode fallback | [PROJECT.md](../pr-03-episode-search/PROJECT.md) |
| pr-08-v2-delivery (planned) | `internal/core/delivery/` (ledger, engine, ranking), `internal/mcpserver/` (v2 pull tools), injection ledger, compaction re-injection | [PROJECT.md](../pr-08-v2-delivery/PROJECT.md) |

## Observability & Measurement
| Project | Scope | Context |
|---|---|---|
| pr-02a-transcript-episode ✅ | `internal/core/metrics` (skeleton) | [PROJECT.md](../pr-02a-transcript-episode/PROJECT.md) |
| pr-03-episode-search ✅ | `eval` harness, B1/B2/B3/B5 datasets | [PROJECT.md](../pr-03-episode-search/PROJECT.md) |
| pr-05b-dogfood-pilot ✅ | B5 evaluation, pipeline cost (~112k/episode) | [PROJECT.md](../pr-05b-dogfood-pilot/PROJECT.md) |
| pr-05-v2-foundation (planned) | Cost telemetry, `gen_metadata` reader, `/usage` quota-guard library | [PROJECT.md](../pr-05-v2-foundation/PROJECT.md) |
| pr-09-v2-x6-lite (planned) | X6 replay harness, time-cut dossier evaluation | [PROJECT.md](../pr-09-v2-x6-lite/PROJECT.md) |
| pr-12-v2-pilot (planned) | Full X6, cost report, dossier precision measurement | [PROJECT.md](../pr-12-v2-pilot/PROJECT.md) |
| pr-13-v2-learning (planned) | `internal/core/metrics/coverage.go`, session metrics, replay tuning | [PROJECT.md](../pr-13-v2-learning/PROJECT.md) |

## Review & Governance
| Project | Scope | Context |
|---|---|---|
| pr-11-v2-consolidation (planned) | Two-lane publish (reusing PR-05a worktree), memory PR, branch-scoped notes | [PROJECT.md](../pr-11-v2-consolidation/PROJECT.md) |
| pr-13-v2-learning (planned) | Close conditions, gardening, elicitation questions, weekly review | [PROJECT.md](../pr-13-v2-learning/PROJECT.md) |
| pr-14-v2-pa-integration (planned) | Episode summaries, WORKORDER briefs, PA compatibility mode (cross-repo) | [PROJECT.md](../pr-14-v2-pa-integration/PROJECT.md) |
