# Repo Memory: Specification

| | |
|---|---|
| **Status** | Draft v0.1 |
| **Date** | 2026-09-29 |
| **Scope** | Repository-level memory for coding agents (Antigravity and Claude Code) |
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
| P10 | **Harness-agnostic core, thin adapters.** | Pipeline, stores and views are plain files plus a local service. Antigravity and Claude Code integrations are adapters. |

---

## 3. Terminology

| Term | Meaning |
|---|---|
| **Session** | One agent conversation, including any subagent sessions it spawned. |
| **Raw transcript** | The harness's native record of a session (Antigravity conversation DB, Claude Code JSONL). |
| **Episode** | The compacted, scrubbed, harness-neutral record of a session produced by the sidecar (§5.1). |
| **Event** | Any immutable input to the knowledge projection: an episode, a commit, a human review decision, a human answer, or an imported document. |
| **Entity** | A thing claims are about: a code element (subsystem, file, symbol, config key, build env, topic, MCP tool, error string) or an external element (library@version, SDK version, board, tool). |
| **Anchor** | A reference from a claim to an entity, carrying a fingerprint of the anchored content. |
| **Claim** | A typed, evidence-backed statement about one or more entities (§5.3). |
| **View** | A generated document rendered from claims and entities (§5.5). |
| **Sidecar** | The background process that turns events into claims and views (§6). |
| **Trigger** | A runtime condition that causes knowledge to be delivered to the working agent (§7). |

---

## 4. Architecture overview

```
┌──────────────────────── Working agent (Antigravity / Claude Code) ────────────────────────┐
│  works normally — writes no memory                                                         │
│      ▲ push: start brief · file touch · error · risky action · déjà-vu alert               │
│      │ pull: ask / why / history                                                           │
└──────┼───────────────────────────────┬─────────────────────────────────────────────────────┘
       │                               │ capture hooks (no LLM): session end, commits, errors
       │                               ▼
       │                  ┌──────────────────────────┐
       │                  │  Capture queue (private) │
       │                  └────────────┬─────────────┘
       │                               ▼
       │   ┌────────────────────────── Sidecar pipeline ───────────────────────────┐
       │   │ 1 Compact → 2 Triage → 3 Extract → 4 Reconcile → 5 Verify → 6 Publish │
       │   └───────┬──────────────────────────────────────────────────────┬────────┘
       │           ▼                                                      ▼
       │   ┌───────────────┐    ┌─────────────────────────────┐    ┌──────────────┐
       │   │ Episode log   │    │ Claim store (git, 1 file/   │───▶│ Views        │
       │   │ (private)     │    │ claim) + Entity skeleton    │    │ (generated)  │
       │   └───────────────┘    │ (derived from code)         │    └──────┬───────┘
       │                        └──────────────┬──────────────┘           │
       │                                       ▼                          │
       └──────────────────────── Local index (SQLite, rebuildable) ◀──────┘

  Other event sources: git (merges, reverts, dependency/config changes) → targeted re-checks
                       human (memory-PR review, answers to gap questions) → highest-trust events
```

### 4.1 Components

| Component | Responsibility | Runs |
|---|---|---|
| **Capture adapters** | Harness hooks that queue transcripts and record commits and errors. No LLM. | In-harness, milliseconds |
| **Sidecar daemon** | Runs the pipeline; serves the local query API used by hooks and tools. | Background process per machine |
| **Entity extractor** | Builds the code skeleton (subsystems, files, symbols, config keys, topics, tools, error strings). No LLM. | Sidecar, on commit |
| **Pipeline stages 1–6** | Turn events into claims and views (§6). | Sidecar, asynchronous |
| **Re-check scheduler** | Queues claims for re-verification when their dependencies change (§8). | Sidecar, on git events |
| **Delivery adapters** | Harness hooks that ask the sidecar for push content and inject it (§7). | In-harness, deterministic |
| **Pull tools** | `memory.ask`, `memory.why`, `memory.history`, `memory.brief`, exposed as an MCP server or skill. | Sidecar API |
| **Review surface** | Batched memory PR: claim-level approve/reject/edit. | Git host |

---

## 5. Data model

### 5.1 Episode record

One episode per top-level session. Subagent sessions are nested. Episodes are **private** (§11.1).

Location: `~/.local/share/repo-memory/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst`

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
  "commits": ["<sha>", …],            // recorded by capture hook
  "model": "…",
  "compactor_version": "1.0.0",
  "stats": { "raw_chars": 0, "kept_chars": 0 }
}
// step lines
{ "i": 42, "kind": "user",        "text": "…" }
{ "i": 43, "kind": "reasoning",   "text": "…" }
{ "i": 44, "kind": "agent",       "text": "…" }
{ "i": 45, "kind": "command",     "cmd": "pio run -e esp32s3nano", "exit": 1,
                                  "outcome": "build_fail", "excerpt": "<error lines only>" }
{ "i": 46, "kind": "edit",        "file": "modules/BLEManager/BLEStack.cpp", "commit": "<sha>|null" }
{ "i": 47, "kind": "read",        "file": "…" }             // path only, never content
{ "i": 48, "kind": "test",        "suite": "native", "passed": 882, "failed": 6, "excerpt": "…" }
{ "i": 49, "kind": "device",      "action": "flash|monitor|soak", "outcome": "reset:INT_WDT", "excerpt": "…" }
{ "i": 50, "kind": "subagent",    "episode": "ep-…" }
```

**Compaction rules (deterministic):**

| Keep | Drop or reduce |
|---|---|
| User messages (verbatim) | File contents returned by reads (keep the path) |
| Agent reasoning and text | Edit bodies (keep the path; resolve to commit where possible) |
| Commands, exit codes, classified outcome | Command output, except error/failure lines and summary lines (bounded excerpt) |
| Test, build, flash and soak outcomes | Repeated identical outputs (collapse with count) |
| Subagent tree | Tool schemas, system prompts, injected memory (recorded by ID only) |

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

One file per claim, in the repository:

`.memory/claims/<claim-id>.md`

```markdown
---
id: clm-01J8ZQ4K2V7X
type: invariant                # see §5.3.1
title: BLE host and controller memory must be internal DRAM
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

`failure-signature` claims carry a machine-matchable `match` block:

```yaml
match:
  any: ["INT_WDT", "TG0WDT"]
  all_in_window: ["BLE_INIT", "EM_BASE_REG_LUT"]
  source: [device, command]
```

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
| **System map** | `.memory/views/MAP.md` | ≤ 2k tokens | One row per subsystem: purpose, globs, dependencies, open-issue count, link. Plus global invariants and conventions. |
| **Subsystem page** | `.memory/views/subsystems/<id>.md` | ≤ 5k tokens | How it works now (narrative synthesised from claims plus code), invariants, decisions, ruled-out approaches, failure signatures, procedures, open issues. Every line cites claim IDs. |
| **Task brief** | generated on demand (not stored) | ≤ 8k tokens | For a task: relevant subsystems, invariants, ruled-out approaches, procedures, in-flight work. |
| **Signature index** | `.memory/views/SIGNATURES.md` + index table | n/a | All `failure-signature` claims with their match blocks. |
| **Decision log** | `.memory/views/DECISIONS.md` | n/a | All `decision` claims, chronological, with status and supersession. |
| **Episode summary** | generated on demand | ≤ 1.5k tokens | What a session attempted, outcomes, commits, claims produced. Replaces hand-written completion reports. |

Views are regenerated when any claim they cite changes, or when the entity skeleton changes. Committing them to the repository is optional (default: committed, so they're readable on the git host).

### 5.6 Storage layout

**In the repository** (shared, reviewed):

```
.memory/
├── config.yaml                 # subsystems, budgets, trigger settings
├── claims/                     # one file per claim
├── events/
│   ├── reviews/*.jsonl         # human review decisions
│   └── answers/*.jsonl         # human answers to gap questions
├── views/                      # generated; do not edit
│   ├── MAP.md
│   ├── DECISIONS.md
│   ├── SIGNATURES.md
│   └── subsystems/*.md
└── .gitignore                  # ignores .cache/
.memory/.cache/index.sqlite     # rebuildable index (gitignored)
```

**Private, per machine** (never committed):

```
~/.local/share/repo-memory/<repo-id>/
├── queue/                      # captured, not yet processed
├── episodes/<yyyy>/*.jsonl.zst # compacted, scrubbed episodes
├── raw/                        # optional raw transcript retention (off by default)
├── state/                      # per-session delivery ledger, pipeline cursors
└── logs/
```

---

## 6. Sidecar pipeline

### 6.1 Stage summary

| Stage | Input | Output | Engine | Cost profile |
|---|---|---|---|---|
| 0 Capture | Harness hook | Queue entry | Script | ~0 |
| 1 Compact | Raw transcript | Episode | Deterministic | ~0 LLM |
| 2 Triage | Episode | Keep/skip plus reasons | Rules, then small model | Low; most sessions stop here |
| 3 Extract | Kept episode | Candidate claims | Small model, batched | Medium, input-heavy |
| 4 Reconcile | Candidate plus existing claims on the same anchors | Claim ops | Stronger model, small inputs | Low per candidate |
| 5 Verify | Claim ops | Verified ops | Deterministic plus targeted small-model check | Low |
| 6 Publish | Verified ops | Claim files, views, memory PR | Deterministic plus view synthesis | Low |

### 6.2 Stage 0: Capture

- On session end: enqueue `{harness, session_id, transcript_path, repo_root, branch, head}`. Copy the raw transcript into the queue immediately, because harnesses delete transcripts (Appendix A.3).
- Long-running sessions: also enqueue a checkpoint every N steps (default 200) so a crash or pruning doesn't lose the session.
- On each tool completion, append to the live session ledger:
  - commit SHAs (when a git commit command succeeds);
  - build, test and flash outcomes;
  - error lines (used by the error trigger, §7.4).
- Capture never calls an LLM and never blocks the agent. Hook budget: ≤ 200 ms.

### 6.3 Stage 1: Compact

Apply the compaction rules in §5.1 and scrub secrets. Record `raw_chars` and `kept_chars`. Target a reduction of ≥ 3× (Appendix A.2 suggests ~70% of transcript volume is tool traffic already held by git).

### 6.4 Stage 2: Triage

**Rules (cheap, first):**

- Keep if any hold:
  - a failure outcome followed later by a success on the same target;
  - a revert or abandoned edit;
  - a user message containing a decision or preference signal;
  - a new file under a subsystem glob;
  - a dependency or config change;
  - reasoning spans above a length threshold discussing alternatives;
  - a device or soak outcome.
- Skip if all hold: no failures, no commits, fewer than K steps, and no decision signals.

A **small model** handles the ambiguous remainder, answering "Is there durable, repo-specific knowledge here?" with reasons.

### 6.5 Stage 3: Extract

**Input:** the kept episode plus the entity list for the subsystems it touched (names only).

**Output:** candidate claims, each with type, statement, anchors (by entity reference), conditions, evidence (step indices and outcomes), proposed status and extraction rationale.

Extraction rules, enforced by prompt and schema:

1. **Store only what the code can't tell you.** Reject claims that restate code structure.
2. **Status from outcomes.** `observed` or better requires a tool outcome at the cited step. Otherwise the claim is a `hypothesis`.
3. **Failure triage** (from Dream-RSI's policy prompt). Each failed attempt is classified as `approach-unrecoverable`, `implementation-bug`, `environment` or `insufficient-evidence`. Only `approach-unrecoverable` yields a `ruled-out` claim. Implementation bugs yield at most a `failure-signature`.
4. **Human statements** become candidates with `evidence.kind: human`. Reconcile promotes them to `confirmed` if they're decisions.
5. **Conditions** are captured whenever a claim depends on a version, board, chip or config value.
6. **External content** (web pages, issue text, third-party docs quoted in the session) is tagged `provenance: external` (§11.2).

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
- Batch into a **memory PR** on branch `memory/pending`, opened or updated by default daily. Never commit to the user's current branch. Never commit to main directly.
- The PR description lists ops grouped by subsystem, with evidence links, and highlights anything touching `confirmed` claims, `invariant` claims or security-relevant subsystems.
- **Auto-approval policy** (configurable, default off): `support` and `promote` ops on non-security subsystems can be auto-merged.

### 6.9 Model and cost policy

- **Small model** for triage, extract and behavioural verify. **Stronger model** for reconcile and view synthesis only.
- Use batch APIs and prompt caching (stable prefix: extraction instructions plus entity list).
- Hard caps per episode: extraction input ≤ 60k tokens (split larger episodes by subagent or phase); ≤ 20 candidates per episode.
- The sidecar records its own token spend per stage (§10).

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

A per-session **delivery ledger** records every claim delivered, so delivery is never repeated and the metrics in §10 can be computed.

### 7.2 Task start

1. Always-on: `MAP.md`.
2. At the first model invocation, the adapter sends the task text (first user message) and branch to the sidecar.
3. The sidecar links the task to entities:
   - lexical match on entity names, paths, symbols and error strings;
   - then subsystem inference from the map (small model only if lexical matching yields nothing);
   - plus files already open or mentioned.
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
- one line per claim plus its ID (the agent can `memory.why(<id>)` for evidence).

### 7.4 Error

When a tool outcome (command, test, device) contains text matching a `failure-signature` `match` block, inject the signature's cause and fix before the agent's next reasoning step. Matching is deterministic (string and regex on the ledger excerpt).

### 7.5 Risky action

Before tool calls matching configured patterns, inject the relevant `procedure` and `invariant` claims. Defaults:

- flash, monitor and OTA upload tools;
- release scripts;
- edits to `sdkconfig*`, `platformio.ini`, `dependencies.lock` or partition tables;
- dependency upgrades.

### 7.6 Déjà-vu alerts (optional, warm path)

- The sidecar tails the live session's *agent reasoning and text only* (not tool output) with a small model.
- It compares the agent's current plan or hypothesis against `ruled-out` claims for the entities in play.
- On a match above a threshold, it queues an alert delivered at the next model invocation: "You're about to retry X. It failed on <date> because Y (clm-…)."
- Default: enabled for `ruled-out` claims with status `verified` or higher only.

### 7.7 Pull tools

| Tool | Returns |
|---|---|
| `memory.ask(question)` | Answer synthesised from claims, with claim IDs; falls back to episode search with explicit "unverified" marking |
| `memory.why(target)` | For a file, symbol, config key or claim ID: decisions and invariants plus evidence chain (claim → episodes → commits) |
| `memory.history(entity)` | Chronological claim and supersession history for an entity |
| `memory.brief(task)` | Regenerated task brief |
| `memory.note(text)` | Lets the agent or user flag something explicitly. Recorded as an event with `provenance: agent-note` or `human`, reconciled like any other candidate. This is optional and costs the working agent a single short tool call. |

### 7.8 Performance

- Hot-path triggers (§7.3–7.5) are local index lookups: p95 ≤ 150 ms, no network, no LLM.
- The task brief can use a small model for entity linking only when lexical linking fails: p95 ≤ 5 s.
- If the sidecar is down, adapters no-op. Memory must never block work.

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
| Human correction | The named claim | Apply as a `confirmed` or `refuted` event |

**Explicitly out of scope:** decay by age, decay by retrieval frequency, and confidence scores that change with use. A claim about a subsystem nobody has touched in a year is as valid as the day it was verified, because nothing it depends on has changed.

---

## 9. Coverage and elicitation

### 9.1 Coverage model

For each subsystem, the sidecar computes a coverage record:

| Dimension | Measure |
|---|---|
| Rationale | Number of `decision` claims vs. key design points (public interfaces, major files) |
| Invariants | Number of `invariant` claims and the share enforced by a test or gate |
| Verification | Presence of at least one `procedure` for testing or verifying the subsystem |
| Failure knowledge | `failure-signature` and `ruled-out` counts relative to churn and incident history |
| Freshness | Share of claims that are `suspect` or `conditions-changed` |

**Gap score** = churn (commits in last 90 days) × complexity × (1 − coverage).

### 9.2 Elicitation

- Weekly (configurable), the sidecar proposes **1–3 targeted questions** to the human for the highest gap scores. For example: "Why does DataManager use FetchContext deadlines rather than per-source timeouts?"
- Questions are delivered through the review surface (memory PR comment or a `QUESTIONS.md` view), never mid-session.
- Answers are `human_answers` events. They become `confirmed` claims.

### 9.3 Inferred backfill

For subsystems with no rationale coverage, a one-off pass infers `decision` claims from code, commit messages and PR descriptions. These claims have status `hypothesis` and are tagged `inferred`. They're shown in views as "inferred, unconfirmed" and are never pushed until a human confirms them.

### 9.4 Invariant enforcement

Each `invariant` claim may carry `enforced_by: <test|lint|gate path>`. Unenforced invariants in high-churn subsystems are listed in the coverage report as candidates for tests or gates. Enforced invariants are re-checked by CI, not only by the sidecar.

---

## 10. Measurement

### 10.1 Per-session signals (computed by the sidecar after each episode)

| Metric | Definition | Target direction |
|---|---|---|
| **Rediscovery rate** | Share of extracted candidates that `support` an existing pushable claim which was *not* delivered in that session | ↓ |
| **Repeated dead-end rate** | Sessions in which the agent attempted an approach matching a `verified` `ruled-out` claim | ↓ |
| **Push precision** | Delivered claims that the agent referenced or acted on ÷ delivered claims | ↑ |
| **Tokens pushed per session** | From the delivery ledger | Within budget |
| **Sidecar tokens per session** | By stage | ↓ over time |
| **Main-agent memory tokens** | Tokens the working agent spent writing memory artefacts | → 0 |
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
| B4 | **Cost.** Sidecar tokens (by stage and model) vs. the main-agent tokens spent on PA distillation for a comparable period. |

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

Verified against the Antigravity hooks documentation (2026-09-29):

| Need | Hook | Notes |
|---|---|---|
| Capture at end | `Stop` | Enqueue only. Return no decision, so the agent is never forced to continue. |
| Commit and outcome ledger | `PostToolUse` | Output is `{}`; side effects only. Inspect `toolCall` for commits, builds, tests and flashes. |
| Task brief | `PreInvocation` with `invocationNum == 0` | Inject via `injectSteps` → `ephemeralMessage`. |
| Touch, error, déjà-vu | `PreInvocation` (every call) | Read new steps from `transcriptPath` since the last cursor; query the sidecar; inject if anything is due. Must be deterministic and fast (default hook timeout 30 s; target ≤ 150 ms). |
| Risky action | `PreInvocation` (after the model proposes, before execution is not injectable) or `PreToolUse` | `PreToolUse` can only return allow/deny/ask with a reason. Use `deny` with the procedure text only for configured hard gates; otherwise inject on the next invocation. |
| Pull tools | MCP server in plugin `mcp_config.json` | Keep the tool count and descriptions small, because schemas cost tokens every session. |
| Always-on map | Rule (Always On) or `PreInvocation` | Prefer the rule; it's cached in the system prompt. |
| Native alternative | Glob and Model Decision rules | Can serve subsystem pages without the sidecar (degraded mode). |

**Open item:** whether `ephemeralMessage` persists in conversation history is undocumented. The adapter must dedupe via the delivery ledger either way.

Plugin layout (installed globally; per-repo data lives in `.memory/`):

```
~/.gemini/config/plugins/repo-memory/
├── plugin.json
├── hooks.json
├── mcp_config.json
├── rules/memory-map.md          # Always On: points at .memory/views/MAP.md
├── skills/memory/SKILL.md       # how to use ask/why/history; review workflow
└── scripts/                     # adapter scripts (capture, deliver)
```

### 12.2 Claude Code

| Need | Mechanism |
|---|---|
| Capture | `Stop` / `SessionEnd` hook (enqueue transcript path) |
| Ledger | `PostToolUse` hook |
| Map and task brief | `SessionStart` / `UserPromptSubmit` hook (additional context) |
| Touch, error, risky action | `PostToolUse` / `PreToolUse` hooks returning additional context |
| Pull tools | MCP server |

Adapter details are to be confirmed against current Claude Code hook documentation during Phase 2.

### 12.3 Sidecar process

- Single binary or Python package per machine; one queue per repo.
- Local API over a Unix socket for adapters and the MCP server.
- Idle when the queue is empty. Processes the queue on session end, on git events (via a `post-merge` / `post-checkout` hook or file watcher), and on demand.

---

## 13. Principal-architect integration

| Today | With repo memory |
|---|---|
| Worker writes a Completion Report by hand | Sidecar generates an **episode summary** from the worker's session; the PA reads that. The worker may add a short free-text note. |
| PA writes session logs into PROJECT.md | PROJECT.md holds intent, plan, work-order ledger and approvals only. Knowledge goes to claims automatically. |
| PA hand-writes "Framework Facts Workers Need" per project | WORKORDERs embed or reference a generated **task brief**. |
| PA maintains MEMORY.md, PROJECT_INDEX.md, MODULE_INDEX.md | Coordination state (active projects) stays; knowledge indexes are replaced by generated views. |
| Abandoned work orders lose dead-ends | Branch-abandonment promotion (§6.7) keeps `ruled-out` claims. |

---

## 14. Migration and backfill

There is no separate migration path. Existing artefacts are ingested as `import` episodes and processed by stages 3–6.

| Source (esp32-app-foundation) | Treatment |
|---|---|
| PA `PROJECT.md` session logs, `WORKORDER.md` completion reports, `context_bridge.md` files | Import episodes (one per file), dated from file or git history; high-yield for `ruled-out`, `failure-signature`, `decision` |
| `docs/design_specification.md` §17 decision log, `docs/developer/adr/*` | `decision` candidates. Status `confirmed` where human-approved per the PA approval records; otherwise `merged` |
| `framework-expert` skill and `docs/developer/*` | `fact`, `procedure`, `convention` and `invariant` candidates; linked, not duplicated |
| Git history (commit messages, PR descriptions, reverts) | Evidence and `decision` candidates; reverts as refutation evidence |
| Surviving Antigravity and Claude Code transcripts | Normal compaction and processing |
| Antigravity conversation summaries (`conversation_summaries.db`) | Low-weight import episodes (titles and previews only) for coverage and dating |

**Order:**

1. Configure subsystems.
2. Import OTA and BLE first (deep history and known stale records).
3. Review.
4. Import the rest, highest churn first.

**Verification is mandatory.** Stage 5 runs against current HEAD. Historical records that describe since-fixed defects (e.g. the June OTA `_silentCheck` record) must end as `superseded` or `refuted`, not as open issues.

---

## 15. Build phases

| Phase | Scope | Exit criteria |
|---|---|---|
| **P0 Capture** (urgent) | Capture adapters for both harnesses; compactor; scrubber; private episode store. No LLM. | Every new session produces an episode within 5 min of ending; compaction ratio measured (target ≥ 3×); zero secrets in a scan of 50 episodes. |
| **P1 Knowledge** | Entity extractor; subsystem config; stages 2–6; claim schema; lint gate; memory PR; views (map, subsystem pages, signatures, decisions); import of OTA and BLE history. | OTA and BLE claim sets reviewed; B2 question bank shows improvement over baseline; sidecar cost per episode measured. |
| **P2 Delivery** | Adapters for map, task brief, file touch, error, risky action; delivery ledger; pull tools. | B1 and B3 run; per-session push within budget; hot-path p95 ≤ 150 ms. |
| **P3 Learning loop** | Déjà-vu alerts; coverage and elicitation; metrics (§10.1); offline replay tuning (§10.2). | Rediscovery and repeated dead-end rates reported weekly; first policy update accepted by replay. |
| **P4 PA integration** | Generated episode summaries and task briefs in the PA workflow; retire knowledge indexes. | Main-agent memory-writing tokens ≈ 0 for a full project cycle; B4 shows lower total cost. |
| **P5 Full backfill** | Import all remaining history; inferred rationale pass; re-projection tooling. | Every subsystem has a page; coverage report published. |

---

## 16. Open questions and risks

| # | Question or risk | Current position |
|---|---|---|
| Q1 | Extraction precision: small models over-extract | Triage, outcome-grounded status, reconciliation and review. Measure in P1 with a labelled sample. |
| Q2 | Narrative quality of generated subsystem pages | Check against the existing design spec sections in P1. |
| Q3 | Where episodes live if more developers join | Default private. An opt-in shared episode store (scrubbed) is possible later. |
| Q4 | Antigravity raw transcript format (protobuf steps) | Prefer `transcriptPath` from hooks. Confirm what it points to in P0. |
| Q5 | `ephemeralMessage` persistence semantics | Undocumented. Dedupe via the ledger regardless. |
| Q6 | Symbol fingerprinting for C/C++ with macros and conditional compilation | Tree-sitter with fallback to file-window hashing. Validate on the esp32 codebase in P1. |
| Q7 | Cost of the sidecar vs. current PA distillation | Unknown until P0/P1 measurement (B4). The design shifts spend from frontier-model output tokens to small-model input tokens but reads more volume. |
| Q8 | Memory-PR review fatigue | Grouping by subsystem, auto-approval for low-risk `support`/`promote` ops, and weekly cadence. |
| Q9 | Claims that span repositories (framework vs. consumer apps) | Out of scope for v1. Consumer repos can import framework views read-only later. |

---

## Appendix A: Evidence from esp32-app-foundation (2026-09-29)

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
- Only **41** top-level and **138** subagent conversation databases remain on disk. The rest survive only as ~1 KB summaries in `conversation_summaries.db`.
- Capture must therefore happen at session end (P0).

### A.4 Harness facts relied upon

- **Antigravity hooks:**
  - `PreToolUse` (allow/deny/ask with reason);
  - `PostToolUse` (output `{}`);
  - `PreInvocation` (before every model call; `injectSteps` including `ephemeralMessage`; receives `invocationNum`, `transcriptPath`);
  - `PostInvocation`;
  - `Stop` (`decision: "continue"` plus `reason`).
  - No `SessionStart` event.
- **Antigravity rules:**
  - Always On;
  - Model Decision (description upfront, content on demand);
  - Glob (on file interaction);
  - Manual.
  - 24 KB per file; 20k-token aggregate budget applies to always-on and global rules, with overflow demoted to pointers.

## Appendix B: Relationship to prior inputs

- **Agentic Repo-Level Memory Systems (survey):** adopted Markdown-in-git as the shared substrate and a rebuildable local index. Rejected embedding-first retrieval as the primary mechanism, the post-tool-use extraction loop, and usage-based decay.
- **Dream-RSI (arXiv:2609.14858):** adopted outcome-over-claims trust, the failure-triage taxonomy, prefix-only replay for tuning the delivery policy, and in-sample no-regret policy selection. Not adopted: the transcript-as-discovery-tree mapping and counterfactual "would card X have prevented dead-end Z" analysis, which the replay mechanism can't support.
- **repo-memory-plugin-architecture-analysis.md:** superseded. See the inline review comments in that document.
