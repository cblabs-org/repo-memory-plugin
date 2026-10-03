# repo-memory v2: Distillation and Retrieval

**Status:** Draft for review
**Date:** 2026-10-03
**If accepted, this replaces these parts of SPEC v0.9:**
- §5.3 (claims) and §5.5 (views);
- §6.4–6.7 (triage, extract, reconcile, verify);
- parts of §7 (delivery).

Capture, enrolment, the job table and the tick (SPEC §4, §6.2) don't change.

**Background reading:**
- [Dogfood Evaluation 01](dogfood-evaluation-01.md), including the review in its Appendix C;
- [experiments E01–E12](experiments/SUMMARY.md);
- open platform questions in [issue #2](https://github.com/cblabs-org/repo-memory-plugin/issues/2).

**For reviewers:** the questions we most want answered are in §14.

---

## 1. Summary

The first build distilled knowledge after the fact, in a multi-stage LLM pipeline run by a sidecar. On this repository's own history, it produced accurate but shallow, atomic claims, at about 112k input tokens per episode.

v2 changes three things:

1. **Distil in context, at the end of a turn.** When an agent finishes a turn and enough has happened, the `Stop` hook asks it for a short note covering everything since its last note:
   - decisions and their reasons;
   - dead ends;
   - errors and their fixes;
   - discoveries.

   The request is made in the same conversation while the prompt cache is still warm. The note is written by the agent that did the work, from its full context. The extra input is a few thousand uncached tokens, not a re-read of the conversation. Sessions that never get a note fall back to a batched sidecar.
2. **Store knowledge as dossiers.** There is one markdown page per module, architectural topic or external system. Each page has fixed sections and short entries, and each entry has an ID and anchors into the code. Agents read pages; hooks deliver and verify entries.
3. **Deliver less, and more precisely.** Small pushes happen at the moment of need: the first time a file is read, an error, a risky action. Each one is checked against the code before delivery. Everything else is pulled on request.

---

## 2. Background

### 2.1 Where each existing approach is strong

| | Principal Architect (PA) completion reports | repo-memory v1 |
|---|---|---|
| Quality | Rich. Written in context by the agent that did the work. | Shallow: atomic claims with no reasons. In the dogfood run, they were extracted from PA summaries rather than transcripts. |
| Length | More verbose than memory needs | Too terse |
| Coverage | Only work the PA manages | Every session |
| Model and cost | The session's model, which may be expensive | A model we choose, but with 17k tokens of overhead per call. The dogfood run made 468 calls. |
| Retrieval | Bulk-loads about 9k tokens per session; hit and miss | Targeted, at the point of need (designed, not yet built) |
| Determinism | An LLM reads files | Hooks and index lookups need no LLM |

v2 aims to keep the PA column's quality, and the v1 column's coverage, targeting and determinism.

### 2.2 What the research says

Sources and details are in Appendix C of the evaluation.

- **Form decides whether a memory helps.**
  - VibeMemBench tested four memory systems on repository coding tasks. They failed mainly because of how records were written: raw transcripts hurt through volume, and atomic facts lost the fix.
  - What helped was about five structured lines: bug class, root cause, fix pattern, lesson and an anchor.
  - Injecting memory into tasks the agent could already solve made it worse.
- **A wrong or stale memory is worse than none.**
  - Wrongly selected summaries scored below no memory (SWE-ContextBench).
  - "A stale convention file costs more than no file" (Coherence Debt).
  - LLM-generated AGENTS.md files lowered success and raised cost by about 20%.
- **Checking a memory when it's used works.** GitHub Copilot's memories cite code locations and are checked against the current code before use. Its merge rate rose from 83% to 90%.
- **Products converged on files, an index and batched background work.**
  - Codex, Claude Code and Letta store memory as markdown files with a short index.
  - They consolidate in one batched background pass.
  - None runs a per-item pipeline.
- **Repeated LLM rewriting degrades memory** (ACE; Useful Memories Become Faulty).
  - Update one item at a time, never by rewriting the whole page.
  - Keep raw episodes as evidence.

### 2.3 Measured costs on Antigravity

**Headless `agy -p`** (experiments E07, E10, E11):

| Measure | Result |
|---|---|
| Fixed input per call | 16.8k tokens on Gemini 3.8 Flash; 9.9k on Gemini 3.6 Flash |
| Ways to reduce it | None found. `--agent`, `--disable-slash-commands` and changing directory had no effect. |
| Cache reads across fresh calls | 0 |
| Wall time | 10–14 s per call, including 6–7 s of CLI startup |
| IDE visibility | None: the CLI keeps its own conversation store |

**IDE prompt cache.** Measured from `~/.gemini/antigravity/conversations/<id>.db`: 692 conversations, 45,191 model calls, grouped by the gap since the previous call.

| Gap since the previous call | Gemini: share from cache | Gemini: uncached input (median) | Claude Opus: share from cache | Claude Opus: uncached input (median) |
|---|---|---|---|---|
| < 5 min | 94% | 4.5–5.2k | 99% | 1.2k |
| 5–15 min | 0% | 64k | 0% | 125k |
| ≥ 15 min | 0% | 55–74k | 0% | 130–134k |

**What follows from this:**
- The cache lives for about five minutes.
- Asking an idle conversation for anything after that re-reads its whole context uncached, on the conversation's own model: 55–135k tokens.
- Asking at the moment the turn ends costs only the new instruction.
- The first call of a new IDE conversation costs a median 13.5k uncached tokens. That's the harness's own baseline, comparable to `agy -p`.

---

## 3. Goals and non-goals

**Goals**
- **G1 Quality:** distillations at least as useful as PA completion reports, at about a third of the length, for every session, not only PA work.
- **G2 Cost:** cheap in quota terms. A small fixed cost per note; no per-item LLM calls; no re-reading cold contexts.
- **G3 Visibility:**
  - no extra conversations in the IDE;
  - nothing visible in most turns;
  - at most a short note at the end of some turns.
- **G4 Delivery:** precise, at the point of need, and checked against the code first.
- **G5 Organisation and scale:**
  - organised the way agents work: by module, architectural topic and external system;
  - scales to hundreds of PRs and thousands of entries without pushing more per session.
- **G6 Determinism:** no LLM on any hook path. An LLM runs only to write a note and in batched consolidation.
- **G7 Migration:** PA reports can be imported, and the PA can keep running alongside.
- **G8 Portability:** the same design works on Claude Code (§10).

**Non-goals**
- Replacing the PA's orchestration role.
- Vector databases, embeddings or external memory services.
- API keys or pay-as-you-go billing. Everything runs on the subscription quota.
- Recovering transcripts lost before enrolment (programme decision PD10).

---

## 4. Overview

```
  Working session (IDE or CLI)
  ┌────────────────────────────────────────────────────────────┐
  │  turn ... turn ─► Stop hook ─(gate passes)─► "continue":     │
  │                                 agent writes a note         │
  │                                 (same conversation, warm    │
  │                                  cache) ─► Stop hook        │
  │                                 harvests the note, moves    │
  │                                 the mark                    │
  │                                                             │
  │  PreInvocation / PreToolUse / PostToolUse hooks ◄───────┐    │
  └──────────────────────────────┬──────────────────────────┼───┘
                 transcripts,     │ notes                     │ small
                 artifacts        ▼                           │ verified
  ┌────────────────────┐   ┌──────────────┐   ┌──────────────┴──┐
  │ Evidence store      │◄──│ Inbox (notes)│──►│ Consolidation    │
  │ (private): episodes,│   └──────▲───────┘   │ (tick, batched   │
  │ artifacts, notes    │          │           │ agy -p): changes │
  └─────────┬──────────┘   Fallback: sidecar   │ applied to the   │
            │              replays sessions    │ dossiers         │
            └─────────────► that got no note   └────────┬────────┘
                                                        ▼
                         .memory/ (git): ATLAS.md + dossiers/*.md
                                + rebuildable SQLite index
                                (anchors, full-text, error patterns)
                                        │
                     pull: memory.brief / why / history / search
```

| Term | Meaning |
|---|---|
| **Turn note** | A short, structured distillation the working agent writes at the end of a turn, when asked by the `Stop` hook |
| **Mark** | Per-conversation state recording how far notes have covered. It holds the step index and byte offset of the last harvested note. |
| **Inbox** | Notes waiting for consolidation |
| **Consolidation** | A batched sidecar job that turns notes into changes to the dossiers |
| **Dossier** | A markdown page for one module, architectural topic, external system or procedure area |
| **Entry** | One item in a dossier. It has an ID, a type, the reason, what to do, anchors and evidence. |
| **Evidence** | Raw material an entry points to: transcript spans, artifacts, commits, PA reports |
| **Atlas** | A one-line-per-dossier index |

---

## 5. Distillation

### 5.1 Turn notes

The `Stop` hook fires whenever the agent finishes a turn. It can return `{"decision": "continue", "reason": "..."}`. The reason is injected as a system message and the agent carries on in the same conversation. Because the agent stopped moments earlier, its context is still in the prompt cache.

**Gate.** The hook decides deterministically, in milliseconds, whether to ask for a note. Every condition must hold.

| Condition | Default |
|---|---|
| The repo is enrolled and the kill switch is off | Required |
| The session is not a housekeeping run | Required |
| The agent stopped normally and isn't waiting on background work or a subagent | `terminationReason == "model_stop"` and `fullyIdle == true` |
| A note hasn't already been requested in this stop sequence | Required. Otherwise this is the harvest pass. |
| Something worth recording has happened since the mark (read from the transcript from the mark's byte offset) | Any one of: an edit followed by running a command; a failed command or test followed by more work; a user message with correction or decision markers; a git commit; or 25 or more steps |
| Cooldown since the last note in this conversation | 30 minutes, waived at milestones (a commit, a PR opened, a work order completed) |
| Daily cap per repository | 20 notes |
| Subagent conversations | Off, see below |

**Request.** This is a draft; the wording will be tuned in experiment X2.

> repo-memory: before you stop, write a memory note for this repository. Cover only what happened since your last memory note, or since the start of the conversation if there is none. Don't continue the task, don't call tools, and don't repeat your previous answer.
>
> Record only what a capable engineer new to this code would get wrong without being told:
> - decisions, with why and the alternatives rejected;
> - approaches that failed, and why;
> - error → cause → fix;
> - rules that must hold;
> - how external tools or systems behaved;
> - procedures you verified;
> - open issues.
>
> Leave out anything the code or docs already say, progress narration, and plans you didn't carry out.
>
> Reply with one `repo-memory-note` block in the format below, or the single word NONE.

**Note format.** A fenced block in the agent's reply:

````markdown
```repo-memory-note
- type: ruled-out
  title: Relative hook command paths in hooks.json
  why: Antigravity resolves hook commands relative to the directory containing hooks.json, not the repo root. A wrong path exits non-zero, which PreToolUse treats as deny, so every tool call was blocked.
  do: Generate absolute paths at install time; never hand-write relative hook paths.
  anchors: [packaging/antigravity/hooks.json, cmd/repo-memory/cmd_install.go]
  evidence: probe hook denied all tool calls until the path was made absolute
```
````

- **Allowed types:** `decision`, `ruled-out`, `failure-signature`, `invariant`, `external-behaviour`, `procedure`, `open-issue`, `convention`.
- **Required fields:**
  - every type needs `why`, except `procedure`;
  - `do` is required for `ruled-out`, `failure-signature`, `invariant` and `procedure`.
- **Typical size:** 0–6 items and up to about 600 tokens.

**Harvest.**
1. `Stop` fires again when the note has been written.
2. The hook reads the transcript from the step where it made the request and finds the `repo-memory-note` block in the agent's reply.
3. It parses and lints the block, then writes it to the inbox. The note is recorded as an event with:
   - the conversation and the steps it covers;
   - the model;
   - the harness version.
4. It moves the mark forward and lets the agent stop.

No tool calls or file permissions are involved.

**Failures and loop protection.**
- The hook asks at most once per stop sequence.
- If the reply is missing or doesn't parse, the raw text is kept with a parse error, the mark still moves, and the agent is allowed to stop.
- If the agent carries on with the task instead, the hook doesn't ask again in that sequence.

**Picking a conversation up again.** The mark records how far notes have reached. The previous note is also in the agent's context, so a later note doesn't repeat it.

**Subagents.** Off at first. A subagent's final message is what its parent receives. We don't yet know whether a continuation would change what is sent back (issue #2, Q22 and Q23). The sidecar fallback covers subagent transcripts in the meantime.

**What the user sees.** Nothing in most turns. In qualifying turns, a short note block appears after the agent's answer. A setting turns turn notes off and relies on the fallback alone.

### 5.2 What qualifies

**The test:** would a capable engineer new to this module get it wrong without being told?

| Type | Captures | Example from this repo |
|---|---|---|
| `decision` | A choice, why it was made, and the alternatives rejected | Read one transcript source in priority order, rather than merging four |
| `ruled-out` | What was tried, why it failed, and when that applies | Relative hook paths in `hooks.json` |
| `failure-signature` | Symptom → cause → fix, with patterns hooks can match | Injected context silently ignored → top-level `ephemeral_message` key → nest it as `injectSteps[].ephemeralMessage` (E01) |
| `invariant` | A rule that must hold | `internal/core` never imports a concrete adapter |
| `external-behaviour` | How a tool, platform or library behaves, found by experiment | `agy -p` costs ~17k input tokens per call; `--project` is ignored |
| `procedure` | A verified way to do something | Cutting a release; running the probe hook |
| `open-issue` | A known defect or debt, with the condition that would close it | Non-enrolled hook path took 57 ms in E05; closes when a fast-path check lands |
| `convention` | A house rule not already written down | Token estimates use chars/4 |

**Left out:** restatements of the code ("the search command uses tabwriter"), progress narration, plans that weren't carried out, and anything already in docs or rules files.

### 5.3 Artifacts

Antigravity conversations produce artifacts in `brain/<id>/`: implementation plans, walkthroughs, task lists and reports such as `wifi-regression-analysis.md`. 107 of 648 conversations on this machine have them, and they're deleted along with the conversation.

- Capture copies them into the evidence store. This is deterministic and costs no tokens.
- Entries can link to them as further reading.
- The fallback (§5.4) uses them as high-signal input.

### 5.4 Sidecar fallback

Some conversations end without a note covering everything past the mark:
- they were closed mid-way;
- turn notes are off;
- they are subagents;
- or they ran before enrolment.

**When it runs.** For each such conversation, once it has been idle for at least 2 hours (configurable) and the uncovered work passes the same salience test as the gate, the tick queues a fallback job.

**What a job does.**
1. It batches several conversations into one `agy -p` call on Gemini 3.6 Flash, with `--json-schema`.
2. Each conversation contributes its compacted transcript from the mark onwards, plus its artifacts.
3. The call returns notes in the same format, which go to the same inbox.

**Provenance.** Fallback notes are tagged `source: sidecar`. A cheaper model reading a compacted transcript is expected to do worse than the working agent, and consolidation and evaluation need to tell the two apart.

The same route handles backfill, starting with the 78 transcripts already captured from this repo.

### 5.5 PA reports

Completion reports already have "Decisions Made" and "Dead Ends & Warnings" sections. These are parsed deterministically into notes tagged `source: pa`.

Later, the work-order template can ask for the note block directly. That is a change to the PA skill, made through its own repository.

---

## 6. Consolidation

Consolidation is a tick job. It runs when 10 or more notes are waiting, or once a day, whichever comes first. It is skipped when the daily cap is reached or quota is low, if quota can be read (issue #2, Q7).

1. **Route** (deterministic). Each note item is matched to dossiers by its anchors, using the path globs and symbols in each dossier's scope.
   - Items without anchors go to an external-system or topic dossier by declared keywords.
   - Anything left goes to `unassigned`.
2. **Merge exact duplicates** (deterministic). The same type, normalised title and overlapping anchors means the item adds evidence to the existing entry.
3. **Decide changes** (LLM, batched).
   - Input: the relevant sections of the target dossiers plus the new items, packed up to about 60k tokens per `agy -p` call.
   - Output: a JSON list of changes. Each change is one of:
     - `add`;
     - `amend` (refine wording or add evidence);
     - `supersede` (with a reason);
     - `retire` (with a reason);
     - `move`;
     - `propose-split`.
   - The LLM never rewrites a page.
4. **Apply** (deterministic). Apply the changes, regenerate the atlas, and lint:
   - required fields are present;
   - size caps hold;
   - anchors exist in the code.
5. **Publish.** Use the two-lane model from SPEC §6.8.
   - **Local lane:** changes are usable by delivery at once.
   - **Shared lane:** changes go through the memory PR for review.

Notes and changes are append-only events, so dossiers can be rebuilt from scratch when the format or prompts change.

A weekly **gardening** job is optional and gated, with every change reviewed in the memory PR. It:
- proposes splits for dossiers near their cap;
- merges near-duplicate entries;
- retires entries whose anchors have gone and haven't been re-verified.

---

## 7. Storage

### 7.1 Layout

```
.memory/                         (in git, shared)
  ATLAS.md                       one line per dossier, generated
  config.yaml                    dossier scopes: path globs, symbols, keywords
  dossiers/
    core/transcript-parser.md
    core/transcript-parser.history.md   superseded and retired entries
    harness/antigravity-hooks.md
    external/antigravity-cli.md
    procedures/release.md
<data-dir>/<repo-id>/            (private, per machine)
  evidence/episodes/ artifacts/ notes/ pa/
  state/marks/<conversation-id>.json
  index.sqlite                   rebuildable
```

### 7.2 Dossier kinds

| Kind | Scope | Example |
|---|---|---|
| **Module** | Path globs and symbols | `core/transcript-parser`: `internal/harness/antigravity/transcript*.go` |
| **Architectural topic** | Several modules, plus keywords | Persistence and SQLite concurrency |
| **External system** | Keywords and tool names | Antigravity hooks, `agy` CLI, ESP-IDF, NimBLE |
| **Procedure area** | Keywords and commands | Release, flashing, soak testing |

### 7.3 Dossier format

Every dossier has the same sections, in order:
1. Purpose and boundaries
2. Invariants
3. Decisions
4. Ruled out
5. Failure signatures and gotchas
6. Procedures
7. Interfaces and dependencies
8. Open issues
9. Evidence index

```markdown
## Ruled out

### [rul-7Q2M] Relative hook command paths in hooks.json
`ruled-out · anchors: packaging/antigravity/hooks.json, cmd/repo-memory/cmd_install.go · verified 2026-10-01 (E09) · evidence: note-01M4…, E09`
Why: Antigravity resolves hook commands relative to the hooks.json directory, not the repo root.
A wrong path exits non-zero, which PreToolUse treats as deny.
Do: generate absolute paths at install time.
```

- **Entries:** 2–6 lines each, under a strict, lintable format.
- **IDs:** stable, made of a type prefix and a short ULID.
- **Cap:** about 3k tokens per dossier. Past that, consolidation proposes a split into submodules, so pages stay readable as the repo grows.
- **History:** superseded and retired entries move to the dossier's `.history.md` file. They're out of the delivery paths but still available to `memory.why`.

### 7.4 Why pages rather than one file per claim (SPEC §5.3)

- **They read as working context.** The evaluation found PA reports useful because they read as a whole; per-claim files don't.
- **Agents and people can read and search them** with normal tools.
- **This is what shipping products do** (§2.2).
- **Entry-level operations still work.** Stable IDs let delivery, verification and evidence links work per entry.
- **There are no merge conflicts.** Consolidation is the only writer, through one memory branch.

### 7.5 Index (rebuildable)

The SQLite index holds:
- a map from anchors (paths, symbols, config keys) to entry IDs, with fingerprints for verification;
- full-text search (FTS5) over entries and session synopses;
- an error-pattern table for `failure-signature` entries.

It is rebuilt from the dossiers, using the pointer-file swap from SPEC §18.4.

---

## 8. Retrieval and delivery

### 8.1 Principles

1. **Precision over recall.** An irrelevant or wrong memory costs more than a missing one.
2. **Push little, at the moment of need. Pull for depth.**
3. **Check anchors before pushing.** The hook checks each entry's anchors when it's about to deliver it. This is deterministic: the file and symbol exist and the fingerprint matches. An entry that fails is marked `suspect` and not pushed.
4. **Never push overviews or maps,** and nothing the code or docs already say.

### 8.2 Moments

| Moment | Trigger | What's delivered | Budget |
|---|---|---|---|
| **Task start** | First `PreInvocation` of the first prompt | The names of relevant dossiers, from prompt terms and the branch's diff paths, plus their invariants and open issues. Often nothing. | ≤ 500 tokens |
| **First read in a dossier's scope** | `PostToolUse` on a file read marks it pending; the next `PreInvocation` delivers | `do` lines from invariants, ruled-out entries and gotchas for the anchors involved, with IDs to pull. Once per dossier per session. | ≤ 200 tokens |
| **Edit that crosses a strong invariant** (optional) | `PreToolUse` on the edit | Deny once with the invariant and "re-issue unchanged if consistent". Only for entries marked `gate: edit`. | ≤ 200 tokens |
| **Error** | A failed command or tool result | The matching failure signature's fix | ≤ 200 tokens |
| **Risky action** | `PreToolUse` on a destructive, hardware or release action | The procedure or invariant, deny once (SPEC §7.5) | ≤ 200 tokens |
| **Everything else** | Pull | `memory.brief <module\|path\|task>` returns the dossier or sections of it. `memory.why <id>` returns the reason and short evidence spans, never bulk transcript. `memory.history <path>` returns session synopses. `memory.search <query>` searches. `memory.note` records an explicit "remember this". | Paginated |

**Antigravity channel limits.**
- `ephemeralMessage` is seen only in the invocation where it's injected (E01). That's why touch and error pushes are delivered at the very next invocation: at the moment of use.
- Whether `userMessage` persists across invocations is open (issue #2, Q2). It decides whether the task-start push can carry content, or only point at `memory.brief`.
- Deny-with-reason is delivered verbatim, and is isolated within parallel tool calls (E09).

### 8.3 Scale

- **Lookups stay fast.** Anchor lookups take constant time, and full-text search runs over entries rather than transcripts.
- **Pushes stay bounded.** Every push has a per-dossier budget, so what a session receives doesn't grow with the repository.
- **The atlas grows with modules, not history.** Dossiers split as they reach their cap, and history files keep the push paths small.
- **Expected size:** low thousands of active entries across a few hundred dossiers for a repository like esp32-app-foundation.

---

## 9. Cost model

These are estimates, to be replaced by measurements. Per-call token use can now be read from `gen_metadata` (§12).

| Item | Input | Output | Frequency |
|---|---|---|---|
| **Turn note** | ~1–5k uncached, plus the cached context (median 75k on Gemini, 130k on Opus in our data) | 300–1,000 | Gated: about 3–10 a day at esp32-app-foundation's session rates |
| **Harvest** | none (deterministic) | none | Per note |
| **Fallback** | ~10k overhead per call plus 5–15k per conversation, batched 3–4 per call | ~500 per conversation | Missed conversations only |
| **Consolidation** | ~10k overhead + ~0.5k per note + 1–3k per dossier section | ~1–2k | Daily, 1–2 calls: ~25–50k a day |
| **Delivery** | none (deterministic) | none | Injects ≤ ~0.5–2k per session |

**For comparison:**
- v1 dogfood pipeline: ~112k per episode.
- PA: ~5k per work order to produce, plus ~9k per session to load.

**The main unknown is how the quota counts cached tokens** (issue #2, Q6).
- If cache reads are discounted, as on the public APIs, a turn note is very cheap.
- If they count in full, each note costs the whole context. The gate would then need to fire only at milestones, and the fallback would carry more of the load.

The gate is configurable for this reason.

---

## 10. Portability to Claude Code

| Concern | Claude Code equivalent |
|---|---|
| Turn notes | A `Stop` hook returning `{"decision": "block", "reason": ...}` makes Claude continue. The `stop_hook_active` input field gives built-in loop protection. |
| Prompt cache | Claude Code's prompt cache is also short-lived, so asking at the end of the turn is the cheap moment there too. |
| Pushes | `PreToolUse` and `UserPromptSubmit` `additionalContext` stay in context, which is a better push channel than Antigravity's transient `ephemeralMessage`. |
| Fallback and consolidation | `claude -p` on the subscription. Transcripts are in `~/.claude/projects/`. |

The harness-specific parts stay behind the harness interfaces in SPEC §18:
- the hook codecs;
- the transcript reader;
- the executor that runs headless model calls (`agy -p` here, `claude -p` on Claude Code).

---

## 11. What changes in the current build

**Kept as is:**
- capture, the spool and the per-conversation cursors (marks use the same mechanism);
- the transcript parser and compaction;
- the scrubber and the episode store;
- the job table, the tick, enrolment and the kill switch;
- the events store, the SQLite index and full-text search;
- the MCP server and the hook codec;
- the eval harness.

**Replaced:**

| v1 | v2 |
|---|---|
| Per-item stages: triage, extract, reconcile, verify | Turn notes, the fallback and batched consolidation |
| One file per claim | Dossiers |
| The views generator | The atlas and dossier renderer |

**Retired:**
- LLM verification jobs. Anchors are checked at delivery time instead.
- The 125 v1 claims. They're archived; external-behaviour items can be re-fed as notes.

**Added:**
- the `Stop` hook gate and harvest;
- marks;
- copying artifacts at capture;
- dossier scopes in `config.yaml`;
- delivery budgets and checks at the point of use.

---

## 12. Validation plan

| # | Experiment | Pass criterion |
|---|---|---|
| X1 | **`Stop` continue behaviour** (issue #2, Q23–Q24). Prototype the gate and harvest on this repo. Read uncached, cached and output tokens per note from `gen_metadata`. Check what the user sees, the second `Stop`, and whether a user message can ever arrive mid-continuation. | Median uncached input ≤ 10k per note. Harvest succeeds on ≥ 95% of requests. No interleaving. |
| X2 | **Note quality.** Run turn notes alongside the PA on new work orders. Compare notes with the matching completion-report sections, blind. Tune the request wording. | Notes judged at least as useful in ≥ 50% of pairs, at ≤ ⅓ of the length |
| X3 | **Quota accounting** (issue #2, Q6–Q7). Compare quota used by 20 turn notes with 20 `agy -p` calls. | Shows whether the gate can stay as designed |
| X4 | **Consolidation.** One week of notes into dossiers, with every change reviewed. | ≥ 80% of changes accepted; caps respected |
| X5 | **Delivery channels** (issue #2, Q2). Test whether `userMessage` persists. Compare pushing on first read with deny-once on edit: what the agent does next, and how many extra steps it takes. | Choose the task-start and touch channels |
| X6 | **B1 replay.** Re-run 6–8 past work orders from their parent commits with no memory, with PA documents, and with dossiers. Measure tests passed, steps and tokens. | Dossiers do better than no memory, and at least match PA documents at fewer tokens. **If they don't beat no memory, stop.** |

---

## 13. Risks and open questions

| Risk | Mitigation |
|---|---|
| Quota counts cached tokens at full rate | Make the gate milestone-only; lean more on the fallback (X3) |
| `Stop` continue behaves differently from the docs: visibility, loops, collisions | X1 before anything is built on it; one request per stop sequence; a setting to turn it off |
| Users find end-of-turn notes noisy | Strict gate, cooldown and daily cap; a setting for fallback only |
| Agents write wrong or self-serving notes, such as plausible but invented reasons | Evidence field; `verified` only from tests, commands, merges or a human; review in the memory PR; anchor checks at delivery |
| Note quality varies with the session's model (Flash sessions) | Tag the model on every note; consolidation can weigh notes by source; measure in X2 |
| The gate misses important turns | The fallback replays anything past the mark |
| Dossiers go stale | Anchor checks at delivery; gardening retires entries whose anchors are gone |
| Poisoned or private content | Notes come only from enrolled repositories; the scrubber and lint run before anything reaches a dossier; shared changes are reviewed in a PR |
| Subagent behaviour under `continue` is unknown | Turn notes off for subagents; the fallback covers them |

The platform questions behind these risks are tracked in [issue #2](https://github.com/cblabs-org/repo-memory-plugin/issues/2). The ones this design depends on most:
- Q2: does `userMessage` persist across invocations?
- Q5–Q7: cache lifetime and quota accounting.
- Q22–Q24: subagents, `Stop` continue, and the idle signal.
- Q32: when conversation folders are deleted.

---

## 14. Questions for reviewers

1. **Turn notes or sidecar replay.** Is distilling in context at the end of a turn the right primary path, given the visible note it adds to some turns? Or should replay by the sidecar be primary, and turn notes optional?
2. **Gate defaults.** Are the salience signals, the 30-minute cooldown and the cap of 20 notes a day reasonable starting points?
3. **The test for what qualifies** (§5.2). Will it keep notes short and high-value? What important knowledge would it exclude?
4. **Dossiers as the source of truth.** Pages with ID'd entries, instead of one file per claim. What do we lose?
5. **Delivery on Antigravity.** Should the touch push come on first read (a transient message) or as deny-once on edit (an extra step)? Which entries, if any, deserve the edit gate?
6. **Subagents.** Should they get turn notes at all? If so, how, without changing what they return to their parent?
7. **The kill criterion** (X6). Is it strict enough?

---

## 15. Alternatives considered

| Alternative | Why not |
|---|---|
| **v1 post-hoc pipeline**, even fixed and batched | Better than the dogfood run, but it distils from outside the session with a cheaper model. Its per-item stages repeat LLM judgement that one consolidation pass can make. |
| **Injecting into an idle conversation** after 60 minutes, using `agentapi send-message` | The cache has expired, so it re-reads 55–135k tokens uncached on the session's model. It's visible, and it adds to that conversation's context for good. |
| **MCP record tools called whenever the agent discovers something** (Copilot style) | Each call is a visible extra model step, and agents call them inconsistently. `memory.note` stays available for an explicit "remember this". |
| **External memory services** (Mem0, Zep and others) | No better than no memory on coding tasks (VibeMemBench), and they add a service dependency |
| **Direct model API** | Needs an API key and pay-as-you-go billing, which is out of scope |
| **Pushing raw transcript excerpts** | Raw volume hurts. Evidence is available to pull through `memory.why` as short spans. |
