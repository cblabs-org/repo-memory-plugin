# repo-memory Programme

> **Programme:** Build `repo-memory`, a repository-level memory system for coding agents.
> **Specification:** [SPEC.md](../../../SPEC.md) (v0.9, 2026-10-01)
> **Status:** Planning
> **Base branch:** `main`

## Programme Summary

Build a Go binary (`repo-memory`) that captures agent sessions, distils them into typed, evidence-backed claims, and delivers relevant knowledge back to working agents at the moment of need. Release 1 targets Antigravity; Release 2 adds Claude Code as an adapter (§15).

The system has two invocation paths (event hooks and a scheduled tick), a six-stage pipeline (Capture → Compact → Triage → Extract → Reconcile → Verify → Publish), and a delivery layer that pushes knowledge at task start, file touch, error, risky action and déjà-vu. Pull tools provide on-demand access. All data lives in files and SQLite; no daemon, no server, no cloud.

## Repositories in Scope

| Repository | Programme work |
|---|---|
| **repo-memory-plugin** (this repo) | SPEC, the Go binary, Antigravity packaging, CI and releases: all PRs. Also the dogfood repository from PR-02b. |
| **esp32-app-foundation** (pilot) | Enrolment at PR-05b and PA workspace migration at PR-09, each as a PR in that repo |
| **mm-skills/principal-architect** | PA compatibility-mode change at PR-09, raised through that repo's own process |

## Architecture Domains

| Domain | SPEC Sections | Key Components |
|---|---|---|
| **Core Infrastructure** | §18.1–18.6 | Go binary skeleton, CLI subcommands, config, data-dir layout, harness interfaces, capability types, package-boundary enforcement |
| **Enrolment & Operations** | §4.6, §18.1 | `init`/`leave`, registry, kill switch, install/upgrade/uninstall, CI and release builds |
| **Harness: Antigravity** | §12.1 | Transcript parser, hook codec, `agy` executor, `agentapi` reporter, install templates |
| **Harness: Fake** | §18.2 | Test adapter driving the core test suite |
| **Capture** | §4.1–4.3, §6.2 | `Stop` hook, spool, transcript copy, sweep, idle promotion |
| **Compaction** | §5.1, §6.3 | Deterministic compaction, author classification, exposure markers, scrubber |
| **Episode Store** | §5.1, §5.6 | Private JSONL+zstd episodes, `episodes/` layout |
| **Job Table** | §4.3 | SQLite job table, spool ingestion, tick lock, leases, back-pressure |
| **Tick** | §4.3, §12.3 | Spool ingest → job table → sweep → jobs → calendar; tick.lock |
| **Entity Extraction** | §5.2 | Code-only (no LLM) entity extractor: files, symbols, configs, topics, tools, errors, externals; extractor rules |
| **Pipeline Stages 2–6** | §6.4–6.8 | Triage, Extract, Reconcile, Verify, Publish (including LLM housekeeping via `agy -p`) |
| **Claim Store** | §5.3, §5.4, §5.6 | Claim file schema, month-sharded `.memory/claims/`, relations, events store (`.memory/events/`), `pipeline_version` |
| **Views** | §5.5 | MAP.md, claim indexes, subsystem pages, SIGNATURES.md, DECISIONS.md |
| **Index** | §5.6, §18.4 | SQLite rebuildable index in `.memory/.cache/index.sqlite` |
| **Delivery** | §7 | Task brief, touch, error, risky action, déjà-vu tiers 0–2, budgets, ledger |
| **Pull Tools (MCP)** | §7.7, §18.1 | `memory.ask`, `memory.why`, `memory.history`, `memory.brief`, `memory.note`, `memory.stats` |
| **Validity & Re-checking** | §8 | Fingerprint change → suspect, conditions-changed, closure condition evaluation |
| **Coverage & Elicitation** | §9 | Coverage model, gap scores, elicitation questions, review reports, inferred backfill |
| **Evaluation** | §10.3 | B1–B5 datasets and harness; B2 baseline; P1 gate (B5 at PR-05, B1/B2 at PR-06) |
| **Measurement** | §10 | Per-session signals, offline replay tuning, observability |
| **Security** | §11 | Scrubbing, provenance tagging, lint gate |
| **Publishing** | §6.8 | Two-lane publish, memory PR, review profiles, git worktree |
| **PA Integration** | §13 | Episode summaries, generated task briefs in WORKORDERs |
| **Backfill** | §14 | PA file import, transcript backfill |

## PR Sequence (linear dependency chain)

See [PROGRAM-PLAN.md](PROGRAM-PLAN.md) for the full dependency graph, experiments, scope per PR, human tasks and exit criteria.

| # | PR | SPEC Phase | Status | Depends On |
|---|---|---|---|---|
| 1 | `pr-01-go-skeleton` (incl. enrolment, kill switch, CI, probe hook) | P0 (partial) | ☐ Planned | — |
| 2 | `pr-01b-experiment-spike` (docs only, E1–E12) | P0 experiments | ☐ Planned | PR-01 |
| 3 | `pr-02a-transcript-to-episode` | P0 | ☐ Planned | PR-01b |
| 4 | `pr-02b-capture-runtime` (+ dogfood enrolment) | P0 | ☐ Planned | PR-02a |
| 5 | `pr-03-episode-search-eval` | P0.5 + §10.3 datasets | ☐ Planned | PR-02b |
| 6 | `pr-04-entities-claims-events` | P1 (partial) | ☐ Planned | PR-03 |
| 7 | `pr-05a-pipeline` (incl. local publish) | P1 | ☐ Planned | PR-04 |
| 8 | `pr-05b-pilot-b5` (milestone, alongside PR-06) | P1, **B5 gate** | ☐ Planned | PR-05a |
| 9 | `pr-06-publish-review-brief` | P1, **B1/B2 gate** | ☐ Planned | PR-05a |
| 10 | `pr-07-delivery` | P2 | ☐ Planned | PR-06; P1 gate (PR-05b + PR-06) |
| 11 | `pr-08-learning-loop` | P3 | ☐ Planned | PR-07 |
| 12 | `pr-09-pa-integration` (cross-repo) | P4 | ☐ Planned | PR-08 |
| 13 | `pr-10-backfill` | P5 | ☐ Planned | PR-09 |

## Human Approval

- [ ] Programme plan reviewed and approved
- [ ] PR sequence confirmed

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
- **Also fixed:**
  - portable tick lock and Windows CI;
  - install lifecycle;
  - recorded-response executor for tests, replacing the "identical results" idempotency criterion;
  - local publish moved to PR-05 (PD7);
  - session attribution and commit linkage in PR-02;
  - housekeeping runs excluded from the sweep;
  - Antigravity version fixtures and canaries;
  - human tasks scheduled.
- **Not changed (user decision, PD10):** no emergency transcript preservation ahead of PR-02.

### 2026-10-01 — Third-party review, GitHub issue #1 (session 3)

Every claim in the review was checked against the built-in Antigravity docs and the transcripts on disk. SPEC moved to v0.8 and the plan to 13 PRs.
- **Verified and adopted (SPEC v0.8):**
  - transcript file set and `truncated_fields` resolution (487 of 535 transcripts affected);
  - incremental transcript copying;
  - template-safe repo ID and per-clone publishing worktrees;
  - decision-marker salience windows;
  - a mechanical P13 precondition;
  - `gate-once` limited to irreversible actions;
  - tier-1 reasoning check at `PreToolUse`;
  - sidecar `restart_policy` and `agent_permissions`, with user enablement in the Automations dashboard;
  - `model_decision` instead of `glob`;
  - SQLite concurrency and a Windows-safe index swap;
  - new experiment E12.
- **Not adopted:** `PostToolUse` ignoring its matcher (contradicted by the hooks doc); the 46 MB transcript figure (largest on disk: 3.9 MB); section and path citations that don't match the spec.
- **Plan changes:**
  - the probe hook folded into PR-01;
  - experiments became the docs-only PR-01b spike (PD3 revised);
  - PR-02 split into 02a/02b (PD11);
  - PR-05 split into 05a/05b, with the pilot as a milestone alongside PR-06 (PD12);
  - every PR at 4–7 WOs (PD13);
  - dogfood enrolment of this repo (PD14);
  - Module Index populated.
- **The revised PR structure needs re-approval** (Human Approval above).

### 2026-10-01 — Review #1 follow-up (session 4)

The reviewer's follow-up on `chunks/` was verified on all 537 conversations:
- every non-final chunk is exactly 102,400 bytes;
- concatenated chunks equal the monolithic file byte for byte;
- `transcript_full.jsonl` has no truncation and matches `transcript.jsonl` line for line.

The v0.8 four-source merge was dropped in favour of reading one source in priority order (SPEC v0.9 §6.2, PD16). E12 is reduced to append-only and rewrite behaviour and the cause of step gaps. PR-02a wo1, PR-02b wo1 and PR-01b wo1 were updated.
