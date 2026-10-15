# Repo Memory: Specification

| | |
|---|---|
| **Status** | **v1.0** (`v2.2` Architecture — see Revision history) |
| **Date** | 2026-10-15 |
| **Scope** | Repository-level memory for coding agents. **Release 1: Antigravity. Release 2: Claude Code.** Text marked **[R2]** belongs to Release 2; Release 1 is built so that Release 2 is an adapter, not a rewrite (§18.6). |
| **Implementation** | One Go binary, `repo-memory` (§18) |
| **Reference repos** | `esp32-app-foundation` (Appendix A) and `repo-memory-plugin` (dogfood & spike corpus) |
| **Supersedes** | `repo-memory-plugin-architecture-analysis.md` (annotated critique kept for the record), `SPEC.md Draft v0.10` (v1 4-stage atomic-claim pipeline) |
| **Companion docs** | [v2.2 Design (Revision 4)](docs/design-v2-distillation-retrieval.md) · [Addendum A: Provenance, Sharing and Hardening (Revision 3)](docs/addendum-a-provenance-and-hardening.md) · [Antigravity Hooks Reference](docs/antigravity-hooks-reference.md) |

---

## 1. Purpose

An agent starting work on a large, long-lived repository should behave as if it had been on the project from day one. It should know how each part of the system works and why. It should know what invariants must hold and what has already been tried and failed. It should know what a given error usually means and how to verify a change on real hardware.

This specification defines a memory system that provides that knowledge:

- **automatically**, with **zero user or agent cooperation required**: end-of-turn notes are requested and harvested automatically via the `Stop` hook while the 5-minute prompt cache is warm, and platform `CHECKPOINT` summaries and `brain/` artifacts are harvested at zero token cost;
- **efficiently**, delivering only what the current task, file, error or action needs and keeping persistent knowledge in context until compaction;
- **accurately**, keeping each entry tied to the code anchors, execution outcomes and evidence it depends on;
- **completely**, covering every part of the system — including curated developer docs and stable subsystems nobody has touched for months.

### 1.1 Goals

| # | Goal |
|---|---|
| G1 | **Zero user or agent cooperation required.** Neither the user nor the agent needs to remember to invoke a tool to record or retrieve memory. Capture and distillation run automatically via the milestone-gated `Stop` hook (`NO_TOOL_CALL`, warm 5-minute cache), free `CHECKPOINT`/`brain/` harvest, and the `repo-memory-distiller` sidecar fallback. (`memory_note` is available strictly as an optional mid-turn override.) |
| G2 | Every subsystem in the repo has memory coverage, whether from agent sessions, PA work orders (`seed`), or curated developer documentation (`seed-docs`). |
| G3 | Knowledge is delivered proactively at the moment of need (`task-start`, `first-entry`, `first-touch`, `about-to-edit`, `error-match`, `gate-once`), not only when an agent thinks to pull it. |
| G4 | An entry stays valid according to its cognitive lifecycle (`semantic` by code anchors, `procedural` by command execution outcomes and build manifests, `episodic` until superseded). Age and lack of use never retire knowledge. |
| G5 | Trust is earned from outcomes (tests, builds, flash logs, merges, verified `HEAD` anchors, human confirmation), never from retrieval frequency or uncorroborated self-rated confidence. |
| G6 | Memory is event-sourced and rebuildable. Dossiers, `ATLAS.md`, graduated `.agents/skills/rm-<slug>/SKILL.md` files, and the local SQLite index (`index.db`) are deterministic projections of the append-only op log (`.memory/ops/`). |
| G7 | Knowledge that lives in git is human-readable as cohesive subsystem dossiers (`.memory/dossiers/*.md`), reviewable per op, and conflict-free across machines and branches (`.memory/ops/<yyyy-mm>/<ulid>.json`). |
| G8 | The system measures its own effectiveness using paired, pre-registered evaluations (Addendum A §9) and per-session telemetry. |
| G9 | Existing artefacts (PA workspaces, curated developer guides and skills via `seed-docs`, ADRs, design specs, commit history) are ingested into the same op-log and dossier pipeline. |

### 1.2 Non-goals

- Generic chat or personal-assistant memory across unrelated projects.
- Replacing git, the issue tracker or human-facing documentation. Memory distills and anchors documentation (`seed-docs`, §6.3) and can feed it, but doesn't replace it.
- A graph database, cloud vector store or any server infrastructure.
- Autonomous commits to the user's working branches.
- Storing what can be derived cheaply from the code at read time (structure, signatures, call graphs).

---

## 2. Design principles

| # | Principle | Consequence |
|---|---|---|
| P1 | **Zero cooperation required; warm-cache distillation.** | Normal turns are never interrupted. At the end of a milestone turn (`terminationReason == "NO_TOOL_CALL"`, `fullyIdle == true`, `executionNum == 0`), the `Stop` hook requests a short structured note while the working agent's 5-minute prompt cache is warm (~90% input token discount), and harvests it on the second `Stop`. Free `CHECKPOINT` summaries and `brain/` artifacts are harvested at zero token cost, and `repo-memory-distiller` covers any missed sessions asynchronously in `tick`. |
| P2 | **Event-sourced knowledge.** | Turn notes, `CHECKPOINT` summaries, artifacts, `seed-docs` extracts, PA reports, commits, and human reviews feed an append-only **op log** (`.memory/ops/<yyyy-mm>/<ulid>.json`). Dossiers, `ATLAS.md`, graduated skills, and `index.db` are deterministic projections of that log. |
| P3 | **Store only what the code can't tell you.** | Memory holds invariants, contracts, gotchas, failure signatures, verified recipes, architectural decisions and ruled-out dead ends. Structure is read from the code. |
| P4 | **Cohesive subsystem dossiers rendered from typed ops across three cognitive lifecycles.** | Instead of one file per atomic claim, knowledge is organised into cohesive markdown **dossiers** (`.memory/dossiers/<scope>.md`, ≤ 3k tokens, with optional `{section?}` resolution) rendered from typed ops (`add`, `amend`, `supersede`, `retire`, `move`, `split`). The 8 entry types are governed by three formal cognitive lifecycles: **`semantic`** (`invariant`, `contract`, `gotcha`, `open-issue`), **`procedural`** (`recipe`, `failure-signature`), and **`episodic`** (`decision`, `ruled-out`). |
| P5 | **Truth from outcomes (Pre-Consolidation Corroboration Gate).** | Candidate `semantic` and `procedural` rules with `verified: null` must be corroborated by a passing `PostToolUse` build/test step, a verified static anchor in `HEAD`, or explicit `USER_INPUT` confirmation before becoming active imperatives. Uncorroborated assertions route to `open-issue`. |
| P6 | **Code is the index.** | Dossier scopes and entry anchors are keyed on file paths, symbols, config keys, commands and error patterns — never on unanchored embedding similarity. |
| P7 | **Validity follows lifecycle dependencies.** | `semantic` entries are checked against file/symbol fingerprints at collection and before every push; `open-issue` entries close on deterministic `closes_when` conditions; `procedural` recipes track `PostToolUse` exit codes (`success_count` / `failure_count`) and build-manifest changes; `episodic` entries remain as historical fact until superseded. |
| P8 | **Push, with a strict budget and channel lifetime.** | Conversation-spanning knowledge (`task-start`, `first-entry`, `first-touch`, `re-injection`) is injected persistently (`userMessage` on `agy 1.3.3` / `systemMessage` where supported) until compaction and deduplicated via `InjectedSet`. Immediate 1-step hints (`about-to-edit`, `error-match`) use `ephemeralMessage`. |
| P9 | **Humans are the highest authority.** | Human statements and memory-PR reviews carry the highest trust. Hand edits to rendered dossiers are converted into ops on the next tick. |
| P10 | **Harness-agnostic core, thin adapters, workspace-isolated packaging.** | Stores, op logs, dossiers and SQLite (`index.db`) live in `internal/core`. Antigravity (Release 1) and Claude Code (Release 2) sit behind `internal/harness`. Packaging splits the global sidecar (`repo-memory-daemon`) from the workspace-scoped hooks/MCP bundle (`repo-memory`, enabled via `.agents/plugins.json`), guaranteeing zero overhead in unenrolled repositories. |
| P11 | **Never slow the loop.** | Hooks run synchronously and block the agent. Hook commands only read precomputed files and `index.db`, check anchors on the filesystem, and write spool/mark files. Consolidation runs in `tick`. |
| P12 | **Five-tier state scope hierarchy; review gates sharing, not local use.** | Ephemeral (`temp:`), conversation (`session:`), personal developer (`user:`), and unmerged branch (`branch:` in `.memory/local/`) state is usable immediately on the developer's machine. Shared repository truth (`repo:` in `.memory/ops/` and `.memory/dossiers/`) goes through the memory PR. |
| P13 | **Evidence must be independent of the memory it supports.** | Every conversation's `ExposureList` records all entries pushed to or pulled by that session. Consolidation's deterministic echo filter ignores any note item repeating an exposed entry unless the session modified an anchored file and passed a subsequent build, test or check. |
| P14 | **Push-first delivery; pull for depth.** | Empirical audits of 231 PA sessions (`1.3%` `MEMORY.md` read rate, `0` MCP pull calls) and 77 `esp32` sessions (`1.3%–7.8%` static doc read rate) prove agents rarely pull memory unprompted. Critical constraints, gotchas and failure signatures must be pushed via hooks when files or errors are encountered; MCP pull tools (`memory_brief`, `memory_why`, `memory_history`, `memory_search`) provide deeper context on demand. |

---

## 3. Terminology

| Term | Meaning |
|---|---|
| **Session** | One agent conversation, including any subagent sessions it spawned (`state/subagents.sqlite` maps child conversations to parents). |
| **Raw transcript** | The harness's native record of a session (`transcript_full.jsonl` on Antigravity; session JSONL on Claude Code). |
| **Episode** | The compacted, scrubbed, harness-neutral record of a session produced by Stage 0/1 (§5.1). |
| **Turn note** | A short, structured `repo-memory-note` distillation written automatically by the working agent at the end of a milestone turn when `Stop` returns `"continue"` (`source: turn-note`), or explicitly mid-turn via `memory_note` (`source: mcp-note`). |
| **Mark** | Per-conversation state (`<data-dir>/repos/<repo-id>/state/marks/<conv-id>.json`) recording how far notes have covered: step index, byte offset, timestamp, `ExposureList`, and `InjectionLedger`. |
| **Exposure list** | The entry IDs pushed to, pulled by, or read by a conversation. Attached to that conversation's notes so consolidation's P13 echo filter ignores repeats (§6.5). |
| **Injection ledger** | Per-conversation record of injected entry IDs, `InjectedSet` deduplication map, delivery channel (`userMessage`/`systemMessage` vs `ephemeralMessage`), step index, `LastKnownStepCount`, and content hash. Drives re-injection after `CHECKPOINT` compaction or `/rewind` (§7.4). |
| **Inbox** | Scrubbed candidate notes waiting for Stage 2 consolidation (`source: turn-note \| mcp-note \| checkpoint \| artifact \| pa \| doc-seed \| sidecar`). |
| **Op** | A typed operation against a dossier entry: `add`, `amend`, `supersede`, `retire`, `move`, `split`. |
| **Op log** | The append-only, month-sharded record of ops in `.memory/ops/<yyyy-mm>/<ulid>.json` (or `.memory/local/ops/` for unmerged branch ops). The git-merged source of truth for all dossiers. |
| **Dossier** | A cohesive markdown page (`.memory/dossiers/<scope>.md`, ≤ 3k tokens, with optional `{section?}` resolution) for one module, architectural topic, external system or procedure area, rendered deterministically from the op log. Superseded and retired entries render to `<scope>.history.md`. |
| **Entry** | One structured item in a dossier (`2–6` lines): stable ID (`<prefix>-<short-ulid>`), entry type, cognitive lifecycle (`kind: semantic \| procedural \| episodic`), `Why`, `Do`, code anchors with fingerprints, verification stamp, and evidence links (§5.3). |
| **Atlas** | A one-line-per-dossier index rendered to `.memory/ATLAS.md` and referenced by the managed `v2` instruction block in `GEMINI.md` / `CLAUDE.md`. |
| **Event path** | Synchronous work started by a harness hook (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`). Must finish in milliseconds; only reads `index.db`/mark files, checks anchors, injects context, and writes spool/mark files (§4.1). |
| **Time path (`tick`)** | Asynchronous work started by the scheduled sidecar (`repo-memory tick`). Ingests spool files, harvests `CHECKPOINT`/`brain/` artifacts, runs `repo-memory-distiller` fallback and multi-turn `stream-json` consolidation, refreshes `doc-seed` entries ("dreaming"), rebuilds `index.db`, and updates the memory PR (§4.1). |

---

## 4. Architecture overview

```
          EVENT PATH (hooks)                               TIME PATH (scheduled tick, every minute)
  ┌──────── Working agent (Antigravity / Claude Code) ─────────┐
  │  works normally — zero manual cooperation required          │
  │  ▲ push: persistent userMessage (dossiers/atlas)            │
  │  │       + 1-step ephemeralMessage (gates/alerts)           │
  │  │       + Stop NO_TOOL_CALL milestone reflection           │
  │  │ pull: memory_brief / memory_search / memory_history ────┼──▶ MCP server ──▶ SQLite index.db
  └──┼───────────────┬─────────────────────────────────────────┘
     │ inline        │ Stop · PreInvocation · PreToolUse · PostToolUse
     │ (no LLM)      ▼ write spool files & session ledger
     │        ┌─────────────┐        ┌───────────────────────────────────────────────┐
     │        │   spool/    │──────▶ │ Tick: ingest spool → job table → next steps   │
     │        └─────────────┘        │  • mechanical work in-process                 │
     │                               │  • Stage 2 Consolidation: agy -p --agent      │
     │                               │    repo-memory-distiller (stream-json, <5min) │
     │                               │  • Doc seeding & async doc freshness (dream)  │
     │                               │  • reports: agentapi (visible in the IDE)     │
     │                               └───────────────────────┬───────────────────────┘
     │                                                       ▼
     │       Stage 1: Reflection & Harvest ──▶ Stage 2: Consolidation & Materialise
     │             │                                         │
     │             ▼                                         ▼
     │     Episode log (private)             Op Log (.memory/ops/) + Scope Dossiers
     │                                                       │
     └──────────── alerts/ + local index (index.db, rebuildable) ◀───────────────────┘

  Human events: answers in review reports, memory-PR review → highest-trust events
```

### 4.1 Invocation paths

Nothing runs continuously. Two paths start all work, and each has a strictly separate job:

| | **Event path: hooks** | **Time path: scheduled tick** |
|---|---|---|
| Fired by | Agent, model, tool and session events in the harness | Cron, every minute (Antigravity plugin sidecar, `builtin: schedule`, active while the IDE is running; see E6). In CLI-only environments, the OS scheduler running `repo-memory tick`. **[R2]** On Claude Code, harness-run `async: true` hooks also start it (§12.2). |
| Knows | Exactly which conversation, transcript, workspace and step just changed | Discovers state from the spool, the job table, transcripts and git |
| Constraint | Synchronous and blocking the agent loop, so it must finish in milliseconds | A time budget per tick (default 50 s); never overlaps itself (`tick.lock`) |
| Does | Reads `index.db`, the session ledger, and `alerts/`; injects persistent `userMessage` dossiers and 1-step `ephemeralMessage` nudges; gates risky commands; triggers milestone-gated `Stop` reflection; writes spool files | All asynchronous work: ingest the spool, advance jobs, run Stage 2 consolidation (`repo-memory-distiller`), verify procedures, run async doc-freshness checks, materialise dossiers, rebuild `index.db` |
| Never | Starts processes, calls a model directly, uses the network, or writes the job table | Leaves work running after it exits |

**[R2]** On Claude Code, a hook registered with `async: true` is run in the background *by the harness*. That hook's command simply is `repo-memory tick`, so it is another time-path trigger, not a hook doing work, and the tick stays the only executor.

**Rule:** work may outlive a tick only as durable job state. Every executor is either a child process that the tick starts and waits for (with a timeout), or a harness-supervised conversation (reports). Nothing is detached.

**Why harness-level rather than OS-level:**
- **Portability.** Nothing depends on per-OS process-detachment behaviour, inherited output pipes, or how each harness supervises hook children.
- **Visibility.** Each tick's output lands in `sidecar_data/<id>/logs`, and there is a single job table.
- **Simplicity.** There is one place to look for what ran, when, and at what cost.

The cost is that asynchronous LLM results arrive at tick granularity: up to about a minute, plus the call time.

### 4.2 Invocation map

| Work | Event path | Time path |
|---|---|---|
| Capture a finished turn / session (Stage 1) | `Stop` (`stopReason == "NO_TOOL_CALL"`, `executionNum == 0`): when session passes the Milestone Gate (≥ 1 file edit, ≥ 1 non-zero command fixed, or ≥ 5 steps) and `memory_note` has not already fired, return `{"decision": "continue", "userMessage": "<reflection prompt>"}` so the warm-cache agent emits structured candidate notes before terminating (`executionNum == 1` allows stop and spools transcript; E8, X1) | Sweep catches sessions with no `Stop` (crash, kill, sessions from before install); harvests `CHECKPOINT` steps and `<appDataDir>/brain/<session-id>/*.md` artifacts at zero LLM cost; promotes episodes to Stage 2 |
| Consolidate & materialise (Stage 2) | none | Tick runs multi-turn `--input-format stream-json` `repo-memory-distiller` within 5 minutes of Stage 1 to keep prefix cache warm; applies Pre-Consolidation Corroboration Gate; writes `.memory/ops/YYYY-MM.jsonl`; renders `.memory/scopes/<scope>.md`; updates `index.db` |
| Atlas & Startup Dossiers | Loaded by `.memory/ATLAS.md` (`@` include in `GEMINI.md`) and pushed as persistent `userMessage` on `PreInvocation` when `InjectedSet` is empty or after `DetectCompaction` / `DetectRewind` | Regenerated deterministically by `repo-memory materialise` when ops change |
| Task & Scope Dossiers | `PreInvocation` resolves scopes from prompt tokens, active files, and session ledger; injects top un-injected entries as persistent `userMessage`. On-demand refresh via MCP pull tool `memory_brief`. | Deterministic rendering + `index.db` update on op log mutation |
| Touch, error & procedure delivery | `PreInvocation` inline (persistent `userMessage` for newly entered scopes/error signatures); `PostToolUse` tracks command exit codes (`success_count` / `failure_count`) for active `procedure` entries | Graduates validated procedures (`success_count >= 3`, `failure_count == 0` across ≥ 2 sessions) into `.agents/skills/rm-<slug>/SKILL.md` |
| Risky action & Déjà-vu | `PreToolUse` `gate-once` (`{"decision": "deny"}` on 1st attempt per `(session, entry_id, args_hash)`, `"allow"` on retry); `PreInvocation` 1-step `ephemeralMessage` for immediate alerts (§7.2) | Tick checks semantic tier-2 alerts when queued |
| Re-check & Async Doc Freshness ("Dreaming") | `PostToolUse` on `run_command`: if the step at `stepIdx` is a merge, checkout, pull, rebase or dependency update, spool a re-check | Tick compares `HEAD`, build manifests, and `source_sha` of doc-seeded entries (`source: doc-seed`); re-extracts mutated docs and flags stale procedures |
| Publish the memory PR | none | Daily |
| Review report (§4.4) | none | Weekly, only if there is something to review |
| Metrics roll-up, coverage | none | Daily |
| Index rebuild | `repo-memory reindex` (manual) | Automatically updated on `materialise`, `seed`, and `seed-docs`, or rebuilt if `index.db` is missing or its schema is outdated |

### 4.3 Job table

Asynchronous work is tracked in a private SQLite job table (`<data-dir>/<repo-id>/jobs.sqlite`). Hooks never write to it; they write spool files, which the tick ingests. The hot path therefore never waits on a database lock.

| Field | Purpose |
|---|---|
| `id`, `kind`, `priority` | e.g. `harvest`, `consolidate`, `seed-docs`, `dream-freshness`, `materialise`, `publish`, `dejavu`, `recheck`, `report` |
| `idempotency_key` | Hash of inputs + stage + pipeline version, so duplicates are no-ops |
| `state` | `queued` → `running` → `done`, or `failed` → retried with backoff → `dead` after N attempts. Reports: `submitted(conversation_id)` → `done` when the conversation goes idle. |
| `lease_expires`, `attempts`, `next_attempt_at` | Recovery after a crashed or killed tick |
| `cost` | Tokens, `cache_read_tokens`, duration and model, taken from the `agy` JSON output |

**Each tick, in order:**
1. Take `tick.lock`; exit if a previous tick is still running.
2. Ingest spool files as jobs.
3. Reclaim expired leases.
4. Sweep transcripts, harvest `CHECKPOINT` / `brain/` artifacts, and check git hashes (cheap, mechanical).
5. Poll report conversations.
6. Run Stage 2 `consolidate` jobs within the 5-minute warm-cache window of Stage 1 capture.
7. Run batch jobs (`seed-docs`, `dream-freshness`, `recheck`, `materialise`) until the time budget is spent.
8. Run due calendar jobs.
9. Exit.

**Concurrency rules.** Consolidation and materialisation are single-writer on `.memory/ops/` and `.memory/scopes/`, so op IDs cannot collide and git operations cannot race.

**Back-pressure:**
- time budget per tick;
- daily budgets for housekeeping calls and tokens;
- a circuit breaker after N consecutive failures;
- a cap on open report conversations.

### 4.4 Executors and session visibility

| Class | Surface | Executor | Used for |
|---|---|---|---|
| **Repo work** | IDE | Human sessions, PA workers, the bug-fixer loop | The memory system never changes the repository's application code. It *proposes* such work in reports or graduates validated procedures into `.agents/skills/rm-<slug>/SKILL.md`. |
| **Reports** | IDE | Antigravity: `agentapi new-conversation`, title prefix `[repo-memory]`, started by the tick. **[R2]** Claude Code: a Desktop local scheduled task (visible under **Scheduled**). | Weekly memory review (entries awaiting review, gap questions, coverage and health, proposed work); one-off notices. Only started when there is something to review. |
| **Housekeeping** | Headless | `agy -p --agent repo-memory-distiller` with `--model gemini-3.6-flash`, `--output-format json`, `--input-format stream-json` (multi-turn stdin session keeping the prefix cache warm across batches), `--print-timeout`. **[R2]** On Claude Code-only machines: `claude -p --safe-mode --tools "" --system-prompt-file … --json-schema … --no-session-persistence` (§12.2). | Stage 2 Consolidation, `seed-docs` initial extraction, and async doc freshness |
| **Mechanical** | No session | The `repo-memory` Go binary in the tick and hooks (§18) | Stage 1 milestone gating & `CHECKPOINT`/`brain/` harvest, Pre-Consolidation Corroboration Gate, deterministic dossier materialisation, `index.db` maintenance, git worktree publishing |

Everything runs on the user's subscriptions, with no API key and no pay-as-you-go billing:

- **Executor setting.** `housekeeping.executor` is set per machine. In Release 1 it is always `agy`. **[R2]** `claude` becomes available for Claude Code-only machines (decision D2, §17).
- **Optional `api` backend.** It exists for other deployments and is off by default.

**Cost of a housekeeping run** (measured in E10 & X5):

| Metric | Single-call `gemini-3.8-flash-low` | Single-call `gemini-3.6-flash-low` (E10) | Multi-turn `stream-json` `repo-memory-distiller` (X5) |
|---|---|---|---|
| Time | 12.8 s end to end | ~10.5 s end to end | ~8–11 s per turn |
| Input tokens (turn 1) | 16,813 | 9,920 (41% reduction) | ~3,200 (minimal custom agent prompt) |
| Cached tokens (turns 2..N) | `0` (`cache_read_tokens = 0` across processes) | `0` across processes | **85%+ `cache_read_tokens`** within the same `stream-json` process |
| Where recorded | Only in the CLI store (`~/.gemini/antigravity-cli/`, project `default-cli-project`); not in the IDE history | Same | Same |

#### 4.4.1 Cost model

Housekeeping cost has two parts: **fixed overhead** and **payload**. Two architectural mechanisms minimize both:

1. **Stage 1 Warm-Cache Reflection (`Stop` `NO_TOOL_CALL` continuation):** Because the active coding agent already holds the entire session transcript in its warm prompt cache at the end of a turn, returning `{"decision": "continue"}` on `Stop` reuses 95%+ of the session tokens as `cache_read_tokens` rather than re-reading a 52k–120k token transcript cold in a new process.
2. **Stage 2 Multi-Turn `stream-json` Consolidation (`repo-memory-distiller`):** Rather than spawning a separate `agy -p` child process per scope or candidate (which pays the full fixed overhead every time because prefix caching is not shared across separate CLI processes; E10), the tick opens a single `agy -p --agent repo-memory-distiller --input-format stream-json` process within 5 minutes of Stage 1 and streams all candidate batches and existing scope dossiers as sequential turns in one warm-cache session (X5).

**Isolating housekeeping runs:**
- They run with the working directory `<data-dir>/runs/<repo-id>/`, not the repository, so workspace rules and hooks don't load. (Note: `--project` is silently ignored by `agy -p`, so isolation relies strictly on the isolated working directory and environment variables; E7).
- They set `REPO_MEMORY_HOUSEKEEPING=1`. Hooks inherit `agy`'s environment, so this plugin's own hooks exit immediately inside housekeeping runs.
- Capture excludes them by working directory URI and `REPO_MEMORY_HOUSEKEEPING=1`.
- `agy --continue` must never resume one (E7).
- **[R2]** On Claude Code, `claude -p --safe-mode --no-session-persistence` loads no plugins or hooks and writes no transcript, so there is nothing to exclude and recursion is impossible.

**Report conversations are captured**, but only messages a human typed count as evidence. The report's own text, and the automated kickoff prompt that started the conversation, were generated from memory, so re-extracting them would reinforce memory with itself.

### 4.5 Components

| Component | Responsibility | Invoked by |
|---|---|---|
| **Hook entry point** (`repo-memory hook <harness> <event>`) | Deterministic delivery (persistent `userMessage` dossiers + 1-step `ephemeralMessage` alerts); `PreToolUse` `gate-once` enforcement; `PostToolUse` command exit-code tracking; `Stop` milestone-gated reflection continuation; spool writing (§7, §6). No child processes, network, or job-table writes. | Event path |
| **Tick** (`repo-memory tick`) | Spool ingestion, the job table, sweep, `CHECKPOINT`/`brain/` harvest, Stage 2 consolidation (`repo-memory-distiller`), async doc freshness, procedure skill graduation, reports, calendar jobs | Antigravity scheduled sidecar; OS scheduler; CLI (`repo-memory run` is an alias) |
| **Materialiser** (`repo-memory materialise`) | Deterministically replays `.memory/ops/YYYY-MM.jsonl`, resolves 5-tier scope inheritance and `{section?}` rules, writes `.memory/scopes/<scope>.md` and `.memory/ATLAS.md`, and updates `<data-dir>/<repo-id>/index.db`. No LLM. | Tick, `seed`, `seed-docs`, or CLI |
| **Seeder** (`repo-memory seed` & `repo-memory seed-docs`) | Bootstraps `.memory/ops/` from pluggable historical sources (`PAWorkOrderSource`, `GitCommitSource`, `DocSeedSource`) and runs `materialise` + `index.db` build (§6.6, §14) | `repo-memory init` or CLI |
| **Pull tools (MCP)** | `memory_brief`, `memory_search`, `memory_history`, `memory_note`, `memory_feedback`, `memory_stats`. Exposed by a stdio MCP server (`repo-memory mcp`) backed by `<data-dir>/<repo-id>/index.db`. | Spawned by the harness per session |
| **Review surface** | Weekly review report (IDE) and the batched memory PR (git host) | Tick |

Processes communicate only through files and SQLite in WAL mode. There is no socket, no long-running process and nothing detached. Everything works on macOS, Linux and Windows.

**[R2] Claude Code-only machines:**
- Hooks behave the same.
- Asynchronous work needs a tick: the Antigravity sidecar if Antigravity is installed on the same machine, otherwise one OS scheduler entry (`repo-memory schedule install`).
- LLM stages need the Antigravity CLI (`agy`) installed and signed in. Without it, capture and delivery keep working, and LLM jobs wait in the queue.

### 4.6 Two-plugin packaging split, enrolment and the kill switch

Empirically (Phase 0 experiment `phase0-plugin-split`), Antigravity only spawns `sidecar.json` scheduled background daemons (`builtin: schedule`) from **global** plugins under `~/.gemini/config/plugins/<name>/`, whereas workspace-scoped hooks (`hooks/hooks.json`), MCP servers, and skills can be activated per repository via `.agents/plugins.json`. To prevent global hooks from firing on non-enrolled repositories while keeping the scheduled tick active, `repo-memory` uses a **Two-Plugin Packaging Split**:

1. **Global Daemon Plugin (`~/.gemini/config/plugins/repo-memory-daemon/`):** Contains `plugin.json` and `sidecar.json` (`builtin: schedule`, every 1 minute running `repo-memory tick`), plus the nested workspace plugin bundle under `workspace-plugin/repo-memory/`. It declares **no global hooks**.
2. **Per-Workspace Plugin (`.agents/plugins.json` -> `<absolute-path>/workspace-plugin/repo-memory`):** Activated per enrolled repository using a resolved **absolute path** (because `.agents/plugins.json` does not expand `~` or `$HOME`). Registers `hooks/hooks.json` (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`), the `repo-memory mcp` server, the `repo-memory-distiller` custom agent, and skills exclusively for that workspace.

**Enrolling a repository.** `repo-memory init`, run in the repository root:

1. creates `.memory/` (`config.yaml`, `ops/`, `scopes/`, `ATLAS.md`) and `.memory/.gitignore`;
2. writes or updates `.agents/plugins.json` with the resolved absolute path to `workspace-plugin/repo-memory`;
3. updates the machine-managed `<!-- repo-memory:start v2 --> ... <!-- repo-memory:end -->` instruction block in `GEMINI.md` (creating `GEMINI.md` only if needed, and never leaving a stale v1 `@[memory map](.memory/views/MAP.md)` line; **[R2]** `CLAUDE.md`);
4. registers the repository in `<data-dir>/registry.json`: repo ID, root path and known worktree paths. The repo ID is `sha256(root commit + ":" + canonical origin URL)`, or `sha256(root commit + ":" + real path of the main worktree)` when there is no remote;
5. runs `repo-memory seed-docs` (and `repo-memory seed` if PA work orders are present) to bootstrap initial dossiers and `<data-dir>/<repo-id>/index.db` so Day-1 sessions have high-signal architectural context immediately.

**What enrolment controls:**

| Component | Behaviour |
|---|---|
| Hooks | In the two-plugin split, hooks only load in workspaces with `.agents/plugins.json`. As defence in depth, every hook also checks the fast kill switch (`REPO_MEMORY_DISABLED=1`, ~2.8ms; E5) and checks for a local `.memory/` directory marker (~0.1ms) before touching disk state, guaranteeing ≤ 5 ms exit if ever invoked in a non-enrolled directory. |
| Capture sweep | Copies only transcripts whose workspace matches an enrolled root or one of its worktrees. Sessions in other workspaces are never read or copied. |
| Tick | Processes enrolled repositories only. |
| Worktrees | Share the repo ID; sessions are attributed by path prefix. |

**Leaving.** `repo-memory leave` removes the registry entry and `.agents/plugins.json` reference. It doesn't touch `.memory/`, which is shared repository content. `--purge` also deletes that repository's private episodes, `index.db`, and data directory.

**Kill switch.** One check comes before everything else in every hook and every tick: the environment variable `REPO_MEMORY_DISABLED=1`, or `enabled: false` in the machine configuration (`<data-dir>/config.yaml`). `repo-memory disable` and `repo-memory enable` set it, globally or with `--repo` for one repository. When disabled, hooks return no-op output (`{}` for `PreInvocation`/`PostToolUse`/`Stop`, `{"decision": "allow"}` for `PreToolUse`) and the tick exits.

**[R2]** Claude Code uses the same registry; its adapter maps the hook's `cwd` to the repository.

---

## 5. Data model

### 5.1 Episode record

One episode per top-level session. Subagent sessions are nested. Episodes are **private** (§11.1).

Location: `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst`. Here `<data-dir>` is the platform user-data directory: `~/Library/Application Support/repo-memory` on macOS, `~/.local/share/repo-memory` on Linux, and `%LOCALAPPDATA%\repo-memory` on Windows. It's deliberately harness-neutral, because Claude Code hooks and the Antigravity sidecar both write there.

```jsonc
// header line
{
  "episode_id": "ep-<ulid>",
  "harness": "antigravity" | "claude-code",
  "session_id": "<native id>",
  "parent_episode": null | "ep-…",
  "repo_id": "<sha256(root commit : canonical origin URL)>", // §4.6
  "branch": "ble/wo6-dis-service",
  "started": "2026-09-22T19:04:11Z", "ended": "…",
  "head_start": "<sha>", "head_end": "<sha>",
  "commits": ["<sha>", …],            // linked by the sweep from git log/reflog (§6.2)
  "step_gaps": [[812, 820]],          // unwritten internal dispatch steps (standard; E12)
  "model": "…",
  "harness_version": "…",             // e.g. Claude Code 2.1.284; parser selection (§12.2)
  "started_by": "human" | "automation", // automation: agentapi reports, bug-fixer ticks,
                                       // desktop scheduled tasks, -p runs
  "compactor_version": "1.0.0",
  "stats": { "raw_chars": 0, "kept_chars": 0 }
}
// step lines
{ "i": 42, "kind": "user",        "author": "human" | "automation", "text": "…" }
{ "i": 43, "kind": "reasoning",   "text": "…" }
{ "i": 44, "kind": "agent",       "text": "…" }
{ "i": 45, "kind": "command",     "cmd": "pio run -e esp32s3nano", "exit": 1,
                                  "outcome": "build_fail", "excerpt": "<error lines only>" }
{ "i": 46, "kind": "edit",        "file": "modules/BLEManager/BLEStack.cpp", "commit": "<sha>|null" }
{ "i": 47, "kind": "read",        "file": "…" }             // path only, never content
{ "i": 48, "kind": "test",        "suite": "native", "passed": 882, "failed": 6, "excerpt": "…" }
{ "i": 49, "kind": "device",      "action": "flash|monitor|soak", "outcome": "reset:INT_WDT", "excerpt": "…" }
{ "i": 50, "kind": "subagent",    "episode": "ep-…" }
{ "i": 51, "kind": "exposure",    "via": "push" | "pull" | "read", "trigger": "touch",
                                  "claims": ["clm-…", "clm-…"] }   // content never kept
{ "i": 52, "kind": "note",        "file": "<memory artefact path>", "text": "…" }
                                  // text an agent wrote to a memory artefact (§6.4);
                                  // a lead for extraction, never evidence (P13)
```

**Compaction rules (deterministic):**

| Keep | Drop or reduce |
|---|---|
| User messages (verbatim) | File contents returned by reads (keep the path) |
| Agent reasoning and text | Edit bodies (keep the path; resolve to commit where possible) |
| Commands, exit codes, classified outcome | Command output, except error/failure lines and summary lines (bounded excerpt) |
| Test, build, flash and soak outcomes | Repeated identical outputs (collapse with count) |
| Subagent tree | Tool schemas and system prompts |
| Text written to memory artefacts (§6.4), as `note` steps, up to 2k tokens per write | Edit bodies of every other file |
| Who wrote each `user` step (`human` or `automation`) | **Memory content the session received** (hook injections, pull-tool results, reads of `.memory/**`): replaced by `exposure` steps listing claim IDs. The content itself is never kept. |

**Transcript ordering and gaps (E12):**
- Step indices in raw JSONL transcripts contain gaps (typically delta=2 after tool calls) representing internal dispatch steps that the harness never serializes. Parsers must not treat gaps as dropped data.
- Parallel tool executions result in child tool results being appended before the parent `PLANNER_RESPONSE` is written to disk, producing non-monotonic step indices in raw JSONL. Compaction sorts steps by `step_index`.
- Conversations reused across multiple prompts or reset sessions restart `step_index` at 0 while appending to the same file. Compaction splits episodes on prompt boundaries.

**Author classification.** A `user` step is `human` only if a person typed it:

- **Antigravity:** a step with `source: USER_EXPLICIT` and `type: USER_INPUT`, excluding the kickoff prompt of a conversation the tick or another automation started. The tick records each kickoff's conversation ID and prompt hash in the job table.
- **[R2] Claude Code:** a `user` entry whose content is typed text, not a tool result, a hook-injected context block, or a system reminder. The instruction prompt of a Desktop scheduled task or a `-p` run is excluded.
- **Anything else** is `automation`.

**Scrubbing:** a secret scanner (pattern rules plus entropy heuristic) runs before anything is written. Matches are replaced with `«secret:type»`. Scrubbing is mandatory and irreversible.

### 5.2 Entity

Entities are derived from the repository by the entity extractor (no LLM) and cached in the index. Only the subsystem grouping is configured by hand.

| Kind | Source | Example |
|---|---|---|
| `subsystem` | `.memory/config.yaml` (globs) | `ota`: `modules/OTAUpdateManager/**`, `lib/OTA*/**`, `scripts/release*` |
| `file` | tree | `modules/BLEManager/BLEStack.cpp` |
| `symbol` | tree-sitter or ctags | `BLEStack::init`, `IDataSource` |
| `config` | sdkconfig*, `platformio.ini`, Kconfig | `CONFIG_BT_NIMBLE_MEM_ALLOC_MODE_EXTERNAL` |
| `env` | build config | `esp32s3nano_ble` |
| `topic` | code scan | EventBus `data.source.health` |
| `tool` | code scan | MCP tool `ota_get_status` |
| `error` | code scan plus claims | `"No manifest URL configured"`, `INT_WDT` |
| `external` | lockfiles plus claims | `esp-nimble-cpp@2.5.0`, `esp-idf@5.5.5`, `board:waveshare-s3` |

Entities link to each other with `contains`, `depends_on`, `implements` and `configures`, all derived from the code where possible.

**Exactness.** Entity extraction uses no LLM, so some kinds are exact and some are approximate:

| Kind | Method | Exactness |
|---|---|---|
| `file` | Tree walk, respecting `.gitignore` | Exact |
| `config` | Parse sdkconfig/Kconfig/`platformio.ini`/lockfiles | Exact |
| `env` | Parse build config | Exact |
| `symbol` | Tree-sitter (ctags fallback). Macros and conditional compilation can hide or duplicate symbols. | Approximate. Anchors fall back to file-window fingerprints when a symbol can't be resolved. |
| `topic`, `tool` | Per-repo **extractor rules** in `config.yaml`: call-site patterns such as `EventBus::publish("…")` or `addTool("…")` | Exact for configured patterns, blind to everything else |
| `error` | String literals passed to configured logging/error calls, plus error strings that appear in failure-signature claims | Approximate. Noisy literals are filtered by a minimum length and an allow/deny list. |
| `subsystem` | `config.yaml` globs | Exact once configured. Files matching no glob are reported as **unassigned** in the coverage report. |

**Subsystem bootstrap.** Hand-writing `config.yaml` for a large repo is a chore. On first install, a one-off LLM pass proposes subsystems and globs from the directory structure and existing docs (for esp32-app-foundation: `modules/`, `lib/`, the Module Index domains and the design spec sections). A human edits and approves the proposal. After that it changes only by hand, or by an explicit re-proposal the human reviews.

**Subsystem configuration** (`.memory/config.yaml`):

```yaml
subsystems:
  ota:
    title: OTA updates
    purpose: Manifest-first signed firmware updates with channel and rollback support
    globs: [modules/OTAUpdateManager/**, lib/OTADownloadEngine/**, lib/OTAManifest/**,
            lib/OTACerts/**, scripts/release*]
  ble:
    title: Bluetooth LE
    globs: [modules/BLEManager/**, sdkconfig.ble.defaults]
  # cross-cutting subsystems are allowed and may overlap
  radio-coexistence:
    globs: [modules/BLEManager/**, modules/WifiManager/**, sdkconfig*]
```

### 5.3 Op Log, Dossier Entries, and Three Cognitive Lifecycles

Instead of one markdown file per atomic claim, `v2.2` stores all mutations in an **append-only op log** (`.memory/ops/<yyyy-mm>/<ulid>.json`) and projects them deterministically into cohesive **subsystem dossiers** (`.memory/dossiers/<scope>.md` and `<scope>.history.md`) plus `.memory/ATLAS.md`.

#### 5.3.1 Op Log Schema (`.memory/ops/<yyyy-mm>/<ulid>.json`)

Each consolidation batch writes one immutable JSON file named by ULID, sharded by creation month (`<yyyy-mm>`). Because ops target entry IDs rather than markdown line numbers and each batch creates a new ULID file, concurrent branches and developers never experience git merge conflicts on op files.

Allowed op types:
- `add`: creates a new entry with a stable ID (`<prefix>-<short-ulid>`), `type`, `kind` (`semantic` / `procedural` / `episodic`), `dossier` scope, `title`, `why`, `do`, `anchors` (with file/symbol fingerprints), `verified`, `evidence`, and optional `closes_when` / `error_pattern` / `commands` / `gate`.
- `amend`: updates an existing entry's wording, anchors, `verified` stamp, `success_count`/`failure_count`, or `render_as_skill: rm-<slug>`, or appends new evidence IDs.
- `supersede`: replaces an entry with a new entry ID, moving the superseded entry to `<scope>.history.md` with its reason.
- `retire`: retires an entry (for example when an `open-issue`'s `closes_when` condition fires or an anchored subsystem is removed), moving it to `<scope>.history.md`.
- `move`: relocates an entry to another dossier scope.
- `split`: splits a dossier that exceeds its ~3k-token cap into child dossiers, recording `split_from` and registering an alias so `memory_brief(<old-scope>)` resolves transparently.

#### 5.3.2 Eight Entry Types Across Three Formal Cognitive Lifecycles (`kind`)

Every entry belongs to one of **three formal cognitive lifecycles (`kind: semantic | procedural | episodic`)**, governing how it is corroborated, invalidated, and delivered:

| Cognitive Lifecycle (`kind`) | Entry Type (`prefix-`) | Captures | Verification & Invalidation Rules | Primary Delivery |
|---|---|---|---|---|
| **`semantic`** *(Module Truths & Rules)* | `invariant` (`inv-`) | A structural rule or boundary that must hold (`gate: edit` optional for critical invariants) | **Code/Anchor-Coupled:** Checked against file/symbol fingerprints at harvest and before every push; requires Pre-Consolidation Corroboration Gate (§6.4) | Persistent `PreInvocation` (`task-start`, `first-entry`, `first-touch`); `PreToolUse` `gate-once` |
| **`semantic`** | `contract` (`con-`, alias `convention`) | A module interface contract or house rule not obvious from a single file | **Code/Anchor-Coupled:** Fingerprint-checked before delivery | Persistent `PreInvocation` (`first-entry`, `first-touch`) |
| **`semantic`** | `gotcha` (`got-`, alias `external-behaviour`) | How a tool, platform, SDK, hardware peripheral or library actually behaves | **Code/Version-Coupled:** Checked against anchored files and SDK/toolchain conditions | Persistent `PreInvocation` (`first-touch`) |
| **`semantic`** | `open-issue` (`iss-`) | A known defect, tech debt or in-flight gap, paired with a deterministic `closes_when` condition | **Event-Coupled (`closes_when`):** Retired automatically when its `closes_when` condition fires; pull-only if `closes_when` is omitted | Persistent `PreInvocation` (`task-start`, `first-touch`) when `closes_when` is present |
| **`procedural`** *(Executable Workflows & Error Playbooks)* | `recipe` (`rec-`, alias `procedure`) | A verified multi-step command sequence or workflow (build, flash, release, soak test) | **Execution-Outcome Coupled:** `PostToolUse` tracks matching command exit codes (`success_count` / `failure_count`); flagged `stale_suspect` when build manifests (`Makefile`, `CMakeLists.txt`, `platformio.ini`, `go.mod`, `package.json`) change. Graduates deterministically to `.agents/skills/rm-<slug>/SKILL.md` at `success_count >= 2` across ≥ 2 sessions (Addendum A §10) | Graduated `.agents/skills/rm-<slug>/SKILL.md` + 1-line `first-touch` pointer; `PreToolUse` `gate-once` on risky actions |
| **`procedural`** | `failure-signature` (`err-`) | Symptom pattern → root cause → concrete fix, with machine-matchable `error_pattern` and command prefix | **Execution-Outcome Coupled:** Verified when applying the fix resolves the matching command/test error | `ephemeralMessage` on `error-match`; persistent `first-touch` on anchored files |
| **`episodic`** *(Historical Reasoning & Dead Ends)* | `decision` (`dec-`) | A design choice, why it was made, and the alternatives rejected | **Immutable Historical Fact (Supersession-Bound):** Never invalidated by code edits alone; superseded only by a newer `decision` op | MCP `memory_why` / `memory_history` (and `task-start` when directly matched) |
| **`episodic`** | `ruled-out` (`rul-`) | An approach that was tried and failed, why it failed, and when that applies | **Immutable Historical Fact:** Stays active so agents do not repeat the dead end; flagged `conditions-changed` if the underlying SDK/hardware version changes | Persistent `first-touch` push; `about-to-edit` pointer; `PreToolUse` déjà-vu check |

**Closure conditions for `open-issue` (`closes_when`).** Every pushed `open-issue` entry carries machine-checkable `closes_when` conditions evaluated deterministically by `tick`:
- an issue is closed;
- a branch is merged;
- a PA work order is completed;
- a command/test outcome succeeds in a later episode;
- a path or symbol exists or is absent;
- a file contains, or no longer contains, a pattern.
When a condition fires, `tick` emits a `retire` op with that reason. An `open-issue` without `closes_when` is pull-only, never pushed.

### 5.4 Dossier Format, Optional Section Resolution (`{section?}`), and `ATLAS.md`

Dossiers are deterministically rendered from `.memory/ops/` into `.memory/dossiers/<scope>.md` (capped at ~3k tokens per dossier; active entries only) and `.memory/dossiers/<scope>.history.md` (superseded and retired entries).

**Optional Section Resolution (`{section?}`, `VD16`):** To eliminate empty boilerplate headings, only the header (`Purpose and boundaries`) and non-empty sections are rendered:
- **Required when the dossier exists:** Header + `Purpose and boundaries` (1–3 lines) and at least one active entry section (typically `## Invariants & contracts`).
- **Optional sections (`{section?}` — omitted completely when empty):**
  - `## Invariants & contracts` (`invariant`, `contract`/`convention`)
  - `## Ruled out, failure signatures and gotchas` (`ruled-out`, `failure-signature`, `gotcha`/`external-behaviour`)
  - `## Decisions` (`decision`)
  - `## Procedures / Recipes` (`recipe`/`procedure`)
  - `## Open issues` (`open-issue`)
  - `## Evidence index`

Example rendered entry:

```markdown
### [rul-7Q2M] Relative hook command paths in hooks.json
`ruled-out · kind: episodic · anchors: packaging/antigravity/hooks.json, cmd/repo-memory/cmd_install.go · verified 2026-10-01 (E09) · evidence: note-01M4…, E09`
Why: Antigravity runs hook commands from the directory containing hooks.json, not the repo root. A wrong path exits non-zero, which PreToolUse treats as deny.
Do: Generate absolute paths at install time; never hand-write relative hook paths.
```

**`.memory/ATLAS.md`:** A compact one-line-per-dossier index rendered from the op log and included in `GEMINI.md` / `CLAUDE.md` via the managed `<!-- repo-memory:begin v2 -->` block (Addendum A §8).

### 5.5 Five-Tier State Scope Hierarchy (`VD17`) and Storage Layout

State is partitioned across five explicit scope tiers so invocation scratch, conversation state, personal preferences, unmerged branch ops, and shared repository truth never pollute one another:

| Scope Tier | Prefix | Location | Git Tracked? | Purpose |
|---|---|---|---|---|
| **1. Ephemeral** | `temp:` | In-memory during hook/tick execution | No | Single-invocation `ephemeralMessage` hints and scratch ranking scores |
| **2. Session** | `session:` | `<data-dir>/repos/<repo-id>/state/marks/<conv-id>.json` | No | Per-conversation mark, `ExposureList`, and `InjectionLedger` (`InjectedSet`, `LastKnownStepCount`) |
| **3. Personal** | `user:` | `<data-dir>/repos/<repo-id>/user-dossiers/` | No | Machine-local developer workflow preferences and local hardware port notes |
| **4. Branch-Local** | `branch:` | `.memory/local/ops/` & `.memory/local/dossiers/` | No (gitignored local lane) | Ops and dossiers from unmerged feature branches; usable immediately on that branch and promoted to `.memory/ops/` when the branch merges |
| **5. Repository Shared** | `repo:` | `.memory/ops/<yyyy-mm>/<ulid>.json` & `.memory/dossiers/` | **Yes (Committed)** | Canonical shared repository memory reviewed via the memory PR |

**In the repository** (shared, reviewed):

```
.memory/
├── config.yaml                          # declared scopes, budgets, review profile, doc_seeds
├── scrub.yaml                           # shared secret-scrubber rules (Addendum A §5)
├── ATLAS.md                             # generated one-line-per-dossier index
├── ops/<yyyy-mm>/<ulid>.json            # append-only op log: the source of truth
├── dossiers/                            # deterministically rendered from ops/
│   ├── core/transcript-parser.md
│   ├── core/transcript-parser.history.md
│   ├── harness/antigravity-hooks.md
│   └── procedures/release.md
├── local/                               # gitignored branch-local lane (`branch:` tier)
└── .gitignore                           # ignores local/ and .cache/
.agents/
├── plugins.json                         # enables workspace-plugin/repo-memory (resolved absolute path)
├── agents/repo-memory-distiller.md      # lean custom agent for fallback & consolidation
└── skills/rm-<slug>/SKILL.md            # graduated procedural recipes (generated_by: repo-memory)
```

**Private, per machine** (never committed):

```
<data-dir>/
├── config.yaml                          # machine config: enabled, housekeeping.executor, spend guards
└── registry.json                        # enrolled repositories (§4.6)

<data-dir>/repos/<repo-id>/
├── index.db                             # rebuildable SQLite FTS5 + anchor index (via index.IndexPath)
├── jobs.sqlite                          # durable job table (§4.3); written by tick only
├── tick.lock
├── spool/                               # written by hooks only
├── queue/                               # incremental transcript copies awaiting compaction
├── evidence/
│   ├── episodes/<yyyy>/*.jsonl.zst      # compacted, scrubbed episodes
│   ├── checkpoints/                     # harvested CHECKPOINT summaries
│   ├── artifacts/                       # harvested brain/<id>/ markdown artifacts
│   ├── notes/                           # harvested turn notes, mcp notes, doc-seed & sidecar notes
│   └── pa/                              # parsed PA completion reports
├── user-dossiers/                       # personal developer dossiers (`user:` tier)
├── state/
│   ├── marks/<conversation-id>.json     # mark, ExposureList, InjectionLedger (`session:` tier)
│   ├── subagents.sqlite                 # child conversation → parent linkage
│   └── freshness.json                   # index & consolidation freshness stamps (Addendum A §7)
├── quarantine/                          # notes blocked by the layered scrubber (Addendum A §5)
├── worktrees/<clone-id>/                # dedicated git worktree for the memory/pending branch
├── runs/                                # isolated working directory for housekeeping runs
├── metrics/                             # per-session and tick telemetry (§10.4)
└── logs/
```

---

## 6. Capture, Distillation, and Consolidation Pipeline

### 6.1 Stage summary (`v2.2`)

| Stage | Input | Output | Engine | Cost profile |
|---|---|---|---|---|
| **0 Capture & Compact** | `Stop` hook (event) or `tick` sweep (time); raw `transcript_full.jsonl` | Incremental queue copy + compacted, scrubbed Episode (`.jsonl.zst`) | Deterministic Go | 0 LLM tokens |
| **1a Primary Warm-Cache Turn Note** | Active working session at milestone turn end (`Stop` `NO_TOOL_CALL`, `executionNum == 0`) | Structured `repo-memory-note` block harvested on second `Stop` (`executionNum >= 1`) + `ExposureList` + anchor checks (`source: turn-note`) | Working session model (`Stop` `"continue"`, 5-min warm KV cache) | ~1–5k uncached tokens + ~90%-discounted cached context |
| **1b Free & Deterministic Sources** | `CHECKPOINT` steps, `brain/<id>/` artifacts, mid-turn `memory_note` calls, PA `WORKORDER.md` reports | Candidate notes in inbox (`source: checkpoint \| artifact \| mcp-note \| pa`) | Deterministic Go | 0 LLM tokens |
| **1c Doc-Seeded Dossiers & Sidecar Fallback** | Curated repo docs (`repo-memory seed-docs`); sessions idle ≥ 2h with uncovered milestone work past the mark | Candidate notes in inbox (`source: doc-seed \| sidecar`) | `agy -p --agent repo-memory-distiller` (after `/usage` check) | ~10.6k fixed overhead + payload, shared across `stream-json` turns |
| **2 Warm Batch Consolidation & Publish** | Pending inbox notes + target dossiers + `ATLAS.md` | Validated op-log batch (`.memory/ops/<yyyy-mm>/<ulid>.json`), rendered `.memory/dossiers/*.md`, `.memory/ATLAS.md`, graduated `.agents/skills/rm-*/SKILL.md`, updated `index.db`, memory PR | Deterministic P13 echo filter + Corroboration Gate + 1 multi-turn `stream-json` `repo-memory-distiller` session | 1 multi-turn session per tick (~1–3k uncached tokens per dossier after Turn 1) |

### 6.2 Stage 0: Capture and Compaction

Capture uses both invocation paths (§4.1). The event path captures sessions and harvests warm-cache turn notes promptly; the time path guarantees that nothing is missed.

**Event path (`Stop` hook).** Whenever `Stop` fires:
1. writes `{harness, conversation_id, transcript_path, repo_root, branch, head}` to `spool/` and copies new bytes from `transcript_full.jsonl` into `queue/` **incrementally** (verifying the prefix MD5 hash; E12);
2. if `executionNum == 0`, evaluates the **Stage 1 Milestone Gate (§6.3.1)**:
   - if the gate passes, returns `{"decision": "continue", "reason": "<turn-note-request>"}` so the working agent writes a `repo-memory-note` block immediately from its warm 5-minute prompt cache;
   - if the gate does not pass, returns `{}` and lets the agent stop normally;
3. if `executionNum >= 1` (the harvest pass right after a `"continue"` request), parses and scrubs the `repo-memory-note` block from the transcript tail, checks its anchors against the working tree, attaches the conversation's `ExposureList`, writes the note to the inbox, advances the conversation's `mark`, and returns `{}` so the agent stops cleanly.

**Time path (`tick` sweep).** Every tick sweeps `~/.gemini/antigravity/brain/` and `~/.gemini/antigravity-cli/brain/` (**[R2]** `~/.claude/projects/`) for changed transcripts in enrolled workspaces:
- **Antigravity transcript file priority (E12):** reads one source in strict priority order and never merges sources:
  1. `transcript_full.jsonl` (**Primary** — complete and untruncated);
  2. `chunks/transcript_full/*.jsonl` (**Fallback 1** — 100 KiB byte slices concatenated in lexical order via `io.MultiReader` before splitting on newlines);
  3. `transcript.jsonl` (**Fallback 2** — compact file with `truncated_fields` flagged);
  4. `chunks/transcript/*.jsonl` (**Last resort**).
- **Compaction rules:** applies the deterministic compaction rules in §5.1, sorts out-of-order parallel tool steps by `step_index`, records unwritten internal dispatch `step_gaps` without error (E12), strips all memory exposures into `exposure` markers, and runs the layered secret scrubber (Addendum A §5).

### 6.3 Stage 1: Distillation Sources

#### 6.3.1 Primary: Automatic End-of-Turn Warm-Cache Harvest (`Stop` Continue)

The milestone gate in `Stop` runs deterministically in milliseconds. Every condition must hold:
1. The repository is enrolled and the kill switch (`REPO_MEMORY_DISABLED`) is off.
2. The session is not a housekeeping run (`REPO_MEMORY_HOUSEKEEPING` / `REPO_MEMORY_INTERNAL`) and not a subagent (`state/subagents.sqlite`, recorded when a parent's `PostToolUse` fires for `invoke_subagent`).
3. `terminationReason` equals `NO_TOOL_CALL` (case-insensitive) and `fullyIdle == true` (verified across 201 recorded `Stop` payloads; X1).
4. `executionNum == 0` (first stop in the sequence).
5. **Milestone since the mark:** reading the transcript from the mark's byte offset shows at least one of: (a) a `git commit`, (b) a PR opened, (c) a work order completed, (d) an error or failing test resolved after ≥ 15 steps, or (e) edits to ≥ 2 files followed by a build or test.
6. **Cooldown and cap:** ≥ 30 minutes since the last note in this conversation (waived for commits and PRs), and ≤ 20 notes per repository per day.
7. No autonomous `/goal` or workflow is active.

**Optional mid-turn override (`memory_note` MCP tool) and deduplication.** If an agent explicitly calls `memory_note` mid-turn (`source: mcp-note`), the handler writes the note to the inbox and advances the conversation's mark to the current step index. When the turn ends, the `Stop` gate inspects only steps after the updated mark — automatically skipping the end-of-turn continuation if the milestone was already recorded.

**Note format.** A single fenced `repo-memory-note` YAML block containing 0–6 items (≤ 600 tokens) with `type` (`invariant`, `contract`/`convention`, `gotcha`/`external-behaviour`, `open-issue`, `recipe`/`procedure`, `failure-signature`, `decision`, `ruled-out`), `title`, `why`, `do`, `anchors`, `evidence`, and optional `closes_when` / `error_pattern` / `commands`.

#### 6.3.2 Stage 1b: Free Zero-Token Distillations (`CHECKPOINT`, `brain/` Artifacts, `source: pa`)

Three sources require zero LLM calls:
1. **Platform `CHECKPOINT` summaries:** Compaction steps contain structured `User Knowledge`, `Model Knowledge`, and `Work Accomplished` sections written by the working model.
2. **Conversation artifacts (`<appDataDir>/brain/<conversation-id>/`):** Implementation plans, walkthroughs, and root-cause reports.
3. **PA Completion Reports (`repo-memory seed` / `source: pa`):** Deterministic parser extracts `Decisions Made` and `Dead Ends & Warnings` from `WORKORDER.md` and `PROJECT.md` files.

#### 6.3.3 Stage 1c: Doc-Seeded Dossiers (`repo-memory seed-docs`) & Sidecar Fallback

1. **Doc-Seeded Dossiers (`repo-memory seed-docs`, `source: doc-seed`):** Scans curated repository documentation configured under `doc_seeds` in `.memory/config.yaml` (defaulting to `docs/developer/**/*.md` and hand-authored `.agents/skills/**/SKILL.md` without `generated_by: repo-memory`). Runs `repo-memory-distiller` to extract anchored `invariant`, `contract`, `gotcha`, `recipe`, and `failure-signature` notes, recording `source_doc_path` and `source_doc_blob_sha` (Addendum A §11).
2. **Sidecar Fallback (`source: sidecar`):** For sessions idle ≥ 2 hours with uncovered milestone work past the mark (subagents, `/goal` sessions, or conversations closed mid-turn), `tick` verifies `/usage` headroom (≥ 25%) and runs `agy -p --agent repo-memory-distiller` over the session's latest `CHECKPOINT` summary + transcript tail + artifacts.

### 6.4 Preparation Layer, Layered Scrubber, and Pre-Consolidation Corroboration Gate

Before any transcript tail, checkpoint, doc, or note reaches `repo-memory-distiller` or the op log:

1. **Layered Secret Scrubber (Addendum A §5):** Runs at every write boundary (harvest, compaction, inbox, op-log write, publish) using shared rules in `.memory/scrub.yaml` plus local additions in `<data-dir>/repos/<repo-id>/scrub.local.yaml`, a known-ID allow-set (commit SHAs, ULIDs, entry IDs), pattern detectors, and entropy scanning. Any scanner error fails closed and quarantines the note in `quarantine/`.
2. **Instruction Stripping & Data Framing (Design v2.2 §5.6):**
   - File reads of skills/rules become 1-line stubs (`[viewed .agents/skills/X/SKILL.md]`).
   - All `[repo-memory ...]` injected steps, `Stop` continuation prompts, and user-input wrappers are stripped deterministically.
   - Steps are wrapped as JSON data records inside random-delimiter containers with explicit framing that imperatives inside the log are historical data, not instructions to the distiller.
3. **Pre-Consolidation Corroboration Gate (`VD18`):** Any candidate `semantic` or `procedural` item (`invariant`, `contract`/`convention`, `recipe`/`procedure`, or `gate: edit`) arriving with `verified: null` must be corroborated by at least one of:
   - a passing `PostToolUse` build/test step in the source episode after the anchored edit;
   - a verified static code anchor (`file` + `symbol` / fingerprint) present in `HEAD`; or
   - explicit human `USER_INPUT` confirmation.
   Uncorroborated assertions are downgraded to `open-issue` (with a `closes_when` check) rather than promoted to active imperatives.

### 6.5 Stage 2: Warm Batch Consolidation and Five-Tier Publish

Consolidation runs in `tick` when ≥ 10 inbox notes are waiting or once daily (subject to `/usage` ≥ 25% pool headroom and daily spend guards, Addendum A §6):

1. **Filter echoes (deterministic, P13):** Any note item repeating an entry in its conversation's `ExposureList` is dropped as a no-op unless the session modified one of the entry's anchored files **and** subsequently passed a build, test, or the entry's check.
2. **Route to dossiers (deterministic):** Matches items to dossiers by anchor globs in `.memory/config.yaml` (or derives a new module dossier scope by rolling up package directories when enough entries accumulate).
3. **Merge exact duplicates & check existing anchors (deterministic):** Identical normalized titles + overlapping anchors append evidence to the existing entry; existing entries whose anchors no longer resolve are flagged for amendment or retirement.
4. **Decide changes via Multi-Turn `stream-json` Session (`repo-memory-distiller`):** Runs a **single `agy -p --agent repo-memory-distiller --input-format stream-json --output-format stream-json` process per tick**. Turn 1 loads the system instructions, `ATLAS.md`, and Dossier 1's active entries + candidate items; Turns $2..N$ reuse the warm 5-minute prefix cache (~90% input discount) to emit typed ops (`add`, `amend`, `supersede`, `retire`, `move`, `split`) for each remaining dossier without ever rewriting whole markdown pages.
5. **Validate, write op log, and render projections (deterministic Go):** Go validates every op (schema, existing target IDs, required fields, length caps), writes `.memory/ops/<yyyy-mm>/<ulid>.json` (or `.memory/local/ops/` for unmerged branch notes), and deterministically renders `.memory/dossiers/<scope>.md` (with optional `{section?}` resolution), `<scope>.history.md`, `.memory/ATLAS.md`, graduated `.agents/skills/rm-<slug>/SKILL.md` files, and `index.db`.
6. **Async Doc Freshness ("Dreaming"):** If `git diff` shows that any `source_doc_path` changed its blob SHA since `source_doc_blob_sha`, `tick` re-distills that markdown file and emits `amend`/`supersede`/`retire` ops for its `source: doc-seed` entries.
7. **Publish across the Five-Tier Scope Hierarchy (§5.5):** Unmerged branch notes stay in `.memory/local/` (`branch:` tier) until the branch merges; shared `repo:` ops are published via a dedicated git worktree (`<data-dir>/repos/<repo-id>/worktrees/<clone-id>/`, branch `memory/pending`) into the batched memory PR.

---

## 7. Delivery Engine (Read Side)

### 7.1 Channels and Lifetime Contract (`VD13`)

In experiments E1 and X5 (`docs/experiments/X5-channels.md`, `docs/experiments/X5-injectSteps-investigation.md`), we verified that `ephemeralMessage` lasts for **one model invocation only** and is never written to `transcript.jsonl`, whereas `userMessage` is a plain string persisted in `transcript.jsonl` with `source: SYSTEM_SDK` until a `CHECKPOINT` compaction step (`systemMessage` is rejected in `agy 1.3.3` with `type mismatch: cannot convert string to message`).

Using `ephemeralMessage` for dossier pushes while recording `InjectedSet` deduplication caused pushed entries to vanish after 1 model step (`8d7ad7b` + `d3ab994` regression). `v2.2` enforces a strict split between **persistent conversation pushes** and **1-step hints**:

| Channel | Hook Mechanism | Lifetime | Used For | Budget & Dedup |
|---|---|---|---|---|
| **Always-on Atlas** | `@[memory atlas](.memory/ATLAS.md)` inside managed `<!-- repo-memory:begin v2 -->` block in `GEMINI.md` / `CLAUDE.md` (Addendum A §8) | Entire session | One-line-per-dossier index + anti-echo and MCP tool instructions | ≤ 1k tokens |
| **Persistent Push (`userMessage` on `agy 1.3.3` / `systemMessage` where supported)** | `PreInvocation` `injectSteps` (prefixed with `[repo-memory ...]` and logged with `source: SYSTEM_SDK`) | **Persistent across turns until a `CHECKPOINT` compaction step** | **1. `task-start`** (`initialNumSteps <= 1`): matched dossier invariants & open issues (≤ 500 tokens)<br>**2. `first-entry`**: module-wide invariants/contracts when entering a dossier scope (≤ 150 tokens)<br>**3. `first-touch`**: file/symbol-anchored invariants, ruled-out approaches, failure signatures, gotchas (≤ 150 tokens)<br>**4. `re-injection`**: consolidated active entries after `CHECKPOINT` or `/rewind` (≤ 1,000 tokens) | Deduplicated per entry/dossier/anchor in `InjectedSet` until compaction or rewind |
| **Single-Step Hint (`ephemeralMessage`)** | `PreInvocation` `injectSteps` | **One invocation only** (never written to `transcript.jsonl`) | **1. `about-to-edit`**: 1-line pointer to entries already in context for the target file (≤ 50 tokens)<br>**2. `error-match`**: matching `failure-signature` cause & fix after a failed command/test (≤ 200 tokens) | Per occurrence (does not grow transcript) |
| **Hard Gate (`gate-once`)** | `PreToolUse` `{"decision": "deny", "reason": "..."}` | Stays in tool history | Invariants marked `gate: edit` and risky hardware/release actions (§7.3). Returns `{"decision": "allow"}` once acknowledged or when no gate matches. | ≤ 200–800 tokens; once per session per entry |
| **On-Demand Pull (MCP)** | Stdio MCP server (`repo-memory mcp`) | Tool result in transcript | Depth on demand: `memory_brief`, `memory_why`, `memory_history`, `memory_search`, `memory_note`, `memory_status` (alias `memory_stats`) | Bounded per call |

**Anchor verification before every push.** Before injecting any entry, the hook checks that its anchored files exist and match their recorded fingerprints. An entry whose anchor check fails is marked `suspect` and excluded from push delivery.

### 7.2 Push Moments and Ranking (`PR-09b`)

1. **`task-start` (`initialNumSteps <= 1`):** Matches prompt file/symbol references, branch diff paths, and FTS5 query terms against `index.db`. FTS5 bm25 scores (~0.5–3.0) are scaled by `100` (`int(ftsScore * 100)`) so they combine commensurably with structural path/symbol scores, and candidates below `DefaultMinRelevanceScore = 50` are suppressed so generic prompt words (`tick`, `test`, `stage`) never trigger noisy task-start pushes.
2. **`first-entry` into a dossier scope:** When `PostToolUse` sees a file read or edit inside a dossier's scope for the first time in the session, the next `PreInvocation` persistently injects that dossier's top module-wide invariants and contracts (≤ 150 tokens) and records the scope in `InjectedSet`.
3. **`first-touch` of an anchored file or symbol:** When `PostToolUse` sees a read or edit touching a specific anchored file or symbol, the next `PreInvocation` persistently injects the entries anchored to that file/symbol (`invariant`, `ruled-out`, `failure-signature`, `gotcha`, or graduated skill pointer `rm-<slug>`, ≤ 150 tokens) and records the anchor in `InjectedSet`.
4. **`about-to-edit` pointer:** When the latest reasoning names an edit target whose entries are already in context, `PreInvocation` injects a ≤ 50-token `ephemeralMessage` pointer (`"[repo-memory] [inv-7Q2M] above applies to this file"`).
5. **`error-match`:** When `PostToolUse` observes a failed command or test (reading the tool output from the transcript's `GENERIC` step at `stepIdx`), and the output matches a `failure-signature`'s `error_pattern` **and** either the command prefix matches or the dossier scope is active, the next `PreInvocation` injects the fix via `ephemeralMessage` (≤ 200 tokens).

**Ranking order within token budgets:**
1. Exact file or symbol anchor match beats directory/scope match;
2. Entry type priority: `invariant` / `contract` → `failure-signature` → `ruled-out` / `gotcha` → `recipe` → `decision` / `open-issue`;
3. Verified entries beat unverified ones;
4. Higher evidence count and scaled FTS5 relevance score win ties.

### 7.3 Edit Gates and Risky Actions (`PreToolUse` `gate-once`)

Default risky actions (configurable in `.memory/config.yaml`):
- flash, monitor and OTA upload tools;
- release scripts;
- edits to `sdkconfig*`, `platformio.ini`, `dependencies.lock`, partition tables, or files governed by an invariant with `gate: edit`.

**Mechanics (verified in E8, E9, X9):**
- At `PreToolUse`, if a matching `gate: edit` invariant or risky-action `recipe`/`invariant` has not yet been acknowledged in this conversation, the hook returns `{"decision": "deny", "reason": "[REPO-MEMORY <id>] <why / do>. Re-issue the call after checking this."}` and marks the gate acknowledged in the conversation's ledger so the retry passes.
- Denying one tool call in a parallel batch is isolated and does not disrupt sibling tool calls (E9).
- `PreToolUse` hooks **must** explicitly return `{"decision": "allow"}` (or empty stdout) when allowing a call; returning `{}` or exiting with code `2` is treated by the harness as a deny (`exit 1` fails open; E8, E9, X9).

### 7.4 Compaction (`CHECKPOINT`) and Rewind (`DetectRewind`) Recovery

Antigravity compacts long conversations every ~100–200 steps by appending a `CHECKPOINT` step, and users can truncate or rewrite history via `/rewind` or editing an earlier message. Because no dedicated compaction or rewind hook exists (§12.1), `HandlePreInvocation` recovers deterministically on every model call:

1. **Ledger state:** Each conversation's mark file (`state/marks/<conv-id>.json`) stores every injection (`entry_ids`, `channel`, `step_index`, `content_hash`), the `InjectedSet` deduplication map, and `LastKnownStepCount`.
2. **Detection on every `PreInvocation`:**
   - **Compaction (`DetectCompaction`):** A `CHECKPOINT` step appears at a step index greater than the last persistent injection's step.
   - **Rewind (`DetectRewind(steps, ledger.LastKnownStepCount)`):** The transcript has fewer steps than `ledger.LastKnownStepCount` (`len(steps) < ledger.LastKnownStepCount`) or the timestamp of the step at the mark changed.
3. **Pruning, Re-arming, and Consolidated Re-injection:** When either `DetectCompaction` or `DetectRewind` fires, `HandlePreInvocation` prunes invalidated ledger records, clears affected keys from `InjectedSet` (re-arming `first-entry` and `first-touch`), updates `LastKnownStepCount = len(steps)`, and emits one consolidated persistent injection (`userMessage` / `systemMessage`, ≤ 1k tokens) containing the still-relevant entries for dossiers and anchors touched in the active conversation history (skipping any entry ID whose `[repo-memory <id>]` tag survived verbatim inside the `CHECKPOINT` summary).

### 7.5 Pull Tools (MCP)

| Tool | Returns |
|---|---|
| `memory_brief(scope_or_task)` | The rendered dossier for a module/topic scope (resolving `split` aliases), or a ranked task brief across matching dossiers. Starts with a freshness state line and ends with explicit coverage gap lines (Addendum A §7). |
| `memory_why(target)` | For an entry ID, file, symbol, config key, or `path:line` / `path#symbol` (Addendum A §13): active and `.history.md` decisions, invariants, and ruled-out approaches plus the evidence chain (entry → notes/episodes → commits). |
| `memory_history(entity)` | Chronological op-log and supersession history (`.history.md` + active entries) for a dossier, file, or symbol. |
| `memory_search(query)` | Full-text search (SQLite FTS5) over active dossier entries, unconsolidated inbox notes (labelled `unconsolidated`, Addendum A §7.4), episode synopses, and `CHECKPOINT` summaries. |
| `memory_note(note)` | Optional mid-turn override allowing the agent or user to record a structured discovery (`type`, `title`, `why`, `do`, `anchors`) immediately to the inbox (`source: mcp-note`). Advances the conversation's mark so `Stop` does not ask again at turn end (§6.3.1). |
| `memory_status()` (alias `memory_stats()`) | Observability and health summary (§10.4): dossier and entry counts by cognitive lifecycle (`semantic`, `procedural`, `episodic`) and scope; index freshness; pending inbox notes; hook latency and token spend. |

### 7.6 Performance and Degraded Modes

- **Latency budget:** Enrolled repository hooks run in p50 ~3ms, p95 ~6.1ms (well inside the p95 ≤ 60ms fast-path target; E5). Because the Two-Plugin Packaging Split (§4.6) loads `hooks.json` only in enrolled workspaces via `.agents/plugins.json`, unenrolled repositories pay **0 ms** of hook latency.
- **Fail-open safety:** Hooks never call an LLM or use the network. If `index.db` is missing or locked, `PreInvocation` returns `{}` and `PreToolUse` returns `{"decision": "allow"}`.
- **Degraded modes:**
  - **No ticks (CLI-only session or sidecar disabled):** `Stop` still harvests warm-cache turn notes and copies transcripts; `PreInvocation` and `PreToolUse` still deliver from `index.db`; MCP pull tools work normally; consolidation waits for the next `repo-memory tick`.
  - **No `index.db`:** The managed `<!-- repo-memory:begin v2 -->` block in `GEMINI.md` / `CLAUDE.md` still loads `.memory/ATLAS.md`, and agents can read `.memory/dossiers/<scope>.md` directly or run `repo-memory reindex`.

---

## 8. Validity, Lifecycle Re-checking, and Skill Graduation

Re-checks are governed by each entry's **cognitive lifecycle (`kind: semantic | procedural | episodic`, §5.3.2)** and dependency changes — never by age or retrieval frequency.

| Change detected | Entries affected | Action |
|---|---|---|
| Working-tree or commit changes an anchored file/symbol | **`semantic`** entries (`invariant`, `contract`, `gotcha`) anchored to the file/symbol | Checked at collection and before every push (§7.1). If the anchor fingerprint no longer matches, marked `suspect` (not pushed) and flagged for Stage 2 consolidation to amend or retire. |
| Build or environment manifest changes (`Makefile`, `CMakeLists.txt`, `platformio.ini`, `sdkconfig*`, `go.mod`, `package.json`, `.github/workflows/*.yml`) | **`procedural`** `recipe` (`procedure`) entries anchored to that manifest | Flagged `stale_suspect: true` until the recipe's command next executes with exit code `0` (Addendum A §10.1). |
| `PostToolUse` observes a shell command matching an active `recipe` | **`procedural`** `recipe` (`procedure`) entry | Exit code `0` increments `success_count` and clears `stale_suspect`; non-zero exit increments `failure_count` (2 consecutive failed sessions flag `stale_suspect: true`). |
| A `recipe` reaches `success_count >= 2` across ≥ 2 distinct sessions (or has `render_as_skill: rm-<slug>`) | **`procedural`** `recipe` (`procedure`) entry | Deterministically rendered to `.agents/skills/rm-<slug>/SKILL.md` with `generated_by: repo-memory` (Addendum A §10.2). While the skill exists, `first-touch` pushes shrink to a 1-line pointer (`"skill rm-<slug> covers this procedure"`). |
| Curated markdown doc (`source_doc_path`) changes its git blob SHA in `git diff` | `source: doc-seed` entries derived from that doc | `tick` queues an async "dreaming" re-distillation pass (§6.5) that emits `amend`, `supersede`, or `retire` ops. |
| A closure condition fires (`issue` closed, `branch` merged, `workorder` complete, `outcome` passed, or path/symbol/pattern condition met) | **`semantic`** `open-issue` entries whose `closes_when` lists it (§5.3.2) | `tick` emits a `retire` op moving the entry to `<scope>.history.md` with the closing event as evidence. |
| Newer architectural decision or revert on `main` | **`episodic`** `decision` / `ruled-out` entries | `decision` is superseded by a `supersede` op; a reverted commit re-evaluates entries citing that commit as evidence (Addendum A §12.4). |

---

## 9. Coverage and Elicitation

### 9.1 Coverage and Gap Detection (Addendum A §7)

For each dossier scope, the index tracks:
- active entry counts by cognitive lifecycle (`semantic`, `procedural`, `episodic`);
- share of `suspect` or `stale_suspect` entries;
- paths or subsystems with zero dossier entries or coverage below `gaps.threshold`.

When `memory_brief` is called on a scope or task with missing coverage, the response ends with an explicit gap line (`"No memory covers <path>. Absence of memory is not evidence; read the code."`, Addendum A §7.3).

### 9.2 Weekly Review Report

When pending review ops or high-priority coverage gaps exist, `tick` can start a weekly IDE-visible review conversation (`[repo-memory]` title prefix via `agentapi new-conversation`). Structural author classification (§5.1) ensures only human-typed replies (`USER_EXPLICIT`) count as confirmation evidence (P13).

---

## 10. Measurement and Evaluation Discipline

### 10.1 Empirical Findings (PR-06 through PR-10 Stage 1)

1. **X6-lite B1 Replay (`docs/evaluations/x6-lite-verdict.md`):** Replaying 5 historical work orders (92 runs, ~22.1M tokens) showed an **84.0% No-Memory baseline pass rate** on 150-line spec-style work orders (ceiling effect). On the single non-ceiling task (`T2: pr-01/wo4`), a 270-token anchored dossier reduced steps by **31.6%** (19 vs. 27.8) and duration by **29.8%** vs. No-Memory, and beat the 3,500-token PA pack by **24.0% fewer steps** at **92% fewer injected tokens**.
2. **231-Transcript PA Retrieval Study (`docs/evaluations/pa-memory-retrieval-effectiveness.md`):** Across 231 multi-tier sessions (`19,586` steps), `MEMORY.md` was read in only **1.3%** of sessions and workers made **0 MCP pull calls**, proving `P14` (push-first delivery is mandatory).
3. **77-Session `esp32` Static Doc Audit (`docs/evaluations/memory-value-evaluation-proposal.md`):** Curated human-authored repo guides (`docs/developer/`, `.agents/skills/`) were read in only **1.3%–7.8%** of sessions unless prompted, motivating `repo-memory seed-docs` (§6.3.3).

### 10.2 Paired Evaluation Discipline (Addendum A §9 & Eval v2)

All evaluations (`repo-memory eval`, `X6`, and `PR-10-eval-v2`'s 2×2 forward-task design across `repo-memory-plugin` and `esp32-app-foundation`) enforce Addendum A §9 (A10):
- **Non-ceiling task selection:** Tasks are brief, realistic bug-fix or feature prompts where the unassisted No-Memory baseline pass rate or efficiency has measurable headroom.
- **2×2 Factorial Arms (`PR-10-eval-v2`):** Evaluates Push (`PreInvocation` persistent `userMessage` + `ephemeralMessage` hints) × Pull (`ATLAS.md` + MCP tools) against No-Memory and PA packs from identical parent commits, with ≥ 3 trials per arm.
- **Statistical reporting:** Paired Wilcoxon signed-rank / paired t-test with Holm multiple-testing correction, 95% confidence intervals, and a mandatory `"What this does not show"` section.

### 10.3 Observability (`memory_status` / `memory_stats` & `repo-memory report`)

Metrics are recorded in `<data-dir>/repos/<repo-id>/metrics/*.jsonl`:
- push injections by moment (`task-start`, `first-entry`, `first-touch`, `about-to-edit`, `error-match`, `re-injection`), channel (`userMessage`/`systemMessage` vs `ephemeralMessage`), entry IDs, and token counts;
- `DetectCompaction` and `DetectRewind` recovery counts;
- MCP pull tool calls (`memory_brief`, `memory_why`, `memory_history`, `memory_search`, `memory_note`, `memory_status`);
- hook execution latency (p50/p95/p99);
- Stage 1 `Stop` warm-cache turn notes vs. Stage 1b free harvests vs. Stage 1c sidecar fallback and Stage 2 consolidation token spend;
- P13 echo-filter suppressions and Pre-Consolidation Corroboration Gate downgrades.

---

## 11. Security and Governance

1. **Privacy:** Raw transcripts and episodes stay in `<data-dir>/repos/<repo-id>/`. Only scrubbed op-log files (`.memory/ops/`), rendered dossiers (`.memory/dossiers/`), `.memory/ATLAS.md`, and graduated `.agents/skills/rm-<slug>/SKILL.md` files enter git.
2. **Layered Secret Scrubber (Addendum A §5 / A4):** Runs at every write boundary (harvest, compaction, inbox, op-log write, publish) using `.memory/scrub.yaml` + local `scrub.local.yaml`, a known-ID allow-set, regex detectors, and Shannon entropy checks. Any scanner failure fails closed and quarantines the payload in `<data-dir>/repos/<repo-id>/quarantine/`.
3. **Instruction-Pollution & Poisoning Defences (§6.4):** The preparation layer stubs skill/rule file reads, strips `[repo-memory]` injections, wraps transcript steps as JSON data inside random delimiters, and enforces the Pre-Consolidation Corroboration Gate (`VD18`) before any observation can become an active rule or `gate: edit`.
4. **CI Integrity Lint (`repo-memory doctor` / lint):** Verifies op-log schema validity, confirms that `.memory/dossiers/*.md`, `.memory/ATLAS.md`, and `.agents/skills/rm-*/SKILL.md` match deterministic regeneration from `.memory/ops/`, checks `.memory/scrub.yaml`, and ensures zero secrets.

---

## 12. Harness Integration

### 12.1 Antigravity (`agy 1.2.16–1.3.3`)

Verified against Antigravity's built-in docs, binary descriptors, and live experiments `E1`–`E12` and `X1`, `X3`, `X5`, `X8`, `X9` (see [docs/antigravity-hooks-reference.md](docs/antigravity-hooks-reference.md)):

**Event path (hooks — workspace plugin `repo-memory`):**

| Hook | Used for | Contract & Return Value |
|---|---|---|
| `PreInvocation` (every model call) | 1. Run `DetectCompaction` and `DetectRewind(steps, ledger.LastKnownStepCount)`.<br>2. Inject persistent context (`task-start`, `first-entry`, `first-touch`, `re-injection`) via `injectSteps[].userMessage` (plain string with `[repo-memory ...]` prefix, logged as `source: SYSTEM_SDK` and persisted until `CHECKPOINT`; `systemMessage` is rejected in `agy 1.3.3`).<br>3. Inject 1-step hints (`about-to-edit`, `error-match`) via `injectSteps[].ephemeralMessage`. | `{"injectSteps": [{"userMessage": "..."}, {"ephemeralMessage": "..."}]}` when due; otherwise `{}`. Note: in child subagents (`invoke_subagent`), the first `PreInvocation` fires at `initialNumSteps == 0` with an empty prompt before parent message delivery (X5). |
| `PreToolUse` | `gate-once` for invariants marked `gate: edit` and risky hardware/release commands (§7.3). | **MUST return `{"decision": "allow"}` (or empty output) to proceed.** Returning `{}` or exiting with code `2` denies the tool call (`exit 1` fails open; E8, E9, X9). |
| `PostToolUse` | Read the transcript step at `stepIdx` (`GENERIC` step holds tool output; Q28) to: (a) record touched files/scopes for the next `PreInvocation`, (b) match failed command output against `failure-signature` patterns, (c) track `recipe` command exit codes (`success_count`/`failure_count`), and (d) record child subagent `conversationId` on `invoke_subagent`. | `{}` |
| `Stop` | 1. Incremental copy of `transcript_full.jsonl` + spool entry.<br>2. When `executionNum == 0` and the milestone gate (§6.3.1) passes (`terminationReason == "NO_TOOL_CALL"`, `fullyIdle == true`), return `{"decision": "continue", "reason": "<request>"}` to harvest a warm-cache turn note.<br>3. When `executionNum >= 1`, harvest the `repo-memory-note` block + `ExposureList` to the inbox and advance the mark. | `{"decision": "continue", "reason": "..."}` on milestone `executionNum == 0`; `{}` otherwise (X1). |

**Two-Plugin Packaging Layout (Phase 0 Spike):**

```
~/.gemini/config/plugins/repo-memory-daemon/               # Global plugin (enabled globally)
├── plugin.json                                            # NO hooks.json, NO mcp_config.json
├── sidecars/tick/sidecar.json                             # builtin: schedule (* * * * *) → `<bin> tick`
├── bin/repo-memory                                        # Compiled Go binary
└── workspace-plugin/repo-memory/                          # Workspace plugin (enabled per repo via .agents/plugins.json)
    ├── plugin.json
    ├── hooks/hooks.json                                   # Generated with ABSOLUTE paths → `<bin> hook antigravity <event>`
    ├── mcp_config.json                                    # Stdio MCP server → `<bin> mcp`
    ├── rules/AGENTS.md
    └── skills/memory/SKILL.md
```

When `repo-memory enroll` (`init`) runs inside a repository, it writes the resolved **absolute path** of `workspace-plugin/repo-memory` into `<repo>/.agents/plugins.json` (since `~` and `$HOME` are not expanded by the plugin loader) and writes `.agents/agents/repo-memory-distiller.md`.

#### 12.1.1 Resolved Spike & Experiment Ledger (`E1`–`E12`, `X1`–`X9`)

| # | Experiment | Verified Finding |
|---|---|---|
| E1 / X5 | `injectSteps` channels & persistence | `ephemeralMessage` lasts 1 invocation only and is never written to `transcript.jsonl`. `systemMessage` is rejected in `agy 1.3.3` (`cannot convert string to message`). `userMessage` accepts a plain string, is written to `transcript.jsonl` with `source: SYSTEM_SDK`, and persists across turns until `CHECKPOINT`. |
| E2 / A8 | `@[label](path)` & managed blocks | `@[memory atlas](.memory/ATLAS.md)` in `GEMINI.md` inlines relative to the repo root (`24 KB` / `20k` token cap). Managed via `<!-- repo-memory:begin v2 -->` block (`VD19`). |
| E3 | Transcript currency | `transcript_full.jsonl` is flushed before `PreInvocation`, `PreToolUse`, `PostToolUse`, and `Stop` fire. |
| E4 | Native rule frontmatter | `trigger: model_decision` is required (`activation:` is ignored). |
| E5 / Phase 0 | Hook latency & non-enrolled isolation | Enrolled hooks run in p50 ~3ms, p95 ~6.1ms. Splitting `repo-memory-daemon` (global sidecar) from `workspace-plugin/repo-memory` (`.agents/plugins.json`) gives 0ms overhead in unenrolled repos. |
| E6 | Scheduled sidecar lifecycle | Sidecar runs every 60s as a child of the IDE `language_server`; `tick.lock` prevents overlap. |
| E7 | CLI `--project` isolation | `agy -p` ignores `--project`; isolation uses dedicated `runs/` working directory + `REPO_MEMORY_HOUSEKEEPING=1` / `REPO_MEMORY_INTERNAL=1`. |
| E8 | Subagent hook lifecycle | Subagents fire all hooks with their own `conversationId`. In `invoke_subagent`, child `PreInvocation` (`initialNumSteps == 0`) fires before the parent prompt is populated. |
| E9 / X9 | `PreToolUse` exit codes & deny | `{}` or exit code `2` denies the tool call; `exit 1` fails open; `{"decision": "allow"}` or empty stdout allows. Parallel tool denials are isolated. |
| E10 / X8 | `repo-memory-distiller` overhead | Custom agent in `.agents/agents/repo-memory-distiller.md` (`model: flash`, `enable_write_tools: false`, `enable_mcp_tools: false`, `enable_subagent_tools: false`) cuts fixed input overhead by 38.4% (~6.6k tokens saved per call). |
| E11 / X3 | Quota pools & `/usage` | `agy -p "/usage" --output-format json` costs 0 tokens and reports separate 5-hour and weekly pools for Gemini Pro, Gemini Flash, and Claude/GPT. Prompt cache lasts ~5 minutes (94–99% hit rate). |
| E12 | Transcript files & chunks | Read `transcript_full.jsonl` first, then `chunks/transcript_full/*.jsonl` concatenated via `io.MultiReader`, then `transcript.jsonl`. Step gaps are normal internal dispatch steps. |
| X1 | `Stop` `"continue"` warm-cache note | `Stop` checks `terminationReason == "NO_TOOL_CALL"`, `fullyIdle == true`, `executionNum == 0`, returns `{"decision": "continue"}`, and harvests the `repo-memory-note` block on `executionNum >= 1`. |

### 12.2 Claude Code [R2]

Not built in Release 1. Release 1 keeps these paths open through the adapter interface and portability rules in §18.

Verified against the Claude Code documentation (`code.claude.com/docs`, 2026-09-30) and Claude Code 2.1.284. The full analysis is in [`docs/claude_architecture.md`](docs/claude_architecture.md).

**Event path (hooks):**

| Need | Hook | Notes |
|---|---|---|
| Capture | `Stop` / `SubagentStop`: spool entry, transcript copy, side-log append | `SessionEnd` has only a 1.5 s shared budget, so it writes a spool file at most |
| Task brief | `UserPromptSubmit` receives `prompt` directly and returns `additionalContext` (30 s default timeout) | Fires on every prompt, so the brief can refresh when the task changes |
| File touch | `PostToolUse` (`tool_name`, `tool_input`, e.g. `Read`/`Edit`/`Write`) → `additionalContext` | No transcript parsing on the hot path |
| Error | `PostToolUseFailure` (`error`) and `PostToolUse` (`tool_response`) → `additionalContext` | |
| Risky action | `PreToolUse`: `additionalContext` informs without blocking; `permissionDecision` `deny`/`ask` with `permissionDecisionReason` for hard gates; `if` conditions such as `Bash(pio run *)` narrow the matcher | Soft delivery doesn't need the gate-once round-trip |
| Déjà-vu tier 1 | `PreToolUse` on `tool_input`; `PostToolBatch` reads the latest reasoning from the transcript tail | `PostToolBatch` (after a tool batch, before the next model call) is the nearest equivalent of `PreInvocation` |
| Déjà-vu tier 2 | `PostToolBatch` spools the request and starts an `asyncRewake` command hook, which exits 2 with the alert on a match | Seconds, not a tick. A plugin monitor (experimental, interactive sessions only) is a second choice. |
| Subagent briefing | `SubagentStart` → `additionalContext`. Hooks inside subagents carry `agent_id`/`agent_type`. | PA workers get the brief and in-session delivery automatically (answers E8 for Claude Code) |
| Git-change re-checks | `PostToolUse` on `Bash` with a git or dependency command | |
| Tick trigger | `Stop` and `SessionStart` command hooks with `async: true`, whose command is `repo-memory tick` | Harness-run background tick (§4.1). Output is discarded; `tick.lock` dedupes. Lifetime after session end is C3. |

Housekeeping runs set `REPO_MEMORY_HOUSEKEEPING=1`. Hooks exit immediately when it is set, although `--safe-mode` already prevents them loading.

**Other mechanisms:**

| Need | Mechanism | Notes |
|---|---|---|
| Map | `@.memory/views/MAP.md` import in the repo's `CLAUDE.md` | The syntax differs from Antigravity's `@[label](path)`, so each harness's include goes in its own override file (`CLAUDE.md`, `GEMINI.md`), not the shared `AGENTS.md` |
| Path-scoped native delivery | Generated `.claude/rules/*.md` with `paths:` frontmatter, loaded when Claude reads matching files | Where `.claude` links to `.agents` (as in esp32-app-foundation), one generated file in `.agents/rules/` could carry both harnesses' frontmatter (C6, with E4) |
| Pull tools | The same stdio MCP server, via the plugin's `.mcp.json` | Claude Code can defer MCP tool schemas until searched, which lowers the standing cost |
| Housekeeping | `claude -p --safe-mode --tools "" --system-prompt-file <stage> --json-schema <schema> --output-format json --model haiku --max-turns 1 --no-session-persistence --permission-prompts none`, with the payload on stdin | `--safe-mode` skips plugins, hooks, MCP, `CLAUDE.md` and auto memory while keeping subscription auth. `--bare` would also do that but requires an API key. Used only where `housekeeping.executor: claude` (D2). The standalone CLI must be signed in (`claude auth login` or `claude setup-token`); the desktop app's sign-in doesn't carry over. |
| Reports | A Desktop local scheduled task (weekly), visible under **Scheduled** | Runs only while the app is open and the machine awake, with one catch-up run after sleep. Set its permission mode and always-allow the memory tools on the first run, or it stalls (C7). |
| Tick when Antigravity isn't installed | Async-hook triggers (above); optional OS scheduler entry for calendar jobs | Desktop scheduled tasks and cloud routines run model sessions or lack local files, so they're unsuitable for the tick |
| Plugin packaging | `.claude-plugin/plugin.json`, `hooks/hooks.json`, `.mcp.json`, `skills/`; distributed via a marketplace | Plugin-installed dependencies go in `${CLAUDE_PLUGIN_DATA}`; all memory state stays in the shared `<data-dir>` |

#### 12.2.1 Transcript parsing

Anthropic documents the transcript JSONL as internal and version-dependent. The Claude Code parser:
- depends only on stable essentials: role, text, thinking, tool use and result, timestamps, subagent linkage;
- ignores unknown entry types rather than failing;
- records `harness_version` per episode;
- is tested against a fixture corpus per Claude Code version, with a canary on each update that flags drops in parsed-step counts.

The `Stop` side-log (§6.2) limits the loss when a parser breaks.

#### 12.2.2 Coexisting with Claude Code auto memory

Claude Code's auto memory is on by default. For esp32-app-foundation it already holds 13 files, mixing personal workflow preferences with repo gotchas such as native stub shadowing and CrowPanel bring-up. It's written by the working agent, private to the machine, and loaded into every main session, but not into subagents.

1. **Import as evidence.** Auto-memory files are ingested as import episodes with `provenance: claude-auto-memory`. `feedback` entries count as human-stated; `project` and `reference` entries become `observed` candidates, verified like any other.
2. **Steer, don't disable.** A line in `CLAUDE.md` says that repository knowledge is captured automatically by repo-memory, so auto memory should keep to personal preferences. `autoMemoryEnabled` is left to the user.
3. **Never write into auto memory.** It's the user's private store.

#### 12.2.3 Claude Code experiments

| # | Question | Method |
|---|---|---|
| C1 | Fixed overhead, latency and quota impact of the housekeeping invocation above, on the subscription | After `claude auth login`, run the same trivial prompt as the `agy -p` measurement; compare `usage` with the default invocation |
| C2 | Does `additionalContext` persist in context on later turns, and does it appear in the transcript? | Inject a unique marker; probe five turns later; inspect the JSONL |
| C3 | Does an `async: true` hook process survive session end, `/clear`, or the desktop app quitting? | An async hook that writes a marker after 60 s |
| C4 | Is `asyncRewake` delivery reliable and non-disruptive mid-turn? | Scripted session with a seeded ruled-out claim |
| C5 | Parser robustness across Claude Code versions | Fixture corpus; re-run on each update |
| C6 | Does one rule file with Claude Code `paths:` and Antigravity `model_decision` frontmatter load correctly in both harnesses? (Antigravity `glob` isn't used; see E4.) | Minimal rule in `.agents/rules/` via the `.claude` symlink |
| C7 | Is a Desktop local scheduled task workable as the report surface? | Weekly test task; check permission stalls and catch-up behaviour |

### 12.3 Tick

`repo-memory tick` is a subcommand of the `repo-memory` Go binary (§18). It is not a daemon: each run does its work and exits. The order of work, the job table and the budgets are defined in §4.3.

- **Idle cost.** A tick with nothing to do makes no LLM calls and finishes in well under a second (program start-up plus a few file and hash checks).
- **Git events** are detected by comparing HEAD, `dependencies.lock` and the sdkconfig hashes with the previous tick. No git hooks are installed.
- **Housekeeping runs** are child processes started with `REPO_MEMORY_HOUSEKEEPING=1`, the working directory `<data-dir>/runs/<repo-id>/`, the configured model and `--print-timeout`. The tick waits for them within its budget; a run still going when the budget ends keeps the tick alive until its timeout, and the next tick skips (`tick.lock`).
- **Reports** are started with `agentapi new-conversation --title="[repo-memory] …"` and polled on later ticks, as in the bug-fixer tick dispatcher.
- **Publishing** runs git in a dedicated worktree (`<data-dir>/<repo-id>/worktrees/<clone-id>/`, branch `memory/pending`), never in the user's checkout. Each clone of the same repository gets its own publishing worktree.

**`repo-memory run`** is kept as an alias for `tick`, for manual use.

---

## 13. Principal-architect integration

| PA Workflow Activity | With `repo-memory v2.2` |
|---|---|
| Worker finishes a milestone turn | `Stop` hook (`NO_TOOL_CALL`, `executionNum == 0`) automatically requests and harvests a warm-cache `repo-memory-note` block; `repo-memory seed` also deterministically parses PA `WORKORDER.md` `Decisions Made` and `Dead Ends & Warnings` (`source: pa`). |
| Worker starts a task or touches files in a subagent | Because passive `MEMORY.md` files go 98.7% unread by workers (§10.1), `PreInvocation` pushes anchored dossier invariants, contracts, gotchas, and ruled-out approaches directly into the worker's context (`userMessage` / `systemMessage`). |
| Repeated multi-step procedures | `PostToolUse` tracks command exit codes (`success_count` / `failure_count`) and graduates verified `recipe` entries into `.agents/skills/rm-<slug>/SKILL.md` (Addendum A §10). |
| Abandoned work orders | Branch-local ops (`.memory/local/`) preserve `ruled-out` and `failure-signature` entries even when a feature branch is abandoned. |

---

## 14. Migration, Seeding (`seed`), and Doc-Seeding (`seed-docs`)

Existing repository artefacts and documentation are ingested directly into the v2.2 inbox and op-log pipeline:

| Source | CLI Command & Provenance | Treatment |
|---|---|---|
| Curated developer guides (`docs/developer/**/*.md`) & hand-authored skills (`.agents/skills/**/SKILL.md`) | `repo-memory seed-docs` (`source: doc-seed`, Priority 0 in Addendum A §11.1) | Distilled via `repo-memory-distiller` into anchored `invariant`, `contract`, `gotcha`, `recipe`, and `failure-signature` entries with `source_doc_path` and `source_doc_blob_sha`; kept fresh asynchronously on `git diff` ("dreaming" in `tick`). |
| PA `WORKORDER.md` completion reports & `PROJECT.md` session logs | `repo-memory seed` (`source: pa`) | Deterministic parser extracts `Decisions Made` (`decision`) and `Dead Ends & Warnings` (`ruled-out`, `failure-signature`, `gotcha`), routes to dossiers, writes `.memory/ops/`, renders `.memory/dossiers/` and `.memory/ATLAS.md`, and builds `index.db`. |
| Surviving Antigravity & **[R2]** Claude Code transcripts | `repo-memory tick` sweep + fallback (`source: sidecar`, `source: checkpoint`, `source: artifact`) | `CHECKPOINT` summaries and `brain/` artifacts harvested at 0 token cost; uncovered milestone tails distilled by `repo-memory-distiller`. |

---

## 15. Build Phases

### 15.1 Release 1: Antigravity (`v1` → `v2.2`)

| Phase | Scope | Status & Exit Criteria |
|---|---|---|
| **PR-01 – PR-05 (v1 Foundation & Capture)** | Go binary skeleton, `hook`/`tick`/`install`, transcript parser (`E12`), compaction, scrubber, job table, FTS5 episode search, `E1`–`E12` experiments. | **Complete.** All `E1`–`E12` experiments resolved (`docs/experiments/SPEC-CHANGES.md`). |
| **PR-06 / PR-06b (v2 Spike & Two-Plugin Split)** | Experiments `X1`, `X3`, `X5`, `X8`, `X9`; `Stop` `"continue"` warm-cache note prototype; `repo-memory-distiller` custom agent; global `repo-memory-daemon` vs. workspace `repo-memory` split (`.agents/plugins.json`). | **Complete.** 38.4% fixed token reduction in X8; 0ms non-enrolled overhead (`docs/experiments/spike-summary.md`, `phase0-plugin-split.md`). |
| **PR-07 (v2 Op-Log & Dossier Storage)** | Append-only op log (`.memory/ops/`), dossier schema & deterministic renderer (`.memory/dossiers/`, `.history.md`, `.memory/ATLAS.md`), scope resolver, SQLite `index.db` migration, PA work-order seeder (`repo-memory seed`). | **Complete.** 174 entries seeded across dossiers from 49 PA work orders. |
| **PR-08 / PR-09b (v2 Delivery Engine & Ranker)** | `PreInvocation` push moments, `PreToolUse` `gate-once`, cross-process `InjectionLedger` (`InjectedSet`, `DetectCompaction`, `DetectRewind`), scaled FTS5 ranking (`ftsScore * 100`, `DefaultMinRelevanceScore = 50`), canonical `index.IndexPath`, 6 MCP pull tools. | **Complete.** |
| **PR-09 (X6-lite Evaluation & Retrieval Audit)** | 5-task historical replay (`X6-lite`) + 231-transcript PA retrieval study (`docs/evaluations/`). | **Complete.** Identified 84% spec-task baseline ceiling, 31.6% step reduction on `T2`, 1.3% passive `MEMORY.md` read rate (`P14`), and the `ephemeralMessage` + `InjectedSet` persistence bug. |
| **PR-10a (`v2.2` Specs & Design Rev 4)** | Upgrade `design-v2-distillation-retrieval.md` to Rev 4 (`v2.2`), `addendum-a-provenance-and-hardening.md` to Rev 3, `SPEC.md` to `v1.0`, `antigravity-hooks-reference.md`, and `README.md`. | **Complete (`PR-10a`).** |
| **PR-10b (Delivery Persistence & Storage Fixes)** | Restore multi-step persistence via `UserMessage` (`VD13`), wire `DetectRewind` in `HandlePreInvocation`, optional `{section?}` dossier rendering, Pre-Consolidation Corroboration Gate (`VD18`), multi-turn `stream-json` consolidation (`VD15`), and issues `#10`–`#13`, `#16`. | **Next (`PR-10b`).** |
| **PR-11 (Doc-Seeded Dossiers & Cognitive Lifecycles)** | `repo-memory seed-docs` (`source: doc-seed`) + async "dreaming" freshness (`VD16`), 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`) + 5-tier scope hierarchy (`VD17`), `PostToolUse` procedural command validation + `.agents/skills/rm-<slug>/SKILL.md` graduation. | **Planned (`PR-11`).** |
| **PR-10-eval-v2 Stage 2 (`WO4`–`WO7`)** | 2×2 forward-task evaluation on `esp32-app-foundation` and `repo-memory-plugin` with the complete `v2.2` stack. | **Resumes after `PR-10b` / `PR-11`.** |

### 15.2 Release 2: Claude Code `[R2]`

Release 2 reuses `internal/core` unchanged and adds `internal/harness/claudecode` (`UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `SubagentStart`, `Stop` `{"decision": "block"}`, `PreCompact`, and `claude -p --safe-mode`; §12.2).

---

## 16. Resolved Questions and Risk Mitigations

| # | Question or Risk | Resolution in `v1.0` (`v2.2`) |
|---|---|---|
| Q1–Q2 | Atomic claim fragmentation & LLM page-rewriting drift | **Resolved:** Replaced atomic claim files with cohesive subsystem dossiers rendered deterministically from an append-only op log (`.memory/ops/`), with optional `{section?}` resolution (§5.3, §5.4). |
| Q4 / E12 | Antigravity raw transcript format & chunks | **Resolved:** Read `transcript_full.jsonl` first, then `chunks/transcript_full/*.jsonl` concatenated via `io.MultiReader`, then `transcript.jsonl` (§6.2). |
| Q5 / VD13 | `ephemeralMessage` vs. `userMessage` / `systemMessage` persistence | **Resolved:** `ephemeralMessage` lasts 1 invocation only; `systemMessage` is rejected in `agy 1.3.3`; persistent pushes (`task-start`, `first-entry`, `first-touch`, `re-injection`) use `userMessage` (`source: SYSTEM_SDK`, `[repo-memory]` prefix) until `CHECKPOINT`, while `ephemeralMessage` is reserved for 1-step hints (§7.1). |
| Q7 / Q14 | Fixed ~17k token overhead of `agy -p` & per-item LLM calls | **Resolved:** Stage 1 primary distillation runs in-session at turn end via `Stop` `"continue"` reusing the 5-minute warm KV cache (~90% discount; X1, X3). Sidecar fallback and Stage 2 consolidation use `.agents/agents/repo-memory-distiller.md` (38.4% lower overhead; X8) over a single multi-turn `stream-json` session per tick (§4.4, §6.5). |
| Q11 / Q24 | Non-enrolled repository hook latency (~57ms in E5) | **Resolved:** Two-Plugin Packaging Split (§4.6, §12.1) puts `sidecar.json` in global `repo-memory-daemon` and `hooks.json` + `mcp_config.json` in workspace `repo-memory` (enabled only in enrolled repos via `.agents/plugins.json`), reducing non-enrolled overhead to **0 ms**. |
| Q15 / E8 | Subagent hook coverage | **Resolved:** Subagents fire all hooks with unique `conversationId`s; parent `PostToolUse` on `invoke_subagent` links child → parent in `state/subagents.sqlite` (§6.3.1, §12.1). |
| Q19 | Circular reinforcement (echoes) & uncorroborated hypotheses | **Resolved:** P13 deterministic echo filter using per-conversation `ExposureList` + Pre-Consolidation Corroboration Gate (`VD18`, §6.4–§6.5). |
| Q22 | Stale open issues & broken procedures | **Resolved:** `open-issue` entries carry deterministic `closes_when` conditions (§5.3.2); `procedural` recipes track `PostToolUse` command exit codes (`success_count` / `failure_count`) and build-manifest changes (§8, Addendum A §10). |
| Q23 / P14 | Do agents pull memory unprompted? | **Resolved (Empirically No):** 231-session PA study showed 1.3% `MEMORY.md` reads and 0 MCP pull calls; 77-session `esp32` audit showed 1.3%–7.8% static doc reads. Delivery is therefore push-first (`PreInvocation` + `seed-docs`), with MCP tools for on-demand depth (§2 P14, §10.1). |

---

## 17. Implementation Decisions

| # | Decision | Status |
|---|---|---|
| D1 | Implementation language for hooks, tick, pipeline and MCP server | **Decided: Go.** One cgo-free binary (`CGO_ENABLED=0`, §18). |
| D2 | Housekeeping executor per machine | **Decided.** Release 1: `agy -p --agent repo-memory-distiller` over `stream-json` after `/usage` quota check. **[R2]** `claude -p --safe-mode` on Claude Code-only machines. |
| D3 | Tick triggers & packaging split | **Decided.** Global `repo-memory-daemon` plugin runs the scheduled sidecar (`repo-memory tick`); per-repo `.agents/plugins.json` enables the workspace `repo-memory` hooks and MCP server. |
| D4 | Harness order | **Decided.** Release 1 targets Antigravity; Release 2 adds Claude Code behind `internal/harness`. |

---

## 18. Implementation Architecture (Go)

### 18.1 One Binary (`repo-memory`)

`repo-memory` is a single cgo-free Go binary (`CGO_ENABLED=0`) for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64.

| Subcommand | Role | Invoked by |
|---|---|---|
| `hook <harness> <event>` | Event path: read hook JSON on stdin, run `PreInvocation` / `PreToolUse` / `PostToolUse` / `Stop` logic, write harness JSON to stdout | Harness hooks |
| `tick` (alias `run`) | Time path: ingest spool, harvest `CHECKPOINT`/`brain/`, check `/usage`, run fallback & multi-turn `stream-json` consolidation, refresh `doc-seed` entries, render dossiers/skills/index, update memory PR | `repo-memory-daemon` sidecar; OS scheduler; CLI |
| `mcp` | Stateless stdio MCP server exposing `memory_brief`, `memory_why`, `memory_history`, `memory_search`, `memory_note`, `memory_status` (`memory_stats`) | Harness MCP config |
| `enroll` (alias `init`), `unenroll [--purge]` (alias `leave`) | Enrol or unenrol the current repository: `.memory/`, `.agents/plugins.json` (resolved absolute path), `.agents/agents/repo-memory-distiller.md`, managed `v2` block in `GEMINI.md`/`CLAUDE.md`, and `<data-dir>/registry.json` (§4.6) | User, once per repository |
| `seed`, `seed-docs` | Bootstrap ops, dossiers, `ATLAS.md`, and `index.db` from PA work orders (`seed`, `source: pa`) and curated repository markdown guides/skills (`seed-docs`, `source: doc-seed`) (§6.3, §14) | User / CI |
| `reindex` | Rebuild `<data-dir>/repos/<repo-id>/index.db` (via canonical `index.IndexPath`) and re-render dossiers/`ATLAS.md`/graduated skills from `.memory/ops/` | User / `tick` |
| `enable`, `disable [--repo]` | Toggle the fast kill switch (§4.6) | User |
| `install --harness antigravity`, `uninstall` | Install/remove the global `repo-memory-daemon` and nested `workspace-plugin/repo-memory` bundles with resolved absolute paths | User, once per machine |
| `search`, `status`, `report`, `usage`, `snapshot`, `doctor` | Operator inspection, `/usage` quota check, evaluation snapshot management, and CI lint | User / CI |

### 18.2 Package Layout

```
cmd/repo-memory/                 main and subcommand wiring (enroll, seed, seed-docs, reindex, hook, tick, mcp, …)
internal/core/                   harness-neutral; must not import any concrete adapter
  episode/  ops/  dossiers/  seed/  index/  jobs/  tick/
  prep/  consolidate/  delivery/  gitops/  scrub/  metrics/  config/
internal/harness/                interfaces and normalised types (Event, Injection, Capabilities, Step, CaptureSource)
internal/harness/antigravity/    Release 1: transcript source, hook codec, agy distiller executor,
                                 /usage quota checker, agentapi reporter, split plugin installer
internal/harness/claudecode/     [R2]
internal/harness/fake/           test adapter that drives the core test suite
internal/mcpserver/              stdio MCP server (memory_brief, memory_why, memory_history, memory_search, memory_note, memory_status)
prompts/                         distiller and consolidator prompts (go:embed)
packaging/antigravity/           repo-memory-daemon and workspace-plugin/repo-memory templates
testdata/fixtures/antigravity/   scrubbed transcripts, checkpoints, and hook payloads
```

A package-boundary test verifies that nothing under `internal/core` imports `internal/harness/antigravity` or any other concrete adapter. All SQLite index accesses resolve through the single canonical helper `index.IndexPath(dataDir, repoID)` (`<data-dir>/repos/<repo-id>/index.db`).

### 18.3 Adapter interfaces and capabilities

```go
// internal/harness
type Harness interface {
    Name() string
    Capabilities() Capabilities
    Transcripts() TranscriptSource // discover changed transcripts; parse → []episode.Step
    Hooks() HookCodec              // native hook JSON ⇄ normalised Event / Injection
    Reporter() Reporter            // start and poll an IDE-visible report
}

type Capabilities struct {
    InjectBeforeModelCall bool // Antigravity PreInvocation
    InjectAtPrompt        bool // [R2] Claude Code UserPromptSubmit
    InjectAfterTool       bool // [R2] Claude Code PostToolUse additionalContext
    InjectAtSubagentStart bool // [R2] Claude Code SubagentStart
    SoftContextBeforeTool bool // [R2] Claude Code PreToolUse additionalContext
    BlockWithReason       bool // PreToolUse deny + reason (both)
    AsyncPush             bool // [R2] Claude Code asyncRewake
    HookPayloadHasToolIO  bool // false on Antigravity: read the transcript tail instead
}

// Executors are chosen per machine (D2), independently of the harness in use.
type Housekeeper interface {
    Run(ctx context.Context, stage Stage) (Result, error) // system prompt + schema + payload → JSON + usage
}
```

**The delivery policy is written once against capabilities:**

| Trigger | Mechanism chosen |
|---|---|
| Risky action | `SoftContextBeforeTool` if available; otherwise gate-once via `BlockWithReason` (§7.5) |
| Touch and error | `InjectAfterTool` if available; otherwise the next `InjectBeforeModelCall`, reading the transcript tail when `HookPayloadHasToolIO` is false |
| Déjà-vu tier 2 | `AsyncPush` if available; otherwise the tick |
| Task brief | `InjectAtPrompt` if available; otherwise `InjectBeforeModelCall` at invocation 0 |

Adding Claude Code in Release 2 is then a new codec with a capability declaration; the policy doesn't change.

### 18.4 Libraries and choices

All dependencies are pure Go; there is no cgo in Release 1.

| Need | Choice | Note |
|---|---|---|
| SQLite (index, jobs, FTS) | A pure-Go SQLite driver (e.g. `modernc.org/sqlite`) | `journal_mode=WAL` and `busy_timeout` on every connection; hooks open read-only; writers use short transactions. A full index rebuild writes a new versioned file and then switches a small pointer file. It never renames over the live index, because Windows can't rename over a file that readers have open. Confirm FTS5 support in P0. |
| MCP server | The official MCP Go SDK | Stdio transport |
| Episode compression | Pure-Go zstd | |
| Claim files | YAML front matter + Markdown | |
| Git | Shell out to `git` | Worktree operations for publishing (§6.8) |
| LLM calls | Subprocess `agy -p`; **[R2]** `claude -p` | No LLM SDK |
| Secret scanning | Built-in pattern and entropy rules; optionally a Go secret-detection library | Runs at compaction and before publish (§11.3) |
| C/C++ symbols | A brace-aware scanner plus file-window fingerprints; universal-ctags used if installed | Tree-sitter only if anchor quality demands it (Q6) |

### 18.5 Hooks in Go

- **Invocation.** `hooks.json` entries call `<absolute path>/repo-memory hook antigravity <Event>`, written by `install`, so they work under `sh -c` and `cmd /c` without `PATH`.
- **Path resolution:** Paths in `hooks.json` resolve relative to the directory containing `hooks.json` (i.e. `.agents/`), not the repo root. All generated hook entries must use absolute paths (E9).
- **Fast path.** The hook reads a per-conversation cursor (a byte offset into the transcript), reads only new bytes, checks `alerts/`, and opens SQLite read-only only when there is something to look up.
- **Budget.** p95 ≤ 60 ms on the fast path (§7.8), confirmed for enrolled repos (p95 ~6.1ms; E5). To satisfy the ≤ 5ms target on non-enrolled repos, hooks check for local repository markers before parsing disk state.
- **Failure behaviour.** On any error, the hook logs the error and returns the harness's permissive output (`{"decision": "allow"}` for `PreToolUse`, `{}` for others). Note that returning `{}` or exiting non-zero from `PreToolUse` is treated by the harness as an implicit deny (E8, E9).

### 18.6 Portability rules for Release 1

Release 1 follows these rules so that Release 2 is an adapter, not a rewrite:

1. **Core never sees native field names.** Everything harness-specific enters through the normalised types in `internal/harness` (`Event`, `Injection`, `Step`).
2. **Delivery decides by capability flags,** never by harness name (§18.3).
3. **Stage prompts and JSON schemas are executor-neutral files.** Executors only transport them.
4. **Stores, config and guards are harness-neutral:** the episode format, claim store, views, data directory, config keys (`housekeeping.executor`, `reports.surface`, `tick.trigger`) and the `REPO_MEMORY_HOUSEKEEPING` guard.
5. **Map and rule generation takes a target list.** Release 1 emits the Antigravity include and rules only, but the generator is written per target.
6. **Core tests run against the `fake` adapter,** so core can't silently depend on Antigravity behaviour.
7. **The adapter contract suite is reusable.** It is fixture in → expected normalised events and steps out, written in P2 for Antigravity and reused unchanged for Claude Code in R2.1.
8. **Episodes record `harness` and `harness_version`** from day one.
9. **Report content is generated by core** (Markdown plus question and claim IDs). The reporter only delivers it.
10. **Tick triggers are external.** The tick never assumes which harness or scheduler started it.
11. **Claude Code history is preserved from P0 (recommended).** The sweep copies Claude Code transcripts into `raw/claude-code/`, private and unparsed, with the same permissions as the originals. R2.1 compacts and scrubs them. This costs a few lines in the sweep and prevents the 30-day retention from erasing history before Release 2. Drop it for a strict Antigravity-only Release 1.

---

## Appendix A: Evidence from esp32-app-foundation (2026-09-29/30)

### A.1 Accumulated memory

- 1,201 commits and 118 merge commits since 2026-05-21.
- PA workspace:
  - ~100 project folders;
  - 1,364 markdown files, ~4.7 MB (~1.2M tokens), of which 99 `PROJECT.md` (547 KB) and 431 `WORKORDER.md` (2.9 MB);
  - index files 45 KB (`MEMORY.md` 4 KB, `MODULE_INDEX.md` 18 KB, `PROJECT_INDEX.md` 23 KB).
- OTA history: 10 project folders, 199 files, ~520 KB.
- Semantic layers:
  - `docs/design_specification.md` (196 KB; decision log D1–D54, one-line rationale, no PR links);
  - ~25 developer guides and 3 ADRs;
  - `framework-expert` skill (79 files; manifest last synced at PR-75 on 2026-08-28, while its content includes September BLE material).
- The OTA `_silentCheck` contract appears in none of the design spec, developer guides or skill. It is only in code comments and a June session record.
- The June OTA debugging record (`adhoc-ota-manifest-url-bug`, 2026-06-23) lists two latent defects that have since been fixed in `OTAUpdateManager.cpp`. The record still presents them as open.
- `pr-225-ble-manager/PROJECT.md` contains a hand-written "Framework Facts Workers Need (verified 2026-09-18 against main @ 69d9c813)" section. This is the artefact the task brief (§5.5) generates automatically, and it is used as benchmark B1.

### A.2 Transcript composition (Claude Code, 35 sessions for the repo)

| Content | Share |
|---|---|
| Tool results | 45.5% |
| Tool call inputs (incl. edit bodies) | 24.9% |
| Thinking | 16.1% |
| Assistant text | 9.6% |
| User text | 3.8% |

Total ~13.7M characters (~3.4M tokens). Median session ~190k characters (~47k tokens); largest ~2M characters.

### A.3 Transcript retention (Antigravity)

- 1,145 conversations recorded for the repo since 2026-05-21: 188 top-level and 957 subagent.
- Only **41** top-level and **138** subagent conversations still have transcripts on disk. The JSONL transcript (`brain/<id>/.system_generated/logs/transcript.jsonl`, 46 MB in total) and the conversation database exist for exactly the same sessions. The rest survive only as ~1 KB summaries in `conversation_summaries.db`.
- Of 566 JSONL transcripts across all projects, 340 contain `CHECKPOINT` steps and 257 have small step-index gaps. All start at step 0.
- Capture must therefore run continuously, and the tick sweep does this (P0).

### A.3a Transcript sizes (Antigravity, esp32, surviving transcripts, 2026-09-30)

| | Count | Median | Mean | Total | Tool traffic | Dialogue and reasoning |
|---|---|---|---|---|---|---|
| Top-level | 45 | 52k tokens | 120k tokens | 5.4M tokens | 86% | ~11% |
| Subagent | 138 | 29k tokens | 36k tokens | 5.0M tokens | 90% | ~8% |

Token counts are estimated at 4 characters per token from `content`, `thinking` and `tool_calls` fields.

### A.4 Harness facts relied upon

Sources: the built-in `agy-customizations` docs and the public docs pages, checked 2026-09-30.

- **Antigravity hooks:**
  - `PreToolUse`: `allow`/`deny`/`ask`/`force_ask`, `reason`, `permissionOverrides`, `overwrite`.
  - `PostToolUse`: input `stepIdx`, `error`; output `{}`.
  - `PreInvocation`: before every model call; `injectSteps` with `toolCall`/`userMessage`/`ephemeralMessage`; receives `invocationNum`, `initialNumSteps`.
  - `PostInvocation`: `injectSteps`, `terminationBehavior`.
  - `Stop`: `decision: "continue"` plus `reason`.
  - Common inputs: `conversationId`, `workspacePaths`, `transcriptPath`, `artifactDirectoryPath`, `modelName`.
  - Hooks run synchronously and block the agent loop, via `sh -c` on Unix and `cmd /c` on Windows. Default timeout 30 s.
  - There is no `SessionStart` event.
- **Antigravity sidecars:**
  - `sidecar.json` with `command` or `builtin: "schedule"` (five-field cron), `args`, `restart_policy`, `env`, `display_name`, `description`.
  - Plugin sidecars live under `plugins/<name>/sidecars/<id>/`, are disabled by default, and are enabled in `config.json`.
  - Data, logs and `agentapi` events go to `sidecar_data/<id>/`.
  - `agentapi new-conversation` / `send-message` start or continue agent conversations.
  - In practice they are periodic ticks. The bug-fixer plugin runs three scheduled sidecars on this model.
  - The built-in automation guidance says a plugin must never enable a sidecar itself (the user enables it in the Automations dashboard), and scheduled runs auto-deny anything not listed in `agent_permissions`.
- **Antigravity CLI (`agy`) print mode:**
  - `-p`/`--print` with `--model`, `--json-schema`, `--output-format json|stream-json`, `--input-format stream-json`, `--print-timeout`, `--project`, `--agent`, `--disable-slash-commands`, `--sandbox`.
  - The JSON output includes `conversation_id`, `status`, `response`, `duration_seconds`, `num_turns` and `usage` (input, output, thinking and cache-read tokens).
  - Headless conversations are recorded in the CLI store (`~/.gemini/antigravity-cli/`), not the app/IDE store.
  - `agy models` lists the models available on the subscription.
- **Antigravity Python SDK:** requires `GEMINI_API_KEY` (AI Studio), so it can't run on the subscription quota.
- **Antigravity rules:**
  - Root and directory `AGENTS.md`/`GEMINI.md` files, with `@[label](path)` includes.
  - 24 KB per file; a 20k-token aggregate budget for always-on and global rules, with overflow demoted to pointers.
  - MCP tools and skills count against a separate customization budget.
  - The built-in docs document `always_on` and `model_decision` rule activation; `glob` is undocumented there and reportedly deprecated (E4).
- **Claude Code** (2.1.284; full detail in `docs/claude_architecture.md`):
  - 32 hook events, including `UserPromptSubmit` (`prompt`), `PreToolUse` (`additionalContext`, `permissionDecision`), `PostToolUse` (`tool_input`, `tool_response`), `PostToolUseFailure`, `PostToolBatch`, `SubagentStart`/`SubagentStop`, `Stop`, and `SessionEnd` (1.5 s budget).
  - `async` and `asyncRewake` command hooks; `prompt` and `agent` hook types.
  - `claude -p` with `--safe-mode` (keeps auth), `--bare` (needs an API key), `--json-schema`, `--no-session-persistence`; `-p` sessions are excluded from the picker and from `--continue`.
  - Transcripts at `~/.claude/projects/<project>/<session>.jsonl` (+ `subagents/`), 30-day default retention, internal format.
  - `.claude/rules/` with `paths:`; `@path` imports (four hops); auto memory in `~/.claude/projects/<project>/memory/`.
  - Desktop local scheduled tasks (minimum 1 minute, app open, visible).
  - On this machine the standalone CLI is not signed in.
- **MCP:**
  - Stdio or SSE transport.
  - Discovered tools are listed in the system prompt of every session.
  - No lazy loading is documented.

## Appendix B: Relationship to prior inputs

- **Agentic Repo-Level Memory Systems (survey):** adopted Markdown-in-git as the shared substrate and a rebuildable local index. Rejected embedding-first retrieval as the primary mechanism, the post-tool-use extraction loop, and usage-based decay.
- **Dream-RSI (arXiv:2609.14858):** adopted outcome-over-claims trust, the failure-triage taxonomy, prefix-only replay for tuning the delivery policy, and in-sample no-regret policy selection. Not adopted: the transcript-as-discovery-tree mapping and counterfactual "would card X have prevented dead-end Z" analysis, which the replay mechanism can't support.
- **repo-memory-plugin-architecture-analysis.md:** superseded. See the inline review comments in that document.
- **Claude Code auto memory (reviewed 2026-09-30):** an agent-written memory with no pipeline and no re-checking. Its techniques were adopted in harness-neutral form in v0.5:
  - `relevant_when` and `apply` lines on claims (§5.3);
  - an agent-walkable claim index (§5.5);
  - currency stamps and an anchor-existence check on every push (§7.1);
  - closure conditions for work status (§5.3.1, §8);
  - memory-artefact writes as a triage signal (§6.4);
  - budget repair for synthesised views (§6.8);
  - referenced gaps in coverage (§9.1);
  - approvals as triage signals (§6.4).

  Claude-specific notes are kept in `docs/claude_architecture.md` §4.7 for the Release 2 adapter.
- **Third-party review (GitHub issue #1, 2026-10-01).** Every claim was checked against the built-in Antigravity docs and the transcripts on disk before being adopted.
  - Adopted:
    - `truncated_fields` resolution: read `transcript_full.jsonl`; chunks are byte slices to be concatenated, per the reviewer's follow-up, which was verified on all 537 conversations (§6.2; E12 answered);
    - incremental transcript copying (§6.2);
    - a repo ID that distinguishes template-derived repositories (§4.6);
    - a mechanical decision-marker signal for salience (§6.4);
    - a mechanical P13 precondition (§6.6);
    - `gate-once` restricted to irreversible actions, with E9 widened (§7.5);
    - tier-1 matching of current-turn reasoning at `PreToolUse` (§7.6);
    - sidecar `restart_policy` and `agent_permissions`, and user enablement via the dashboard (§12.1);
    - `model_decision` rather than `glob` for native rules (E4);
    - SQLite WAL, `busy_timeout` and a Windows-safe index swap (§18.4).
  - Not adopted:
    - the claim that `PostToolUse` ignores its matcher (the built-in hooks doc shows a matcher for it; the handler is nonetheless defensive);
    - the 46 MB transcript figure (the largest full transcript on disk is 3.9 MB);
    - section and path references that don't match this specification.
- **docs/claude_architecture.md (2026-09-30):** Claude Code review. All nine proposed changes were adopted in v0.4 (§4.1, §4.4, §6.2, §7.9, §12.2, §15).
- **External review comments (2026-09-30):**
  - housekeeping cost → §4.4.1 cost model, salience windows, interactive-call levers;
  - tier-2 latency → §7.6 tier 0/1/2 framing, generated match blocks;
  - report-reply attribution → §5.1 author classification, §9.2.1;
  - recursion via bug-fixer sessions → P13, exposure markers, §6.5–6.6;
  - Windows hook latency → D1 (§17), brought forward to before P0.
- **spec-critical-evaluation.md (2026-09-30):** these points were adopted:
  - the P1 exit gate on B1;
  - episode search as P0.5;
  - an instrumented observability layer and `memory_stats`, with all tools shipped;
  - a cost breakdown by PA activity tier (B4);
  - explicit entity-extraction exactness;
  - `ephemeralMessage` handled as an experiment;
  - degraded modes specified;
  - sweep-based capture for sessions where `Stop` doesn't fire.

  These were adopted in a different form:
  - **Daemon operations and portability.** Resolved with Antigravity's native scheduled sidecars rather than making the MCP server the sidecar. MCP servers are per-session and would tie the pipeline to open sessions.
  - **Auto-approval.** Resolved with two-lane publish and review profiles rather than a single default.
  - **Sharding.** Claims are sharded by creation month rather than subsystem, because claims span subsystems and the subsystem map gets regrouped.

## Revision history

| Version | Date | Changes |
|---|---|---|
| 0.1 | 2026-09-29 | Initial specification. |
| 0.2 | 2026-09-30 | Following `spec-critical-evaluation.md` and a re-check of the Antigravity built-in docs:<br>• Pipeline runs as Antigravity scheduled sidecars (periodic ticks); no daemon or socket.<br>• Capture moved to a transcript sweep, with `Stop` as a nudge and no `PostToolUse` capture hook.<br>• Recursion guard for pipeline-created conversations.<br>• Pluggable LLM backends (`api` / `agentapi`).<br>• Two-lane publish and review profiles.<br>• Delivery hook latency budget and compiled-hook option; degraded modes.<br>• Two-tier déjà-vu detection (immediate lexical match in the hook, semantic check on a 1-minute tick).<br>• Map loaded via an `AGENTS.md`/`CLAUDE.md` include.<br>• `memory_stats` and an observability layer.<br>• Entity-extraction exactness and subsystem bootstrap; month-sharded claims and scale estimate.<br>• B5 extraction-precision benchmark and P1 gates with a fallback; P0 experiments E1–E6; P0.5 episode search.<br>• Transcript location and format resolved (Q4). |
| 0.3 | 2026-09-30 | Execution model settled with the user:<br>• Two invocation paths with strictly separate jobs: hooks read, inject and spool; the scheduled tick does all asynchronous work. Harness-level, not OS-level; no detached processes.<br>• Durable job table advanced across ticks, with leases, idempotency keys, budgets and a circuit breaker.<br>• One-minute tick; the watcher sidecar folded into it.<br>• All LLM work runs on the user's Antigravity quota: housekeeping via headless `agy -p` (measured at ~17k tokens of fixed overhead and ~13 s per call, so items are batched per call); no API key.<br>• Session visibility policy: repo work and reports in the IDE; housekeeping headless.<br>• Weekly review report as an IDE conversation carrying gap questions, pending claims and proposed work; human replies captured as evidence.<br>• LLM stages defined as pure functions.<br>• `PreToolUse` gate-once delivery for risky actions and déjà-vu.<br>• Capture sweep extended to the Antigravity CLI store.<br>• Publishing through a dedicated git worktree.<br>• Experiments revised (E7 isolation, E10 overhead, E11 quota). |
| 0.4 | 2026-09-30 | Claude Code support and external review:<br>• P13 evidence independence with per-step exposure markers and author classification; independence rules in extraction and reconcile.<br>• Report-reply mechanism (§9.2.1).<br>• §4.4.1 cost model from measured Antigravity transcript sizes; salience windows and packing; the fixed overhead identified as dominating small interactive calls.<br>• Déjà-vu reframed as tier 0 (proactive) / tier 1 (lexical safety net, generated match blocks) / tier 2 (late correction; seconds on Claude Code via `asyncRewake`).<br>• §12.2 rewritten from verified Claude Code docs: hook mapping, `claude -p --safe-mode` executor, Desktop scheduled task reports, async-hook tick triggers, transcript parser policy, auto-memory coexistence, experiments C1–C7.<br>• Native-floor degraded mode.<br>• §17 implementation decisions (D1 open, recommendation: one compiled binary; D2 and D3 decided).<br>• P0 extended to both harnesses; P2 builds a normalised event model, Antigravity first. |
| 0.5 | 2026-09-30 | Harness-neutral techniques from a review of Claude Code auto memory (Claude-specific notes in `docs/claude_architecture.md` §4.7):<br>• Claims gain `relevant_when` (for selection) and `apply` (for action) lines; pushes render `apply`.<br>• Claim index view per subsystem, so agents can walk MAP → index → claim with plain reads; used as the fallback when task linking fails and in degraded modes.<br>• Every pushed claim carries a currency stamp; the hook checks that anchored files exist.<br>• `open-issue` claims carry `closes_when` conditions evaluated by the tick; new `resolved` status; unknown closure means pull-only.<br>• Writes to memory artefacts (PA files, harness-native stores) and approval signals are triage keep rules; written text kept as `note` steps, leads rather than evidence.<br>• Over-budget views are re-synthesised rather than truncated.<br>• Extraction reports referenced `gaps`, which feed coverage. |
| 0.6 | 2026-09-30 | User decisions:<br>• **Release order:** Release 1 targets Antigravity; Release 2 adds Claude Code (D4). Claude Code-specific text is tagged **[R2]**; build phases split into §15.1 (P0–P5) and §15.2 (R2.1–R2.3), with Release 2 able to start after P2.<br>• **Go** (D1): one cgo-free binary with `hook`, `tick`, `mcp`, `install` and operator subcommands (§18.1); a package layout with an enforced core/adapter boundary (§18.2); harness interfaces and capability-based delivery (§18.3); libraries (§18.4); hook fast path (§18.5); portability rules for Release 1 (§18.6), including a recommended sweep-only copy of Claude Code transcripts from P0.<br>• Plugin files generated by `repo-memory install` with absolute paths; D2 and D3 scoped to Release 1. |
| 0.7 | 2026-10-01 | From the programme-plan review: repository enrolment (`repo-memory init`/`leave`, registry, hooks no-op within 5 ms outside enrolled workspaces, sweep scoped to enrolled repositories) and a kill switch (`REPO_MEMORY_DISABLED`, `enabled: false`, `enable`/`disable`) in §4.6; machine configuration and registry in §5.6; new subcommands in §18.1; Q24. |
| 0.8 | 2026-10-01 | Third-party review (issue #1), verified against the runtime:<br>• Antigravity transcript file set and `truncated_fields` resolution (§6.2, E12); incremental transcript copying.<br>• Template-safe repo ID and per-clone publishing worktrees (§4.6, §5.6).<br>• Decision-marker salience windows (§6.4); mechanical P13 precondition (§6.6).<br>• `gate-once` limited to irreversible actions; E9 widened (§7.5); tier-1 reasoning check at `PreToolUse` (§7.6).<br>• Sidecar permissions, dashboard enablement and `model_decision` native rules (§12.1, E2/E4/E6 updated).<br>• SQLite concurrency and Windows-safe index swap (§18.4).<br>• Cost estimates flagged as lower bounds pending full-transcript measurement (§4.4.1). |
| 0.9 | 2026-10-01 | Review #1 follow-up, verified on all 537 conversations:<br>• `chunks/` are 100 KiB byte slices of the monolithic files.<br>• `transcript_full.jsonl` is complete and matches `transcript.jsonl` line for line.<br>• §6.2 now reads one source in priority order (full → full chunks concatenated as a byte stream → compact), with no merge.<br>• Incremental copy covers the primary source only.<br>• E12 answered except for append-only and rewrite behaviour and the cause of step gaps.<br>• Corrects the v0.8 claim that chunks might fill step gaps. |
| 0.10 | 2026-10-02 | Applied P0 experiment findings (`E1`–`E12`) from `docs/experiments/SPEC-CHANGES.md`:<br>• Verified `injectSteps` format, `@[label](path)` rules include, zero transcript lag, `trigger: model_decision`, hook latency benchmarks, IDE-only sidecar behavior, `REPO_MEMORY_HOUSEKEEPING=1` isolation, subagent hook firing, `PreToolUse` `{"decision": "allow"}` requirement, Gemini 3.6 Flash overhead reduction, and `transcript_full.jsonl` append-only MD5 prefix checks. |
| **1.0** | **2026-10-15** | **Upgraded to `v1.0` (`v2.2` Architecture — unified with [Design v2.2 Rev 4](docs/design-v2-distillation-retrieval.md) and [Addendum A Rev 3](docs/addendum-a-provenance-and-hardening.md)):**<br>• **Two-Stage 5-Minute Warm-Cache Distillation (`VD14`, `VD15`):** Replaced v1's 4-stage post-hoc LLM pipeline (`triage`, `extract`, `reconcile`, `verify`) with (1) 100% automatic milestone-gated `Stop` (`terminationReason == "NO_TOOL_CALL"`, `fullyIdle == true`, `executionNum == 0`) continuation reusing the working agent's warm 5-minute KV cache (~90% input discount) + free zero-token `CHECKPOINT`/`brain/`/`source: pa` harvest + optional `memory_note` mid-turn override with mark deduplication + `repo-memory-distiller` sidecar fallback (38.4% fixed overhead reduction in X8), and (2) Warm Batch Consolidation over a single multi-turn `stream-json` session per `tick` after `/usage` quota verification (X3).<br>• **Op-Log Event Sourcing, Cohesive Subsystem Dossiers & Three Cognitive Lifecycles (`VD16`, `VD17`, `VD18`):** Replaced `.memory/claims/` and `.memory/views/` with `.memory/ops/<yyyy-mm>/<ulid>.json` (`add`, `amend`, `supersede`, `retire`, `move`, `split`), `.memory/dossiers/<scope>.md` & `.history.md` (with optional `{section?}` resolution), and `.memory/ATLAS.md`. Grouped the 8 entry types under **`semantic`** (`invariant`, `contract`, `gotcha`, `open-issue`), **`procedural`** (`recipe`, `failure-signature`, with `PostToolUse` exit-code tracking and graduation to `.agents/skills/rm-<slug>/SKILL.md`), and **`episodic`** (`decision`, `ruled-out`). Added the Pre-Consolidation Corroboration Gate and Five-Tier State Scope Hierarchy (`temp:`, `session:`, `user:`, `branch:`, `repo:`).<br>• **Doc-Seeded Dossiers (`repo-memory seed-docs`) & Async Freshness ("Dreaming") (`VD16`):** Ingests curated repo guides (`docs/developer/`, `.agents/skills/`) as `source: doc-seed` entries and refreshes them asynchronously on `git diff`.<br>• **Push-First Delivery, Persistent `userMessage` Contract & Rewind Recovery (`P14`, `VD13`):** Grounded in the 231-session PA study (1.3% `MEMORY.md` reads, 0 MCP pull calls) and `X6-lite`. Specified persistent `userMessage` (`source: SYSTEM_SDK`, `[repo-memory]` prefix) until `CHECKPOINT` for `task-start`, `first-entry`, `first-touch`, and `re-injection` (fixing the `8d7ad7b` + `d3ab994` 1-invocation vanishing memory bug), `ephemeralMessage` strictly for 1-step hints (`about-to-edit`, `error-match`), scaled FTS5 ranking (`ftsScore * 100`, `DefaultMinRelevanceScore = 50`), `DetectRewind` wiring in `HandlePreInvocation`, and the Two-Plugin Packaging Split (`repo-memory-daemon` + workspace `repo-memory`). |
