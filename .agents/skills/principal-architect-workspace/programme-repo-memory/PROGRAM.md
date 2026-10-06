# repo-memory Programme

> **Programme:** Build `repo-memory`, a repository-level memory system for coding agents.
> **Specification:** [SPEC.md](../../../SPEC.md) v0.10 + [v2 Design](../../../docs/design-v2-distillation-retrieval.md) rev 3
> **Status:** Executing — v2 pivot at PR-05. **9/19 complete** (PR-01 through PR-06b). PR-06 in review (PR #6). PR-05-v2-foundation next after X-gate.
> **Base branch:** `main`
> **Revision:** v2.1 (2026-10-04). Supersedes the v1 plan for PRs 05a onward. v2.1 applies the plan review (session 15).

## Programme Summary

Build a Go binary (`repo-memory`) that captures agent sessions, distils knowledge in context using turn notes and a sidecar fallback, organises it into dossiers and an op log, and delivers it precisely at the moment of need. Release 1 targets Antigravity; Release 2 adds Claude Code as an adapter (§18.6).

**v2 pivot (2026-10-03).** The dogfood evaluation of the v1 post-hoc pipeline (PR-05b) found that it produced accurate but shallow, atomic claims at ~112k input tokens per episode. The v2 design replaces the distillation and retrieval approach:
- **Distil in context** at the end of milestone turns (turn notes via `Stop` continue), supplemented by free distillations (checkpoint summaries, artifacts) and a lean-agent sidecar fallback.
- **Store as dossiers**, one markdown page per module/topic/system, rendered from an append-only op log that never conflicts in git.
- **Deliver precisely**, using persistent `systemMessage` for knowledge that lasts, `ephemeralMessage` for hints at the moment of need, and `PreToolUse` deny-once for invariant gates.

The infrastructure built in PRs 01–05a (capture, spool, job table, tick, entities, index, FTS5 search, eval harness, headless executor, job chaining, local publish) carries forward, after four known defects are fixed in PR-05-v2-foundation. The v1 pipeline stages (triage, extract, reconcile, verify) and per-file claims are removed. **v2 is built on `main`** (which already contains PR-05a), not on PR-04's tip.

## Repositories in Scope

| Repository | Programme work |
|---|---|
| **repo-memory-plugin** (this repo) | SPEC, the Go binary, Antigravity packaging, CI and releases: all PRs. Also the dogfood repository from PR-02b. |
| **esp32-app-foundation** (pilot) | Phase 2 capture enrolment after PR-05-v2-foundation validates capture (PD17). Full enrolment at PR-12-v2-pilot and PA workspace migration at PR-14-v2-pa-integration, each as a PR in that repo |
| **mm-skills/principal-architect** | PA compatibility-mode change at PR-14-v2-pa-integration, raised through that repo's own process |

## Architecture Domains (v2)

| Domain | SPEC / v2 Design Sections | Key Components |
|---|---|---|
| **Core Infrastructure** | §18.1–18.6 | Go binary skeleton, CLI subcommands, config, data-dir layout, harness interfaces, package-boundary enforcement |
| **Enrolment & Operations** | §4.6, §18.1 | `init`/`leave`, registry, kill switch, install/upgrade/uninstall, CI and release builds |
| **Harness: Antigravity** | §12.1 | Transcript parser, hook codec, `agy` executor, install templates |
| **Harness: Fake** | §18.2 | Test adapter driving the core test suite |
| **Capture** | §4.1–4.3, §6.2 | `Stop` hook, spool, transcript copy, sweep, idle promotion |
| **Compaction** | §5.1, §6.3 | Deterministic compaction, author classification, scrubber |
| **Episode Store** | §5.1, §5.6 | Private JSONL+zstd episodes, `episodes/` layout |
| **Job Table** | §4.3 | SQLite job table, spool ingestion, tick lock, leases, back-pressure |
| **Tick** | §4.3, §12.3 | Spool ingest → job table → sweep → jobs → calendar; tick.lock |
| **Entity Extraction** | §5.2 | Code-only entity extractor: files, symbols, configs, topics, tools, errors, externals |
| **Turn Notes** | v2 §5.1 | Stop gate, continue request, harvest, marks, exposure lists, subagent registry |
| **Free Distillations** | v2 §5.3 | Artifact capture, checkpoint summary capture |
| **Sidecar Fallback** | v2 §5.4 | Lean Markdown agent (`repo-memory-distiller`), quota check, batched processing |
| **Processing Defences** | v2 §5.6 | Input stripping/framing, instruction isolation, output validation |
| **Op Log** | v2 §7 | Append-only ops (add, amend, supersede, retire, move, split), month-sharded, git-committed |
| **Dossiers** | v2 §7.2–7.3 | Module/topic/external/procedure pages, rendered from op log, history files, atlas |
| **Consolidation** | v2 §6 | Echo filter, routing, duplicate merge, anchor check, LLM decide, validate, publish |
| **Index** | §5.6, §18.4, v2 §7.5 | SQLite rebuildable: anchor→entry map, FTS5 over entries, error patterns, aliases |
| **Delivery** | v2 §8 | systemMessage persistent, ephemeralMessage hints, deny-once gates, injection ledger, compaction re-injection |
| **Pull Tools (MCP)** | §7.7, v2 §8.2 | `memory.brief`, `memory.why`, `memory.history`, `memory.search`, `memory.note`, `memory.stats` |
| **Close Conditions** | v2 §5.2 | open-issue deterministic closure evaluation |
| **Coverage & Elicitation** | §9 | Coverage model per dossier, gap scores, elicitation questions |
| **Evaluation** | §10.3, v2 §12 | B1–B5 datasets, X1–X11 experiments, B1 replay (X6 kill criterion) |
| **Security** | §11 | Scrubbing, provenance tagging, lint gate |
| **Publishing** | §6.8 | Two-lane publish, memory PR, review profiles, git worktree |
| **PA Integration** | v2 §5.5, §13 | PA report parsing, episode summaries, workorder briefs, compatibility mode |
| **Backfill** | §14 | Transcript import, doc import, git import, inferred rationale, re-projection via op-log replay |

## PR Sequence

See [PROGRAM-PLAN.md](PROGRAM-PLAN.md) for the full dependency graph, experiments, scope per PR, human tasks and exit criteria.

### Phase 1: Infrastructure (Complete)

| # | PR | Status | Depends On |
|---|---|---|---|
| 1 | `pr-01-go-skeleton` | ✅ Complete | — |
| 2 | `pr-01b-experiment-spike` (E1–E12) | ✅ Complete | PR-01 |
| 3 | `pr-02a-transcript-episode` | ✅ Complete | PR-01b |
| 4 | `pr-02b-capture-runtime` (+ dogfood) | ✅ Complete | PR-02a |
| 5 | `pr-03-episode-search-eval` | ✅ Complete | PR-02b |
| 6 | `pr-04-entities-claims-events` | ✅ Complete | PR-03 |

### Phase 2: v1 Pipeline (Complete, Superseded)

| # | PR | Status | Notes |
|---|---|---|---|
| 7 | `pr-05a-pipeline-code` | ✅ Merged to `main` | v1 stages superseded; the executor, job chaining and local publish are reused. Stages removed in PR-07-v2-storage. |
| 8 | `pr-05b-dogfood-pilot` | ✅ Complete; **close-out pending** | Its branch holds the executor fixes and all v2 documents (evaluation, Appendix C, design rev 1–3). Merged to `main` in PR-05-v2-foundation wo1. |

### Phase 3: v2 Distillation & Retrieval

Read side first: the early kill gate (X6-lite) tests whether delivered dossiers help before turn notes and consolidation are built.

| # | PR | Objective | Status | Depends On |
|---|---|---|---|---|
| 9 | `pr-05-v2-foundation` | Close PR-05b onto `main`; fix capture→episode, re-enable hooks safely, cost telemetry, artifact capture, probe-log cleanup, non-enrolled fast path | ☐ Planned | `main` + PR-05b branch |
| 10 | `pr-06-v2-spike` | Prototype-validate platform mechanics: X1, X3, X5, X8, X9 (prototype on a spike branch, not merged) | 🔄 **In Review** (PR #6) | PR-05-v2-foundation |
| 10b | `pr-06b-v2-plugin-split` | Daemon + workspace bundle architecture | ✅ **Complete** (PR #5, merged) | PR-06-v2-spike |
| 11 | `pr-07-v2-storage` | Op log, dossiers, atlas, scopes, index migration, v1 removal, dossier seeding (PA reports + time-cut lean-agent pass) | ☐ Planned | PR-06-v2-spike (X-gate) |
| 12 | `pr-08-v2-delivery` | Channels, moments, injection ledger, compaction re-injection, pull tools, latency budgets | ☐ Planned | PR-07-v2-storage |
| 13 | `pr-09-v2-x6-lite` | **Early kill gate:** pre-registered B1 replay with time-cut seeded dossiers | ☐ Planned | PR-08-v2-delivery |
| 14 | `pr-10-v2-distillation` | Turn notes, marks, exposure lists, subagent registry, preparation layer, fallback; X2 on real work orders | ☐ Planned | PR-09-v2-x6-lite (gate) |
| 15 | `pr-11-v2-consolidation` | Echo filter, routing, lean-agent ops, validate/record, two-lane publish | ☐ Planned | PR-10-v2-distillation |
| 16 | `pr-12-v2-pilot` | esp32 pilot, full X6 with distilled dossiers, cost report | ☐ Planned | PR-11-v2-consolidation |
| 17 | `pr-13-v2-learning` | Close conditions, gardening, coverage, elicitation, metrics, replay tuning | ☐ Planned | PR-12-v2-pilot |
| 18 | `pr-14-v2-pa-integration` (cross-repo) | Episode summaries, workorder briefs, PA compatibility mode | ☐ Planned | PR-13-v2-learning |
| 19 | `pr-15-v2-backfill` | Full import, re-projection via op-log replay | ☐ Planned | PR-14-v2-pa-integration |

## Gates

| Gate | When | Condition | Consequence of failure |
|---|---|---|---|
| **X-gate** | PR-06-v2-spike exit | X1 harvest ≥ 95% over ≥ 20 requests; X8 overhead measured and agent discoverable from the runtime cwd; X5 channels confirmed or delivery design revised; X9 documented; X3 method sound | Revise v2 design before PR-07 |
| **Early kill gate (X6-lite)** | PR-09-v2-x6-lite exit | Pre-registered B1 replay: time-cut dossiers beat no-memory on the primary metric | **Stop or redirect the project** before distillation and consolidation are built |
| **Confirmatory gate (X6)** | PR-12-v2-pilot exit | Same protocol with dossiers produced by the full pipeline: beat no-memory, match PA docs at fewer tokens | Stop, or fall back to seeded/PA-sourced dossiers |
| **Ship gate** | PR-15-v2-backfill exit | All subsystems have dossiers; cost per session measured; coverage reported | — |

## Human Approval

- [x] v1 programme plan reviewed and approved (2026-10-01)
- [x] v1 PR sequence confirmed (2026-10-01)
- [x] **v2.1 programme plan reviewed and approved** (2026-10-04, with full autonomous implementation authority)
- [x] v2.1 PR sequence confirmed (read side first; early kill gate)
- [x] PD17 decided: two-phase capture — stabilize in repo-memory-plugin first, then enrol esp32 or another active repo after PR-05-v2-foundation (2026-10-04)

## Session Log

### 2026-10-01 — Programme planning (session 1)

- Read SPEC.md v0.6 (1615 lines) cover to cover.
- Identified 11 PRs across Release 1's 6 build phases (P0–P5).
- The SPEC's phases are large: P0 alone covers the Go skeleton, all four hooks, the tick sidecar, the job table, compaction, scrubbing, episodes, metrics, and 11 experiments. This is decomposed across PR-01 (skeleton), PR-02 (capture), and PR-11 (experiments).
- P1 is the heaviest phase: entity extraction, all 6 pipeline stages, claims, views, the lint gate, two-lane publish, review reports, and OTA/BLE import. Decomposed across PR-04, PR-05, and PR-06.
- Release 2 (Claude Code) is intentionally excluded from this plan per the SPEC: it starts after P2 and is an adapter, not a rewrite, thanks to §18.6 portability rules.
- Created the programme workspace, provisioned the operating-model rule, initialized memory.

### 2026-10-01 — Plan review (session 2)

Review against SPEC found six major misses; five were fixed, one was set aside by the user (PD10).
- **Enrolment and kill switch** added to PR-01. This is also a SPEC gap, now §4.6 (SPEC v0.7).
- **P1 gate made runnable:** evaluation harness and datasets in PR-03; OTA/BLE pilot import in PR-05; B5 gate at PR-05; B1/B2 gate at PR-06 via new core `brief`/`ask` (PD6).
- **Experiments moved earlier:** PR-11 branches off PR-01, with per-experiment gates (PD3 revised).
- **Cross-repo scope made explicit:** PR-09 restructured into this repo, `mm-skills/principal-architect` (compatibility mode) and esp32-app-foundation (PD8).
- **Event-sourcing foundations:** events store and `pipeline_version` in PR-04; review and answer events written in PR-06.

### 2026-10-01 — Third-party review, GitHub issue #1 (session 3)

Every claim in the review was checked against the built-in Antigravity docs and the transcripts on disk. SPEC moved to v0.8 and the plan to 13 PRs.

### 2026-10-01 — Review #1 follow-up (session 4)

The reviewer's follow-up on `chunks/` was verified on all 537 conversations. The v0.8 four-source merge was dropped in favour of reading one source in priority order (SPEC v0.9 §6.2, PD16).

### 2026-10-02 — PR-01 through PR-05a execution (sessions 5–12)

All infrastructure and v1 pipeline code built across 8 projects (PR-01 through PR-05a), totalling 42 work orders. The Go binary, capture runtime, episode store, search, entities, claims, and pipeline stages are all implemented and tested.

### 2026-10-02 — PR-05b dogfood pilot (session 13)

Ran the v1 pipeline on this repo's own 71 PA-imported episodes. Produced 125 claims. The evaluation showed accurate but shallow atomic claims at ~112k input tokens per episode. This led to the v2 design exploration.

### 2026-10-03 — v2 design and pivot (session 14)

The v2 design ([design-v2-distillation-retrieval.md](../../../docs/design-v2-distillation-retrieval.md) rev 3) emerged from the dogfood evaluation, two rounds of review on issue #3, and platform answers on issue #2.

**Key v2 decisions:**
- Turn notes via `Stop` continue replace the post-hoc 4-stage pipeline
- Dossiers rendered from an op log replace per-file claims
- `systemMessage` persistent pushes replace the designed-but-never-built delivery layer
- A lean Markdown agent (`excludeDefaultComponents`) replaces the default `agy -p` overhead
- The echo filter (P13) is retained but simplified
- Anchor checks happen at collection and delivery, not as a separate verify stage

**Programme impact:**
- PRs 01–04 carry forward unchanged (infrastructure is valid)
- PR-05a and PR-05b are archived (v1 pipeline superseded; dogfood results informed v2)
- 9 new PRs (05-v2-spike through 13-v2-backfill) replace the remaining 7 v1 PRs
- The B5 gate is replaced by the X6 kill criterion: dossiers must beat no-memory on the B1 replay
- Cross-repo scope is unchanged (esp32 pilot moves to PR-10-v2-pilot, PA integration to PR-12)

**Architecture evolution:** event sourcing survives — the op log is the same pattern as the v1 events store, but with different event types (ops instead of review/answer events). The entity extractor feeds dossier scope derivation. The index is adapted (claims tables → entry/anchor/alias tables). The two-lane publish model carries over unchanged.

### 2026-10-04 — Plan review and v2.1 revision (session 15)

The v2 plan was reviewed before kick-off against the code, git topology, the live install and the data dir. Changes applied (PD17–PD26):
- **Base branch corrected (PD18).** PR-05a is already merged into `main`, with two later fixes (fail-open `PreToolUse`, FTS). The unmerged PR-05b branch holds the executor fixes and every v2 document. v2 now starts from `main` after closing PR-05b, instead of PR-04's tip.
- **Foundation PR added (PD19)** for defects the plan had treated as "unchanged": captured transcripts never become episodes (`tick/pipeline.go` `convertCapturesToTriage`; 124 captured, 0 processed); installed `hooks.json` has been `{}` since 2026-10-02 22:45 so no hooks run; cost telemetry empty (`Cost{}`); 1.5 GB of probe logs; non-enrolled hook path 57 ms (E05). Artifact capture moved here because of pruning.
- **Spike made a prototype spike (PD20).** X1/X5/X8/X9 need prototype code in live hooks: spike branch, feature flag, local install, rollback. X2 moved to PR-10 (real work orders). X3 method fixed (`remaining_fraction` reports the nearest cap only; idle pool; repeated runs). Lean-agent discovery from `<data-dir>/runs/<repo-id>/` added to X8.
- **Read side first, early kill gate (PD21, PD22).** Storage → delivery → X6-lite with time-cut seeded dossiers, before distillation and consolidation. Full X6 kept at the pilot as confirmatory.
- **X6 protocol fixed (PD23):** time-cut dossiers and PA docs (no leakage), pre-registered metric, seeds, WO selection and quota budget.
- **Hook latency exit criteria (PD24)** for delivery; precompute in the tick.
- **Turn notes:** off switch, defined milestone signals, PA-session interaction tested, role opt-out (PD25).
- **v1 events store** retired explicitly in favour of `.memory/ops/` (PD26).
- **Pruning observed:** brain folders 709 → 509 and conversation DBs 703 → 508 overnight; oldest transcript 31 Aug → 15 Sep (~500 kept, ~19 days). PD17 (esp32 capture-only enrolment, X6 snapshot) awaits the user.

