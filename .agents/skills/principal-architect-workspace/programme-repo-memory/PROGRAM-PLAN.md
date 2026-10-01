# repo-memory — Programme Plan

> **Programme:** repo-memory
> **Specification:** [SPEC.md](../../../SPEC.md) v0.7
> **Release:** 1 (Antigravity)
> **Created:** 2026-10-01 · **Revised:** 2026-10-01 (plan review, session 2)
> **Total PRs:** 11
> **Dependency model:** Linear chain (PR-01 → PR-10). PR-11 (experiments) branches off PR-01, and its results gate specific later PRs (see Experiment gates).

---

## Repositories in Scope

This programme spans three repositories. Work is done in the repository that owns it. Changes to another repository go through that repository's own process (issue or PR), never by direct edits from this programme.

| Repository | Owns | Programme work there |
|---|---|---|
| **repo-memory-plugin** (this repo) | SPEC, the `repo-memory` Go binary, Antigravity plugin packaging, CI and releases | PR-01 to PR-11, except as noted below |
| **esp32-app-foundation** (pilot) | Its `.memory/` store, `GEMINI.md` map include, PA workspace, existing `.agents/hooks.json` gates | Enrolment (`repo-memory init`) at PR-05; PA workspace migration at PR-09. Each is done as a PR in that repo. |
| **mm-skills/principal-architect** (`github.com/mm-skills/principal-architect`, managed via skill-manager) | The PA skill, which is used by other projects too | The PA change request at PR-09: raised as an issue/PR in that repo, with a compatibility mode so non-enrolled projects keep today's behaviour |

---

## Dependency Graph

```
PR-01  Go Skeleton, Interfaces, Enrolment & CI
  │
  ├──────────────────────────────────────────────┐
  ▼                                              ▼
PR-02  Capture  ◄── gate: E6                PR-11  Experiments (E1–E11)
  │                                              │  gates: E6 → PR-02
  ▼                                              │         E7, E10, E11 → PR-05
PR-03  Episode Search & Evaluation Harness       │         E1–E5, E9 → PR-07
  │                                              │         E8 → PR-09
  ▼
PR-04  Entities, Claims, Index & Events Store
  │
  ▼
PR-05  Pipeline + Local Publish  ◄── gates: E7, E10, E11      [B5 gate]
  │
  ▼
PR-06  Shared Publish, Views, Review, Brief/Ask Core           [P1 gate: B1, B2]
  │
  ▼
PR-07  Delivery  ◄── gates: E1–E5, E9
  │
  ▼
PR-08  Learning Loop
  │
  ▼
PR-09  PA Integration (cross-repo)  ◄── gate: E8
  │
  ▼
PR-10  Full Backfill
```

### Experiment Gates

| Experiment | Question (SPEC §12.1.1) | Gates | Why |
|---|---|---|---|
| E6 | Sidecar ticking (CLI-only, sleep/wake, overlap) | PR-02 | Decides the tick and lock design |
| E7 | Housekeeping run isolation | PR-05 | Decides executor flags and sweep exclusion |
| E10 | Housekeeping overhead reduction | PR-05 | Decides batching and packing in the cost model (§4.4.1) |
| E11 | Quota impact and rate limits | PR-05 | Decides daily budgets; may change pipeline viability |
| E1 | `ephemeralMessage` persistence | PR-07 | Injection and dedupe semantics |
| E2 | `@[…](path)` include resolution | PR-07 | Map include mechanism |
| E3 | Transcript currency at `PreInvocation` | PR-07 | Whether the hook can read the transcript tail |
| E4 | Glob / Model Decision rule activation | PR-07 | Native-floor path-scoped rules |
| E5 | Go hook latency on macOS and Windows | PR-07 | Confirms the §7.8 budget |
| E9 | `deny` + `reason` behaviour | PR-07 | Gate-once risky-action delivery |
| E8 | Subagent hook coverage | PR-09 | Whether PA workers get hook delivery or only WORKORDER briefs |

---

## PR-01: Go Skeleton, Interfaces, Enrolment & CI

**SPEC Sections:** §18.1–18.6, §4.5, §4.6, §5.6 (layout only)

**Objective:** Establish the Go module, binary entry point, subcommand wiring, package layout with the enforced core/adapter boundary, the harness interface types, and the `fake` test adapter. Add repository enrolment and the kill switch, so that a global plugin acts only on enrolled repositories. Add CI across macOS, Linux and Windows, and the install lifecycle. The binary compiles and responds to all subcommands; most are stubs or no-ops.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-go-module` | `go mod init`, `cmd/repo-memory/main.go`, subcommand wiring (cobra or manual), `Makefile`/`taskfile` for cross-compile (`CGO_ENABLED=0`). Verify all five targets cross-compile: `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64`. | standard |
| 2 | `wo2-package-layout` | Create all `internal/core/*` and `internal/harness/*` packages as empty stubs with package docs. Write the import-graph boundary test: `internal/core` must not import any concrete adapter. | standard |
| 3 | `wo3-harness-interfaces` | Define `Harness`, `Capabilities`, `TranscriptSource`, `HookCodec`, `Reporter`, `Housekeeper` interfaces in `internal/harness/`. Define normalised types: `Event`, `Injection`, `Step`. Define `Stage` enum for pipeline stages. | standard |
| 4 | `wo4-fake-adapter` | Implement `internal/harness/fake/`, a test adapter that satisfies all interfaces with in-memory state. Write the core test suite shell that runs against `fake`. | standard |
| 5 | `wo5-config-datadir` | `internal/core/config/`: load repo `config.yaml` (subsystems, budgets, trigger settings) and machine `<data-dir>/config.yaml` (`enabled`, `housekeeping.executor`, `reports.surface`, `tick.trigger`, budgets). `internal/core/config/datadir.go`: platform-appropriate data directory (`~/Library/Application Support/repo-memory` on macOS, etc.). Create the storage layout on first run. | standard |
| 6 | `wo6-subcommands-install` | Wire `hook`, `tick`, `mcp`, `install`, `uninstall`, `search`, `report`, `status`, `reindex`, `doctor` as subcommands; most print a stub message. `install --harness antigravity` generates plugin files from `packaging/antigravity/` templates with the absolute binary path. It is idempotent, so re-running upgrades the binary and regenerates the files. `uninstall` removes them. Install doesn't edit the user's global Antigravity `config.json`: it prints the snippet that enables the sidecar, or applies it only with `--enable-sidecar`. | standard |
| 7 | `wo7-enrolment-killswitch` | §4.6: `repo-memory init` (creates `.memory/`, `.gitignore`, the `GEMINI.md` include line, registry entry); `leave [--purge]`; `<data-dir>/registry.json` with repo ID (hash of root commit), roots and worktrees. Kill switch: `REPO_MEMORY_DISABLED=1`, `enabled: false`, `enable`/`disable [--repo]`. The hook framework checks the kill switch, then enrolment, before anything else, and returns no-op output for non-enrolled workspaces within 5 ms of process start. | standard |
| 8 | `wo8-ci-release` | GitHub Actions: build all five targets; run tests on macOS, Linux and Windows runners (including the boundary test); release artifacts per OS/arch with a version stamp (`repo-memory --version`). | standard |

### Exit Criteria
- `go build ./cmd/repo-memory` succeeds for all 5 targets
- `repo-memory status` runs and exits 0
- `repo-memory install --harness antigravity` generates correct `hooks.json`, `mcp_config.json`, `sidecar.json`; re-running upgrades in place; `uninstall` removes them
- `repo-memory init` enrols a repository; `leave` unenrols it
- A hook invoked in a non-enrolled workspace returns no-op output within 5 ms of process start; with the kill switch set, hooks no-op and `tick` exits
- Import-graph boundary test passes
- Core tests run against the `fake` adapter
- CI green on macOS, Linux and Windows runners

---

## PR-02: Capture

**SPEC Sections:** §4.1–4.3, §4.6, §5.1, §6.2–6.3, §11.3, §10.4 (metrics skeleton)

**Gate:** E6 answered (PR-11).

**Objective:** Every Antigravity session in an enrolled repository produces a compacted, scrubbed episode within minutes of going idle, correctly attributed to its repository, parent session and commits. The tick sidecar processes the spool and manages the job table. Claude Code transcripts are sweep-copied to `raw/` for future R2 processing.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-spool-format` | Define spool entry schema. Implement `spool/` writer: write to a uniquely named temp file, then rename, with Windows rename semantics verified. Implement spool reader for the tick. | standard |
| 2 | `wo2-stop-hook` | `internal/harness/antigravity/hooks.go`: handle `Stop` event. Write spool entry, copy transcript to `queue/`. Budget: ≤ 200 ms. `REPO_MEMORY_HOUSEKEEPING` environment guard. Enrolment and kill-switch checks come first (from PR-01). | standard |
| 3 | `wo3-transcript-parser` | `internal/harness/antigravity/transcript.go`: parse Antigravity `transcript.jsonl` into `[]episode.Step`. Handle `CHECKPOINT` steps, step gaps, `source`/`type` classification. Scrubbed fixture corpus under `testdata/fixtures/antigravity/<version>/`. Canary test: parse current real transcripts and flag drops in parsed-step counts when the Antigravity version changes. | architect |
| 4 | `wo4-compaction` | `internal/core/pipeline/compact/`: deterministic compaction per §5.1 rules. Author classification (`human` vs `automation`). Exposure markers. Secret scrubber (pattern + entropy). Output: episode JSONL. Target: ≥ 4× reduction. | architect |
| 5 | `wo5-episode-store` | `internal/core/episode/store.go`: write episodes as `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst`. Episode header schema, including `harness`, `harness_version`, `started_by`, `pipeline_version`. Gap recording. Episodes are never pruned (they are the event log for re-projection). | standard |
| 6 | `wo6-job-table` | `internal/core/jobs/`: SQLite job table (§4.3). Schema: `id`, `kind`, `priority`, `idempotency_key`, `state`, `lease_expires`, `attempts`, `next_attempt_at`, `cost`. State machine: `queued` → `running` → `done` / `failed` → `dead`. Lease recovery. | standard |
| 7 | `wo7-tick-core` | `internal/core/tick/`: the tick loop per §4.3. `tick.lock` as a **portable** file lock: `flock` on Unix, `LockFileEx` on Windows. Spool ingestion → job creation. Lease reclaim. Sweep transcripts for **enrolled repositories only** (Antigravity app and CLI stores, plus the Claude Code raw copy). Idle promotion (15 min). Budget enforcement. Overlap behaviour per E6. | architect |
| 8 | `wo8-session-attribution` | Map conversations to enrolled repositories via `workspace_uris` in both `conversation_summaries.db` stores (app and CLI). De-duplicate conversations present in both stores. Link subagent sessions to parents via `parent_conversation_id`, which extraction packing needs. | standard |
| 9 | `wo9-commit-linkage` | §6.2: link commits made on the session's branch between session start and end from `git log`/reflog, confirmed against commit commands in the transcript. Record them in the episode header. | standard |
| 10 | `wo10-antigravity-sidecar` | `packaging/antigravity/sidecars/tick/sidecar.json`: `builtin: schedule`, `* * * * *`. Update `install` to generate it. | mechanical |
| 11 | `wo11-metrics-skeleton` | `internal/core/metrics/`: JSONL metrics writer. Record: capture health (sessions seen, captured, skipped, missed), hook execution time, tick duration, budget exhaustion. | standard |
| 12 | `wo12-claude-raw-copy` | Sweep extension: copy Claude Code transcripts for enrolled repositories from `~/.claude/projects/` to `<data-dir>/<repo-id>/raw/claude-code/`. No parsing. Respects the same permissions. | mechanical |

### Exit Criteria
- Every new Antigravity session in an enrolled repository produces an episode within minutes of going idle
- Sessions from non-enrolled workspaces are never read or copied
- Subagent sessions are linked to their parents; app/CLI duplicates are removed; commits are linked
- Share caught only by sweep: measured
- Compaction ratio ≥ 4× measured
- Zero secrets in a scan of 50 episodes
- `tick.lock` verified on macOS and Windows
- Core tests pass on the `fake` adapter
- `internal/core` imports no concrete adapter

---

## PR-03: Episode Search & Evaluation Harness

**SPEC Sections:** §15.1 P0.5, §10.3

**Objective:** Make captured history searchable before the knowledge pipeline exists, and import PA files as searchable episodes. Build the evaluation harness and datasets (B1–B5) that later PRs are gated on, and measure the episode-search baseline for B2.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-fts-index` | SQLite FTS5 index over episodes. Columns: `episode_id`, `session_id`, `content` (compacted text), `files_touched`, `outcomes`, `date`. Populate from the episode store. | standard |
| 2 | `wo2-search-cli` | `repo-memory search <query>`: full-text search over episodes. Output: episode ID, date, branch, snippet, files. Rank by BM25. | standard |
| 3 | `wo3-pa-import` | Import PA `PROJECT.md`, `WORKORDER.md`, `context_bridge.md` files as episodes with `harness: "import"`. Date from file or git history. One episode per file. | standard |
| 4 | `wo4-ask-fallback` | `memory.ask` MCP tool: when no claims exist yet, fall back to FTS over episodes. Mark results as "unverified (episode search)". | standard |
| 5 | `wo5-eval-harness` | `repo-memory eval <benchmark>`: runner and result store for B1–B5 (§10.3). Dataset formats. Run the **B2 baseline** against episode search. | standard |
| 6 | `wo6-eval-datasets` | **B5:** a sampling tool that selects ≥ 20 OTA/BLE episodes and produces a labelling sheet (correct / wrong / not durable, plus missed knowledge). **B2:** a question bank of ≥ 50 questions drawn from session logs and ADRs. **B1:** the cut-off at commit `69d9c813` and the PA's hand-written "Framework Facts Workers Need" section as the reference. **B3:** 3–5 historical tasks with starting commits. The datasets are drafted by the worker and reviewed by the user (see Human Tasks). | standard |

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
| 3 | `wo3-claim-schema` | `internal/core/claims/`: claim file parser and writer. YAML frontmatter + Markdown body. All fields from §5.3: `id`, `type`, `title`, `relevant_when`, `apply`, `status`, `flags`, `anchors`, `conditions`, `evidence`, `relations`, `scope`, `verified_at`, `provenance` (including `pipeline_version`), `review`, `match` blocks, `closes_when`. ULID generation. | standard |
| 4 | `wo4-claim-store` | `.memory/claims/<yyyy-mm>/` layout. CRUD operations. Month-sharded writes. Relation index (inverse lookups). | standard |
| 5 | `wo5-sqlite-index` | `internal/core/index/`: rebuildable SQLite index in `.memory/.cache/index.sqlite`. Tables: entities, claims, anchors, relations, conditions. Built from claim files, the entity registry and events. `repo-memory reindex` command. | standard |
| 6 | `wo6-subsystem-config` | `.memory/config.yaml` subsystem configuration. Glob matching for file→subsystem assignment. Unassigned file detection. Subsystem bootstrap proposal (one-off LLM pass, deferred to PR-05). | standard |
| 7 | `wo7-events-store` | §5.4: `.memory/events/reviews/*.jsonl` and `.memory/events/answers/*.jsonl`. Schema keyed by claim ID **and** content hash, so human decisions survive re-projection. Writers and readers used by PR-06; replay order documented for PR-10. | standard |

### Exit Criteria
- Entity extractor produces correct entities from the esp32-app-foundation structure (or test fixture)
- Claim files round-trip (write → read → write produces identical output)
- Index is rebuildable from claim files, entities and events
- `repo-memory reindex` works
- All entity kinds from §5.2 are extractable
- A review event recorded against a claim still applies after that claim's file is regenerated (round-trip test)

---

## PR-05: Pipeline + Local Publish

**SPEC Sections:** §6.4–6.7, §6.8 (local lane), §6.9, §4.4.1

**Gates:** E7, E10 and E11 answered (PR-11).

**Objective:** The knowledge pipeline: triage, extract, reconcile and verify, plus the local publish lane so that reconciled claims land in the store and are usable at once. Each LLM stage is a pure function dispatched to `agy -p`. Enrol the pilot repository and run its OTA and BLE history through the pipeline for the B5 gate.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-agy-executor` | `internal/harness/antigravity/executor.go`: `Housekeeper` implementation. Invoke `agy -p` with `--model`, `--json-schema`, `--output-format json`, `--input-format stream-json`, `--print-timeout`. Parse JSON response and `usage`. Set `REPO_MEMORY_HOUSEKEEPING=1`. Working dir: `<data-dir>/runs/<repo-id>/`; dedicated project per E7. **Capture sweep excludes housekeeping runs** by project and workspace. | standard |
| 2 | `wo2-recorded-executor` | A test `Housekeeper` that replays recorded `agy` responses (cassettes) keyed by stage, inputs and pipeline version. All stage tests run against it, so they are deterministic and cost nothing. | standard |
| 3 | `wo3-triage` | `internal/core/pipeline/triage/`: rule-based triage (§6.4). Keep/skip rules. Salience window selection. Synopsis generation. Small-model fallback for ambiguous episodes. Prompt and JSON schema in `prompts/triage/`. | architect |
| 4 | `wo4-extract` | `internal/core/pipeline/extract/`: extraction stage (§6.5). Pack episodes to the 60k input cap (parent with subagents first). 20-candidate-per-episode limit. Prompt enforces all extraction rules. Output: candidate claims with anchors, evidence, `gaps`. Evidence independence (P13). `prompts/extract/`. | architect |
| 5 | `wo5-reconcile` | `internal/core/pipeline/reconcile/`: reconciliation (§6.6). Look up existing claims by anchor (bounded to 30). Stronger model. 7 operations: create, support, refine, contradict, supersede, promote, discard. P13 enforcement. Single-writer constraint. `prompts/reconcile/`. | architect |
| 6 | `wo6-verify` | `internal/core/pipeline/verify/`: verification (§6.7). Anchor resolution and fingerprinting. Behavioural check (small model on code snippets). Branch scoping rules, including merge promotion and abandoned-branch promotion of ruled-out claims. | standard |
| 7 | `wo7-local-publish` | `internal/core/pipeline/publish/local.go` (moved from PR-06): write new/changed claims to `<data-dir>/<repo-id>/pending/`. Index immediately. Mark as `unreviewed`. Security and external-provenance gating. | standard |
| 8 | `wo8-subsystem-bootstrap` | One-off LLM pass to propose subsystems and globs from directory structure and existing docs. Output: proposed `config.yaml` for human review. | standard |
| 9 | `wo9-pipeline-orchestration` | Wire stages into the tick's job processing. Respect concurrency rules (extraction parallel; reconcile and publish single-writer). Idempotency keys per stage. Cost recording per stage. Daily budgets from E11. | standard |
| 10 | `wo10-pilot-ota-ble` | Enrol esp32-app-foundation (`repo-memory init`, as a PR in that repo), apply the reviewed subsystem config, and run its OTA and BLE history (PA imports from PR-03 plus surviving transcripts) through the pipeline. | standard |

### Exit Criteria
- **Gate: B5** precision ≥ 0.8 for pushable claims (verified or higher), using the user's labels on the PR-03 sample
- Reconciled claims land in the local lane and are queryable from the index
- Pipeline cost per episode and per stage measured (compare with §4.4.1)
- Duplicate jobs are no-ops via idempotency keys; all stage tests pass against recorded executor responses

---

## PR-06: Shared Publish, Views, Review & Brief/Ask Core

**SPEC Sections:** §5.5, §6.8 (shared lane), §7.2 (brief generation), §9.2.1, §11.4, §10.3 (B1, B2)

**Objective:** The shared publish lane (memory PR), generated views, the lint gate, weekly review reports with reply parsing, and review profiles. Human decisions are written as events. Build the core task-brief generator and claims-based ask (exposed on the CLI) so that the P1 gate (B1, B2) can be run here; PR-07 reuses them for hooks and MCP.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-shared-publish` | `internal/core/pipeline/publish/shared.go`: batch pending ops into a memory PR on `memory/pending` branch. Git worktree operations (`<data-dir>/<repo-id>/worktree/`). PR description grouped by subsystem. Never commit to the user's branch. | architect |
| 2 | `wo2-views` | `internal/core/views/`: generate MAP.md (≤ 2k tokens), claim indexes per subsystem (≤ 1.5k tokens), subsystem pages (≤ 5k tokens), SIGNATURES.md, DECISIONS.md. Budget enforcement with re-synthesis. Regenerate on claim/entity changes. | architect |
| 3 | `wo3-lint-gate` | CI lint gate for `.memory/`: schema validation, views match regeneration, every claim has ≥ 1 evidence, every anchor resolves or flagged, no secrets. `repo-memory lint` command. | standard |
| 4 | `wo4-review-profiles` | `config.yaml` → `review.profile`: `solo` (default) and `team` profiles per §6.8. Auto-merge rules per operation type. Every review decision (auto or human) is written as a review event (PR-04 events store). | standard |
| 5 | `wo5-review-report` | Weekly review report via `agentapi new-conversation`. Reply parsing (§9.2.1): explicit command parsing, conservative free-text interpretation. Author classification. Human replies → `human_answers` events and review events in the events store. | architect |
| 6 | `wo6-brief-ask-core` | `internal/core/delivery/brief`: task-brief generator (§7.2 priority order, budgets, claim-index fallback). Claims-based answer synthesis for ask. Exposed as `repo-memory brief "<task>"` and `repo-memory ask "<question>"`. PR-07 reuses both for hooks and MCP. | architect |

### Exit Criteria
- **P1 gate, B1:** a fresh agent given pr-225's original request plus `repo-memory brief` output (claim store cut at `69d9c813`) produces facts at parity with the PA's hand-written section, as judged by the user
- **P1 gate, B2:** `repo-memory ask` beats the PR-03 episode-search baseline on the question bank
- Memory PR is generated with correct grouping and evidence links
- Views regenerate correctly and stay within budget
- Lint gate catches: missing evidence, hand-edited views, unresolved anchors, secrets
- Review report parses `approve clm-…` and `Q-017: <answer>` commands; decisions are stored as events
- Review profiles apply correctly

---

## PR-07: Delivery

**SPEC Sections:** §7, §18.3 (capability-based policy), §18.5

**Gates:** E1, E2, E3, E4, E5 and E9 answered (PR-11).

**Objective:** The read side: hooks deliver knowledge at task start, file touch, error, risky action, and déjà-vu tier 0/1. Pull tools provide on-demand access. Performance within §7.8 budgets.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-delivery-engine` | `internal/core/delivery/`: normalised delivery engine. Capability-based policy (§18.3). Delivery ledger (per-session, per-claim). Token budgets (§7.1). Rendering: `apply` line + ID + currency stamp. Anchor-existence fast check. Reuses the PR-06 brief generator. | architect |
| 2 | `wo2-preinvocation` | `PreInvocation` hook handler: fast path (p95 ≤ 60 ms). Check `alerts/`, read transcript tail from cursor (per E3). `invocationNum == 0`: task brief. Touch and error injection. Déjà-vu tier 1 on reasoning. Spool tier-2 requests. | architect |
| 3 | `wo3-pretooluse` | `PreToolUse` hook handler: risky-action gates (`gate-once`, `ask`, `next-invocation`) per E9. Déjà-vu tier 1 on concrete tool calls. Coexistence with existing gates (e.g. esp32's `flash-tag-gate`). | standard |
| 4 | `wo4-posttooluse` | `PostToolUse` hook handler: detect git merge/checkout/pull/rebase/dependency update from transcript step at `stepIdx`. Spool re-check request. | mechanical |
| 5 | `wo5-map-include` | `GEMINI.md` include of `.memory/views/MAP.md` (written by `init`, per E2). Fallback: `PreInvocation` injection at `invocationNum == 0`. | mechanical |
| 6 | `wo6-pull-tools` | `internal/mcpserver/`: stdio MCP server. All six tools: `memory.ask`, `memory.why`, `memory.history`, `memory.brief`, `memory.note`, `memory.stats`. `ask` and `brief` reuse the PR-06 core and may invoke synchronous housekeeping runs. Instrument all calls. | architect |
| 7 | `wo7-rules-generation` | Generate `rules/AGENTS.md` for the plugin: how to use memory tools, walk MAP → claim index, currency stamp semantics. Generate path-scoped rules (per E4). | standard |
| 8 | `wo8-degraded-modes` | Test and implement all degraded modes from §7.9. Graceful fallback at each level. | standard |
| 9 | `wo9-hook-fixtures` | Versioned Antigravity hook-payload fixtures under `testdata/fixtures/antigravity/<version>/`; the adapter contract suite (§18.6 rule 7) runs against them. | standard |

### Exit Criteria
- B3 task replays run (PR-03 task list)
- Per-session push within budget (20k tokens)
- Hook latency: fast path p95 ≤ 60 ms, injection p95 ≤ 150 ms (macOS and Windows)
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
| 6 | `wo6-replay-tuning` | Offline replay (§10.2): simulate delivery on episode prefixes. Label from suffix. Policy comparison. | architect |
| 7 | `wo7-report-cli` | `repo-memory report`: aggregate metrics from `metrics/*.jsonl`. Output: claim counts, delivery hit-rate, pipeline cost, coverage gaps. Also backs `memory.stats` detail. | standard |

### Exit Criteria
- Rediscovery and repeated dead-end rates reported weekly
- First policy update accepted by replay
- Coverage model produces meaningful gap scores
- Elicitation questions appear in review reports

---

## PR-09: PA Integration (cross-repo)

**SPEC Sections:** §13

**Gate:** E8 answered (PR-11).

**Objective:** Replace hand-written PA memory with generated episode summaries and task briefs, without breaking the PA skill for projects that don't use repo-memory. The generation features are built here. The PA change happens in the PA skill's own repository. The pilot migration happens in esp32-app-foundation.

### Work Orders

| # | WO | Repository | Scope | Complexity |
|---|---|---|---|---|
| 1 | `wo1-episode-summaries` | repo-memory-plugin | `repo-memory summary <session>`: generated episode summaries (≤ 1.5k tokens): what a session attempted, outcomes, commits, claims produced. Replaces hand-written completion reports. | standard |
| 2 | `wo2-workorder-briefs` | repo-memory-plugin | `repo-memory brief --workorder <path>`: a brief formatted for embedding in or referencing from a WORKORDER. | standard |
| 3 | `wo3-pa-change-request` | mm-skills/principal-architect | Raise an issue/PR in the PA skill repository proposing a **compatibility mode**. When the repository is enrolled (`.memory/` present and listed by `repo-memory status`), the PA embeds generated briefs and reads generated summaries. Otherwise its behaviour is unchanged. Whether workers also get hook delivery follows E8. The change goes through that repository's own process (skill-manager); it is not edited from here. | architect |
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

## PR-11: Experiments

**SPEC Sections:** §12.1.1

**Objective:** Answer experiments E1–E11 early enough to shape the PRs they gate (see Experiment Gates). Branches off PR-01, because a logging hook is all most experiments need.

### Work Orders

| # | WO | Scope | Gates | Complexity |
|---|---|---|---|---|
| 0 | `wo0-probe-hook` | `repo-memory hook antigravity <event> --probe`: records the hook payload and a transcript-tail snapshot to a probe log. Used by E1, E3, E8 and E9. | — | standard |
| 1 | `wo1-e1-ephemeral` | E1: Does `ephemeralMessage` persist in context and appear in the transcript? | PR-07 | interactive |
| 2 | `wo2-e2-includes` | E2: Do `@[…](path)` includes in `GEMINI.md`/`AGENTS.md` resolve and refresh? | PR-07 | interactive |
| 3 | `wo3-e3-transcript-currency` | E3: Is `transcript.jsonl` current when `PreInvocation` fires? | PR-07 | interactive |
| 4 | `wo4-e4-rule-activation` | E4: Are Glob and Model Decision rule activations supported? | PR-07 | interactive |
| 5 | `wo5-e5-latency` | E5: Delivery hook latency of the Go binary on macOS and Windows. | PR-07 | interactive |
| 6 | `wo6-e6-sidecar-behaviour` | E6: Sidecar ticking with CLI-only, sleep/wake, overlap handling. | PR-02 | interactive |
| 7 | `wo7-e7-isolation` | E7: Housekeeping run isolation (`--project`, `--continue`, history). | PR-05 | interactive |
| 8 | `wo8-e8-subagent-hooks` | E8: Do subagent conversations fire their own hooks? | PR-09 | interactive |
| 9 | `wo9-e9-deny-behaviour` | E9: How does `PreToolUse` `deny` + `reason` behave in practice? | PR-07 | interactive |
| 10 | `wo10-e10-overhead` | E10: Can the ~17k-token overhead be reduced? Cache prefix behaviour. | PR-05 | interactive |
| 11 | `wo11-e11-quota` | E11: How do housekeeping runs count against quota? Rate limits. | PR-05 | interactive |

### Exit Criteria
- Each experiment has a documented answer with evidence, recorded before the PR it gates starts
- Results feed into the design of the gated PRs (PR-02, PR-05, PR-07, PR-09)

---

## Human Tasks

These are the user's tasks. They gate the critical path, so they are scheduled here rather than left implicit.

| When | Task | Needed by |
|---|---|---|
| PR-03 | Review the B2 question bank (≥ 50) and confirm the B3 task list and the B1 reference | PR-03 exit |
| PR-05 | Review the subsystem bootstrap proposal for esp32-app-foundation | PR-05 wo8/wo10 |
| PR-05 | Label the B5 sample: each extracted claim correct / wrong / not durable; list missed knowledge | PR-05 gate |
| PR-06 | Judge the B1 output against the PA's hand-written facts section | PR-06 gate |
| PR-06 onward | Review memory PRs; answer weekly review reports | Ongoing |
| PR-11 | Observe the interactive experiments (E1, E3, E4, E6, E8, E9); run E5 on a Windows machine | The PRs they gate |
| PR-09 | Approve the PA compatibility-mode change in `mm-skills/principal-architect` | PR-09 |

---

## Risk Register

| Risk | Mitigation | Owner |
|---|---|---|
| B5 extraction precision < 0.8 at P1 exit | Defined fallback in SPEC §10.3: raise triage threshold, restrict push, review-all, iterate. Episode search keeps the system useful. | PR-05 PA |
| Housekeeping overhead dominates small calls | E10 results feed into batching and caching strategy. Per-day caps. | PR-11, then PR-05/PR-08 |
| Quota cannot sustain the pipeline | E11 before PR-05; daily budgets; backfill throttled | PR-11, then PR-05 |
| Hook latency budget exceeded | Go binary removes interpreter start-up. E5 benchmarks before delivery is built. | PR-11, then PR-07 |
| A global hook misbehaves and disrupts every agent session | Enrolment (no-op outside enrolled repos), kill switch, fail-open hooks | PR-01 |
| Capture reads sessions from unrelated projects | Sweep and hooks scoped to enrolled repositories | PR-01/PR-02 |
| Compaction ratio < 4× | Tuning rules. Tool traffic is 70–90% of transcripts, which is removed. | PR-02 PA |
| Sidecar doesn't tick in CLI-only mode | E6 answers this before PR-02. OS scheduler fallback. | PR-11 |
| Windows-specific failures (locking, rename, paths) | Portable lock; CI on Windows runners; E5 on Windows | PR-01/PR-02 |
| Antigravity update changes transcript or hook formats | Versioned fixtures and canary tests | PR-02/PR-07 |
| Human tasks stall the critical path | Scheduled in Human Tasks with the PR that needs them | Programme |
| PA change disrupts other projects using the skill | Compatibility mode; change made through the PA repository's own process | PR-09 |
| Claims at scale cause GitHub UI issues | Month sharding keeps < 1k per directory. Verified in §5.3. | PR-04 |

---

## Decision Log

| # | Decision | Date | Rationale |
|---|---|---|---|
| PD1 | Decompose SPEC P0 across PR-01 (skeleton) and PR-02 (capture) | 2026-10-01 | P0 is too large for a single PR. Skeleton is independently testable. |
| PD2 | Decompose SPEC P1 across PR-04 (data model), PR-05 (pipeline), PR-06 (publish) | 2026-10-01 | P1 covers entities, 5 pipeline stages, views, publish, and review — too much for one PR. |
| PD3 | ~~PR-11 branches off PR-02~~ **Revised:** PR-11 branches off PR-01, and individual experiments gate the PRs whose design they decide | 2026-10-01 | A probe hook is enough for most experiments; E6, E7, E10 and E11 must be answered before PR-02 and PR-05 are designed, not after. |
| PD4 | Release 2 (Claude Code) excluded from this plan | 2026-10-01 | Per SPEC §15.2: starts after P2 (PR-07). Portability rules in §18.6 ensure R2 is an adapter. |
| PD5 | All PRs in a linear chain (except PR-11) | 2026-10-01 | Each PR builds directly on the previous one's types and stores. Parallel PRs would cause constant merge conflicts in shared packages. The entity extractor (PR-04 wo2) may be started early if capacity allows. |
| PD6 | P1 gate split: B5 at PR-05; B1 and B2 at PR-06 | 2026-10-01 | B1 needs a brief and B2 needs ask over claims; both are built as core CLI features in PR-06 and reused by PR-07. |
| PD7 | Local publish lane moved from PR-06 to PR-05 | 2026-10-01 | Reconcile output needs somewhere to land within the pipeline PR. |
| PD8 | The programme spans three repositories; cross-repo changes go through each repository's own process | 2026-10-01 | The PA skill is a separate repository used by other projects; esp32 changes belong in esp32. |
| PD9 | Enrolment, kill switch, install lifecycle and CI in PR-01 | 2026-10-01 | A global plugin must act only on enrolled repositories from its first hook; a kill switch is the safety valve for hooks that run on every model call. |
| PD10 | No emergency transcript preservation ahead of PR-02 | 2026-10-01 | User decision: transcripts have always been lost and critical repositories continue on PA memory meanwhile. Keep the plan straightforward. |
