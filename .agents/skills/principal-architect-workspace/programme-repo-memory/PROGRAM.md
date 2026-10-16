# repo-memory Programme

> **Programme:** Build `repo-memory`, a repository-level memory system for coding agents.
> **Specification:** [SPEC.md](../../../../SPEC.md) v1.0 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 4 / v2.2 + [Addendum A](../../../../docs/addendum-a-provenance-and-hardening.md) rev 3
> **Status:** Executing — **v2.2 revision approved (2026-10-15)**. **19/22 complete** (PR-01 through PR-09b, PR-10a-v2-2-docs-and-specs [PR #17], PR-10b-delivery-and-storage-fixes [PR #18 APPROVED], PR-11-v2-doc-seeding-and-lifecycles [PR #19 APPROVED], PR-12-v2-distillation [PR #20 APPROVED], PR-13-v2-consolidation [PR #21 APPROVED]; PR-10-eval-v2 at WO3/7 Stage 1 complete). **Next active:** **PR-14-v2-pilot** and/or **PR-10-eval-v2** Stage 2 (WO4–WO7).
> **Base branch:** `pr-13-v2-consolidation/wo6-tick-orchestration-and-verification` (ahead of `pr-12-v2-distillation/wo6-managed-instruction-block-and-verification` `e6ee499`, `pr-11-v2-doc-seeding-and-lifecycles/wo5-dogfood-doc-seed-and-verification` `860610f`, `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification` `4abdc55`, `pr-10a-v2-2-docs-and-specs/wo5-verification-and-handover` `723593b`, `pr-10-eval-v2/wo3-index-build` `ec96755`, and `main` `83edd25`)
> **Revision:** v2.2 (2026-10-15). Synthesizes empirical findings from PR-09 (X6-lite + 231-transcript study) and PR-10 Stage 1 (77-session doc/skill audit), fixes the `8d7ad7b`+`d3ab994` `EphemeralMessage` persistence regression, and incorporates the 6 v2.2 architectural pillars.

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

### Phase 3: v2.2 Distillation, Retrieval & Evaluation

Read side and empirical evaluation first, upgraded in v2.2 (`2026-10-15`) after PR-09 X6-lite and PR-10 Stage 1:

| # | PR | Objective | Status | Depends On |
|---|---|---|---|---|
| 9 | `pr-05-v2-foundation` | Close PR-05b onto `main`; fix capture→episode, re-enable hooks safely, cost telemetry, artifact capture, probe-log cleanup, non-enrolled fast path | ✅ **Complete** (PR #4, merged) | `main` + PR-05b branch |
| 10 | `pr-06-v2-spike` | Prototype-validate platform mechanics: X1, X3, X5, X8, X9 | ✅ **Complete** (PR #6, merged) | PR-05-v2-foundation |
| 10b | `pr-06b-v2-plugin-split` | Daemon + workspace bundle architecture | ✅ **Complete** (PR #5, merged) | PR-06-v2-spike |
| 11 | `pr-07-v2-storage` | Op log, dossiers, atlas, scopes, index migration, v1 removal, dossier seeding | ✅ **Complete** (PR #9, merged) | PR-06-v2-spike (X-gate) |
| 12 | `pr-08-v2-delivery` | Channels, moments, injection ledger, compaction re-injection, pull tools, latency budgets | ✅ **Complete** (PR #14, merged) | PR-07-v2-storage |
| 13 | `pr-09-v2-x6-lite` | Pre-registered B1 replay + 231-session PA retrieval study (STOP verdict on spec-style tasks → pivoted to Eval v2 on esp32) | ✅ **Complete** (`45a68cd`) | PR-08-v2-delivery |
| 13b | `pr-09b-product-fixes` | Stage 0 product fixes: canonical `IndexPath`, bm25 FTS scoring, relevance threshold, per-session `InjectedSet` dedup | ✅ **Complete** (`208b0e0`) | PR-09-v2-x6-lite |
| 14a | `pr-10a-v2-2-docs-and-specs` | **v2.2 Spec & Design Upgrade:** Design v2 Rev 4, Addendum A Rev 3, SPEC v1.0, hooks reference, README, and v2.2 Programme Plan | ✅ **Complete** (PR #17, APPROVED) | PR-10 Stage 1 (`ec96755`) |
| 14b | `pr-10b-delivery-and-storage-fixes` | Fix `8d7ad7b`+`d3ab994` `EphemeralMessage`→`UserMessage` vanishing-memory bug, wire `DetectRewind`, fix open issues `#10`–`#13` & `#16`, rebuild binary & esp32 index | ✅ **Complete** (PR #18, APPROVED) | PR-10a-v2-2-docs-and-specs |
| 14 | `pr-10-eval-v2` | **Eval v2 (2×2 factorial on esp32):** WO1–WO3 (Stage 0b & Stage 1) complete; resume WO4 (dry-run) → WO5 (Stage 2a pilot) → WO6 (Stage 2b main) → WO7 (report) | 🔄 **Next (Resume WO4/7)** | PR-10b-delivery-and-storage-fixes |
| 15 | `pr-11-v2-doc-seeding-and-lifecycles` | Doc-Seeded Dossiers (`repo-memory seed-docs`), async doc-diff freshness ("Dreaming"), `semantic`/`procedural`/`episodic` lifecycles, and `{section?}` optional rendering | ✅ **Complete** (PR #19, APPROVED) | PR-10b-delivery-and-storage-fixes |
| 16 | `pr-12-v2-distillation` | Automatic milestone-gated `Stop` (`NO_TOOL_CALL`) warm-cache turn notes, free harvest (`CHECKPOINT` + `brain/`), `memory_note` dedup, A8 managed block, §5.6 preparation layer, sidecar fallback | ✅ **Complete** (PR #20, APPROVED) | PR-10-eval-v2 & PR-11 |
| 17 | `pr-13-v2-consolidation` | Warm batch consolidation (`stream-json` `repo-memory-distiller`), Pre-Consolidation Corroboration Gate, P13 echo filter, `user:`/`branch:`/`repo:` scope isolation, two-lane publish | ✅ **Complete** (PR #21, APPROVED) | PR-12-v2-distillation |
| 18 | `pr-14-v2-pilot` | End-to-end live distillation & consolidation paired evaluation (Addendum A10) & cost report | 🔄 **Next** | PR-13-v2-consolidation |
| 19 | `pr-15-v2-learning` | Procedural execution validation (`PostToolUse` exit-code tracking), deterministic skill graduation (`.agents/skills/rm-<slug>/SKILL.md`, Addendum A7), `closes_when` & gardening | ☐ Planned | PR-14-v2-pilot |
| 20 | `pr-16-v2-pa-and-rollout` | PA compatibility-mode integration, full repository enrolment (`seed` + `seed-docs`), and issue close-out | ☐ Planned | PR-15-v2-learning |

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
- [x] **v2.2 programme plan reviewed and approved** (2026-10-15: upgrade specs/design docs first in `PR-10a`, fix `UserMessage` persistence & storage bugs in `PR-10b` before `PR-10` Stage 2, and adopt the 6 v2.2 architectural pillars)

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

The v2 design ([design-v2-distillation-retrieval.md](../../../../docs/design-v2-distillation-retrieval.md) rev 3) emerged from the dogfood evaluation, two rounds of review on issue #3, and platform answers on issue #2.

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

### 2026-10-06 — PR-07-v2-storage complete (sessions 16–17)

**Summary:** The v2 storage layer is fully built. 6 WOs delivered 8,425 lines of new code across `ops/`, `dossiers/`, `config/scopes.go`, `index/` (v2 tables), `seed/`, and the `seed` CLI subcommand. Removed ~3,943 lines of v1 pipeline stages, claims, events, and views.

**Cross-PR decisions:**
- Content-addressed seeding IDs (`seed-<hash>`) enable idempotent re-seeding — important for PR-09 X6-lite where time-cut dossiers are rebuilt per work order.
- Time-cut is built into both the op iterator (`IterateBefore`) and the dossier projector (`ProjectBefore`), ready for PR-09's `--before <commit>` protocol.
- The exposure list slot is reserved in the ledger (PR-08 wo1) from the start, so the echo filter (PR-11) has data from day one.

**Architecture evolution:**
- The codebase has shifted from a v1 four-stage pipeline to a v2 event-sourced model. The op log is the single source of truth. Dossiers are rendered views, not stored state.
- The index is now a projection: rebuilt from the op log in a single transaction, not incrementally updated by pipeline stages.
- Scope resolution is a three-source combiner (config, file-anchor derivation, alias tracking), not a static mapping.

**Unresolved items escalated from PA:** CI checks had a macOS test failure on the merged PR #9 (pre-existing `TestFastPath_NonEnrolledLatency` doubled output buffer issue — documented in bridge prompt). Not caused by PR-07 changes.

### 2026-10-07 — PR-08-v2-delivery complete (session 18)

**Summary:** The v2 delivery engine is fully built. 6 WOs delivered +1,147 / −119 lines. Implemented persistent `systemMessage`, ephemeral `userMessage` hints, deny-once gates, cross-process ledger safety with flock, compaction re-injection, and 6 MCP pull tools (`memory_brief`, `memory_search`, etc.). Merged as PR #14 after 5 rounds of review yielding 34 fixes.

**Key architectural decisions:**
- **Gate lifecycle:** PreToolUse → PostToolUse handshake (`PendingGates`).
- **Cross-process ledger safety** via `flock(LOCK_EX)` abstracting Windows to a no-op.
- **Incremental transcript scanning:** `LastProcessedStepIndex` high-water mark.
- **MCP server always starts** (even unenrolled) to guide models.
- **Tool names use underscores** (`memory_brief`, etc.) instead of dots for portability.

### 2026-10-11 — PR-09-v2-x6-lite & PR-09b-product-fixes complete (sessions 19–20)

**Summary:**
- **PR-09-v2-x6-lite:** Executed 92 agent runs across 4 work orders ([x6-lite-verdict.md](../../../../docs/evaluations/x6-lite-verdict.md)). Verdict: **STOP (wrong instrument, not a product fail)** — 150-line spec-style work orders on an 18K-line repo scored 84% (42/50) with no memory, leaving no headroom. Also completed the 231-transcript retrieval study ([pa-memory-retrieval-effectiveness.md](../../../../docs/evaluations/pa-memory-retrieval-effectiveness.md)), proving that **unprompted pull memory is unused** (0 MCP pull calls in X6-lite; only 1.3% of non-PA sessions read `MEMORY.md` and 0% read `PROJECT.md`).
- **PR-09b-product-fixes:** Fixed Stage 0 defects identified in X6-lite ([PROJECT.md](../pr-09b-product-fixes/PROJECT.md)): unified `index.IndexPath(dataDir, repoID)` across all 4 code paths, added bm25 FTS relevance weight (`ftsScore * 100`), added `DefaultMinRelevanceScore = 50`, and implemented per-session `InjectedSet` deduplication in `Engine`/`Ledger`.

### 2026-10-15 — PR-10-eval-v2 Stage 1 & v2.2 Programme Revision (session 21)

**Summary:**
- **PR-10-eval-v2 Stage 0b & Stage 1 (`WO1`–`WO3`):** Built the `esp32-app-foundation` 4-condition evaluation templates (`tpl-a/b/c/d`, 989/989 native tests passing), local remote-protection harness (`eval_run.sh`), seeded **260 entries** (157 decisions, 58 ruled-out, 45 open-issues; 2,206 anchors) from 79 PA workorders, froze 2 forward tasks (DS18B20 temperature module & Bitcoin price module), and verified 70–90% offline precision@10. A 77-session transcript audit revealed that curated repo skills and developer guides (`framework-expert` 5%, dev guides 8%, `creating-a-module` 2%) are almost never pulled by coding agents.
- **Critical Regression Found Before Stage 2 (`8d7ad7b` + `d3ab994`):** Commit `8d7ad7b` switched `task-start`, `first-touch`, `first-entry`, and post-`CHECKPOINT` `re-injection` from `systemMessage` to `EphemeralMessage`, while `PR-09b` (`d3ab994`) added per-session `InjectedSet` deduplication. Together, injected memories vanish after 1 tool call and are blocked from re-injection for the rest of the session. Also found `DetectRewind` (`compaction.go:54`) is never called in `HandlePreInvocation`.
- **v2.2 Architecture Synthesis & Programme Revision Approved:** User approved the `v2.2` revision sequencing `PR-10a-v2-2-docs-and-specs` (Design v2 Rev 4, Addendum A Rev 3, SPEC v1.0, hooks reference, README, and Programme v2.2) and `PR-10b-delivery-and-storage-fixes` (`EphemeralMessage` $\to$ `UserMessage` fix, `DetectRewind` wiring, and open issues `#10`–`#13` & `#16`) **before** resuming `PR-10-eval-v2` at `WO4` (`wo4-dry-run`) and `WO5` (`wo5-pilot`).
- **Key v2.2 Decisions (`VD13`–`VD19`):**
  - `VD13`: Persistent push moments (`task-start`, `first-touch`, `first-entry`, `re-injection`) use `UserMessage` (plain string, persistent until `CHECKPOINT`) where `systemMessage` is rejected; `EphemeralMessage` is strictly for 1-step hints.
  - `VD14`: Automatic milestone-gated `Stop` (`NO_TOOL_CALL`, `executionNum == 0`) end-of-turn continuation + free `CHECKPOINT`/`brain/` harvest is the primary Stage 1 capture mechanism (preserving `G1`/`P1` zero-cooperation); `memory_note` is an optional mid-turn override that deduplicates against the `Stop` gate.
  - `VD15`: Stage 2 batch consolidation uses a multi-turn `stream-json` `repo-memory-distiller` session per tick to share the warm 5-minute prefix cache across dossiers.
  - `VD16`: Doc-Seeded Dossiers (`repo-memory seed-docs` / `DocSeedSource`) + async git-diff doc freshness ("Dreaming") convert static repo guides/skills into push-delivered dossier entries.
  - `VD17`: The 8 entry types are governed by 3 distinct cognitive lifecycles (`semantic`, `procedural`, `episodic`) with optional `{section?}` dossier rendering.
  - `VD18`: Five-tier state scope hierarchy (`temp:`, `session:`, `user:`, `branch:`, `repo:`) + Pre-Consolidation Corroboration Gate block uncorroborated hypotheses from becoming active `do:` imperatives.
  - `VD19`: Remove stale v1 `E02` root `GEMINI.md` and `.memory/views/`; keep `GEMINI.md` strictly machine-managed by `repo-memory init` (Addendum A8).

### 2026-10-15 — PR-10a-v2-2-docs-and-specs complete (session 22)

**Summary:**
- Completed all 5 work orders in [PR-10a-v2-2-docs-and-specs](../pr-10a-v2-2-docs-and-specs/PROJECT.md):
  1. `wo1-design-v2-rev4`: Upgraded `docs/design-v2-distillation-retrieval.md` to Revision 4 (`v2.2`).
  2. `wo2-addendum-a-rev3`: Upgraded `docs/addendum-a-provenance-and-hardening.md` to Revision 3 (`A7` procedural validation & `.agents/skills/rm-<slug>/SKILL.md` graduation, `A8` `v2` managed block, `A9` `DocSeedSource`).
  3. `wo3-spec-v1-0`: Upgraded `SPEC.md` from `Draft v0.10` to **`v1.0`** (`v2.2` Architecture) and marked `docs/experiments/SPEC-CHANGES.md` applied.
  4. `wo4-info-docs-and-cleanup`: Updated `docs/antigravity-hooks-reference.md`, created `README.md`, and removed stale v1 `GEMINI.md` and `.memory/views/`.
  5. `wo5-verification-and-handover`: Verified cross-document links, external hygiene, and full Go test suite (`go test ./...` green); remediated post-`CHECKPOINT` schema/path/CLI drift (`723593b`) and published as [PR #17](https://github.com/cblabs-org/repo-memory-plugin/pull/17).

### 2026-10-15 — PR-10b-delivery-and-storage-fixes complete & Cold-Firewall PR #18 Approved (session 23)

**Summary:**
- Executed all 6 work orders (`wo1`–`wo6`) in [PR-10b-delivery-and-storage-fixes](../pr-10b-delivery-and-storage-fixes/PROJECT.md) via dedicated Tier 3 Execution Worker sub-agents (zero PA context compaction) and validated via the 2-round Cold-Firewall PR Audit loop on [PR #18](https://github.com/cblabs-org/repo-memory-plugin/pull/18):
  1. `wo1-persistent-usermessage-and-rewind` (`89595e4`): Switched `MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, and `BuildReinjection` from `EphemeralMessage` to persistent `UserMessage` (`ChannelUser`), keeping `EphemeralMessage` strictly for 1-step `about-to-edit`, `error-match`, and FTS warnings; wired `DetectRewind(steps, ledger.LastKnownStepCount)` into `HandlePreInvocation`.
  2. `wo2-index-and-projector-fixes` (`a808fa5`): Fixed GitHub Issue **#13** (`ON CONFLICT(id) DO UPDATE` + child row cleanup in `entry_anchors`/`error_patterns` + recursive triggers) and Issue **#11** (`moveEntry` cross-scope deduplication in the dossier projector).
  3. `wo3-seeder-fixes` (`bed436e`): Fixed GitHub Issue **#10** (`IsValidAnchorPathSyntax` + `buildRepoPathValidator` rejecting arbitrary code tokens/non-paths) and Issue **#12** (`--before` cutoff-relative commit timestamp stamping).
  4. `wo4-ops-cleanup-and-rebuild` (`2b97c02`): Fixed GitHub Issue **#16** via `repo-memory ops gc`, cleaning `.memory/` and all 8 `evaluation/data/<wo>` snapshots, rebuilding `~/.local/bin/repo-memory`, and re-indexing `<data-dir>/<repo-id>/index.db` and the `esp32` evaluation templates (`tpl-c`, `tpl-d`).
  5. `wo5-dry-run-verification` (`60fd200`): Added `TestPreInvocation_MultiInvocationLifecycle` verifying 5-phase multi-invocation persistence, deduplication, compaction re-injection, and rewind re-arming.
  6. `wo6-review-remediation` (`bcdfae0`, `30123f3`): Resolved all 5 Cold-Firewall Audit findings on PR #18 (`[MAJOR-1]` parenthesized `(none)` filtering in `IsJunkText` + GC re-run, `[MAJOR-2]` preserving cleared `InjectedSet` across process boundaries in `loadLedgerLocked`, `[MINOR-1]` pruning empty source dossiers in `moveEntry`/`Writer.WriteAll`, `[MINOR-2]` `path#symbol`/`:symbol` anchor validation, `[MINOR-3]` `busy_timeout(5000)` + `foreign_keys(1)` on `NewReadOnlyIndex`). Approved in Round 2 Cold-Firewall Verification.

### 2026-10-15 — PR-11-v2-doc-seeding-and-lifecycles complete & Cold-Firewall PR #19 Approved (session 24)

**Summary:**
- Executed all 6 work orders (`wo1`–`wo6`) in [PR-11-v2-doc-seeding-and-lifecycles](../pr-11-v2-doc-seeding-and-lifecycles/PROJECT.md) via dedicated Tier 3 Execution Worker sub-agents and validated via the 2-round Cold-Firewall PR Audit loop on [PR #19](https://github.com/cblabs-org/repo-memory-plugin/pull/19):
  1. `wo1-cognitive-lifecycles-schema` (`c9640fc`): Added `CognitiveKind` (`semantic`, `procedural`, `episodic`), `DeriveCognitiveKind`, `EffectiveKind`, canonical `EntryType` constants (`contract`, `convention`, `external-behaviour`, `recipe`), `SourceDocSeed = "doc-seed"`, and `v2.2` provenance/procedural tracking fields across `dossiers`, `ops`, `delivery`, `index` (with idempotent `migrateEntriesColumns` and `ListEntriesBySource`), and `jobs.Kind` (`KindSeedDocs`, `KindDreamFreshness`).
  2. `wo2-optional-section-rendering` (`16d0be3`): Updated `internal/core/dossiers/renderer.go` to enforce `{section?}` Optional Section Resolution (`## Invariants & contracts`, `## Ruled out, failure signatures and gotchas`, `## Decisions`, `## Procedures / Recipes`, `## Interfaces and dependencies`, `## Open issues`), render `kind:` and `source_doc:` in entry metadata lines, and added deterministic `InferEntryTypeFromID` in `handedit.go`.
  3. `wo3-doc-seed-source` (`37f2051`): Added `DocSeeds []string` and `ResolveDocSeedGlobs` in `internal/core/config/config.go`, `DocSeedSource` / `SeedDocs` / `ParseDocToOps` / `ComputeGitBlobSHA` in `internal/core/seed/doc_seed.go`, and `repo-memory seed-docs` CLI in `cmd/repo-memory/cmd_seed_docs.go`.
  4. `wo4-async-doc-freshness-dreaming` (`f54c27f`): Implemented `CheckDocFreshness` / `checkDocFreshness` in `internal/core/tick/dream_freshness.go` and wired into `PipelineExecutor.Run` (`internal/core/tick/pipeline.go`).
  5. `wo5-dogfood-doc-seed-and-verification` (`572ddb5`): Configured `doc_seeds` in `.memory/config.yaml`, dogfooded `repo-memory seed-docs` across `repo-memory-plugin`, re-projected `.memory/dossiers/` and `.memory/ATLAS.md`, and rebuilt `~/.local/bin/repo-memory` + `<data-dir>/<repo-id>/index.db`.
  6. `wo6-review-remediation` (`23f082d`): Resolved all 3 Round 1 Cold-Firewall PR Audit findings (`[MAJOR-1]` polluted dot-prefixed/root-file scope filtering in `SeedDocs`/`ParseDocToOps`, `[MAJOR-2]` pruning `0`-entry `<scope>.md` files and `0001-01-01T00:00:00Z` `ATLAS.md` rows in `dossiers.Writer.WriteAll`, `[MINOR-1]` `FilesSkipped` counter when `candidateOps` is empty), mutually idempotent with `repo-memory ops gc`. Approved in Round 2 Cold-Firewall Verification.

### 2026-10-16 — PR-12-v2-distillation complete & Cold-Firewall PR #20 Approved (session 25)

**Summary:**
- Executed all 6 work orders (`wo1`–`wo6`) in [PR-12-v2-distillation](../pr-12-v2-distillation/PROJECT.md) via dedicated Tier 3 Execution Worker sub-agents and validated via the Cold-Firewall PR Audit loop on [PR #20](https://github.com/cblabs-org/repo-memory-plugin/pull/20) (`APPROVED: 0 blocking, 0 major, 3 minor`):
  1. `wo1-marks-inbox-and-subagent-registry` (`c9dade3`): Implemented per-conversation marks (`internal/core/state/marks.go` with `ClampIfRewound`), structured inbox notes (`internal/core/inbox/note.go` across `turn-note | mcp-note | checkpoint | artifact | sidecar` with `ExposureList` and `AnchorChecks`), SQLite subagent registry (`internal/core/state/subagents.go` populated on `PostToolUse` `invoke_subagent`), and updated `memory_note` (`source: "mcp-note"` + mark advancement) and `memory_status`/`memory_stats`.
  2. `wo2-stop-milestone-gate-and-harvest` (`f053df6`): Added `turn_notes` (`on | fallback-only | off`) and `turn_notes_role_opt_out` to `config.RepoConfig`, and implemented the `executionNum == 0` Milestone Gate (`NO_TOOL_CALL` + `fullyIdle`, 5 milestone signals, 30m cooldown waived for commits/PRs, 20/day cap) and `executionNum >= 1` `repo-memory-note` YAML block parser/harvester (`internal/harness/antigravity/stop_distill.go`).
  3. `wo3-free-harvest-checkpoint-and-artifacts` (`7a743c4`): Implemented deterministic zero-token Stage 1b harvest (`HarvestCheckpointsAndArtifacts` in `internal/core/inbox/free_harvest.go`, wired into `PipelineExecutor.Run`).
  4. `wo4-preparation-layer-and-scrubber` (`5b052eb`): Implemented the 4-layer secret scrubber (`internal/core/scrub/scrubber.go`, Addendum A §5 `A4`, preserving UUIDs/ULIDs/entry IDs and git commit SHAs + `QuarantinePayload`) and `PrepareSession` / `ValidateDistillerOutput` (`internal/core/pipeline/prepare/`, `SPEC.md` §6.4 / `design-v2` §5.6).
  5. `wo5-sidecar-fallback` (`09fc797`): Updated `packaging/antigravity/agents/repo-memory-distiller/agent.md` to the `v2.2` distiller specification, wired `StageDistill` / `StageConsolidate` in `Housekeeper.Run` (`--agent repo-memory-distiller`), and implemented Stage 1c Sidecar Fallback (`RunSidecarFallback` in `internal/core/tick/fallback.go`) gated by 2h idle threshold (or subagent/`fallback-only`) and $\ge 25\%$ Flash quota headroom.
  6. `wo6-managed-instruction-block-and-verification` (`5e9f197`, `e038f91`): Implemented Addendum A §8 (`A8: Managed instruction blocks (v2)`) in `internal/core/config/managed_block.go`, wired into `enroll`, `unenroll`, and `doctor`, resolved all 3 minor audit findings (`e038f91`), and rebuilt `~/.local/bin/repo-memory`.

### 2026-10-16 — PR-13-v2-consolidation complete & Cold-Firewall PR #21 Approved (session 26)

**Summary:**
- Executed all 6 work orders (`wo1`–`wo6`) in [PR-13-v2-consolidation](../pr-13-v2-consolidation/PROJECT.md) via dedicated Tier 3 Execution Worker sub-agents and validated via the Cold-Firewall PR Audit loop on [PR #21](https://github.com/cblabs-org/repo-memory-plugin/pull/21) (`APPROVED: 0 blocking, 0 major, 2 minor`):
  1. `wo1-echo-filter-and-corroboration-gate` (`0454981`): Implemented deterministic `P13` Echo Filter (`FilterEchoes`, honoring known and unknown exposure lists unless reconfirmed by edit+build/test pass) and `VD18` Pre-Consolidation Corroboration Gate (`EnforceCorroborationGate`, downgrading uncorroborated `semantic`/`procedural` rules to `open-issue` with deterministic `closes_when` conditions) in `internal/core/consolidate/filter.go`.
  2. `wo2-routing-dedup-and-anchor-checks` (`3adc140`): Implemented Five-Tier scope classification (`ClassifyTier`: `user:`, `branch:`, `repo:`), dossier routing (`RouteItems` via `ScopeResolver` + `DeriveModuleScopes`), exact-duplicate `OpAmend` merging (`MergeExactDuplicates`), and existing-entry anchor validation (`CheckDossierAnchors`) in `internal/core/consolidate/route.go`.
  3. `wo3-multi-turn-stream-json-consolidator` (`4d38909`): Added `MultiTurnHousekeeper` (`RunMultiTurn` via `agy -p --input-format stream-json --output-format stream-json --agent repo-memory-distiller` with NDJSON `{"event":"user","message":{"content":...}}` input lines and cumulative token delta calculation) in `internal/harness/antigravity/executor.go`, updated `packaging/antigravity/agents/repo-memory-distiller/agent.md`, and implemented `<consolidation-batch-TOKEN>` prompt framing, `ParseConsolidatorOutput`, `SynthesizeDeterministicOps`, and `ConsolidateDossiers` in `internal/core/consolidate/llm.go`.
  4. `wo4-validate-record-and-render` (`16fc65b`): Implemented `ValidateAndPrepareOps` (state-aware target checks, $\le 6$-line cap, `ops.ValidateOp`, 4-layer `scrub.Scrub` + `QuarantinePayload`), `RecordAndRenderDirs`, and `RecordAndRenderTier` in `internal/core/consolidate/record.go`.
  5. `wo5-five-tier-scope-and-two-lane-publish` (`65c94ea`): Added `user-dossiers/{ops,dossiers}`, `worktrees`, and `EnsureMemoryGitignore` (`local/`, `.cache/`) in `internal/core/config/layout.go`, `RebuildFromOpLogs` in `internal/core/index/rebuild.go`, `EnsureMemoryWorktree` and `PublishToMemoryWorktree` (`memory/pending`) in `internal/core/gitops/worktree.go`, and Five-Tier recording, `PromoteMergedBranchOps`, and `RebuildMultiTierIndex` in `internal/core/consolidate/publish.go`.
  6. `wo6-tick-orchestration-and-verification` (`b71e77e`, `236d169`): Implemented `RunConsolidationWithOptions` and `FreshnessState` (`state/freshness.json`) in `internal/core/tick/consolidate.go`, wired `RunConsolidation` ($\ge 10$ notes or 24h cadence + 25% Flash quota guard) and `RebuildMultiTierIndex` into `PipelineExecutor.Run`, added `repo-memory consolidate` CLI (`cmd/repo-memory/cmd_consolidate.go`), and rebuilt `~/.local/bin/repo-memory`.
