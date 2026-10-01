# Repo Memory: Specification

| | |
|---|---|
| **Status** | Draft v0.7 (see Revision history) |
| **Date** | 2026-09-30 |
| **Scope** | Repository-level memory for coding agents. **Release 1: Antigravity. Release 2: Claude Code.** Text marked **[R2]** belongs to Release 2; Release 1 is built so that Release 2 is an adapter, not a rewrite (§18.6). |
| **Implementation** | One Go binary, `repo-memory` (§18) |
| **Reference repo** | `esp32-app-foundation` (evidence in Appendix A) |
| **Supersedes** | `repo-memory-plugin-architecture-analysis.md` (annotated critique kept for the record) |

---

## 1. Purpose

An agent starting work on a large, long-lived repository should behave as if it had been on the project from day one. It should know how each part of the system works and why. It should know what invariants must hold and what has already been tried and failed. It should know what a given error usually means and how to verify a change on real hardware.

This specification defines a memory system that provides that knowledge:

- **automatically**, without the working agent spending tokens to record it;
- **efficiently**, delivering only what the current task, file, error or action needs;
- **accurately**, keeping each fact tied to the code and evidence it depends on;
- **completely**, covering every part of the system, including stable parts nobody has touched for months.

### 1.1 Goals

| # | Goal |
|---|---|
| G1 | The working agent writes no memory. All capture and distillation is done by a background sidecar. |
| G2 | Every subsystem in the repo has memory coverage, whether or not a conversation has discussed it. |
| G3 | Knowledge is delivered at the moment of need (task start, file touch, error, risky action), not only on request. |
| G4 | A fact stays valid until something it depends on changes. Age and lack of use never retire knowledge. |
| G5 | Trust is earned from outcomes (tests, builds, flash logs, merges, human confirmation), never from retrieval frequency or self-rated confidence. |
| G6 | Memory is rebuildable. Improving the pipeline and re-running it over the full history improves all past memory. |
| G7 | Knowledge that lives in git is human-readable, reviewable per fact, and merge-safe. |
| G8 | The system measures its own effectiveness using signals every session produces. |
| G9 | Existing artefacts (PA workspaces, ADRs, design specs, commit history) can be ingested without a special migration path. |

### 1.2 Non-goals

- Generic chat or personal-assistant memory (user preferences across unrelated projects).
- Replacing git, the issue tracker or human-facing documentation. Memory can *feed* documentation (§7.3) but doesn't own it.
- A graph database, cloud vector store or any server infrastructure.
- Autonomous commits to the user's working branches.
- Storing what can be derived cheaply from the code at read time (structure, signatures, call graphs).

---

## 2. Design principles

| # | Principle | Consequence |
|---|---|---|
| P1 | **Separate working from remembering.** | The main agent only works. Capture uses hooks (no LLM). Distillation happens asynchronously in the sidecar. |
| P2 | **Event-sourced knowledge.** | Sessions, commits, human reviews and imported documents are immutable *events*. Knowledge is a *projection* over events and can be recomputed. |
| P3 | **Store only what the code can't tell you.** | Memory holds intent, rationale, ruled-out approaches, failure signatures, hardware and library behaviour, verified procedures, conventions and decisions. Structure is read from the code. |
| P4 | **The claim is the unit of knowledge.** | Knowledge is stored as small, typed, evidence-backed claims anchored to code. Documents are generated *views* over claims. |
| P5 | **Truth from outcomes.** | A claim's status comes from evidence in the trace (tool results, test outcomes, merges, reverts, human statements), not from the agent's prose. Hypotheses remain hypotheses. |
| P6 | **Code is the index.** | Entities are derived from the repository. Retrieval is keyed on the code the agent is touching, the errors it sees and the task text, not on embedding similarity over the opening prompt. |
| P7 | **Validity follows dependencies.** | Claims are re-checked when their anchored code, library versions, toolchain or config change. Nothing expires with time or disuse. |
| P8 | **Push, with a strict budget.** | Hot-path delivery is deterministic, bounded in tokens and deduplicated per session. Only high-trust claims are pushed. |
| P9 | **Humans are the highest authority.** | Human statements and reviews are events with the highest trust. The system asks targeted questions to fill rationale gaps. |
| P10 | **Harness-agnostic core, thin adapters.** | Pipeline, stores and views are plain files plus SQLite. There is no custom daemon or socket. Antigravity (Release 1) and Claude Code (Release 2) are adapters behind one interface. Delivery is chosen by each harness's declared *capabilities*, never by its name (§18.3). |
| P11 | **Never slow the loop.** | Hooks run synchronously and block the agent. Hook scripts only read precomputed files and indexes. All LLM work happens in the tick. |
| P12 | **Review gates sharing, not use.** | A developer's own machine uses new claims as soon as they reach `verified`. Review controls what enters the shared repository. |
| P13 | **Evidence must be independent of the memory it supports.** | Only tool outcomes and human-authored messages are evidence; agent text never is. Anything a session was shown by memory (its *exposure*) cannot come back as support for the same claim unless a new outcome actually tested it (§6.6). This prevents memory from reinforcing itself through the sessions it informs, including bug-fixer sessions and report conversations. |

---

## 3. Terminology

| Term | Meaning |
|---|---|
| **Session** | One agent conversation, including any subagent sessions it spawned. |
| **Raw transcript** | The harness's native record of a session (Antigravity conversation DB, Claude Code JSONL). |
| **Episode** | The compacted, scrubbed, harness-neutral record of a session produced by the pipeline (§5.1). |
| **Event** | Any immutable input to the knowledge projection: an episode, a commit, a human review decision, a human answer, or an imported document. |
| **Entity** | A thing claims are about: a code element (subsystem, file, symbol, config key, build env, topic, MCP tool, error string) or an external element (library@version, SDK version, board, tool). |
| **Anchor** | A reference from a claim to an entity, carrying a fingerprint of the anchored content. |
| **Claim** | A typed, evidence-backed statement about one or more entities (§5.3). |
| **View** | A generated document rendered from claims and entities (§5.5). |
| **Claim index** | A generated view listing, per subsystem, each pushable claim's ID and `relevant_when` line, so an agent can find relevant claims itself with ordinary file reads (§5.5). |
| **Memory artefact** | A file a working agent or the PA writes to record knowledge (PA `MEMORY.md`, `PROJECT.md` session logs, completion reports, a harness-native memory store). Writes to one are a triage signal (§6.4). |
| **Event path** | Work started by a harness hook. Knows exactly which session changed, must return in milliseconds, and only reads, injects and writes spool files (§4.1). |
| **Time path** | Work started by a scheduled tick. Does all asynchronous work (§4.1). |
| **Sidecar** | An Antigravity-managed scheduled job: a periodic tick, not a daemon. |
| **Tick** | One scheduled run of `repo-memory tick`. It advances pending jobs within a time budget, then exits. |
| **Job** | A durable unit of asynchronous work in the job table (§4.3), advanced step by step across ticks. |
| **Housekeeping run** | A headless `agy -p` call made by the pipeline, on the user's Antigravity quota. Not visible in the IDE (§4.4). |
| **Report** | An IDE-visible conversation (`agentapi`) that the tick starts for the human, only when there is something worth reviewing (§4.4). |
| **Trigger** | A runtime condition that causes knowledge to be delivered to the working agent (§7). |
| **Exposure** | A session receiving memory content: pushed by a hook, returned by a pull tool, or read directly from `.memory/`. Recorded per step in the episode (§5.1) and used to enforce P13. |
| **Salience window** | A range of episode steps selected mechanically around failures, fixes, reverts, human decisions and trade-off reasoning. Extraction reads windows rather than whole episodes (§6.4). |

---

## 4. Architecture overview

```
          EVENT PATH (hooks)                               TIME PATH (scheduled tick, every minute)
  ┌──────── Working agent (Antigravity / Claude Code) ─────────┐
  │  works normally — writes no memory                         │
  │  ▲ push: brief · touch · error · risky action · déjà-vu    │
  │  │ pull: ask / why / history / brief / note / stats ───────┼──▶ MCP server ──▶ agy -p (synchronous)
  └──┼───────────────┬─────────────────────────────────────────┘
     │ inline        │ Stop · PreInvocation · PreToolUse · PostToolUse
     │ (no LLM)      ▼ write spool files only
     │        ┌─────────────┐        ┌───────────────────────────────────────────────┐
     │        │   spool/    │──────▶ │ Tick: ingest spool → job table → next steps   │
     │        └─────────────┘        │  • mechanical work in-process                 │
     │                               │  • LLM steps: agy -p child processes          │
     │                               │    (headless housekeeping; waited, bounded)   │
     │                               │  • reports: agentapi (visible in the IDE)     │
     │                               │  • sweep, git checks, calendar jobs           │
     │                               └───────────────────────┬───────────────────────┘
     │                                                       ▼
     │          1 Compact → 2 Triage → 3 Extract → 4 Reconcile → 5 Verify → 6 Publish
     │             │                             │                          │
     │             ▼                             ▼                          ▼
     │     Episode log (private)   Claim store (git) + entity skeleton   Views (generated)
     │                                           │                          │
     └──────────── alerts/ + local index (SQLite, rebuildable) ◀────────────┘

  Human events: answers in review reports, memory-PR review → highest-trust events
```

### 4.1 Invocation paths

Nothing runs continuously. Two paths start all work, and each has a strictly separate job:

| | **Event path: hooks** | **Time path: scheduled tick** |
|---|---|---|
| Fired by | Agent, model, tool and session events in the harness | Cron, every minute (Antigravity plugin sidecar, `builtin: schedule`). **[R2]** On Claude Code, harness-run `async: true` hooks also start it (§12.2). Elsewhere, the OS scheduler running `repo-memory tick`. |
| Knows | Exactly which conversation, transcript, workspace and step just changed | Discovers state from the spool, the job table, transcripts and git |
| Constraint | Synchronous and blocking the agent loop, so it must finish in milliseconds | A time budget per tick (default 50 s); never overlaps itself (`tick.lock`) |
| Does | Reads the index and `alerts/`; injects; writes spool files | All asynchronous work: ingest the spool, advance jobs, run LLM steps, sweep, calendar jobs |
| Never | Starts processes, calls a model, uses the network, or writes the job table | Leaves work running after it exits |

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
| Capture a finished session | `Stop`: write a spool entry and copy the transcript | Sweep catches sessions with no `Stop` (crash, kill, sessions from before install, subagents per E8); promotes idle sessions to processing |
| Process episodes (stages 1–6) | none | Jobs advanced each tick within budget |
| System map | Loaded by the `AGENTS.md`/`CLAUDE.md` include | Regenerated when claims change |
| Task brief | `PreInvocation` (`invocationNum == 0`) inline from the index. If lexical linking fails, spool a brief request. | Next tick runs it as a housekeeping run; the next `PreInvocation` delivers it. The agent can also call `memory.brief` synchronously. |
| Touch, error delivery | `PreInvocation` inline | none |
| Risky action | `PreToolUse` gate modes; `PreInvocation` injection (§7.5) | none |
| Déjà-vu tier 1 (lexical) | `PreToolUse` on the concrete call; `PreInvocation` on the latest reasoning | none |
| Déjà-vu tier 2 (semantic) | `PreInvocation` spools a request when new reasoning mentions an entity with `ruled-out` claims | Next tick checks it; the next `PreInvocation` delivers any alert |
| Re-check after a git change by the agent | `PostToolUse` on `run_command`: if the step at `stepIdx` is a merge, checkout, pull, rebase or dependency update, spool a re-check | Tick compares HEAD, lockfile and sdkconfig hashes to catch git operations made outside the agent |
| Publish the memory PR | none | Daily |
| Review report (§4.4) | none | Weekly, only if there is something to review |
| Metrics roll-up, coverage | none | Daily |
| Index rebuild | `repo-memory reindex` (manual) | If the index is missing or its schema is outdated |

### 4.3 Job table

Asynchronous work is tracked in a private SQLite job table (`<data-dir>/<repo-id>/jobs.sqlite`). Hooks never write to it; they write spool files, which the tick ingests. The hot path therefore never waits on a database lock.

| Field | Purpose |
|---|---|
| `id`, `kind`, `priority` | e.g. `compact`, `triage`, `extract`, `reconcile`, `verify`, `publish`, `brief`, `dejavu`, `recheck`, `report` |
| `idempotency_key` | Hash of inputs + stage + pipeline version, so duplicates are no-ops |
| `state` | `queued` → `running` → `done`, or `failed` → retried with backoff → `dead` after N attempts. Reports: `submitted(conversation_id)` → `done` when the conversation goes idle. |
| `lease_expires`, `attempts`, `next_attempt_at` | Recovery after a crashed or killed tick |
| `cost` | Tokens, duration and model, taken from the `agy` JSON output |

**Each tick, in order:**
1. Take `tick.lock`; exit if a previous tick is still running.
2. Ingest spool files as jobs.
3. Reclaim expired leases.
4. Sweep transcripts and check git hashes (cheap, mechanical).
5. Poll report conversations.
6. Run interactive jobs (briefs, déjà-vu) first.
7. Run batch jobs until the time budget is spent, with bounded concurrency (default: 2 `agy -p` children at once).
8. Run due calendar jobs.
9. Exit.

**Concurrency rules.** Extraction runs in parallel. Reconcile and publish are single-writer, so claims can't be duplicated and git operations can't race.

**Back-pressure:**
- time budget per tick;
- daily budgets for housekeeping calls and tokens;
- a circuit breaker after N consecutive failures (like the bug-fixer tick);
- a cap on open report conversations.

### 4.4 Executors and session visibility

| Class | Surface | Executor | Used for |
|---|---|---|---|
| **Repo work** | IDE | Human sessions, PA workers, the bug-fixer loop | The memory system never changes the repository's code. It *proposes* such work in reports (e.g. "turn invariant X into a CI check", "guide Y contradicts verified claim Z"), for the human or the bug-fixer loop to act on. |
| **Reports** | IDE | Antigravity: `agentapi new-conversation`, title prefix `[repo-memory]`, started by the tick. **[R2]** Claude Code: a Desktop local scheduled task (visible under **Scheduled**). | Weekly memory review (claims awaiting approval, gap questions, coverage and health, proposed work); one-off notices such as "OTA backfill ready for review". Only started when there is something to review. |
| **Housekeeping** | Headless | `agy -p` with `--model`, `--json-schema`, `--output-format json`, `--input-format stream-json` (stdin, for large payloads and the Windows command-line limit), `--print-timeout`. **[R2]** On Claude Code-only machines: `claude -p --safe-mode --tools "" --system-prompt-file … --json-schema … --no-session-persistence` (§12.2). | All LLM pipeline stages; synthesis for pull tools |
| **Mechanical** | No session | The `repo-memory` Go binary in the tick and hooks (§18) | Capture, compaction, entity extraction, index, publishing through a dedicated git worktree |

Everything runs on the user's subscriptions, with no API key and no pay-as-you-go billing:

- **Executor setting.** `housekeeping.executor` is set per machine. In Release 1 it is always `agy`. **[R2]** `claude` becomes available for Claude Code-only machines (decision D2, §17).
- **Optional `api` backend.** It exists for other deployments and is off by default.

**Cost of a housekeeping run** (measured 2026-09-30, `gemini-3.8-flash-low`, trivial prompt, empty working directory):

| | |
|---|---|
| Time | 12.8 s end to end (~5 s model) |
| Input tokens | 16,813, almost all fixed agent overhead |
| Output tokens | 5 |
| Where recorded | Only in the CLI store (`~/.gemini/antigravity-cli/`, project `default-cli-project`); not in the IDE history |

#### 4.4.1 Cost model

Housekeeping cost has two parts: a **fixed overhead per call** (~17k input tokens and ~13 s for `agy -p`) and the **payload**.

**Measured inputs:**

| Transcripts | Count | Median | Mean | Tool traffic |
|---|---|---|---|---|
| Antigravity top-level, esp32 | 45 surviving | 52k tokens | 120k tokens | 86% |
| Antigravity subagent, esp32 | 138 surviving | 29k tokens | 36k tokens | 90% |
| Claude Code, esp32 | 35 | ~47k tokens | ~98k tokens | 70% |

Session rates since May: ~1.4 top-level and ~7 subagent Antigravity sessions a day, and ~0.9 Claude Code sessions a day.

**Payload per episode, before any overhead.** Compaction keeps dialogue, reasoning and outcome excerpts, and those are 10–30% of a transcript, so compaction should reach 4× or more on Antigravity and 3× on Claude Code. Salience windows (§6.4) should roughly halve that again. A median Antigravity top-level episode then comes out as:

| Step | Tokens |
|---|---|
| Raw transcript | 52k |
| After compaction | ~13k |
| In salience windows, plus synopsis | ~7k |

**Estimates.** These rest on the stated assumptions, and P0/P1 measurements (compaction ratio, triage keep rate, B4) replace them.

| Workload | Estimate | Dominant cost | Levers |
|---|---|---|---|
| Backfill: surviving transcripts (~14M raw tokens) plus the PA archive (~1.2M tokens) | ~4–6M tokens one-off, throttled by the daily budget over days | Payload | Compaction, triage, salience windows, packing episodes to the 60k payload cap |
| Steady-state extraction, triage, reconcile and verify | ~100–200k tokens a day | Payload and overhead roughly equal | Pack a parent session with its subagents into one call; reconcile per subsystem batch |
| Interactive small calls (LLM brief fallback, déjà-vu tier 2, pull-tool synthesis) | ~17–20k tokens **each**, ~85% of it overhead; 5–15 a day ≈ 100–300k tokens | **Fixed overhead** | Deterministic paths first; batch all pending tier-2 checks into one call per tick; cache briefs by entity set; per-day caps; E10 |

The fixed overhead matters most for the small, interactive calls, not for extraction. E10 (reducing the overhead) and E11 (how usage counts against the quota) therefore gate how freely those features are enabled. The P1 exit reports cost per stage from the `agy` usage output.

**Packing targets per call:**
- triage: 20 episode synopses;
- extraction: salience windows from one or more episodes (a parent with its subagents first), up to 60k tokens;
- verification: 30 claims;
- reconciliation: one subsystem's candidates.

**Isolating housekeeping runs:**
- They run with the working directory `<data-dir>/runs/<repo-id>/`, not the repository, so workspace rules and hooks don't load. They use a dedicated `--project` (E7).
- They set `REPO_MEMORY_HOUSEKEEPING=1`. Hooks inherit `agy`'s environment, so this plugin's own hooks exit immediately inside housekeeping runs.
- Capture excludes them by project and workspace URI.
- `agy --continue` must never resume one (E7).
- **[R2]** On Claude Code, `claude -p --safe-mode --no-session-persistence` loads no plugins or hooks and writes no transcript, so there is nothing to exclude and recursion is impossible.

**Report conversations are captured**, but only messages a human typed count as evidence. The report's own text, and the automated kickoff prompt that started the conversation, were generated from memory, so re-extracting them would reinforce memory with itself. The mechanism is in §9.2.1.

### 4.5 Components

| Component | Responsibility | Invoked by |
|---|---|---|
| **Hook entry point** (`repo-memory hook <harness> <event>`) | Deterministic delivery; spool writing (§7, §6.2). No processes, network, LLM or job-table writes. | Event path |
| **Tick** (`repo-memory tick`) | Spool ingestion, the job table, sweep, git checks, housekeeping runs, reports, calendar jobs | Antigravity scheduled sidecar; OS scheduler; CLI (`repo-memory run` is an alias) |
| **Entity extractor** | Builds the code skeleton (subsystems, files, symbols, config keys, topics, tools, error strings). No LLM. | Tick, when HEAD or configuration changes |
| **Pipeline stages 1–6** | Turn events into claims and views (§6) | Tick |
| **Re-check scheduler** | Queues claims for re-verification when their dependencies change (§8) | Tick, from spooled `PostToolUse` events and its own hash checks |
| **Pull tools** | `memory.ask`, `memory.why`, `memory.history`, `memory.brief`, `memory.note`, `memory.stats`. Exposed by a stdio MCP server that reads the SQLite index; `ask` and `brief` may make one synchronous housekeeping run. | Spawned by the harness per session |
| **Review surface** | Weekly review report (IDE) and the batched memory PR (git host) | Tick |

Processes communicate only through files and SQLite in WAL mode. There is no socket, no long-running process and nothing detached. Everything works on macOS, Linux and Windows.

**[R2] Claude Code-only machines:**
- Hooks behave the same.
- Asynchronous work needs a tick: the Antigravity sidecar if Antigravity is installed on the same machine, otherwise one OS scheduler entry (`repo-memory schedule install`).
- LLM stages need the Antigravity CLI (`agy`) installed and signed in. Without it, capture and delivery keep working, and LLM jobs wait in the queue.

### 4.6 Repository enrolment and the kill switch

The plugin is installed globally, so its hooks fire in every workspace the harness opens, and `PreInvocation` fires on every model call. Memory must therefore act only on repositories the user has explicitly enrolled, and must cost almost nothing everywhere else.

**Enrolling a repository.** `repo-memory init`, run in the repository root:

1. creates `.memory/` (`config.yaml`, `claims/`, `events/`, `views/`) and `.memory/.gitignore` for `.cache/`;
2. adds the map include line to the harness's own instruction file (`GEMINI.md` in Release 1; **[R2]** `CLAUDE.md`), never to the shared `AGENTS.md` (§12.2);
3. registers the repository in `<data-dir>/registry.json`: repo ID (hash of the root commit), root path and known worktree paths;
4. offers the subsystem bootstrap (§5.2), whose proposal the user reviews.

**What enrolment controls:**

| Component | Behaviour |
|---|---|
| Hooks | The first thing every hook does is map the workspace path to the registry (a small file, cached by modification time). If the workspace isn't enrolled, the hook returns the harness's no-op output at once: within 5 ms of process start. |
| Capture sweep | Copies only transcripts whose workspace matches an enrolled root or one of its worktrees. Sessions in other workspaces are never read or copied. |
| Tick | Processes enrolled repositories only. |
| Worktrees | Share the repo ID; sessions are attributed by path prefix. |

**Leaving.** `repo-memory leave` removes the registry entry. It doesn't touch `.memory/`, which is shared repository content. `--purge` also deletes that repository's private episodes and data directory.

**Kill switch.** One check comes before everything else in every hook and every tick: the environment variable `REPO_MEMORY_DISABLED=1`, or `enabled: false` in the machine configuration (`<data-dir>/config.yaml`). `repo-memory disable` and `repo-memory enable` set it, globally or with `--repo` for one repository. When disabled, hooks return no-op output and the tick exits. This is the safety valve if a global hook ever misbehaves, because a broken hook would otherwise affect every agent session on the machine.

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
  "repo_id": "<hash of root commit>",
  "branch": "ble/wo6-dis-service",
  "started": "2026-09-22T19:04:11Z", "ended": "…",
  "head_start": "<sha>", "head_end": "<sha>",
  "commits": ["<sha>", …],            // linked by the sweep from git log/reflog (§6.2)
  "step_gaps": [[812, 820]],          // missing step indices, if any
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

### 5.3 Claim

One file per claim, in the repository, sharded by creation month:

`.memory/claims/<yyyy-mm>/<claim-id>.md`

- **Why month, not subsystem.** A claim can anchor to several subsystems, and the subsystem map gets regrouped over time. Sharding by subsystem would force file moves that break `git log` history. Creation month never changes, and claim IDs are ULIDs, so they sort by time anyway.
- **Why shard at all.** Git copes with flat directories of this size, but GitHub's web UI lists at most 1,000 entries per directory. Views group claims by subsystem for reading.
- **Expected scale.** Roughly hundreds of claims from the esp32-app-foundation backfill and low thousands of active claims after several years. That estimate comes from the 20-candidate-per-episode cap (§6.9), triage and reconciliation. Superseded and refuted claims stay, because they are history. By comparison, the PA workspace already holds 1,364 files.

```markdown
---
id: clm-01J8ZQ4K2V7X
type: invariant                # see §5.3.1
title: BLE host and controller memory must be internal DRAM
relevant_when: changing BLE initialisation, NimBLE memory settings or PSRAM allocation
apply: Keep CONFIG_BT_NIMBLE_MEM_ALLOC_MODE_EXTERNAL=n and allocate no BLE buffers in PSRAM
status: verified               # see §5.3.2
flags: []                      # suspect | conditions-changed | needs-review
anchors:
  - { kind: config,  ref: CONFIG_BT_NIMBLE_MEM_ALLOC_MODE_EXTERNAL, fp: "val:n" }
  - { kind: symbol,  ref: BLEStack::init, fp: "sha1:9f2c…" }
  - { kind: subsystem, ref: ble }
conditions:
  esp-idf: "5.5.5"
  esp-nimble-cpp: "2.5.0"
  chip: esp32s3
evidence:
  - { kind: episode, ref: ep-01J8X…, steps: [112, 140], outcome: "reset:INT_WDT" }
  - { kind: episode, ref: ep-01J8Y…, steps: [31],       outcome: "soak_pass:15m" }
  - { kind: commit,  ref: 4e1a9c2, outcome: merged }
relations:
  explains: [clm-01J8ZQ4K2V7Y]  # the failure signature
  supersedes: []
scope: { branch: main }
verified_at: 69d9c813
provenance: { pipeline: 1.0.0, extractor_model: small, reconciler_model: large }
review: { state: approved, by: "@matthewmcneill", at: 2026-09-24 }
created: 2026-09-23T22:10:00Z
updated: 2026-09-24T09:02:00Z
---

BLE controller DMA cannot access PSRAM. With `MEM_ALLOC_MODE_EXTERNAL=y`,
`NimBLEDevice::init()` holds cross-core heap spinlocks long enough to trigger
INT_WDT/TG0WDT resets on ESP32-S3. All BLE memory must be internal DRAM.
```

Body text: one to three sentences for most types; up to ~150 words for `decision` and `procedure`.

**Reader-facing lines.** Besides its title, each claim has two one-line fields, written for two different moments:

| Field | Says | Written for | Limit | Required for |
|---|---|---|---|---|
| `relevant_when` | *When* the claim matters, in terms an agent recognises while planning: tasks, symptoms, files | Selection: it is what the claim index lists (§5.5). It is not a summary. | ≤ 150 characters | All pushable claims |
| `apply` | *What to do or check* when it matters | Action: push delivery renders this line, not the statement (§7.1) | One imperative sentence, ≤ 25 words | `invariant`, `ruled-out`, `procedure`, `failure-signature` (the fix), `open-issue` |

The body keeps the statement and the reason, so an agent that follows the claim ID also gets the why. Extraction proposes both lines; reconcile may refine them; the lint gate enforces the limits.

#### 5.3.1 Claim types

| Type | Captures | Typical trigger for delivery |
|---|---|---|
| `invariant` | A rule or contract that must hold (e.g. boot-time OTA checks never surface errors) | File touch, risky action |
| `decision` | A design choice, its rationale, the alternatives considered and who decided | Task start, `why` |
| `ruled-out` | An approach that was tried and failed, why it failed, and under what conditions | File touch, déjà-vu |
| `failure-signature` | Symptom pattern → cause → fix | Error |
| `procedure` | A verified way to do something (flash a board, soak test, release) | Risky action, task start |
| `fact` | Hardware, library or environment behaviour not visible in the code | File touch |
| `convention` | House style, tooling rules, naming | Task start |
| `open-issue` | A known defect, tech debt or in-flight work | Task start, file touch |

`failure-signature` claims, and optionally `ruled-out` claims (for déjà-vu tier 1, §7.6), carry a machine-matchable `match` block:

```yaml
match:
  any: ["INT_WDT", "TG0WDT"]
  all_in_window: ["BLE_INIT", "EM_BASE_REG_LUT"]
  source: [device, command]
```

**Closure conditions for `open-issue`.** In-flight work and known defects change when something *happens*: an issue closes, a branch merges, a work order completes, a fix lands. That rarely touches the claim's anchored code, so the code-anchored re-checks in §8 miss it. The June OTA record in Appendix A.1, which still presents two fixed defects as open, is this failure. Every `open-issue` claim therefore carries a `closes_when` block:

```yaml
closes_when:
  any:
    - { kind: issue,     ref: "#239", state: closed }
    - { kind: branch,    ref: ble-manager/wo6-validation, state: merged }
    - { kind: workorder, ref: pr-225-ble-manager/WO-6, state: complete }       # PA ledger
    - { kind: outcome,   target: "pio run -e esp32s3nano_ble", result: success } # a later episode
```

- Extraction attaches at least one condition, or sets `closes_when: unknown`.
- An `open-issue` with `closes_when: unknown` is pull-only: it is never pushed as current fact.
- Pushed `open-issue` claims always show when their conditions were last evaluated ("open as of <date>", §7.1).
- The tick evaluates the conditions (§8). When one fires, the claim becomes `resolved`.

#### 5.3.2 Status (trust ladder)

| Status | Meaning | Set by | Pushed? |
|---|---|---|---|
| `hypothesis` | Proposed but untested | Extract | No (pull only) |
| `observed` | Directly visible in a tool outcome in one episode | Extract | No (pull only) |
| `verified` | Confirmed by a test, build, flash or soak outcome, or observed in ≥2 independent episodes | Reconcile, Verify | Yes |
| `merged` | Supported by code that landed on main and was not reverted | Verify | Yes |
| `confirmed` | Stated or approved by a human | Human event | Yes |
| `refuted` | Contradicted by stronger evidence | Reconcile, human | No (kept for history) |
| `superseded` | Replaced by a newer claim | Reconcile, human | No (kept for history) |
| `resolved` | An `open-issue` whose closure condition fired (§5.3.1) | Tick (§8), human | No (kept for history) |

**Flags** don't change status but alter delivery:

- `suspect`: an anchor fingerprint changed and the claim hasn't been re-checked yet. It is delivered with a "verify before relying on this" marker.
- `conditions-changed`: a condition (library version, SDK, chip) no longer matches the repo. `ruled-out` claims with this flag are delivered as "previously failed under X; may work now".
- `needs-review`: awaiting human review. Delivered only if the status is `verified` or higher.

**Rule:** retrieval, injection or use never changes status. Only evidence and human events do.

#### 5.3.3 Relations

`supports`, `contradicts`, `supersedes`, `explains`, `refines`, `depends_on`. They are stored on the claim that asserts them, and the index holds the inverse.

### 5.4 Events and projection

The knowledge base is a projection:

```
claims = project(pipeline_version, events)
events = episodes ∪ commits ∪ human_reviews ∪ human_answers ∪ imported_documents
```

- Human review decisions are stored as events (`.memory/events/reviews/*.jsonl`) keyed by claim ID *and* a content hash, so they survive re-projection.
- Re-projection with a new pipeline version is a controlled operation. It produces a diff (claims added, changed or retired) delivered as a memory PR, never an in-place rewrite.
- Imported documents (PA files, ADRs, spec decision logs, PR descriptions) are episodes with `harness: "import"`.

### 5.5 Views

All views are generated. Hand edits are rejected by the lint gate (§11.4); humans change claims or answer questions instead.

| View | Path | Budget | Contents |
|---|---|---|---|
| **System map** | `.memory/views/MAP.md` | ≤ 2k tokens | One row per subsystem: purpose, globs, dependencies, open-issue count, links to its page and claim index. Plus global invariants and conventions. |
| **Claim index** | `.memory/views/index/<id>.md` | ≤ 1.5k tokens per file | One line per pushable claim of the subsystem: ID, type, `relevant_when`. Split by claim type if over budget. Generated mechanically; no LLM. |
| **Subsystem page** | `.memory/views/subsystems/<id>.md` | ≤ 5k tokens | How it works now (narrative synthesised from claims plus code), invariants, decisions, ruled-out approaches, failure signatures, procedures, open issues. Every line cites claim IDs. |
| **Task brief** | generated on demand (not stored) | ≤ 8k tokens | For a task: relevant subsystems, invariants, ruled-out approaches, procedures, in-flight work. |
| **Signature index** | `.memory/views/SIGNATURES.md` + index table | n/a | All `failure-signature` claims with their match blocks. |
| **Decision log** | `.memory/views/DECISIONS.md` | n/a | All `decision` claims, chronological, with status and supersession. |
| **Episode summary** | generated on demand | ≤ 1.5k tokens | What a session attempted, outcomes, commits, claims produced. Replaces hand-written completion reports. |

Views are regenerated when any claim they cite changes, or when the entity skeleton changes. Committing them to the repository is optional (default: committed, so they're readable on the git host).

**A path the agent can walk itself.** MAP (always loaded) → claim index (one subsystem, ~1.5k tokens) → claim file is a three-step route any agent can follow with ordinary file reads. The agent does the selection, so it costs no LLM call and has no delay. It is the fallback when automatic task linking fails (§7.2), and it keeps working when hooks, the tick or the index are unavailable (§7.9). The plugin's rules file tells the agent the route exists. Reads of these files are exposures (§5.1), like any other read of `.memory/`.

### 5.6 Storage layout

**In the repository** (shared, reviewed):

```
.memory/
├── config.yaml                 # subsystems, budgets, trigger settings
├── claims/<yyyy-mm>/           # one file per claim, sharded by creation month
├── events/
│   ├── reviews/*.jsonl         # human review decisions
│   └── answers/*.jsonl         # human answers to gap questions
├── views/                      # generated; do not edit
│   ├── MAP.md
│   ├── index/*.md              # claim index, one file per subsystem
│   ├── DECISIONS.md
│   ├── SIGNATURES.md
│   └── subsystems/*.md
└── .gitignore                  # ignores .cache/
.memory/.cache/index.sqlite     # rebuildable index (gitignored)
```

**Private, per machine** (never committed):

```
<data-dir>/
├── config.yaml                 # machine configuration: enabled, housekeeping.executor,
│                               #   reports.surface, tick.trigger, budgets
└── registry.json               # enrolled repositories (§4.6)

<data-dir>/<repo-id>/
├── spool/                      # written by hooks only: capture, brief, déjà-vu, re-check requests
├── queue/                      # transcript copies awaiting compaction
├── jobs.sqlite                 # job table (§4.3); written by the tick only
├── tick.lock
├── runs/                       # working directory for housekeeping runs (§4.4)
├── worktree/                   # dedicated git worktree for the memory/pending branch (§6.8)
├── episodes/<yyyy>/*.jsonl.zst # compacted, scrubbed episodes
├── raw/                        # optional raw transcript retention (off by default)
├── pending/                    # claims awaiting review, usable locally (§6.8)
├── alerts/<conversation-id>/   # prepared briefs and déjà-vu alerts waiting for the next PreInvocation
├── state/                      # delivery ledgers, sweep and pipeline cursors
├── metrics/                    # observability store (§10.4)
└── logs/
```

The Antigravity sidecar also gets its own `sidecar_data/<sidecarId>/` directory (logs and `agentapi` events). The pipeline writes only its logs there and keeps its data in `<data-dir>`, so Claude Code-only machines work the same way.

---

## 6. Pipeline

### 6.1 Stage summary

| Stage | Input | Output | Engine | Cost profile |
|---|---|---|---|---|
| 0 Capture | `Stop` hook (event) or tick sweep (time) | Queue entry plus transcript copy | Script | ~0 |
| 1 Compact | Raw transcript | Episode | Deterministic | ~0 LLM |
| 2 Triage | Episode | Keep/skip plus reasons | Rules, then small model | Low; most sessions stop here |
| 3 Extract | Kept episode | Candidate claims | Small model, batched | Medium, input-heavy |
| 4 Reconcile | Candidate plus existing claims on the same anchors | Claim ops | Stronger model, small inputs | Low per candidate |
| 5 Verify | Claim ops | Verified ops | Deterministic plus targeted small-model check | Low |
| 6 Publish | Verified ops | Claim files, views, memory PR | Deterministic plus view synthesis | Low |

### 6.2 Stage 0: Capture

Capture uses both invocation paths (§4.1). The event path captures sessions promptly; the time path guarantees that nothing is missed.

**Event path (primary).** The `Stop` hook:

1. writes `{harness, conversation_id, transcript_path, repo_root, branch, head}` to `spool/`;
2. copies the transcript into `queue/` at once, so later pruning can't lose it;
3. returns immediately.

It never returns `"decision": "continue"`. `Stop` fires whenever the execution loop ends, so a conversation with several user turns produces several `Stop` events. Each one re-copies the transcript. Processing waits until the conversation has been idle for 15 minutes (checked by the next tick), so the same conversation isn't extracted repeatedly mid-flight.

**Time path (backstop).** Every tick sweeps the transcript locations for files whose modification time or size changed since the last cursor:

- Antigravity app and IDE: `~/.gemini/antigravity/brain/<conversation-id>/.system_generated/logs/transcript.jsonl` (steps with `step_index`, `source`, `type`, `status`, `created_at`, `content` / `thinking` / `tool_calls`);
- Antigravity CLI: the same layout under `~/.gemini/antigravity-cli/brain/`. The two stores share most conversation IDs, so sessions are de-duplicated by conversation ID.
- **[R2]** Claude Code: `~/.claude/projects/<project>/<session-id>.jsonl`, plus subagent transcripts in `<session-id>/subagents/*.jsonl` (PA workers spawned with the `Agent` tool live there).
  - Default retention is 30 days (`cleanupPeriodDays`).
  - Anthropic documents the entry format as internal and version-dependent, so the Claude Code parser is version-aware and tolerant (§12.2).
  - As insurance against parser breaks, the Claude Code `Stop` hook also appends the turn's essentials (prompt, tool names, error excerpts, commit commands), taken from hook inputs, to a harness-neutral side-log in the spool.

A `transcript_full.jsonl` sits alongside most Antigravity transcripts (562 of 567). It covers the same steps and doesn't fill the step gaps described below. Which file to prefer depends on per-step content differences, to be checked in P0.

It copies each changed transcript into the queue. Sessions are attributed to a repo via `conversation_summaries.db` `workspace_uris` for Antigravity and the project directory for Claude Code. This covers:

- crashed or killed sessions where `Stop` never fired;
- subagent sessions, if they don't fire their own `Stop` hooks (experiment E8);
- sessions from before installation (backfill of whatever still survives);
- long sessions: a session still in progress is re-copied each tick, and processing waits until it has been idle for 15 minutes.

**Commit linkage.** This happens in the tick, not in hooks. Commits made on the session's branch between session start and end are matched from `git log` and reflog, and confirmed against commit commands in the transcript. No hook records tool outcomes for capture: the transcript already has them. (`PostToolUse` is used only to detect git operations that trigger re-checks, §4.2.)

**Recursion guard.** Housekeeping runs (§4.4) are excluded from capture entirely, identified by their dedicated project and working directory. Report conversations (`[repo-memory]` title prefix) are captured, but only the human's messages in them count as evidence. Other autonomous sessions, such as bug-fixer loop conversations and their trajectory reviews, are captured normally. They are a rich source of failure signatures.

**Transcript completeness.** Antigravity inserts `CHECKPOINT` steps into long transcripts. In a sample of 566 transcripts, 340 had checkpoints and 257 had small gaps in their step indices (at most a few dozen steps out of 1,000+). The sweep records the gaps in the episode header; it doesn't treat them as errors.

Capture never calls an LLM. The `Stop` hook budget is ≤ 200 ms.

### 6.3 Stage 1: Compact

Apply the compaction rules in §5.1 and scrub secrets. Record `raw_chars` and `kept_chars`. Target a reduction of ≥ 3× (Appendix A.2 suggests ~70% of transcript volume is tool traffic already held by git).

### 6.4 Stage 2: Triage

**Rules (cheap, first):**

- Keep if any hold:
  - a failure outcome followed later by a success on the same target;
  - a revert or abandoned edit;
  - a user message containing a decision, preference, correction or approval signal (approving an approach is as informative as correcting one);
  - a new file under a subsystem glob;
  - a dependency or config change;
  - reasoning spans above a length threshold discussing alternatives;
  - a device or soak outcome;
  - a write to a memory artefact, or a `memory.note` call (see below).
- Skip if all hold: no failures, no commits, fewer than K steps, and no decision signals.

**Memory-artefact writes.** When a working agent or the PA writes to a memory artefact, that is its own judgement, made with full context at the moment of learning, that something was worth keeping. Detecting it costs nothing, because compaction already sees every edit path. Until P4 retires hand-written PA knowledge (§13), these writes are frequent.

- **Configuration.** The paths are set in `config.yaml` → `triage.memory_artefacts`. The default is the PA workspace's `MEMORY.md`, `PROJECT.md` and `WORKORDER.md` files, plus any harness-native memory store the adapter declares.
- **Leads, not evidence.** The written text is kept as `note` steps (§5.1). It points extraction at the right steps, and P13 applies to it as to any agent text.

A **small model** handles the ambiguous remainder, answering "Is there durable, repo-specific knowledge here?" with reasons. Up to 20 episode synopses go into one call.

**Salience windows (mechanical, no LLM).** For each kept episode, triage also selects the step ranges that extraction will read:

| Window around | Extent |
|---|---|
| A failure and its eventual fix or abandonment | From the first failure to 2 steps after the success or abandonment |
| A revert or abandoned edit | ±5 steps |
| A human message with a decision, preference, correction or approval signal | ±3 steps; human messages themselves are always included |
| A write to a memory artefact | The `note` step, plus the steps back to the start of the failure, discussion or decision it records (at most 20) |
| A reasoning span that weighs alternatives | The span, +2 steps |
| A device, flash or soak outcome | ±5 steps |

Each episode also gets a **synopsis**: files touched grouped by subsystem, commits, the outcome list, exposures, and every human message verbatim. Windows plus synopsis should roughly halve the extraction payload (§4.4.1). Recall is the risk. B5 lists durable knowledge the extractor missed, and if misses fall outside windows, the window rules widen.

### 6.5 Stage 3: Extract

**Input:** the episode's synopsis and salience windows (§6.4), plus the entity list for the subsystems it touched (names only). Episodes are packed into one call up to the 60k payload cap, with a parent session and its subagents first.

**Output:**
- candidate claims, each with type, statement, `relevant_when` and `apply` lines (§5.3), anchors (by entity reference), conditions, `closes_when` for `open-issue` candidates (§5.3.1), evidence (step indices and outcomes), proposed status and extraction rationale;
- `gaps`: things the episode relied on that match neither a claim nor an entity, such as an external service, a protocol detail or a board quirk. They feed coverage (§9.1).

Extraction rules, enforced by prompt and schema:

1. **Store only what the code can't tell you.** Reject claims that restate code structure.
2. **Status from outcomes.** `observed` or better requires a tool outcome at the cited step. Otherwise the claim is a `hypothesis`.
3. **Failure triage** (from Dream-RSI's policy prompt). Each failed attempt is classified as `approach-unrecoverable`, `implementation-bug`, `environment` or `insufficient-evidence`. Only `approach-unrecoverable` yields a `ruled-out` claim. Implementation bugs yield at most a `failure-signature`.
4. **Human statements** become candidates with `evidence.kind: human`. Reconcile promotes them to `confirmed` if they're decisions.
5. **Conditions** are captured whenever a claim depends on a version, board, chip or config value.
6. **External content** (web pages, issue text, third-party docs quoted in the session) is tagged `provenance: external` (§11.2).
7. **Evidence independence (P13).** Cited evidence may only be tool outcomes (not from memory tools) and `human`-authored steps. Agent text and reasoning are where candidates come from, never what supports them. Automation-authored steps are never evidence.
8. **Exposure awareness.** The extractor sees the episode's exposure steps. A candidate that restates a claim the session was exposed to earlier is marked `echo_of: clm-…` rather than presented as new knowledge.
9. **Match blocks for ruled-out claims.** Every `ruled-out` candidate carries a `match` block: the concrete commands, config keys, symbols and parameter values from the failed attempts. Stage 5 checks that each term actually appears in those attempts, so déjà-vu tier 1 (§7.6) has precise terms to match.
10. **Notes are leads.** A `note` step (§5.1) shows what an agent thought was durable. A candidate drawn from it still needs a tool outcome or a human step as evidence (rule 7), and it is restated from the evidence rather than copied from the note.
11. **Work status is referenced, not restated.** Where the PA ledger, an issue or a branch already records the state of some work, an `open-issue` claim points at it through `closes_when` instead of copying the state into its text.

### 6.6 Stage 4: Reconcile

For each candidate:

1. Look up existing claims sharing any anchor, plus claims on the same subsystem of the same type (deterministic, bounded to 30 or fewer).
2. The stronger model decides one operation:

| Op | Effect |
|---|---|
| `create` | New claim |
| `support` | Add evidence to an existing claim; may raise status (e.g. `observed` → `verified` on second independent episode) |
| `refine` | Edit the statement or conditions of an existing claim (keeps ID; old text kept in history) |
| `contradict` | Record `contradicts`; if the new evidence is stronger by the trust ladder, mark the old claim `refuted` |
| `supersede` | New claim replaces the old one (design changed); old one becomes `superseded` |
| `promote` | e.g. `hypothesis` → `verified` when this episode tested it successfully |
| `discard` | Duplicate or not durable |

3. Ops that would lower the status of a `confirmed` claim are never applied automatically. They're queued for human review.
4. **Evidence independence (P13).** When the episode was exposed to the claim before the candidate's evidence:

| Situation after exposure | Treatment |
|---|---|
| The agent restates or follows the claim; outcomes are consistent but don't test it (e.g. followed an invariant and the build passed) | No `support`. The exposure is recorded as `used` for push-precision metrics only (§10.1). |
| A new outcome **tests** the claim, i.e. it would have come out differently had the claim been false (e.g. a ruled-out approach retried and failed the same way; a failure signature matched and its fix resolved the error) | `support` is allowed, with the outcome as evidence |
| An outcome contradicts the claim | `contradict`, handled normally and flagged as high-value: memory was followed and was wrong |
| A human explicitly confirms or corrects the claim | Counts as human evidence (`confirmed` or a correction). A human message that merely continues the work ("ok, carry on") doesn't. |

This applies to every session, including bug-fixer conversations and report conversations. The count of suppressed supports is reported (§10.1).

### 6.7 Stage 5: Verify

- **Anchor resolution:** every anchor must resolve on the claim's scope branch. Compute fingerprints (symbol body hash, config value, file-window hash).
- **Behavioural check:** for `invariant`, `fact` and `decision` claims about code behaviour, a small model reads *only* the anchored snippets and answers "Is this statement consistent with this code?" Inconsistent claims get the `needs-review` flag.
- **Branch scoping:**

| Event | Effect |
|---|---|
| Claim from a branch not yet merged | `scope.branch = <branch>`; delivered only in sessions on that branch or its descendants |
| Branch merged to main | Scope moves to `main`; `merged` status is available |
| Branch abandoned (deleted unmerged, marked abandoned by the PA, or idle > 30 days unmerged) | `ruled-out` and `failure-signature` claims are promoted to `main` scope; others are retired with a note |
| Commit reverted on main | Claims whose evidence was that commit are re-evaluated; the revert is itself evidence |

### 6.8 Stage 6: Publish

- Write or update claim files and regenerate the affected views.
- Measure each synthesised view against its budget (§5.5). An over-budget view is re-synthesised once, with the overshoot stated and an instruction to shorten, rather than truncated. If it is still over, its lowest-priority sections are replaced by links to the claim index.

Publishing has two lanes, so review never delays value (P12).

**Local lane (immediate).** New and changed claims are written to `<data-dir>/<repo-id>/pending/` and indexed straight away. On this machine, delivery uses them at their evidence status, marked `unreviewed`. Exceptions: security-sensitive claims (§11.2) and `provenance: external` claims are never delivered until reviewed.

**Shared lane (reviewed).**

- Pending ops are batched into a **memory PR** on branch `memory/pending`, opened or updated daily by default. Nothing is ever committed to the user's current branch or directly to main.
- The PR description groups ops by subsystem with evidence links. It highlights anything touching `confirmed` claims, `invariant` claims or security-relevant subsystems.
- Once merged, claims leave `pending/` and are read from `.memory/claims/`.

**Review profiles** (`config.yaml` → `review.profile`):

| Op | `solo` (default) | `team` |
|---|---|---|
| `support`, `promote` on non-security claims | Auto-merged | Auto-merged |
| `create`, `refine` of `fact`, `failure-signature`, `procedure`, `convention` | Auto-merged at `verified` or higher; listed in a weekly digest | Reviewed |
| `create`, `refine` of `invariant`, `decision`, `ruled-out` | Reviewed | Reviewed |
| Anything that lowers a `confirmed` claim, touches security-relevant subsystems, or has `provenance: external` | Reviewed | Reviewed |

Auto-merged ops are still visible in the PR history, and any of them can be reverted.

### 6.9 Model and cost policy

- **Small model** for triage, extract and behavioural verify. **Stronger model** for reconcile and view synthesis only.
- **Executor:** housekeeping runs via `agy -p` on the user's Antigravity quota, or (**[R2]**) via `claude -p --safe-mode` on Claude Code-only machines (§4.4, D2). No API key and no batch API. The cost model is in §4.4.1.
- **Model per stage**, chosen in `config.yaml` from `agy models`. Defaults:
  - `gemini-3.8-flash-low` for triage and verify;
  - `gemini-3.8-flash-medium` for extraction;
  - `gemini-3.1-pro-low` for reconcile and view synthesis.
- **Every LLM stage is a pure function:** deterministic context assembly, one `agy -p` call with `--json-schema`, then the result is written. No tool use inside housekeeping runs. That makes stages batchable, retryable, idempotent and replayable during re-projection.
- **Optional `api` backend.** A direct-API executor (with provider batch APIs and caching) is part of the design for other deployments. It is off by default and unused here.
- Hard caps per episode: extraction input ≤ 60k tokens (split larger episodes by subagent or phase); ≤ 20 candidates per episode.
- Daily budgets for housekeeping calls and tokens (from the `agy` usage output). The backfill is throttled by them: OTA and BLE first, then the rest by churn.
- The tick records token spend per stage and per housekeeping run (§10.4).

---

## 7. Delivery (read side)

### 7.1 Budgets

| Channel | Budget | Dedup |
|---|---|---|
| Always-on (system map) | ≤ 2k tokens | n/a |
| Task brief | ≤ 8k tokens | once per session |
| File-touch injection | ≤ 600 tokens per event | per claim per session |
| Error injection | ≤ 600 tokens per event | per signature per session |
| Risky-action injection | ≤ 800 tokens per event | per claim per session |
| Déjà-vu alert | ≤ 300 tokens | per claim per session |
| Per-session push cap | 20k tokens total | n/a |
| Claim index, read by the agent itself | ≤ 1.5k tokens per subsystem | n/a (pull) |
| MCP tool schemas (standing cost) | Measured, reported (§10.4) | n/a |

MCP tool descriptions are listed in the system prompt of every session and count against Antigravity's customization budget, not the rules budget. Their size is measured and reported rather than guessed. Descriptions are kept terse.

A per-session **delivery ledger** records every claim delivered, so delivery is never repeated and the metrics in §10 can be computed.

**Rendering.** In the task brief and in touch, error and risky-action injections, each claim is one line with three parts:

1. its `apply` line, or its title if it has none;
2. its ID;
3. a stamp saying how current it is: `verified 2026-09-24 @69d9c813`, `open as of 2026-09-30`, or the `suspect` or `conditions-changed` marker.

Before injecting, the hook checks that each claim's anchored files still exist. That is a file-system check, well within §7.8. A missing file renders the claim as `suspect` even if the tick hasn't re-checked it yet. The plugin's rules file tells the agent once that memory reflects the stamped point in time, and that it should check named files and symbols before relying on them.

### 7.2 Task start

1. Always-on: `MAP.md`, loaded by the harness's own instruction file. The repo's root `AGENTS.md` (Antigravity) and `CLAUDE.md` (Claude Code) each include it: `@[memory map](.memory/views/MAP.md)` and `@.memory/views/MAP.md` respectively. This costs ~2k of the 20k rules budget and is cached with the system prompt. Fallback, if include semantics don't work as expected (P0 experiment E2): inject at `PreInvocation` with `invocationNum == 0`.
2. At the first model invocation, the delivery hook reads the task text (first user message) and branch from the transcript.
3. The hook links the task to entities using the local index:
   - lexical match on entity names, paths, symbols and error strings;
   - plus files already open or mentioned;
   - if lexical matching yields nothing, the hook injects the map-level summary with a one-line pointer to the claim indexes (§5.5), so the agent can choose the relevant subsystems itself at once. It also spools a brief request. The next tick prepares the brief in a housekeeping run, and the next `PreInvocation` picks it up from `alerts/` (typically within ~75 s). If it needs the brief sooner, the agent can call `memory.brief` synchronously (~13 s). An LLM is never called inside a hook.
4. It renders the **task brief**, in priority order:
   1. invariants of the involved subsystems;
   2. open issues and in-flight work;
   3. ruled-out approaches;
   4. relevant procedures;
   5. decisions.
5. The agent can call `memory.brief(task)` later to refresh it when the task direction changes.

### 7.3 File touch

When the agent reads or edits a file whose entities (file, symbols, subsystem) have pushable claims not yet delivered this session, inject:

- `invariant`, `ruled-out`, `fact` and `open-issue` claims for those entities, most specific anchor first;
- one line per claim, rendered as in §7.1 (`apply` line, ID and currency stamp). The agent can call `memory.why(<id>)` for evidence.

### 7.4 Error

When a tool outcome (command, test, device) contains text matching a `failure-signature` `match` block, inject the signature's cause and fix before the agent's next reasoning step. Matching is deterministic (string and regex on the ledger excerpt).

### 7.5 Risky action

Default risky actions (configurable):

- flash, monitor and OTA upload tools;
- release scripts;
- edits to `sdkconfig*`, `platformio.ini`, `dependencies.lock` or partition tables;
- dependency upgrades.

**Timing constraint.** `PreInvocation` runs before the model decides what to do, so it can't see a concrete action until after that action has run. The only event between a proposed action and its execution is `PreToolUse`, and it can't inject context. It can return `deny` or `ask` with a `reason`, which the agent (or user) sees.

**Delivery modes** (per pattern, in `config.yaml`):

| Mode | Behaviour | Default for |
|---|---|---|
| `gate-once` | At `PreToolUse`, if the relevant `procedure`/`invariant` claims haven't been delivered this session, return `deny` with a reason containing them (≤ 800 tokens) and an explicit "re-issue the call to proceed". The ledger marks them delivered, so the re-issued call is allowed. Costs one extra round-trip, once per session per claim set. | Flash, release, partition-table edits |
| `ask` | At `PreToolUse`, return `ask` with the claims as the reason, so the user decides. | Off by default; for teams that want a human in the loop |
| `next-invocation` | Allow the call; queue the claims for the next `PreInvocation`. No extra round-trip, but delivery comes after the action. | sdkconfig and dependency edits |
| `anticipate` | Inject at `PreInvocation` when the transcript suggests the action is imminent (e.g. a successful build in a session that has flashed before). | Optional addition to any mode |

This must coexist with existing gates such as esp32-app-foundation's `flash-tag-gate.py`. Hooks for the same event run in sequence; a `deny` from either stops the call.

### 7.6 Déjà-vu alerts (optional, warm path)

The main defence against repeating a dead-end is not an alert at all:

- **Tier 0: proactive delivery (primary).** Ruled-out approaches for an entity are delivered in the task brief (§7.2) and on first touch of that entity (§7.3), before any plan to retry exists. Most prevention happens here.
- **Tier 1: lexical safety net.** It catches a concrete retry as it is proposed.
- **Tier 2: late correction.** On Antigravity it arrives about a minute behind, so it corrects the *next* attempt rather than preventing the first. On Claude Code it arrives in seconds through `asyncRewake` (§12.2).

**Tier 1: lexical (no LLM).**

- Every `ruled-out` claim carries a `match` block, generated at extraction from the failed attempts' concrete commands, config keys, symbols and values (§6.5, rule 9). Example: `any: ["vTaskDelay", "INT_WDT_TIMEOUT", "CHECK_CPU1"]` scoped to the entities `BLEStack::init` and `NimBLEDevice::init`.
- A match requires two conditions:
  1. an anchored entity is in play in the session (touched or mentioned);
  2. the claim's `match` terms appear in the text being checked.
- **At `PreToolUse`**, the hook checks the concrete proposed command or edit. A match against a `verified` or stronger `ruled-out` claim uses `gate-once`: `deny` with "This was tried on <date> and failed because <reason> (clm-…). If your approach differs, re-issue the call." This is the only way to catch the retry *before* it runs.
- **At `PreInvocation`**, the hook checks the latest reasoning in the transcript tail. A match queues a ≤ 300-token alert for this invocation. This catches plans that haven't turned into a tool call yet.

**Tier 2: semantic (housekeeping run on the next tick; on Claude Code, an `asyncRewake` hook).**

- At `PreInvocation`, if new reasoning mentions an entity with `ruled-out` claims and tier 1 didn't fire, the hook spools a request.
- The next tick asks a small model whether the plan is substantively the same as a ruled-out approach. It batches all pending requests into one call.
- On a match, it writes an alert to `alerts/<conversation-id>/`. The next `PreInvocation` delivers it: "You're about to retry X. It failed on <date> because Y (clm-…)."
- On Antigravity, latency is up to about a minute plus the call time. In a fast debugging session the agent may already be committed to the first retry by then, so treat tier 2 as a correction for the *next* attempt. It can still stop the second and third retries, which is where the 22 September BLE session lost most of its time.
- **[R2]** On Claude Code, a `PostToolBatch` hook spools the request and starts an `asyncRewake` hook that runs the check at once. On a match it exits with code 2, and Claude is woken with the alert as a system reminder within seconds.
- Each check is an interactive small call dominated by fixed overhead (§4.4.1). All pending checks are batched into one call per tick, and there are per-day caps.
- Requests are rate-limited per conversation (default: at most one in flight, at most one per 5 model calls).
- Default: enabled for `ruled-out` claims with status `verified` or higher only.

### 7.7 Pull tools

| Tool | Returns |
|---|---|
| `memory.ask(question)` | Answer synthesised from claims, with claim IDs; falls back to episode search with explicit "unverified" marking |
| `memory.why(target)` | For a file, symbol, config key or claim ID: decisions and invariants plus evidence chain (claim → episodes → commits) |
| `memory.history(entity)` | Chronological claim and supersession history for an entity |
| `memory.brief(task)` | Regenerated task brief |
| `memory.note(text)` | Lets the agent or user flag something explicitly. Recorded as an event with `provenance: agent-note` or `human`, reconciled like any other candidate. This is optional and costs the working agent a single short tool call. |
| `memory.stats()` | Observability summary (§10.4): claim counts by type, status and subsystem; delivery and hit-rate figures for recent sessions; pipeline cost; coverage gaps |

All six tools ship from the start. Their use is instrumented (§10.4), so trimming or merging tools later is a data-driven decision.

### 7.8 Performance

Antigravity hooks run synchronously and block the agent loop, and `PreInvocation` runs before *every* model call. Latency therefore adds up: at 100 ms per call, a session with 300 model calls gains 30 s.

- **Delivery hook fast path.** Check the conversation's `alerts/` directory and read the transcript from the last cursor (tail only). If nothing new matches, exit. Target: p95 ≤ 60 ms, p50 ≤ 30 ms.
- **Delivery hook with injection.** Local SQLite read-only queries plus formatting. Target: p95 ≤ 150 ms.
- **Implementation.** Hooks run the compiled Go binary (§18), so start-up costs milliseconds. E5 confirms the fast-path budget on macOS and Windows.
- **Measurement.** Cumulative hook time per session is recorded (§10.4) and reported against a session budget (default ≤ 2% of wall-clock time).
- **Failure behaviour.** No LLM calls and no network in hooks. If the index is missing or locked, the hook exits with an empty injection. Memory must never block or break work.

### 7.9 Degraded modes

| Level | Condition | What still works |
|---|---|---|
| **Full** | Sidecars running, index current | Everything |
| **No ticks** | Scheduled sidecar not running (Antigravity closed, sidecars disabled, machine without Antigravity and no OS schedule) | Capture (`Stop` copies transcripts), delivery from the last index, déjà-vu tier 1, pull tools (`ask`/`brief` synthesis still runs synchronously). Spooled work waits for the next tick. |
| **No housekeeping** | `agy` missing, signed out, or quota exhausted | Capture, delivery and pull tools that don't need synthesis. LLM jobs stay queued; the circuit breaker stops retries until `agy` works again. |
| **No index** | Index missing or corrupt | Map via the `AGENTS.md`/`CLAUDE.md` include; the agent can walk MAP → claim index → claim files (§5.5); pull tools fall back to text search over `.memory/views/` and claim files; hooks no-op. The index can be rebuilt with `repo-memory reindex`. |
| **Native floor** | No hooks and no tick (e.g. a harness where the plugin's hooks are disabled) | The always-on map include; generated path-scoped subsystem rules (`.claude/rules/` with `paths:` on Claude Code; Glob rules on Antigravity, pending E4/C6); MCP pull tools over the last index |
| **Plugin off** | Plugin disabled or not installed (e.g. a collaborator) | `.memory/views/*.md` and claim files remain readable Markdown in the repo; the map include still loads if present, and any agent can follow it to the claim indexes |

---

## 8. Validity and re-checking

Re-checks are triggered by dependency changes, never by time or disuse.

| Change detected | Claims affected | Action |
|---|---|---|
| Merge to main changes a file | Claims anchored to changed symbols or windows (fingerprint mismatch) | Set `suspect`; queue a behavioural verify; on pass, clear and restamp `verified_at`; on fail, send to Reconcile with the diff as evidence |
| Symbol or file deleted or renamed | Claims anchored to it | Attempt re-anchor (rename detection); else `needs-review` |
| `dependencies.lock` / `platformio.ini` / SDK version change | Claims whose `conditions` reference the changed item | Set `conditions-changed`; `ruled-out` claims are delivered as "may work now" |
| sdkconfig value change | Claims anchored to that config key | Set `suspect`; verify |
| Revert on main | Claims with the reverted commit as evidence | Re-evaluate |
| A closure condition fires: issue closed, branch merged, work order complete, or a later episode shows the success outcome | `open-issue` claims whose `closes_when` lists it (§5.3.1) | Set `resolved`, with the closing event as evidence. If a fixing episode exists, its own extraction may yield a `failure-signature` or `decision`. |
| Human correction | The named claim | Apply as a `confirmed` or `refuted` event |

**Evaluating closure conditions.** Branches and PA work orders are checked every tick (local git and PA ledger reads). Outcome conditions are checked as each new episode is processed. Issue state needs the network, so it is checked at most daily through the git host's CLI.

**Explicitly out of scope:** decay by age, decay by retrieval frequency, and confidence scores that change with use. A claim about a subsystem nobody has touched in a year is as valid as the day it was verified, because nothing it depends on has changed. `open-issue` claims are no exception: they close on events, not with age.

---

## 9. Coverage and elicitation

### 9.1 Coverage model

For each subsystem, the pipeline computes a coverage record:

| Dimension | Measure |
|---|---|
| Rationale | Number of `decision` claims vs. key design points (public interfaces, major files) |
| Invariants | Number of `invariant` claims and the share enforced by a test or gate |
| Verification | Presence of at least one `procedure` for testing or verifying the subsystem |
| Failure knowledge | `failure-signature` and `ruled-out` counts relative to churn and incident history |
| Freshness | Share of claims that are `suspect` or `conditions-changed`, and `open-issue` claims with `closes_when: unknown` |
| Referenced gaps | `gaps` reported by extraction (§6.5) for the subsystem and not yet covered by a claim, counted by how many episodes mention each |

**Gap score** = churn (commits in last 90 days) × complexity × (1 − coverage). Frequently referenced gaps are also candidates for elicitation questions (§9.2).

### 9.2 Elicitation

- Weekly (configurable), the tick picks **1–3 targeted questions** for the highest gap scores. For example: "Why does DataManager use FetchContext deadlines rather than per-source timeouts?"
- Questions go into the weekly **review report**, an IDE-visible conversation (§4.4), never into a working session.
- The human answers in that conversation. The sweep captures the conversation, and the human's messages become `human_answers` events, which become `confirmed` claims. The report's own text is never treated as evidence.
- The questions are also written to a `QUESTIONS.md` view, so they can be answered later or by someone else.

#### 9.2.1 Reading replies from report conversations

Report conversations mix three kinds of text: the automated kickoff prompt, the report agent's generated text (derived from memory), and the human's replies. Only the last is evidence. The mechanism must not depend on an LLM guessing who said what:

1. **Record what was asked.** When the tick starts a report, the job table records the conversation ID, the kickoff prompt's hash, and the question IDs and claim IDs presented (e.g. `Q-017`, `clm-…`).
2. **Classify authors structurally.** Author classification (§5.1) uses the harness's own message roles, excluding the kickoff prompt. The report agent's text counts as an exposure to every claim it presented, so nothing it says can support those claims (P13).
3. **Parse explicit commands mechanically, without an LLM,** from human-authored steps: `approve clm-…`, `reject clm-…`, `approve all <subsystem>`, and `Q-017: <answer>`. These become review events and `human_answers` events directly.
4. **Interpret free-text replies conservatively.** A small model maps each reply to the question or claim IDs it addresses. The human's verbatim text is stored as the evidence.
   - A resulting claim is `confirmed` only if the reconciler finds the human's text states the fact explicitly.
   - Otherwise it is `needs-review`. The next report quotes the interpretation back for confirmation: "I understood X from your answer to Q-017. Correct?"
5. **Give the report agent no memory-write tools.** It is instructed to present questions verbatim and never answer on the human's behalf. Anything it types is excluded structurally in any case.

### 9.3 Inferred backfill

For subsystems with no rationale coverage, a one-off pass infers `decision` claims from code, commit messages and PR descriptions. These claims have status `hypothesis` and are tagged `inferred`. They're shown in views as "inferred, unconfirmed" and are never pushed until a human confirms them.

### 9.4 Invariant enforcement

Each `invariant` claim may carry `enforced_by: <test|lint|gate path>`. Unenforced invariants in high-churn subsystems are listed as **proposed work** in the weekly review report, for example "turn this invariant into a CI check". The memory system never changes code itself (§4.4). The human, a PA project or the bug-fixer loop picks up the work in an IDE session. Enforced invariants are re-checked by CI, not only by the pipeline.

Other proposals raised the same way include human-facing docs that contradict verified claims, and stale `framework-expert` skill sections.

---

## 10. Measurement

### 10.1 Per-session signals (computed by the pipeline after each episode)

| Metric | Definition | Target direction |
|---|---|---|
| **Rediscovery rate** | Share of extracted candidates that `support` an existing pushable claim which was *not* delivered in that session | ↓ |
| **Repeated dead-end rate** | Sessions in which the agent attempted an approach matching a `verified` `ruled-out` claim | ↓ |
| **Push precision** | Delivered claims that the agent referenced or acted on ÷ delivered claims | ↑ |
| **Tokens pushed per session** | From the delivery ledger | Within budget |
| **Sidecar tokens per session** | By stage | ↓ over time |
| **Main-agent memory tokens** | Tokens the working agent spent writing memory artefacts | → 0 |
| **Suppressed supports** | Candidates that would have supported a claim but were rejected under P13 because the session had been exposed to it | Monitored. A sudden rise signals a circularity leak or over-strict rules. |
| **Interactive call rate** | LLM brief fallbacks, tier-2 checks and pull-tool syntheses per session, with their tokens | Within caps (§4.4.1) |
| **Coverage** | §9.1 | ↑ |

### 10.2 Offline tuning (replay)

Recorded episodes serve as a fixed benchmark for the **delivery policy** (trigger rules, budgets, ranking):

- For each episode, simulate delivery using only the episode *prefix* up to each step (prefix-only observability).
- Label relevance from the *suffix*: did the session later touch the claim's anchors, hit its signature, or re-derive it?
- A candidate policy replaces the current one only if it scores higher on the fixed set (in-sample no-regret selection, as in Dream-RSI).
- Labels are proxies (delivering a claim would have changed the trajectory). Use replay for tuning and online metrics for acceptance.

### 10.3 Acceptance benchmarks (for the build phases)

| ID | Benchmark |
|---|---|
| B1 | **pr-225 facts benchmark.** Build the claim store from events up to commit `69d9c813`. Give a fresh agent pr-225's original request plus memory delivery, and have it produce the equivalent of "Framework Facts Workers Need". Compare against the PA's hand-written section for correctness, completeness, tokens and tool calls. Baseline: the same task with today's PA memory. |
| B2 | **Question bank.** ≥ 50 questions drawn from session logs and ADRs ("what's been ruled out for…", "why is X…", "what does error Y mean…"), answered with `memory.ask` vs. baseline. |
| B3 | **Task replays.** 3–5 historical tasks re-run from their starting commits in worktrees (no hardware-in-the-loop), with and without memory. Measure turns, tokens and repeats of known dead-ends. |
| B4 | **Cost.** Sidecar tokens (by stage, model tier and token type) vs. PA memory work for a comparable period. The PA side is broken down by activity: boot reads (frontier, input); session-log distillation (frontier, output); worker completion reports (worker model, output); "Framework Facts" sections (frontier, output); index graduation (frontier, output); Chief Architect programme logs (frontier, output). Priced per tier, not just summed. |
| B5 | **Extraction precision.** A labelled sample of ≥ 20 episodes (OTA and BLE history). A human marks each extracted claim as correct / wrong / not durable, and lists durable knowledge the extractor missed. Reports precision and recall per claim type. |

**Gates.**

- **P1 exit** requires B5 precision ≥ 0.8 for claims that would be pushed (status `verified` or higher), plus B1 at parity with the PA's hand-written section on correctness and completeness.
- **If B5 falls short**, the fallback is not to abandon the design:
  1. Raise the triage threshold.
  2. Restrict push to `merged` and `confirmed` claims.
  3. Switch the `solo` profile to review-all.
  4. Iterate on extraction. Because knowledge is event-sourced, re-projection applies the improved extractor to all history.

  The episode search (P0.5) keeps the system useful in the meantime.

### 10.4 Observability

Everything the plugin does is instrumented from day one. Metrics are private (`<data-dir>/<repo-id>/metrics/*.jsonl`), aggregated by `repo-memory report` and `memory.stats`.

| Signal | Source |
|---|---|
| Push injections: trigger, claim IDs, tokens, session | Delivery ledger |
| Exposures per session (push, pull, `.memory/` reads) | Episode exposure steps |
| Pull tool calls: tool, arguments (hashed), response tokens, latency | MCP server |
| Hook execution time per event type (p50/p95/p99) and cumulative per session | Hook scripts |
| MCP tool schema tokens per session | MCP server at start-up |
| Housekeeping runs: count, duration, input/output/thinking tokens (from `agy` JSON), model, per stage and per episode | Tick |
| Tick health: duration, budget exhaustion, skipped overlaps, job backlog by kind, sessions caught only by the sweep, spool-to-delivery latency | Tick |
| Report conversations started, and human replies captured from them | Tick |
| Capture health: sessions seen, captured, skipped (recursion guard), missed (transcript gone before sweep) | Sweep |
| Claim lifecycle: created, supported, refined, superseded, refuted, by type and subsystem | Claim store and review events |
| Agent use of delivered claims (referenced or acted on vs. ignored) | Post-session analysis in stage 3 |
| Review load: ops per PR, time to review, auto-merged vs. reviewed | Publish stage |
| Rediscovery and repeated dead-end rates (§10.1) | Stage 4 |

---

## 11. Security and governance

### 11.1 Privacy

- Raw transcripts and episodes stay on the developer's machine. Only claims, views and human review events go into the repository.
- Claims must not quote conversation verbatim beyond short error strings. Extraction instructions and a post-extraction check enforce this.

### 11.2 Poisoning and untrusted input

- Candidates derived from external content (web pages, third-party issues, fetched docs, dependency READMEs) carry `provenance: external`, max status `observed`, and are never pushed without human confirmation.
- Claims whose statements instruct actions on credentials, deployment, network endpoints, signing keys or security settings are always `needs-review`, whatever their evidence.
- Status can't be raised by retrieval or use (§5.3.2), which prevents self-reinforcement loops.

### 11.3 Secrets

- Scrubbing at compaction (§5.1), plus a second secret scan on claim files before publish. Any hit blocks publish for that claim.

### 11.4 Integrity

- A lint gate runs in CI on `.memory/`:
  - schema validation;
  - views match regeneration (no hand edits);
  - every claim has ≥ 1 evidence entry;
  - every anchor resolves or is flagged;
  - no secrets.
- The memory PR is the only write path to shared memory.

---

## 12. Harness integration

### 12.1 Antigravity

Verified against Antigravity's built-in customization docs (`builtin/skills/agy-customizations/docs/`) and the public hooks and sidecars pages (2026-09-30):

**Event path (hooks):**

| Hook | Used for | Returns |
|---|---|---|
| `Stop` | Capture: spool entry and transcript copy | No `decision`, so the agent is never forced to continue |
| `PreInvocation` (every model call) | `invocationNum == 0`: task brief (inline, or a spooled request). Every call: read `transcriptPath` from the last cursor and `alerts/`; touch, error and déjà-vu tier-1 delivery; spool tier-2 requests. Exit immediately if `REPO_MEMORY_HOUSEKEEPING` is set. Must meet §7.8. | `injectSteps` → `ephemeralMessage` when something is due; otherwise `{}` |
| `PreToolUse` (matcher: `run_command\|write_to_file\|replace_file_content\|call_mcp_tool`) | Risky-action gates and déjà-vu tier 1 on the concrete proposed call (§7.5, §7.6) | `allow`, or `deny`/`ask` with a `reason` for `gate-once`/`ask` modes. Never `overwrite`. |
| `PostToolUse` (matcher: `run_command`) | Detect git merge/checkout/pull/rebase or dependency updates by reading the transcript step at `stepIdx`; spool a re-check | `{}` |

`PostInvocation` is not used.

**Time path (scheduled sidecar):**

| Sidecar | Schedule | Runs |
|---|---|---|
| `repo-memory/tick` | `builtin: schedule`, `* * * * *` | `repo-memory tick` (§4.3, §12.3) |

The tick follows the bug-fixer tick pattern: deterministic checks every run, LLM work only when there is something to process. Sidecars are disabled by default and must be enabled in `config.json`. Only one sidecar is needed. The separate watcher tick from v0.2 is folded into it. Report conversations are started with `agentapi`, which needs the sidecar environment (`projectId`).

**Other mechanisms:**

| Need | Mechanism | Notes |
|---|---|---|
| Pull tools | Stateless stdio MCP server in plugin `mcp_config.json`, reading the SQLite index | Tools are listed in the system prompt of every session; schema tokens are measured (§10.4). `memory.ask` and `memory.brief` may make one synchronous housekeeping run (~13 s). |
| Always-on map | `@[…](.memory/views/MAP.md)` include in the repo's root `AGENTS.md` | Fallback: `PreInvocation` at `invocationNum == 0`. |
| Native alternative | Glob and Model Decision rule activation | Described on the public rules page but not in the built-in rules doc. Treat as unverified until tested (E4). |

Plugin layout (installed globally; per-repo data lives in `.memory/`):

```
~/.gemini/config/plugins/repo-memory/
├── plugin.json
├── hooks.json                   # generated by `repo-memory install`: Stop, PreInvocation,
│                                #   PreToolUse, PostToolUse → `<bin> hook antigravity <event>`
├── mcp_config.json              # generated: stdio MCP server → `<bin> mcp`
├── sidecars/
│   └── tick/sidecar.json        # generated: builtin: schedule → `<bin> tick`
├── rules/AGENTS.md              # how to use memory tools and walk MAP → claim index;
│                                # memory reflects its stamped date; never contains repo data
├── skills/memory/SKILL.md       # ask/why/history usage; review workflow
└── bin/repo-memory              # the Go binary for this OS/arch (§18)
```

### 12.1.1 P0 experiments

These unknowns must be settled before P2 builds on them:

| # | Question | Method |
|---|---|---|
| E1 | Does an `ephemeralMessage` persist in the model's context on later turns, and does it appear in `transcript.jsonl`? | Inject a unique marker at invocation N; ask the model about it at N+5; inspect the transcript. Dedupe via the delivery ledger regardless of the answer. |
| E2 | Do `@[…](path)` includes in a root `AGENTS.md` resolve relative to the workspace, and are they re-read when the file changes? | Include `MAP.md`, change it, start a new session. |
| E3 | Is `transcript.jsonl` current at the moment `PreInvocation` fires? | Compare the last step index in the file with `initialNumSteps` and the hook's view. |
| E4 | Are Glob and Model Decision rule activations supported in the installed version? | Minimal rule with each activation mode. |
| E5 | Delivery hook latency of the Go binary on macOS and Windows (fast path and injection path) | Benchmark the fast path and the injection path. |
| E6 | Do scheduled sidecars tick while only the Antigravity CLI is open? What happens on sleep and wake (missed ticks are skipped or caught up)? What happens when a tick is still running when the next one is due? | Observe `sidecar_data/.../logs`. The tick takes `tick.lock` so overlapping ticks exit immediately, as the bug-fixer tick dispatcher does with its slot check. |
| E7 | Are housekeeping runs isolated? Does `--project` place them in a separate project? Can `agy --continue` or the CLI's history ever resume or surface them? Do they ever appear in the app/IDE store (e.g. through the shared history)? | Run tagged housekeeping calls, then inspect both `conversation_summaries.db` stores and try `agy -c` from the repo directory. |
| E8 | Do subagent conversations (e.g. PA workers started with `invoke_subagent`) fire their own `PreInvocation`, `PreToolUse` and `Stop` hooks, with their own `conversationId` and `transcriptPath`? | Spawn a subagent with logging hooks enabled. If not, worker sessions are captured by the sweep only, and workers get their brief through the WORKORDER (§13) rather than through hooks. |
| E9 | How does `PreToolUse` `deny` + `reason` read to the agent in practice? Does it reliably re-issue the call after a `gate-once` denial, or does it abandon the action? | Scripted sessions against a test gate; tune the reason wording. |
| E10 | Can the ~17k-token fixed overhead of a housekeeping run be reduced (a minimal `--agent` definition, fewer global plugins, no skills)? Does repeated use of the same prefix produce cache reads? | Measure `usage` from `agy -p --output-format json` across variants. |
| E11 | How do housekeeping runs count against the Antigravity subscription quota, and what are the rate limits per model? | Track usage and failures over the first two weeks of P1; set the daily budgets from the results. |

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
| C6 | Does one rule file with both `paths:` and Antigravity glob frontmatter load correctly in both harnesses? | Minimal rule in `.agents/rules/` via the `.claude` symlink |
| C7 | Is a Desktop local scheduled task workable as the report surface? | Weekly test task; check permission stalls and catch-up behaviour |

### 12.3 Tick

`repo-memory tick` is a subcommand of the `repo-memory` Go binary (§18). It is not a daemon: each run does its work and exits. The order of work, the job table and the budgets are defined in §4.3.

- **Idle cost.** A tick with nothing to do makes no LLM calls and finishes in well under a second (program start-up plus a few file and hash checks).
- **Git events** are detected by comparing HEAD, `dependencies.lock` and the sdkconfig hashes with the previous tick. No git hooks are installed.
- **Housekeeping runs** are child processes started with `REPO_MEMORY_HOUSEKEEPING=1`, the working directory `<data-dir>/runs/<repo-id>/`, the configured model and `--print-timeout`. The tick waits for them within its budget; a run still going when the budget ends keeps the tick alive until its timeout, and the next tick skips (`tick.lock`).
- **Reports** are started with `agentapi new-conversation --title="[repo-memory] …"` and polled on later ticks, as in the bug-fixer tick dispatcher.
- **Publishing** runs git in the dedicated worktree (`<data-dir>/<repo-id>/worktree/`, branch `memory/pending`), never in the user's checkout.

**`repo-memory run`** is kept as an alias for `tick`, for manual use.

---

## 13. Principal-architect integration

| Today | With repo memory |
|---|---|
| Worker writes a Completion Report by hand | Sidecar generates an **episode summary** from the worker's session; the PA reads that. The worker may add a short free-text note. |
| PA writes session logs into PROJECT.md | PROJECT.md holds intent, plan, work-order ledger and approvals only. Knowledge goes to claims automatically. |
| PA hand-writes "Framework Facts Workers Need" per project | WORKORDERs embed or reference a generated **task brief**. |
| PA maintains MEMORY.md, PROJECT_INDEX.md, MODULE_INDEX.md | Coordination state (active projects) stays; knowledge indexes are replaced by generated views. |
| Abandoned work orders lose dead-ends | Branch-abandonment promotion (§6.7) keeps `ruled-out` claims. |
| Workers learn only what the WORKORDER tells them | Workers get the generated brief in the WORKORDER in every case. On Claude Code, `SubagentStart` also injects it, and hooks inside subagents give workers touch, error, risky-action and déjà-vu delivery. On Antigravity the same depends on E8. |

---

## 14. Migration and backfill

There is no separate migration path. Existing artefacts are ingested as `import` episodes and processed by stages 3–6.

| Source (esp32-app-foundation) | Treatment |
|---|---|
| PA `PROJECT.md` session logs, `WORKORDER.md` completion reports, `context_bridge.md` files | Import episodes (one per file), dated from file or git history; high-yield for `ruled-out`, `failure-signature`, `decision` |
| `docs/design_specification.md` §17 decision log, `docs/developer/adr/*` | `decision` candidates. Status `confirmed` where human-approved per the PA approval records; otherwise `merged` |
| `framework-expert` skill and `docs/developer/*` | `fact`, `procedure`, `convention` and `invariant` candidates; linked, not duplicated |
| Git history (commit messages, PR descriptions, reverts) | Evidence and `decision` candidates; reverts as refutation evidence |
| Surviving Antigravity transcripts (**[R2]** Claude Code transcripts preserved by the P0 sweep-only copy) | Normal compaction and processing |
| Antigravity conversation summaries (`conversation_summaries.db`) | Low-weight import episodes (titles and previews only) for coverage and dating |

**Order:**

1. Configure subsystems.
2. Import OTA and BLE first (deep history and known stale records).
3. Review.
4. Import the rest, highest churn first.

**Verification is mandatory.** Stage 5 runs against current HEAD. Historical records that describe since-fixed defects (e.g. the June OTA `_silentCheck` record) must end as `superseded` or `refuted`, not as open issues.

---

## 15. Build phases

### 15.1 Release 1: Antigravity

| Phase | Scope | Exit criteria |
|---|---|---|
| **P0 Capture** (urgent) | **Go skeleton (§18):** the `hook`, `tick` and `install` subcommands; the harness interface with the Antigravity adapter and the `fake` test adapter; the package-boundary check. **Capture:** `Stop` hooks (spool and copy); tick sidecar with the job table (spool ingestion; sweep of the app and CLI stores; idle promotion; compaction with author classification and exposure markers); scrubber; private episode store; metrics skeleton (§10.4); experiments E1–E10. **Recommended:** a sweep-only copy of Claude Code transcripts into `raw/claude-code/` (copied, not parsed), so the 30-day retention doesn't erase history before Release 2 (§18.6, rule 11). No LLM stages yet; E10 uses a few measured test runs. | Every new Antigravity session produces an episode within minutes of going idle; sessions captured before their transcripts disappear; the share caught only by the sweep measured; compaction ratio ≥ 4× measured; zero secrets in a scan of 50 episodes; experiments answered; core tests pass on the `fake` adapter; `internal/core` imports no concrete adapter. |
| **P0.5 Episode search** | `repo-memory search` CLI and a `memory.ask` fallback over episodes (SQLite FTS). Import of PA files as searchable episodes (no extraction yet). | You can find what was tried, and when, across all captured history. This also shows whether episodes contain what extraction will need. |
| **P1 Knowledge** | Entity extractor with extractor rules; subsystem bootstrap; stages 2–6 including salience windows and the evidence-independence rules; claim schema; lint gate; two-lane publish, memory PR and weekly review report with reply parsing (§9.2.1); views (map, subsystems, claim index, signatures, decisions); import of OTA and BLE history; E11. | **Gate:** B5 precision ≥ 0.8 for pushable claims **and** B1 at parity with the PA's hand-written facts section. B2 shows improvement over baseline. Pipeline cost per episode and per stage measured (§4.4.1). |
| **P2 Delivery** | Normalised hook-event model and capability-based delivery policy (§18.3), with the Antigravity codec as the first implementation: fast path, map include, task brief, touch, error, risky action, déjà-vu tier 1. Delivery ledger; all six pull tools, instrumented; generated path-scoped rules (native floor, after E4). The adapter contract suite (§18.6) is written against Antigravity fixtures. | B3 run; per-session push within budget; hook latency within §7.8 on macOS and Windows; degraded modes (§7.9) tested; adapter contract suite green. |
| **P3 Learning loop** | Déjà-vu tier 2 (tick housekeeping run); coverage and elicitation; per-session metrics (§10.1); offline replay tuning (§10.2); `repo-memory report`. | Rediscovery and repeated dead-end rates reported weekly; first policy update accepted by replay. |
| **P4 PA integration** | Generated episode summaries and task briefs in the PA workflow; retire knowledge indexes. | Main-agent memory-writing tokens ≈ 0 for a full project cycle; B4 shows lower total cost. |
| **P5 Full backfill** | Import all remaining history; inferred rationale pass; re-projection tooling. | Every subsystem has a page; coverage report published. |

### 15.2 Release 2: Claude Code

Release 2 can start once P2 is complete: the normalised event model and capability-based delivery are then proven on Antigravity. It doesn't wait for P3–P5. Its design is in §12.2 and `docs/claude_architecture.md`.

| Phase | Scope | Exit criteria |
|---|---|---|
| **R2.1 Capture and knowledge** | Claude Code adapter: a versioned, tolerant transcript parser with fixture corpus and canary (§12.2.1); processing of the raw transcripts preserved since P0; `Stop`/`SubagentStop` capture hooks with the side-log; auto-memory import (§12.2.2); experiment C5. | Claude Code sessions produce episodes. The Release 1 adapter contract suite passes unchanged against Claude fixtures. Claude-derived candidates reconcile into the same claim store as Antigravity-derived ones. |
| **R2.2 Delivery** | Claude hook codec on the normalised event model (`UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `PostToolUseFailure`, `PostToolBatch`, `SubagentStart`) and its capability declaration; `CLAUDE.md` map import; dual-frontmatter path-scoped rules (C6); experiments C2, C4, C6. | B3 in Claude Code; hook latency budget met. Cross-harness delivery demonstrated: a claim learned in an Antigravity session is delivered in a Claude Code session, and the other way round. |
| **R2.3 Operations** | Async-hook tick triggers (C3); `claude -p --safe-mode` executor (C1); Desktop scheduled-task reports (C7); `asyncRewake` tier 2; Claude plugin packaging and marketplace entry; `repo-memory install --harness claude-code`. | A Claude Code-only machine runs the full loop. With both harnesses on one machine, they share one data directory and one tick without conflicts. |

---

## 16. Open questions and risks

| # | Question or risk | Current position |
|---|---|---|
| Q1 | Extraction precision: small models over-extract | Measured by B5 and gated at P1 exit, with a defined fallback (§10.3). Episode search (P0.5) keeps the system useful while extraction improves. |
| Q2 | Narrative quality of generated subsystem pages | Check against the existing design spec sections in P1. |
| Q3 | Where episodes live if more developers join | Default private. An opt-in shared episode store (scrubbed) is possible later. The `team` review profile covers the shared lane. |
| Q4 | ~~Antigravity raw transcript format~~ | **Resolved.** `brain/<id>/.system_generated/logs/transcript.jsonl` (§6.2). |
| Q5 | `ephemeralMessage` / `additionalContext` persistence semantics | E1 (Antigravity), C2 (Claude Code). Dedupe via the ledger regardless. |
| Q6 | Symbol fingerprinting for C/C++ with macros and conditional compilation | Release 1 stays cgo-free: a lightweight brace-aware scanner, optional universal-ctags if installed, and file-window hashing as fallback (§18.4). Validate on the esp32 codebase in P1; adopt tree-sitter (cgo) only if anchor quality demands it. |
| Q7 | Cost of the pipeline vs. current PA distillation | Cost model in §4.4.1; B4 broken down by PA activity tier. |
| Q8 | Memory-PR review fatigue | Two-lane publish and review profiles (§6.8). Review load is measured (§10.4). |
| Q9 | Claims that span repositories (framework vs. consumer apps) | Out of scope for v1. Consumer repos can import framework views read-only later. |
| Q10 | ~~LLM backend for the pipeline~~ | **Resolved.** Subscription CLIs only: `agy -p` by default, `claude -p --safe-mode` on Claude Code-only machines; `agentapi` and Desktop scheduled tasks only for IDE-visible reports (§4.4, D2). |
| Q11 | Cumulative hook latency, especially on Windows | Budgets in §7.8, measured per session (E5). The compiled Go binary (D1) removes interpreter start-up. |
| Q12 | Scheduled sidecars only tick while Antigravity is running | Claude Code async-hook ticks and the optional OS scheduler cover other machines. Without any tick, only the backstop and calendar jobs are lost (§7.9). |
| Q13 | Claim file count at scale | Hundreds from backfill, low thousands after years (§5.3). Month sharding keeps directories browsable. |
| Q14 | Fixed overhead of ~17k input tokens and ~13 s per housekeeping run | Dominates the small interactive calls rather than extraction (§4.4.1). Mitigated by packing, batching tier-2 checks per tick, caching briefs, per-day caps, E10 and C1; quota impact from E11. |
| Q15 | Hook coverage of subagents | **Claude Code: resolved** (`SubagentStart`, hooks carry `agent_id`). Antigravity: E8. The WORKORDER brief is the fallback. |
| Q16 | Concurrent sessions and overlapping ticks | Uniquely named spool files (write then rename); only the tick writes the job table; `tick.lock` prevents overlap from any trigger (sidecar, async hook, OS scheduler); job leases recover after crashes. |
| Q17 | Report noise in the IDE | Reports only start when there is something to review. Default: at most one review report a week plus one-off milestone notices, with a cap on open report conversations. |
| Q18 | Claude Code transcript format changes between releases | Versioned tolerant parser, fixture corpus, canary on update, `Stop` side-log (§12.2.1). |
| Q19 | Circular reinforcement: sessions echoing memory they were shown, including bug-fixer and report conversations | Addressed by P13: exposure recorded per step (§5.1), independence rules in extraction and reconcile (§6.5, §6.6), structural author classification (§9.2.1). The suppressed-supports metric watches for leaks (§10.1). |
| Q20 | Tier-2 déjà-vu latency on Antigravity | Accepted as a *next-attempt* correction. Prevention relies on tier 0 (proactive delivery) and tier 1 (lexical, with generated match blocks) (§7.6). |
| Q21 | Overlap with Claude Code auto memory | Import as evidence; steer via `CLAUDE.md`; never write into it (§12.2.2). |
| Q22 | Work status goes stale on events rather than code changes (e.g. the June OTA record, A.1) | `closes_when` on every `open-issue`, evaluated by the tick; `resolved` status; `unknown` closure means pull-only (§5.3.1, §8). |
| Q23 | Does the agent use the claim index when automatic linking fails? | Reads of `.memory/views/index/` are exposures, so they're measured (§10.4). If agents ignore the pointer, the spooled brief remains the fallback. |
| Q24 | ~~Which repositories does a global plugin act on?~~ | **Resolved.** Only explicitly enrolled repositories (§4.6); hooks no-op within 5 ms elsewhere; kill switch for the whole machine or one repository. |

---

## 17. Implementation decisions

| # | Decision | Status |
|---|---|---|
| D1 | Implementation language for hooks, tick, pipeline and MCP server | **Decided: Go.** One cgo-free binary (§18). |
| D2 | Housekeeping executor per machine | **Decided.** Release 1: `agy` only (the user's preferred quota). **[R2]** `claude` on Claude Code-only machines, pending C1. One setting: `housekeeping.executor`. |
| D3 | Tick triggers per harness | **Decided.** Release 1: the Antigravity scheduled sidecar. **[R2]** Claude Code `async: true` hooks, with an optional OS scheduler for calendar jobs. Every trigger runs the same `repo-memory tick`, deduplicated by `tick.lock`. |
| D4 | Harness order | **Decided.** Release 1 targets Antigravity. Release 2 adds Claude Code as an adapter. Release 1 follows the portability rules in §18.6 so that Release 2 needs no core changes. |

**D1 record.** Hooks block the agent loop and run before every model call (Antigravity) or around every tool call (Claude Code), so start-up cost is paid on every one of them. Three options were considered:

| Option | For | Against |
|---|---|---|
| A. Python everywhere | Fastest to build; matches the existing plugins; mature text tooling | Interpreter start-up on every hook, worst on Windows; Python version, virtual-env and `PATH` management in hook and sidecar environments |
| B. Python pipeline + compiled hook binary | Fast hooks; pipeline in Python | Two languages, with the index schema and spool format as a versioned cross-language contract |
| **C. One compiled Go binary** (chosen) | One self-contained executable per OS; no runtime to install; millisecond start-up; no `PATH` or virtual-env issues. LLM calls are CLI subprocesses (`agy -p`, `claude -p`), so no LLM SDK is needed. | A different language from the existing Python plugins; tree-sitter would need cgo, so Release 1 uses a lighter symbol scanner (§18.4) |

---

## 18. Implementation architecture (Go)

### 18.1 One binary

`repo-memory` is a single Go binary per OS and architecture: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64. It is built with `CGO_ENABLED=0`, so every target cross-compiles from one machine.

| Subcommand | Role | Invoked by |
|---|---|---|
| `hook <harness> <event>` | Event path: read the hook JSON on stdin, normalise it, handle it, render the harness's output format | Harness hooks |
| `tick` (alias `run`) | Time path (§4.3, §12.3) | Antigravity sidecar; **[R2]** async hooks, OS scheduler; the CLI |
| `mcp` | Stdio MCP server for the pull tools (§7.7) | The harness's MCP configuration |
| `init`, `leave [--purge]` | Enrol or unenrol the current repository (§4.6) | The user, once per repository |
| `enable`, `disable [--repo]` | Kill switch (§4.6) | The user |
| `install --harness antigravity` | Writes the plugin files (`hooks.json`, `mcp_config.json`, `sidecar.json`) with absolute paths for this machine, the same templating approach as the bug-fixer plugin's installer. That avoids any dependence on `PATH` under `sh -c` or `cmd /c`. | The user, once per machine |
| `uninstall --harness antigravity` | Removes the generated plugin files and sidecar; leaves data unless `--purge` | The user |
| `search`, `report`, `status`, `reindex`, `doctor` | Operator CLI | The user |
| `schedule install` | **[R2]** OS scheduler entry for machines without Antigravity | The user |

### 18.2 Package layout

```
cmd/repo-memory/                 main and subcommand wiring
internal/core/                   harness-neutral; must not import any concrete adapter
  episode/  claims/  entities/  views/  index/  jobs/  tick/
  pipeline/{compact,triage,extract,reconcile,verify,publish}/
  delivery/  gitops/  scrub/  metrics/  config/
internal/harness/                interfaces and normalised types (Event, Injection, Capabilities, Step)
internal/harness/antigravity/    Release 1: transcript source, hook codec, agy executor,
                                 agentapi reporter, install templates
internal/harness/claudecode/     [R2]
internal/harness/fake/           test adapter that drives the core test suite
internal/mcpserver/              pull tools over the index
prompts/                         stage system prompts and JSON schemas (go:embed)
packaging/antigravity/           plugin templates
packaging/claudecode/            [R2]
testdata/fixtures/antigravity/<version>/   scrubbed transcripts and hook payloads
```

A test checks the import graph and fails if anything under `internal/core` imports `internal/harness/antigravity` or any other concrete adapter.

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
| SQLite (index, jobs, FTS) | A pure-Go SQLite driver (e.g. `modernc.org/sqlite`) | WAL mode; confirm FTS5 support in P0 |
| MCP server | The official MCP Go SDK | Stdio transport |
| Episode compression | Pure-Go zstd | |
| Claim files | YAML front matter + Markdown | |
| Git | Shell out to `git` | Worktree operations for publishing (§6.8) |
| LLM calls | Subprocess `agy -p`; **[R2]** `claude -p` | No LLM SDK |
| Secret scanning | Built-in pattern and entropy rules; optionally a Go secret-detection library | Runs at compaction and before publish (§11.3) |
| C/C++ symbols | A brace-aware scanner plus file-window fingerprints; universal-ctags used if installed | Tree-sitter only if anchor quality demands it (Q6) |

### 18.5 Hooks in Go

- **Invocation.** `hooks.json` entries call `<absolute path>/repo-memory hook antigravity <Event>`, written by `install`, so they work under `sh -c` and `cmd /c` without `PATH`.
- **Fast path.** The hook reads a per-conversation cursor (a byte offset into the transcript), reads only new bytes, checks `alerts/`, and opens SQLite read-only only when there is something to look up.
- **Budget.** p95 ≤ 60 ms on the fast path (§7.8), confirmed by E5 on macOS and Windows.
- **Failure behaviour.** On any error, the hook returns the harness's no-op output and logs the error. It never blocks the agent.

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
  - Model Decision and Glob activation modes appear on the public rules page but not in the built-in doc (experiment E4).
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
  - an instrumented observability layer and `memory.stats`, with all tools shipped;
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
| 0.2 | 2026-09-30 | Following `spec-critical-evaluation.md` and a re-check of the Antigravity built-in docs:<br>• Pipeline runs as Antigravity scheduled sidecars (periodic ticks); no daemon or socket.<br>• Capture moved to a transcript sweep, with `Stop` as a nudge and no `PostToolUse` capture hook.<br>• Recursion guard for pipeline-created conversations.<br>• Pluggable LLM backends (`api` / `agentapi`).<br>• Two-lane publish and review profiles.<br>• Delivery hook latency budget and compiled-hook option; degraded modes.<br>• Two-tier déjà-vu detection (immediate lexical match in the hook, semantic check on a 1-minute tick).<br>• Map loaded via an `AGENTS.md`/`CLAUDE.md` include.<br>• `memory.stats` and an observability layer.<br>• Entity-extraction exactness and subsystem bootstrap; month-sharded claims and scale estimate.<br>• B5 extraction-precision benchmark and P1 gates with a fallback; P0 experiments E1–E6; P0.5 episode search.<br>• Transcript location and format resolved (Q4). |
| 0.3 | 2026-09-30 | Execution model settled with the user:<br>• Two invocation paths with strictly separate jobs: hooks read, inject and spool; the scheduled tick does all asynchronous work. Harness-level, not OS-level; no detached processes.<br>• Durable job table advanced across ticks, with leases, idempotency keys, budgets and a circuit breaker.<br>• One-minute tick; the watcher sidecar folded into it.<br>• All LLM work runs on the user's Antigravity quota: housekeeping via headless `agy -p` (measured at ~17k tokens of fixed overhead and ~13 s per call, so items are batched per call); no API key.<br>• Session visibility policy: repo work and reports in the IDE; housekeeping headless.<br>• Weekly review report as an IDE conversation carrying gap questions, pending claims and proposed work; human replies captured as evidence.<br>• LLM stages defined as pure functions.<br>• `PreToolUse` gate-once delivery for risky actions and déjà-vu.<br>• Capture sweep extended to the Antigravity CLI store.<br>• Publishing through a dedicated git worktree.<br>• Experiments revised (E7 isolation, E10 overhead, E11 quota). |
| 0.4 | 2026-09-30 | Claude Code support and external review:<br>• P13 evidence independence with per-step exposure markers and author classification; independence rules in extraction and reconcile.<br>• Report-reply mechanism (§9.2.1).<br>• §4.4.1 cost model from measured Antigravity transcript sizes; salience windows and packing; the fixed overhead identified as dominating small interactive calls.<br>• Déjà-vu reframed as tier 0 (proactive) / tier 1 (lexical safety net, generated match blocks) / tier 2 (late correction; seconds on Claude Code via `asyncRewake`).<br>• §12.2 rewritten from verified Claude Code docs: hook mapping, `claude -p --safe-mode` executor, Desktop scheduled task reports, async-hook tick triggers, transcript parser policy, auto-memory coexistence, experiments C1–C7.<br>• Native-floor degraded mode.<br>• §17 implementation decisions (D1 open, recommendation: one compiled binary; D2 and D3 decided).<br>• P0 extended to both harnesses; P2 builds a normalised event model, Antigravity first. |
| 0.5 | 2026-09-30 | Harness-neutral techniques from a review of Claude Code auto memory (Claude-specific notes in `docs/claude_architecture.md` §4.7):<br>• Claims gain `relevant_when` (for selection) and `apply` (for action) lines; pushes render `apply`.<br>• Claim index view per subsystem, so agents can walk MAP → index → claim with plain reads; used as the fallback when task linking fails and in degraded modes.<br>• Every pushed claim carries a currency stamp; the hook checks that anchored files exist.<br>• `open-issue` claims carry `closes_when` conditions evaluated by the tick; new `resolved` status; unknown closure means pull-only.<br>• Writes to memory artefacts (PA files, harness-native stores) and approval signals are triage keep rules; written text kept as `note` steps, leads rather than evidence.<br>• Over-budget views are re-synthesised rather than truncated.<br>• Extraction reports referenced `gaps`, which feed coverage. |
| 0.6 | 2026-09-30 | User decisions:<br>• **Release order:** Release 1 targets Antigravity; Release 2 adds Claude Code (D4). Claude Code-specific text is tagged **[R2]**; build phases split into §15.1 (P0–P5) and §15.2 (R2.1–R2.3), with Release 2 able to start after P2.<br>• **Go** (D1): one cgo-free binary with `hook`, `tick`, `mcp`, `install` and operator subcommands (§18.1); a package layout with an enforced core/adapter boundary (§18.2); harness interfaces and capability-based delivery (§18.3); libraries (§18.4); hook fast path (§18.5); portability rules for Release 1 (§18.6), including a recommended sweep-only copy of Claude Code transcripts from P0.<br>• Plugin files generated by `repo-memory install` with absolute paths; D2 and D3 scoped to Release 1. |
| 0.7 | 2026-10-01 | From the programme-plan review: repository enrolment (`repo-memory init`/`leave`, registry, hooks no-op within 5 ms outside enrolled workspaces, sweep scoped to enrolled repositories) and a kill switch (`REPO_MEMORY_DISABLED`, `enabled: false`, `enable`/`disable`) in §4.6; machine configuration and registry in §5.6; new subcommands in §18.1; Q24. |
