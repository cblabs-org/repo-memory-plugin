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
| pr-05-v2-foundation ✅ | Close PR-05b, fix capture→episode, hooks restore, cost telemetry, artifact capture | [PROJECT.md](../pr-05-v2-foundation/PROJECT.md) |
| pr-06b-v2-plugin-split ✅ | Two-plugin split (`repo-memory-daemon` global sidecar + `repo-memory` workspace bundle), `enroll`/`unenroll` | [PROJECT.md](../pr-06b-v2-plugin-split/PROJECT.md) |
| pr-10a-v2-2-docs-and-specs ✅ | `SPEC.md` v1.0, `docs/design-v2-distillation-retrieval.md` Rev 4 (`v2.2`), `docs/addendum-a-provenance-and-hardening.md` Rev 3, `README.md` | [PROJECT.md](../pr-10a-v2-2-docs-and-specs/PROJECT.md) |

## Harness Adapters
| Project | Scope | Context |
|---|---|---|
| pr-01-go-skeleton ✅ | Probe hook (`hook antigravity <event> --probe`) | [PROJECT.md](../pr-01-go-skeleton/PROJECT.md) |
| pr-01b-experiment-spike ✅ | E1–E12 reports in `docs/experiments/` | [PROJECT.md](../pr-01b-experiment-spike/PROJECT.md) |
| pr-02a-transcript-episode ✅ | `internal/harness/antigravity/transcript.go` (transcript file-set, `truncated_fields`) | [PROJECT.md](../pr-02a-transcript-episode/PROJECT.md) |
| pr-02b-capture-runtime ✅ | `internal/harness/antigravity/hooks.go` (`Stop`), session attribution, `packaging/antigravity/sidecars/tick` | [PROJECT.md](../pr-02b-capture-runtime/PROJECT.md) |
| pr-05a-pipeline-code ✅ | `internal/harness/antigravity/executor.go` (`agy -p`), recorded executor | [PROJECT.md](../pr-05a-pipeline-code/PROJECT.md) |
| pr-06-v2-spike ✅ | Prototype `Stop` continue, `systemMessage` injection, lean agent | [PROJECT.md](../pr-06-v2-spike/PROJECT.md) |
| pr-08-v2-delivery ✅ | Antigravity hook codec (`PreInvocation`, `PreToolUse`, `PostToolUse`), delivery moments | [PROJECT.md](../pr-08-v2-delivery/PROJECT.md) |
| pr-09b-product-fixes ✅ | Canonical `IndexPath`, native tool-call parser, ranker bm25 + `InjectedSet` dedup | [PROJECT.md](../pr-09b-product-fixes/PROJECT.md) |
| pr-10a-v2-2-docs-and-specs ✅ | `docs/antigravity-hooks-reference.md` updated for `v2.2` (`userMessage` vs `ephemeralMessage`, `Stop` `NO_TOOL_CALL`, `invoke_subagent`) | [PROJECT.md](../pr-10a-v2-2-docs-and-specs/PROJECT.md) |
| pr-10b-delivery-and-storage-fixes ✅ | `UserMessage` persistence fix, `DetectRewind` wiring, cross-process `InjectedSet` persistence, issues `#10`–`#13` & `#16` (`repo-memory ops gc`) | [PROJECT.md](../pr-10b-delivery-and-storage-fixes/PROJECT.md) |
| pr-12-v2-distillation ✅ | Production `Stop` gate and harvest (`stop_distill.go`), turn-note settings, milestone signals, `PostToolUse` `invoke_subagent` subagent registry (`subagents.go`), `v2` managed instruction block (`managed_block.go`) (PR #20) | [PROJECT.md](../pr-12-v2-distillation/PROJECT.md) |

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

### Added in v2 / v2.2
| Project | Scope | Context |
|---|---|---|
| pr-07-v2-storage ✅ | `internal/core/ops/` (op log), `internal/core/dossiers/` (schema, renderer), `.memory/ops/`, `.memory/dossiers/`, `ATLAS.md`, `.memory/config.yaml` scopes, `internal/core/seed/` | [PROJECT.md](../pr-07-v2-storage/PROJECT.md) |
| pr-10b-delivery-and-storage-fixes ✅ | Index orphan row delete (`#13`), cross-scope projector dedup & empty dossier pruning (`#11`), seeder real path + `#symbol`/`:symbol` anchors (`#10`) & `--before` timestamp/SHA (`#12`), ops cleanup (`#16`, `repo-memory ops gc`) | [PROJECT.md](../pr-10b-delivery-and-storage-fixes/PROJECT.md) |
| pr-11-v2-doc-seeding-and-lifecycles ✅ | `DocSeedSource` (`repo-memory seed-docs`), async doc-diff freshness ("Dreaming" in `tick`), `semantic`/`procedural`/`episodic` lifecycles, `{section?}` rendering, empty dossier/ATLAS pruning (PR #19) | [PROJECT.md](../pr-11-v2-doc-seeding-and-lifecycles/PROJECT.md) |
| pr-12-v2-distillation ✅ | `internal/core/state/` (marks, subagent registry), `internal/core/inbox/` (inbox notes, zero-token `CHECKPOINT` + `brain/` artifact harvest), `internal/core/scrub/` (4-layer scrubber + quarantine), `internal/core/pipeline/prepare/` (`<session-log-TOKEN>` preparation layer & validator), `internal/core/tick/fallback.go` (sidecar fallback) (PR #20) | [PROJECT.md](../pr-12-v2-distillation/PROJECT.md) |
| pr-13-v2-consolidation ✅ | `internal/core/consolidate/` (P13 echo filter, `VD18` Corroboration Gate, Five-Tier routing & dedup, `stream-json` `repo-memory-distiller` `RunMultiTurn`, state-aware op validation & render, `user:`/`branch:`/`repo:` scopes, `PromoteMergedBranchOps`, `RebuildMultiTierIndex`), `internal/core/gitops/worktree.go` (`memory/pending`), `internal/core/tick/consolidate.go`, `cmd/repo-memory/cmd_consolidate.go` (PR #21) | [PROJECT.md](../pr-13-v2-consolidation/PROJECT.md) |

## Delivery (Hooks & Pull Tools)
| Project | Scope | Context |
|---|---|---|
| pr-03-episode-search ✅ | `search` CLI, `memory.ask` episode fallback | [PROJECT.md](../pr-03-episode-search/PROJECT.md) |
| pr-08-v2-delivery ✅ | `internal/core/delivery/` (ledger, engine, ranking), `internal/mcpserver/` (v2 pull tools), injection ledger, compaction re-injection | [PROJECT.md](../pr-08-v2-delivery/PROJECT.md) |
| pr-09b-product-fixes ✅ | Canonical `IndexPath`, bm25 ranking, relevance threshold, per-session `InjectedSet` deduplication | [PROJECT.md](../pr-09b-product-fixes/PROJECT.md) |
| pr-10b-delivery-and-storage-fixes ✅ | Persistent `UserMessage` (`ChannelUser`) vs 1-step `EphemeralMessage` (`ChannelEphemeral`), `DetectRewind` wiring, cross-process `InjectedSet` persistence after `RearmDedup()` | [PROJECT.md](../pr-10b-delivery-and-storage-fixes/PROJECT.md) |

## Observability & Measurement
| Project | Scope | Context |
|---|---|---|
| pr-02a-transcript-episode ✅ | `internal/core/metrics` (skeleton) | [PROJECT.md](../pr-02a-transcript-episode/PROJECT.md) |
| pr-03-episode-search ✅ | `eval` harness, B1/B2/B3/B5 datasets | [PROJECT.md](../pr-03-episode-search/PROJECT.md) |
| pr-05b-dogfood-pilot ✅ | B5 evaluation, pipeline cost (~112k/episode) | [PROJECT.md](../pr-05b-dogfood-pilot/PROJECT.md) |
| pr-05-v2-foundation ✅ | Cost telemetry, `gen_metadata` reader, `/usage` quota-guard library | [PROJECT.md](../pr-05-v2-foundation/PROJECT.md) |
| pr-09-v2-x6-lite ✅ | X6 replay harness, time-cut dossier evaluation (92 runs), 231-session PA retrieval study | [PROJECT.md](../pr-09-v2-x6-lite/PROJECT.md) |
| pr-10-eval-v2 (paused WO4/7) | 2×2 forward-task evaluation on `esp32-app-foundation` (Stage 0b, Stage 1 & WO4 complete; Layer 3 bridge prompt & rubrics committed in `pr-15-v2-learning` WO1) | [PROJECT.md](../pr-10-eval-v2/PROJECT.md) |
| pr-14-v2-micro-probes-and-pilot ✅ | Layer 2 Read-Side & Write-Side Micro-Probes, dynamic `repoSymbolIndex`, and `esp32` 1-Task Pilot (`VERDICT: GO`) (PR #22) | [PROJECT.md](../pr-14-v2-micro-probes-and-pilot/PROJECT.md) |
| pr-15-v2-learning ✅ | `PostToolUse` procedural validation, `.agents/skills/rm-<slug>/SKILL.md` graduation, `closes_when`, `garden`, A5/A6 spend guards & short-term recall (PR #23) | [PROJECT.md](../pr-15-v2-learning/PROJECT.md) |

## Review & Governance
| Project | Scope | Context |
|---|---|---|
| pr-13-v2-consolidation ✅ | Two-lane publish (`memory/pending` worktree), Five-Tier scope isolation (`user:`, `branch:`, `repo:`), branch-merge promotion (`PromoteMergedBranchOps`) (PR #21) | [PROJECT.md](../pr-13-v2-consolidation/PROJECT.md) |
| pr-15-v2-learning ✅ | Close conditions (`closes_when`), gardening (`repo-memory garden`), elicitation questions (`.memory/QUESTIONS.md`), skill lint (`repo-memory doctor`) (PR #23) | [PROJECT.md](../pr-15-v2-learning/PROJECT.md) |
| pr-16-v2-pa-and-rollout (active) | Episode summaries (`summary`), WORKORDER briefs (`brief --workorder`), `backfill`, `reproject`, full dogfood rollout | [PROJECT.md](../pr-16-v2-pa-and-rollout/PROJECT.md) |
