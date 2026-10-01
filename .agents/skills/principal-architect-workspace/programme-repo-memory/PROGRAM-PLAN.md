# repo-memory — Programme Plan

> **Programme:** repo-memory
> **Specification:** [SPEC.md](../../../SPEC.md) v0.9
> **Release:** 1 (Antigravity)
> **Created:** 2026-10-01 · **Revised:** 2026-10-01 (session 2: plan review; session 3: third-party review, GitHub issue #1)
> **Total PRs:** 13, each with 4–7 work orders
> **Dependency model:** Linear chain. The experiment spike (PR-01b) runs straight after PR-01 and completes before PR-02a. The pilot and B5 milestone (PR-05b) runs alongside PR-06. The P1 gate must pass before PR-07 merges.

---

## Repositories in Scope

This programme spans three repositories. Work is done in the repository that owns it. Changes to another repository go through that repository's own process (issue or PR), never by direct edits from this programme.

| Repository | Owns | Programme work there |
|---|---|---|
| **repo-memory-plugin** (this repo) | SPEC, the `repo-memory` Go binary, Antigravity plugin packaging, CI and releases | All PRs except as noted below. Also enrolled as the **dogfood** repository from PR-02b, so the programme's own PA and worker sessions become a test corpus. |
| **esp32-app-foundation** (pilot) | Its `.memory/` store, `GEMINI.md` map include, PA workspace, existing `.agents/hooks.json` gates | Enrolment at PR-05b and PA workspace migration at PR-09, each done as a PR in that repo |
| **mm-skills/principal-architect** (`github.com/mm-skills/principal-architect`, managed via skill-manager) | The PA skill, which is used by other projects too | The PA change request at PR-09: raised as an issue/PR in that repo, with a compatibility mode so non-enrolled projects keep today's behaviour |

---

## Dependency Graph

```
PR-01   Go Skeleton, Interfaces, Enrolment, CI & Probe Hook
  │
  ▼
PR-01b  Experiment Spike (E1–E12)         docs only; complete before PR-02a
  │
  ▼
PR-02a  Transcript → Episode (parser, compaction, episode store)
  │
  ▼
PR-02b  Capture Runtime (spool, Stop hook, job table, tick, sidecar)   → dogfood enrolment
  │
  ▼
PR-03   Episode Search & Evaluation Harness
  │
  ▼
PR-04   Entities, Claims, Index & Events Store
  │
  ▼
PR-05a  Pipeline Code (stages 2–5 + local publish)
  │
  ├─────────────────────────────────┐
  ▼                                 ▼
PR-06   Shared Publish, Views,    PR-05b  Pilot & B5 Gate (milestone)
        Review, Brief/Ask Core            esp32 enrolment, OTA/BLE run, B5 labels
  │                                 │
  ▼                                 ▼
  ════════ P1 gate: B5 (PR-05b) + B1, B2 (PR-06) must pass before PR-07 merges ════════
  │
  ▼
PR-07   Delivery
  │
  ▼
PR-08   Learning Loop
  │
  ▼
PR-09   PA Integration (cross-repo)
  │
  ▼
PR-10   Full Backfill
```

### Experiments and the Work They Inform

All experiments run in PR-01b, so every design decision they inform is settled before PR-02a starts. Two are also re-confirmed later against the real implementation.

| Experiment | Informs | Status before the spike |
|---|---|---|
| E12 transcript file set | PR-02a parser, PR-02b incremental copy | Mostly answered (SPEC §6.2): read `transcript_full.jsonl`; chunks are byte slices. Remaining: append-only and rewrite behaviour, and the cause of step gaps. |
| E6 sidecar behaviour | PR-02b tick | `sidecar.json` format known from built-in guidance; behaviour still to observe |
| E7 housekeeping isolation | PR-05a executor | Open |
| E10 overhead reduction | PR-05a packing and budgets | Open |
| E11 quota and rate limits | PR-05a budgets | Preliminary in the spike; usage tracked through PR-05b |
| E1 `ephemeralMessage` persistence | PR-07 | Open |
| E2 include resolution | PR-07 | Size caps known; resolution and refresh still to test |
| E3 transcript currency at hooks | PR-07 (`PreInvocation` and `PreToolUse` tail reads) | Open |
| E4 rule activation | PR-07 native floor | Answered: `model_decision` only; no `glob` |
| E5 hook latency | PR-07 | Probe-hook estimate in the spike; confirmed at PR-07 exit |
| E9 `deny` behaviour, parallel batches, wording | PR-07 risky-action delivery | Open |
| E8 subagent hooks | PR-09 | Open |

---

## PR-01: Go Skeleton, Interfaces, Enrolment, CI & Probe Hook

**SPEC Sections:** §18.1–18.6, §4.5, §4.6, §5.6 (layout only)

**Objective:** Establish the Go module, binary entry point, subcommand wiring, package layout with the enforced core/adapter boundary, the harness interface types, and the `fake` test adapter. Add repository enrolment and the kill switch, so that a global plugin acts only on enrolled repositories. Add CI, the install lifecycle, and the probe hook the experiment spike needs.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-module-layout` | `go mod init`, `cmd/repo-memory/main.go`, subcommand wiring (cobra or manual), `Makefile`/`taskfile` for cross-compile (`CGO_ENABLED=0`) to all five targets. Create all `internal/core/*` and `internal/harness/*` packages as stubs with package docs. Import-graph boundary test: `internal/core` must not import any concrete adapter. | standard |
| 2 | `wo2-interfaces-fake` | Define `Harness`, `Capabilities`, `TranscriptSource`, `HookCodec`, `Reporter`, `Housekeeper` interfaces and the normalised types `Event`, `Injection`, `Step` in `internal/harness/`; `Stage` enum. Implement `internal/harness/fake/` with in-memory state, and the core test-suite shell that runs against it. | standard |
| 3 | `wo3-config-datadir` | Load repo `config.yaml` and machine `<data-dir>/config.yaml` (`enabled`, `housekeeping.executor`, `reports.surface`, `tick.trigger`, budgets). Platform-appropriate data directory. Create the storage layout on first run. | standard |
| 4 | `wo4-subcommands-install` | Wire `hook`, `tick`, `mcp`, `install`, `uninstall`, `search`, `report`, `status`, `reindex`, `doctor`. `install --harness antigravity` generates plugin files from `packaging/antigravity/` templates with the absolute binary path, including `sidecar.json` with `restart_policy: "always"`, `display_name`, `description` and `agent_permissions`. It is idempotent: re-running upgrades in place. It **never enables the sidecar**: it tells the user to enable it in the Antigravity Automations dashboard. `uninstall` removes the generated files. | standard |
| 5 | `wo5-enrolment-killswitch` | §4.6: `init` (creates `.memory/`, `.gitignore`, `GEMINI.md` include line, registry entry); `leave [--purge]`; `<data-dir>/registry.json`. Repo ID = `sha256(root commit : canonical origin URL)`, falling back to the main worktree's real path when there's no remote; `init` warns when `.memory/` was inherited from a template. Kill switch: `REPO_MEMORY_DISABLED=1`, `enabled: false`, `enable`/`disable [--repo]`. The hook framework checks the kill switch, then enrolment, and returns no-op output for non-enrolled workspaces within 5 ms of process start. | standard |
| 6 | `wo6-ci-release` | GitHub Actions: build all five targets; tests on macOS, Linux and Windows runners (including the boundary test); release artifacts per OS/arch with a version stamp. | standard |
| 7 | `wo7-probe-hook` | `repo-memory hook antigravity <event> --probe`: records the hook payload and a snapshot of the transcript file set to a probe log. Configurable probe decision for `PreToolUse` (allow/deny/ask with reason) for E9. Used by the PR-01b spike. | standard |

### Exit Criteria
- `go build ./cmd/repo-memory` succeeds for all 5 targets
- `repo-memory install --harness antigravity` generates correct `hooks.json`, `mcp_config.json` and `sidecar.json` (with `restart_policy` and `agent_permissions`); re-running upgrades in place; `uninstall` removes them; nothing enables the sidecar automatically
- `repo-memory init` enrols a repository with a template-safe repo ID; `leave` unenrols it
- A hook in a non-enrolled workspace returns no-op output within 5 ms of process start; with the kill switch set, hooks no-op and `tick` exits
- The probe hook records payloads for every Antigravity hook event
- Import-graph boundary test passes; core tests run against the `fake` adapter
- CI green on macOS, Linux and Windows runners

---

## PR-01b: Experiment Spike

**SPEC Sections:** §12.1.1

**Objective:** Answer experiments E1–E12 with the probe hook and a few `agy -p` runs, before any capture code is written. The deliverable is documentation: one report per experiment in `docs/experiments/`, plus a summary recording which SPEC sections it confirms or changes. No code is merged here, so it causes no rebase churn in the chain.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-transcript-and-hooks` | **E12 (remaining parts):** watch one live conversation grow and confirm `transcript_full.jsonl` is append-only; branch or edit an earlier step and confirm whether the file is rewritten; explain the step-index gaps. **E3:** is the transcript current at `PreInvocation` and at `PreToolUse`? **E1:** does `ephemeralMessage` persist, and does it appear in the transcript? | interactive |
| 2 | `wo2-delivery-surfaces` | **E2:** include resolution and refresh. **E4:** check that a `model_decision` rule loads when its description matches the task. **E9:** `deny` behaviour, re-issue rate, behaviour inside parallel tool batches, wording that isn't read as a policy block, and whether `overwrite` notices could serve non-destructive commands. **E8:** do subagents fire their own hooks? | interactive |
| 3 | `wo3-runtime-and-cost` | **E6:** sidecar ticking (CLI-only, sleep/wake, overlap). **E5:** probe-hook start-up latency on macOS and Windows. **E7:** housekeeping isolation (`--project`, `--continue`, history). **E10:** can the ~17k-token overhead be reduced? **E11:** preliminary quota and rate-limit readings. | interactive |
| 4 | `wo4-spec-reconciliation` | Summarise the results; list every SPEC section confirmed or changed; propose SPEC edits for user approval before PR-02a starts. | standard |

### Exit Criteria
- Each experiment has a documented answer with evidence in `docs/experiments/`
- SPEC updated (after approval) for any changed assumptions
- E5 and E11 have preliminary figures, to be re-confirmed in PR-07 and PR-05b

---

## PR-02a: Transcript → Episode

**SPEC Sections:** §5.1, §6.2 (transcript file set), §6.3, §11.3, §10.4 (metrics skeleton)

**Objective:** Turn an Antigravity conversation's transcript file set into a compacted, scrubbed, attributed episode. This is pure transformation, testable entirely on fixtures before any runtime capture exists.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-transcript-parser` | `internal/harness/antigravity/transcript.go`: read **one** source in priority order: `transcript_full.jsonl`; else `chunks/transcript_full/*.jsonl` concatenated in lexical order as a byte stream (`io.MultiReader`) before line splitting (chunks are 100 KiB byte slices, never parsed individually); else `transcript.jsonl`, with `truncated_fields` steps flagged. Record step gaps. `CHECKPOINT` handling; `source`/`type` classification. Scrubbed fixture corpus under `testdata/fixtures/antigravity/<version>/`, including a chunked conversation with records straddling chunk boundaries, and a compact-only conversation with truncated lines. Canary test that flags drops in parsed-step counts when the Antigravity version changes. | architect |
| 2 | `wo2-compaction` | `internal/core/pipeline/compact/`: deterministic compaction per §5.1. Author classification (`human` vs `automation`). Exposure markers. Secret scrubber (pattern + entropy). Target: ≥ 4× reduction, measured on merged full transcripts. | architect |
| 3 | `wo3-episode-store` | `internal/core/episode/store.go`: `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst`. Header including `harness`, `harness_version`, `started_by`, `pipeline_version`. Gap recording. Episodes are never pruned. | standard |
| 4 | `wo4-commit-linkage` | §6.2: link commits on the session's branch between start and end from `git log`/reflog, confirmed against commit commands in the transcript. Recorded in the episode header. | standard |
| 5 | `wo5-metrics-skeleton` | `internal/core/metrics/`: JSONL metrics writer. Capture health (sessions seen, captured, skipped, missed), hook execution time, tick duration, budget exhaustion. Re-measure the §4.4.1 transcript sizes from merged full transcripts. | standard |

### Exit Criteria
- Fixture conversations produce episodes with no unresolved `truncated_fields`; the chunk-only fallback yields byte-identical results to the monolithic file
- Compaction ratio ≥ 4× measured on full transcripts; §4.4.1 sizes re-measured
- Zero secrets in a scan of 50 fixture episodes
- Core tests pass on the `fake` adapter; `internal/core` imports no concrete adapter

---

## PR-02b: Capture Runtime

**SPEC Sections:** §4.1–4.3, §4.6, §6.2 (event and time paths), §12.1

**Objective:** Every Antigravity session in an enrolled repository produces an episode within minutes of going idle. The `Stop` hook copies transcripts incrementally; the tick sidecar ingests the spool, runs the job table, sweeps, and builds episodes with PR-02a. Claude Code transcripts are copied to `raw/` for Release 2. This repository is enrolled as the dogfood repository.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-spool-incremental-copy` | Spool entry schema; writer (unique temp name, then rename, with Windows semantics verified); reader for the tick. Incremental transcript copy of the primary source only (`transcript_full.jsonl`, or its chunks): byte offset and prefix hash in `state/`; full re-copy if the prefix changed. | standard |
| 2 | `wo2-stop-hook` | `internal/harness/antigravity/hooks.go`: handle `Stop`. Spool entry plus incremental copy. Budget ≤ 200 ms. `REPO_MEMORY_HOUSEKEEPING` guard. Kill-switch and enrolment checks come first. | standard |
| 3 | `wo3-job-table` | `internal/core/jobs/`: SQLite job table (§4.3) with WAL and `busy_timeout`. Schema: `id`, `kind`, `priority`, `idempotency_key`, `state`, `lease_expires`, `attempts`, `next_attempt_at`, `cost`. State machine and lease recovery. | standard |
| 4 | `wo4-tick-core` | `internal/core/tick/`: tick loop per §4.3. Portable `tick.lock` (`flock` on Unix, `LockFileEx` on Windows). Spool ingestion → jobs. Lease reclaim. Sweep of enrolled repositories only (Antigravity app and CLI stores, plus the Claude Code raw copy), with incremental copying. Idle promotion (15 min). Budget enforcement. Overlap behaviour per E6. | architect |
| 5 | `wo5-session-attribution` | Map conversations to enrolled repositories via `workspace_uris` in both `conversation_summaries.db` stores (app and CLI). De-duplicate conversations present in both stores. Link subagents to parents via `parent_conversation_id`. | standard |
| 6 | `wo6-sidecar-packaging` | `packaging/antigravity/sidecars/tick/sidecar.json`: `builtin: schedule`, `* * * * *`, `restart_policy: "always"`, `display_name`, `description`, `agent_permissions`. Install prints the Automations-dashboard enablement step; test run to confirm permissions. | mechanical |
| 7 | `wo7-claude-raw-copy` | Sweep extension: copy Claude Code transcripts for enrolled repositories from `~/.claude/projects/` to `<data-dir>/<repo-id>/raw/claude-code/`. No parsing. Respects the same permissions. | mechanical |

### Exit Criteria
- Every new Antigravity session in an enrolled repository produces an episode within minutes of going idle
- Sessions from non-enrolled workspaces are never read or copied
- Long sessions don't re-copy whole transcripts on each turn (incremental copy verified)
- Subagents linked to parents; app/CLI duplicates removed; commits linked
- Share caught only by sweep: measured
- `tick.lock` verified on macOS and Windows
- **repo-memory-plugin enrolled as the dogfood repository** and capturing its own PA and worker sessions

---

## PR-03: Episode Search & Evaluation Harness

**SPEC Sections:** §15.1 P0.5, §10.3

**Objective:** Make captured history searchable before the knowledge pipeline exists, and import PA files as searchable episodes. Build the evaluation harness and datasets (B1–B5) that later PRs are gated on, and measure the episode-search baseline for B2.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-fts-index` | SQLite FTS5 index over episodes (WAL, read-only readers). Columns: `episode_id`, `session_id`, `content` (compacted text), `files_touched`, `outcomes`, `date`. Populate from the episode store. | standard |
| 2 | `wo2-search-cli` | `repo-memory search <query>`: full-text search over episodes. Output: episode ID, date, branch, snippet, files. Rank by BM25. | standard |
| 3 | `wo3-pa-import` | Import PA `PROJECT.md`, `WORKORDER.md`, `context_bridge.md` files as episodes with `harness: "import"`. Date from file or git history. One episode per file. | standard |
| 4 | `wo4-ask-fallback` | `memory.ask` MCP tool: when no claims exist yet, fall back to FTS over episodes. Mark results as "unverified (episode search)". | standard |
| 5 | `wo5-eval-harness` | `repo-memory eval <benchmark>`: runner and result store for B1–B5 (§10.3). Dataset formats. Run the **B2 baseline** against episode search. | standard |
| 6 | `wo6-eval-datasets` | **B5:** sampling tool selecting ≥ 20 OTA/BLE episodes and producing a labelling sheet (correct / wrong / not durable, plus missed knowledge). **B2:** question bank of ≥ 50 questions from session logs and ADRs. **B1:** cut-off at commit `69d9c813` and the PA's hand-written "Framework Facts Workers Need" section as reference. **B3:** 3–5 historical tasks with starting commits. Drafted by the worker; reviewed by the user (see Human Tasks). | standard |

### Exit Criteria
- Can find what was tried, and when, across all captured and imported history
- PA workspace files searchable
- `memory.ask` returns relevant episodes with "unverified" marking
- B1, B2, B3 and B5 datasets exist and have been reviewed by the user
- B2 baseline (episode search) measured

---

## PR-04: Entities, Claims, Index & Events Store

**SPEC Sections:** §5.2, §5.3, §5.4, §5.5 (index only), §5.6 (claims/, events/), §18.4

**Objective:** Build the data model layer: entity extraction from the repository (no LLM), the claim file schema and store, the event store for human decisions, and the rebuildable SQLite index. This is the foundation the pipeline writes into and delivery reads from. It includes everything re-projection (PR-10) will need from day one.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-entity-types` | `internal/core/entities/`: entity types (`subsystem`, `file`, `symbol`, `config`, `env`, `topic`, `tool`, `error`, `external`). Entity relations (`contains`, `depends_on`, `implements`, `configures`). Entity registry in the index. | standard |
| 2 | `wo2-entity-extractor` | No-LLM entity extractor: tree walk for files (`.gitignore`-aware), parse `sdkconfig`/Kconfig/`platformio.ini`/lockfiles for configs, brace-aware C/C++ symbol scanner (ctags fallback), extractor rules from `config.yaml` for topics/tools/errors. | architect |
| 3 | `wo3-claim-schema` | `internal/core/claims/`: claim file parser and writer. YAML frontmatter + Markdown body. All fields from §5.3: `id`, `type`, `title`, `relevant_when`, `apply`, `status`, `flags`, `anchors`, `conditions`, `evidence`, `relations`, `scope`, `verified_at`, `provenance` (including `pipeline_version`), `review`, `match` blocks, `closes_when`, `enforced_by`. ULID generation. | standard |
| 4 | `wo4-claim-store` | `.memory/claims/<yyyy-mm>/` layout. CRUD operations. Month-sharded writes. Relation index (inverse lookups). | standard |
| 5 | `wo5-sqlite-index` | `internal/core/index/`: rebuildable index in `.memory/.cache/`. Tables: entities, claims, anchors, relations, conditions. **Concurrency:** WAL and `busy_timeout` on every connection; hooks open read-only; short write transactions. A full rebuild writes a new versioned file and then switches a pointer file; it never renames over the live index (Windows). `repo-memory reindex`. | standard |
| 6 | `wo6-subsystem-config` | `.memory/config.yaml` subsystem configuration. Glob matching for file→subsystem assignment. Unassigned file detection. | standard |
| 7 | `wo7-events-store` | §5.4: `.memory/events/reviews/*.jsonl` and `.memory/events/answers/*.jsonl`. Schema keyed by claim ID **and** content hash, so human decisions survive re-projection. Writers and readers used by PR-06; replay order documented for PR-10. | standard |

### Exit Criteria
- Entity extractor produces correct entities from the esp32-app-foundation structure (or test fixture)
- Claim files round-trip (write → read → write produces identical output)
- Index is rebuildable from claim files, entities and events; a hook-style reader never blocks during a rebuild (tested on macOS and Windows)
- `repo-memory reindex` works
- All entity kinds from §5.2 are extractable
- A review event recorded against a claim still applies after that claim's file is regenerated

---

## PR-05a: Pipeline Code

**SPEC Sections:** §6.4–6.7, §6.8 (local lane), §6.9, §4.4.1

**Objective:** The knowledge pipeline: triage, extract, reconcile and verify, plus the local publish lane, so reconciled claims land in the store and are usable at once. Each LLM stage is a pure function dispatched to `agy -p`. This PR merges once its deterministic and recorded-executor tests pass. Live pilot evaluation is PR-05b.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-executors` | `internal/harness/antigravity/executor.go`: `Housekeeper` via `agy -p` (`--model`, `--json-schema`, `--output-format json`, `--input-format stream-json`, `--print-timeout`); usage parsing; `REPO_MEMORY_HOUSEKEEPING=1`; working dir `<data-dir>/runs/<repo-id>/`; dedicated project per E7; **capture sweep excludes housekeeping runs**. Plus a recorded-response `Housekeeper` (cassettes keyed by stage, inputs and pipeline version) used by all stage tests. | standard |
| 2 | `wo2-triage` | `internal/core/pipeline/triage/`: rule-based triage (§6.4), keep/skip rules, salience windows including **decision-marker windows** (configurable marker list, capped at 2 per episode), synopsis generation, small-model fallback. `prompts/triage/`. | architect |
| 3 | `wo3-extract` | `internal/core/pipeline/extract/`: extraction (§6.5). Pack to the 60k cap (parent with subagents first). 20-candidate limit. All extraction rules. Candidates with anchors, evidence, `gaps`. P13. `prompts/extract/`. | architect |
| 4 | `wo4-reconcile` | `internal/core/pipeline/reconcile/`: reconciliation (§6.6). Anchor lookup (≤ 30). 7 operations. **P13 mechanical precondition:** after an exposure, support needs an outcome that touched an anchored file or symbol, or ran the claim's `enforced_by` check, before the reconciler's judgement applies. Single writer. `prompts/reconcile/`. | architect |
| 5 | `wo5-verify` | `internal/core/pipeline/verify/`: verification (§6.7). Anchor resolution and fingerprinting. Behavioural check (small model on code snippets). Branch scoping, including merge promotion and abandoned-branch promotion of ruled-out claims. | standard |
| 6 | `wo6-local-publish` | `internal/core/pipeline/publish/local.go`: write new/changed claims to `<data-dir>/<repo-id>/pending/`; index immediately; mark `unreviewed`; security and external-provenance gating. | standard |
| 7 | `wo7-orchestration` | Wire stages into the tick's job processing. Concurrency rules (extraction parallel; reconcile and publish single-writer). Idempotency keys per stage. Cost recording per stage. Daily budgets (from E11). | standard |

### Exit Criteria
- Reconciled claims land in the local lane and are queryable from the index
- Duplicate jobs are no-ops via idempotency keys; all stage tests pass against recorded executor responses
- Decision-marker windows and the P13 precondition covered by fixture tests

---

## PR-05b: Pilot & B5 Gate (milestone)

**SPEC Sections:** §10.3 (B5), §14, §4.4.1

**Objective:** Enrol the pilot repository, run its OTA and BLE history through the pipeline, and measure extraction precision (B5) and real pipeline cost. This is a milestone rather than a code PR: it runs alongside PR-06 development and doesn't block PR-06 from merging. Its B5 result is half of the P1 gate.

### Work Orders

| # | WO | Repository | Scope | Complexity |
|---|---|---|---|---|
| 1 | `wo1-subsystem-bootstrap` | repo-memory-plugin | `repo-memory bootstrap`: one-off LLM pass proposing subsystems and globs from directory structure and existing docs. Output: proposed `config.yaml` for human review. | standard |
| 2 | `wo2-pilot-enrolment` | esp32-app-foundation | `repo-memory init` and the reviewed subsystem config, as a PR in that repository. | mechanical |
| 3 | `wo3-ota-ble-run` | repo-memory-plugin | Run the pilot's OTA and BLE history (PA imports from PR-03 plus surviving transcripts) through the pipeline. Measure cost per episode and per stage against §4.4.1; continue E11 usage tracking. | standard |
| 4 | `wo4-b5-evaluation` | repo-memory-plugin | Produce the B5 labelling sheet from the PR-03 sample; after the user labels it, compute precision and recall per claim type; apply the §10.3 fallback if precision < 0.8. | standard |

### Exit Criteria
- **B5** precision ≥ 0.8 for pushable claims (verified or higher), or the §10.3 fallback applied and re-measured
- Real pipeline cost per episode and per stage recorded

---

## PR-06: Shared Publish, Views, Review & Brief/Ask Core

**SPEC Sections:** §5.5, §6.8 (shared lane), §7.2 (brief generation), §9.2.1, §11.4, §10.3 (B1, B2)

**Objective:** The shared publish lane (memory PR), generated views, the lint gate, weekly review reports with reply parsing, and review profiles. Human decisions are written as events. Build the core task-brief generator and claims-based ask (exposed on the CLI) so that B1 and B2 can be run once PR-05b's pilot data exist; PR-07 reuses them for hooks and MCP.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-shared-publish` | `internal/core/pipeline/publish/shared.go`: batch pending ops into a memory PR on `memory/pending`. Per-clone git worktree (`<data-dir>/<repo-id>/worktrees/<clone-id>/`). PR description grouped by subsystem. Never commit to the user's branch. | architect |
| 2 | `wo2-views` | `internal/core/views/`: MAP.md (≤ 2k tokens), claim indexes per subsystem (≤ 1.5k tokens), subsystem pages (≤ 5k tokens), SIGNATURES.md, DECISIONS.md. Budget enforcement with re-synthesis. Regenerate on claim/entity changes. | architect |
| 3 | `wo3-lint-gate` | CI lint gate for `.memory/`: schema validation, views match regeneration, every claim has ≥ 1 evidence, every anchor resolves or flagged, no secrets. `repo-memory lint`. | standard |
| 4 | `wo4-review-profiles` | `config.yaml` → `review.profile`: `solo` (default) and `team` per §6.8. Auto-merge rules per operation type. Every review decision (auto or human) written as a review event. | standard |
| 5 | `wo5-review-report` | Weekly review report via `agentapi new-conversation`, with the grants it needs listed in the sidecar's `agent_permissions`. Reply parsing (§9.2.1): explicit commands, conservative free-text interpretation, author classification. Human replies → answer and review events. | architect |
| 6 | `wo6-brief-ask-core` | `internal/core/delivery/brief`: task-brief generator (§7.2 priority order, budgets, claim-index fallback) and claims-based answer synthesis. Exposed as `repo-memory brief "<task>"` and `repo-memory ask "<question>"`; reused by PR-07. | architect |

### Exit Criteria
- Memory PR generated with correct grouping and evidence links
- Views regenerate correctly and stay within budget
- Lint gate catches: missing evidence, hand-edited views, unresolved anchors, secrets
- Review report parses `approve clm-…` and `Q-017: <answer>`; decisions stored as events
- Review profiles apply correctly
- **Once PR-05b data exist (completes the P1 gate):**
  - **B1:** a fresh agent given pr-225's original request plus `repo-memory brief` output (claim store cut at `69d9c813`) produces facts at parity with the PA's hand-written section, as judged by the user;
  - **B2:** `repo-memory ask` beats the PR-03 episode-search baseline.

---

## PR-07: Delivery

**SPEC Sections:** §7, §18.3 (capability-based policy), §18.5

**Precondition:** the P1 gate (B5, B1, B2) has passed before this PR merges.

**Objective:** The read side: hooks deliver knowledge at task start, file touch, error, risky action, and déjà-vu tier 0/1. Pull tools provide on-demand access. Performance within §7.8 budgets.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-delivery-engine` | `internal/core/delivery/`: normalised delivery engine; capability-based policy (§18.3); delivery ledger; token budgets (§7.1); rendering (`apply` line + ID + currency stamp); anchor-existence fast check; reuses the PR-06 brief generator. | architect |
| 2 | `wo2-preinvocation-posttooluse` | `PreInvocation`: fast path (p95 ≤ 60 ms); `alerts/`; transcript tail from cursor (per E3); task brief at `invocationNum == 0`; touch and error injection (the primary channel for build and edit warnings); déjà-vu tier 1 on reasoning; spool tier-2 requests. `PostToolUse`: detect git and dependency operations from the transcript step at `stepIdx`, confirming the tool from the step; spool re-checks. | architect |
| 3 | `wo3-pretooluse` | `PreToolUse`: `gate-once` **only** for destructive, hardware or irreversible actions (flash/upload, release/publish, force push, partition tables), with wording and batch behaviour per E9; `ask` and `next-invocation` modes; déjà-vu tier 1 on the concrete call **and** the current turn's reasoning at the transcript tail (per E3); coexistence with existing gates (e.g. esp32's `flash-tag-gate`). | standard |
| 4 | `wo4-pull-tools` | `internal/mcpserver/`: stdio MCP server with all six tools; `ask` and `brief` reuse the PR-06 core and may make synchronous housekeeping runs; every call instrumented. | architect |
| 5 | `wo5-includes-and-rules` | `GEMINI.md` include of `.memory/views/MAP.md` (per E2), with `PreInvocation` injection as fallback. Plugin `rules/AGENTS.md` (memory tools, the MAP → claim index walk, currency stamps). Native-floor path-scoped delivery via `model_decision` rules (per E4; no `glob`). | standard |
| 6 | `wo6-degraded-modes` | Test and implement all degraded modes from §7.9. Graceful fallback at each level. | standard |
| 7 | `wo7-hook-fixtures` | Versioned Antigravity hook-payload fixtures under `testdata/fixtures/antigravity/<version>/`; the adapter contract suite (§18.6 rule 7) runs against them. | standard |

### Exit Criteria
- B3 task replays run (PR-03 task list)
- Per-session push within budget (20k tokens)
- Hook latency: fast path p95 ≤ 60 ms, injection p95 ≤ 150 ms (macOS and Windows), confirming E5
- Degraded modes tested (no ticks, no housekeeping, no index, native floor, plugin off)
- Adapter contract suite green

---

## PR-08: Learning Loop

**SPEC Sections:** §7.6 (tier 2), §8, §9, §10.1–10.2

**Objective:** Déjà-vu tier 2 (semantic, via tick housekeeping run), validity re-checking, coverage and elicitation, per-session metrics, and offline replay tuning.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-dejavu-tier2` | Tier-2 déjà-vu: tick batches pending semantic checks into one housekeeping run. Alert written to `alerts/`. Rate limiting (one in-flight, one per 5 model calls). Per-day caps. | standard |
| 2 | `wo2-recheck` | `internal/core/pipeline/verify/recheck.go`: re-check scheduler (§8). Fingerprint comparison, `suspect`/`conditions-changed` flagging. Closure condition evaluation (git + PA ledger + issue state daily). Rename detection for re-anchoring. | architect |
| 3 | `wo3-coverage` | `internal/core/metrics/coverage.go`: coverage model per subsystem (§9.1). Gap score computation. Gap aggregation from extraction. | standard |
| 4 | `wo4-elicitation` | Tick picks 1–3 targeted questions from highest gap scores. Written to the weekly review report and `QUESTIONS.md` view. Human answers → `confirmed` claims via answer events. | standard |
| 5 | `wo5-session-metrics` | Per-session signals (§10.1): rediscovery rate, repeated dead-end rate, push precision, tokens pushed, sidecar tokens, main-agent memory tokens, suppressed supports, interactive call rate. Computed after each episode. | standard |
| 6 | `wo6-replay-tuning` | Offline replay (§10.2): simulate delivery on episode prefixes. Label from suffix. Policy comparison. The dogfood corpus (PA and worker sessions from this programme) is used alongside the pilot. | architect |
| 7 | `wo7-report-cli` | `repo-memory report`: aggregate metrics from `metrics/*.jsonl`. Output: claim counts, delivery hit-rate, pipeline cost, coverage gaps. Also backs `memory.stats` detail. | standard |

### Exit Criteria
- Rediscovery and repeated dead-end rates reported weekly
- First policy update accepted by replay
- Coverage model produces meaningful gap scores
- Elicitation questions appear in review reports

---

## PR-09: PA Integration (cross-repo)

**SPEC Sections:** §13

**Objective:** Replace hand-written PA memory with generated episode summaries and task briefs, without breaking the PA skill for projects that don't use repo-memory. The generation features are built here; the PA change happens in the PA skill's own repository; the pilot migration happens in esp32-app-foundation. Whether PA workers also get hook delivery follows E8.

### Work Orders

| # | WO | Repository | Scope | Complexity |
|---|---|---|---|---|
| 1 | `wo1-episode-summaries` | repo-memory-plugin | `repo-memory summary <session>`: generated episode summaries (≤ 1.5k tokens): what a session attempted, outcomes, commits, claims produced. Replaces hand-written completion reports. | standard |
| 2 | `wo2-workorder-briefs` | repo-memory-plugin | `repo-memory brief --workorder <path>`: a brief formatted for embedding in or referencing from a WORKORDER. | standard |
| 3 | `wo3-pa-change-request` | mm-skills/principal-architect | Raise an issue/PR in the PA skill repository proposing a **compatibility mode**. When the repository is enrolled (`.memory/` present and listed by `repo-memory status`), the PA embeds generated briefs and reads generated summaries. Otherwise its behaviour is unchanged. Tested first on the dogfood repository. The change goes through that repository's own process (skill-manager); it is not edited from here. | architect |
| 4 | `wo4-esp32-migration` | esp32-app-foundation | After the PA change ships: PROJECT.md holds intent, plan, ledger and approvals only; knowledge indexes are replaced by generated views; existing index content is imported as episodes. Done as a PR in that repository. | standard |

### Exit Criteria
- PA compatibility mode merged in `mm-skills/principal-architect` and verified on a non-enrolled project (unchanged behaviour)
- Main-agent memory-writing tokens ≈ 0 for a full project cycle in esp32-app-foundation
- B4 shows lower total cost than PA hand-written memory

---

## PR-10: Full Backfill

**SPEC Sections:** §14, §9.3, §5.4

**Objective:** Import all remaining history. Inferred rationale pass. Re-projection tooling. Every subsystem has a page.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-full-transcript-import` | Import all surviving Antigravity transcripts. Process through the full pipeline. Throttled by daily budget. Highest churn first. | standard |
| 2 | `wo2-doc-import` | Import design spec decision log, ADRs, developer guides, `framework-expert` skill. Map to appropriate claim types. | standard |
| 3 | `wo3-git-import` | Import from git history: commit messages, PR descriptions, reverts as refutation evidence. | standard |
| 4 | `wo4-inferred-rationale` | For subsystems with no rationale coverage: one-off LLM pass to infer `decision` claims from code, commits, PR descriptions. Status: `hypothesis`, tagged `inferred`. | standard |
| 5 | `wo5-reprojection` | `repo-memory reproject`: re-run the pipeline with a new version over all events (episodes plus the PR-04 events store). Produce a diff delivered as a memory PR. Human review and answer events are re-applied by claim ID and content hash. | architect |

### Exit Criteria
- Every subsystem has a page
- Coverage report published
- Historical defects end as `superseded` or `refuted`, not open
- Re-projection preserves every human review decision

---

## Human Tasks

These are the user's tasks. They gate the critical path, so they are scheduled here rather than left implicit.

| When | Task | Needed by |
|---|---|---|
| PR-01b | Observe the interactive experiments (E1–E4, E6, E8, E9); run E5 on a Windows machine; approve the resulting SPEC edits | PR-02a start |
| PR-02b | Enable the `repo-memory` tick in the Antigravity **Automations** dashboard; confirm its permissions with a test run | PR-02b exit |
| PR-03 | Review the B2 question bank (≥ 50) and confirm the B3 task list and the B1 reference | PR-03 exit |
| PR-05b | Review the subsystem bootstrap proposal for esp32-app-foundation | PR-05b wo2 |
| PR-05b | Label the B5 sample: each extracted claim correct / wrong / not durable; list missed knowledge | P1 gate |
| PR-06 | Judge the B1 output against the PA's hand-written facts section | P1 gate |
| PR-06 onward | Review memory PRs; answer weekly review reports | Ongoing |
| PR-09 | Approve the PA compatibility-mode change in `mm-skills/principal-architect` | PR-09 |

---

## Risk Register

| Risk | Mitigation | Owner |
|---|---|---|
| B5 extraction precision < 0.8 at P1 exit | Defined fallback in SPEC §10.3: raise triage threshold, restrict push, review-all, iterate. Episode search keeps the system useful. | PR-05b |
| Truncated transcript fields hide errors and reasoning | Parser reads `transcript_full.jsonl` (verified complete); chunk fallback concatenated as a byte stream (PR-02a) | PR-02a |
| Housekeeping overhead dominates small calls | E10 results feed into batching and caching strategy. Per-day caps. | PR-01b, then PR-05a/PR-08 |
| Quota cannot sustain the pipeline | Preliminary E11 in the spike; daily budgets; usage tracked through PR-05b; backfill throttled | PR-01b, PR-05b |
| Hook latency budget exceeded | Go binary removes interpreter start-up. Probe estimate in the spike; confirmed at PR-07. | PR-01b, PR-07 |
| `gate-once` denials disrupt parallel tool batches or read as policy blocks | Restricted to irreversible actions; wording and batch behaviour tested in E9 | PR-01b, PR-07 |
| A global hook misbehaves and disrupts every agent session | Enrolment (no-op outside enrolled repos), kill switch, fail-open hooks | PR-01 |
| Capture reads sessions from unrelated projects | Sweep and hooks scoped to enrolled repositories | PR-01, PR-02b |
| Template-derived repositories collide in private storage | Repo ID includes the canonical origin URL; per-clone publishing worktrees | PR-01, PR-06 |
| Hook readers blocked by index writes, especially on Windows | WAL, `busy_timeout`, read-only readers, pointer-file swap for rebuilds | PR-04 |
| Compaction ratio < 4× | Tuning rules. Tool traffic is 70–90% of transcripts, which is removed. | PR-02a |
| Sidecar doesn't tick in CLI-only mode | E6 answers this before PR-02b. OS scheduler fallback. | PR-01b |
| Windows-specific failures (locking, rename, paths) | Portable lock; CI on Windows runners; E5 on Windows | PR-01, PR-02b |
| Antigravity update changes transcript or hook formats | Versioned fixtures and canary tests | PR-02a, PR-07 |
| Human tasks stall the critical path | Scheduled in Human Tasks with the PR that needs them | Programme |
| PA change disrupts other projects using the skill | Compatibility mode; change made through the PA repository's own process; tested on the dogfood repo first | PR-09 |
| Claims at scale cause GitHub UI issues | Month sharding keeps < 1k per directory. Verified in §5.3. | PR-04 |

---

## Decision Log

| # | Decision | Date | Rationale |
|---|---|---|---|
| PD1 | Decompose SPEC P0 across PR-01 (skeleton) and the capture PRs | 2026-10-01 | P0 is too large for a single PR. Skeleton is independently testable. |
| PD2 | Decompose SPEC P1 across PR-04 (data model), PR-05a/05b (pipeline and pilot), PR-06 (publish) | 2026-10-01 | P1 covers entities, 5 pipeline stages, views, publish, review and the pilot. |
| PD3 | ~~PR-11 branches off PR-02~~ → ~~PR-11 branches off PR-01 with per-experiment gates~~ → **Experiments become PR-01b, a docs-only spike straight after PR-01, complete before PR-02a** | 2026-10-01 | Review issue #1: a side branch that gates later PRs causes merge and rebase churn. The probe hook moves into PR-01, so the spike merges no code. |
| PD4 | Release 2 (Claude Code) excluded from this plan | 2026-10-01 | Per SPEC §15.2: starts after P2 (PR-07). Portability rules in §18.6 ensure R2 is an adapter. |
| PD5 | All PRs in a linear chain, except the PR-05b milestone running alongside PR-06 | 2026-10-01 | Each PR builds on the previous one's types and stores. The entity extractor (PR-04 wo2) may be started early if capacity allows. |
| PD6 | P1 gate split: B5 at PR-05b; B1 and B2 at PR-06; the whole gate passes before PR-07 merges | 2026-10-01 | B1 needs a brief and B2 needs ask over claims, built as core CLI features in PR-06. |
| PD7 | Local publish lane in the pipeline PR (PR-05a) | 2026-10-01 | Reconcile output needs somewhere to land within the pipeline PR. |
| PD8 | The programme spans three repositories; cross-repo changes go through each repository's own process | 2026-10-01 | The PA skill is a separate repository used by other projects; esp32 changes belong in esp32. |
| PD9 | Enrolment, kill switch, install lifecycle and CI in PR-01 | 2026-10-01 | A global plugin must act only on enrolled repositories from its first hook. |
| PD10 | No emergency transcript preservation ahead of capture | 2026-10-01 | User decision: transcripts have always been lost and critical repositories continue on PA memory meanwhile. Keep the plan straightforward. |
| PD11 | Split PR-02 into PR-02a (transcript → episode) and PR-02b (capture runtime) | 2026-10-01 | Review issue #1: 12 WOs was too many for one project; the transformation half is testable on fixtures alone. |
| PD12 | Split PR-05 into PR-05a (pipeline code) and PR-05b (pilot and B5 milestone, alongside PR-06) | 2026-10-01 | Review issue #1: live pilot evaluation shouldn't hold a code PR open or block PR-06. |
| PD13 | Merge small WOs so every PR has 4–7 work orders (PR-01, PR-07) | 2026-10-01 | Review issue #1: the PA workflow works best at 4–7 WOs per project. |
| PD14 | Enrol repo-memory-plugin itself as a dogfood repository from PR-02b | 2026-10-01 | Review issue #1: the programme's own PA and worker sessions form a large real corpus for PR-05 to PR-09. |
| PD15 | Adopt the verified runtime findings from review issue #1 (transcript file set, sidecar permissions and dashboard enablement, `model_decision` instead of `glob`, SQLite concurrency, template-safe repo ID) | 2026-10-01 | Each was checked against the built-in Antigravity docs or the transcripts on disk; see SPEC v0.8. Claims that didn't check out were not adopted. |
| PD16 | Read one Antigravity transcript source in priority order; no merge | 2026-10-01 | Review #1 follow-up, verified on all 537 conversations: chunks are 100 KiB byte slices identical to the monolithic files, and `transcript_full.jsonl` is complete. A merge was unnecessary, and per-chunk parsing would have broken on every boundary. |
