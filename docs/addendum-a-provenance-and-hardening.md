# repo-memory Addendum A: Provenance, Sharing and Hardening

**Status:** Approved — Revision 3 (`v2.2`)
**Date:** 2026-10-15
**Applies to:** [SPEC v1.0](../SPEC.md) together with the [v2.2 design (Revision 4)](design-v2-distillation-retrieval.md).

This addendum **adds** to those documents and replaces nothing in them. Where it changes an existing section, §16 lists the change.

**For reviewers:** the questions we most want answered are in §18.

**Revisions:**

| Rev | Changes |
|---|---|
| 1 | First draft |
| 2 | Reorganised into three phases by how much each item couples memory to git (§4): **A** no new git coupling, **B** reads git history, **C** writes state to git. Commit links are split into derived local links (A1, phase B) and shared links (A11, phase C). Phase B handles rebases by `patch-id` remapping, so it needs no git writes. Phase C has a data-driven entry criterion. |
| 3 | **v2.2 alignment (`2026-10-15`):** (1) **A8 (`§8`)** upgrades the managed instruction block to `v2`, adding anti-echo framing, MCP pull tool pointers (`memory_brief`, `memory_why`, `memory_history`, `memory_search`), and optional mid-turn `memory_note` guidance alongside automatic `Stop` end-of-turn capture, and enforces `VD19` (`GEMINI.md` strictly machine-managed by `enroll`/`unenroll`). (2) **A7 (`§10`)** integrates `procedural` lifecycle execution-outcome validation (`PostToolUse` exit-code tracking, `success_count`/`failure_count`, build-manifest `stale_suspect` flagging, and graduation into `.agents/skills/rm-<slug>/SKILL.md` with `generated_by: repo-memory` once `success_count >= 2` across sessions). (3) **A9 (`§11`)** adds **`DocSeedSource` (`source: doc-seed`)** as Priority 0 built-in source (`repo-memory seed-docs` + async "Dreaming" git-diff freshness). |

---

## 1. Summary

A review of a comparable product (§2) and our `PR-06`–`PR-10` empirical studies found several techniques that strengthen provenance, procedural reliability, and repository doc coverage. Each is designed here from scratch to fit our principles: P1 (zero cooperation required), P5 (truth from outcomes), P11 (never slow the loop), P12 (review gates sharing), P13 (evidence independence), and P14 (push-first delivery, pull for depth).

| Phase | # | Item | What it gives us |
|---|---|---|---|
| **A** No new git coupling | A4 | **Layered scrubber** | Replaces the four-regex stub. Shared rules can't be weakened locally. Any scanner failure blocks the write. |
| | A5 | **Spend guards and dry runs** | Cost estimates before any token-spending job. Frequency and volume limits are separate knobs. |
| | A6 | **Freshness, gaps and short-term recall** | Pull answers say how current memory is and where it has nothing. Unconsolidated notes are searchable at once. |
| | A7 | **Procedures become skills (`procedural` lifecycle)** | `PostToolUse` tracks `recipe`/`procedure` command exit codes (`success_count`/`failure_count`); recipes verified across ≥ 2 sessions (`success_count >= 2`) or marked `render_as_skill` graduate deterministically to `.agents/skills/rm-<slug>/SKILL.md` |
| | A8 | **Managed instruction blocks (`v2`)** | Safe, reversible `<!-- repo-memory:begin v2 -->` block in `GEMINI.md` and **[R2]** `CLAUDE.md` managed strictly by `enroll`/`unenroll` (`init`/`leave`) |
| | A9 | **Repository (`DocSeedSource`) & external capture sources** | Priority 0 `DocSeedSource` (`repo-memory seed-docs`, `source: doc-seed`) distills curated repo docs into anchored entries and refreshes them on `git diff`; external sessions can also be imported as events |
| | A10 | **Evaluation discipline** | Paired, pre-registered comparisons for X6, Eval v2 (2×2 forward-task design), and the B-series benchmarks |
| **B** Reads git history | A1 | **Commit links (derived, local)** | Every commit an agent session contributed to is linked to that conversation and its notes. Merges and reverts become outcomes we can attribute. |
| | A2 | **Line-level `why`** | `memory_why path:line` follows git blame to the commit, then the conversation, the notes and the entries |
| **C** Writes state to git | A11 | **Shared commit links** | A1's links travel with the repository, in git notes |
| | A3 | **Shared evidence** | Evidence IDs cited by entries resolve on every machine, not only the author's |

---

## 2. Clean-room provenance

**Input.** On 2026-10-06 we reviewed the public product documentation for Entire (docs.entire.io), including its "Brain" memory plugin's public README and command reference. We used it only to spot problem areas and general techniques. No third-party source code was read.

**Rules this addendum follows:**
- Every name, format, ref namespace, trailer, config key, command and data structure here is our own. None is copied from that product.
- The git mechanisms used are standard, documented git features: blame, log, notes, custom refs, plumbing commands, `patch-id`, trailers and `notes.rewriteRef`.
- Where an idea matches something that product does, this document says so, and the design here is derived from our own requirements.

**Rules for implementers:**
1. Don't read third-party source code while implementing an A-item.
2. Implement A9 importers only from a tool's **published format documentation**. Record the document and its version in the importer's header comment.
3. Cite this addendum, not the third-party docs, as the design source in code and PRs.

---

## 3. Goals and non-goals

**Goals**
- **AG1 Traceability.** From any line, commit or entry, reach the conversations, notes and outcomes behind it in one call (phase B).
- **AG2 Portable evidence.** An entry's evidence can be checked on any machine that has the repository, when the team opts in (phase C).
- **AG3 Safe by construction.** Nothing leaves the private store without passing a scrubber that fails closed.
- **AG4 Predictable cost.** Every token-spending job can be estimated first, and is bounded by frequency, volume and quota.
- **AG5 Honest answers.** Pull answers state how fresh memory is and where it is silent.
- **AG6 Coupling only where it pays.** Each extra dependency on git is taken on in its own phase, with its own entry criterion.

**Non-goals** (these add to SPEC §1.2 and v2 §3)
- Rewriting the user's commits, or prompting during `git commit`.
- Installing git hooks by default.
- Line-level authorship attribution, such as "this line was written by AI".
- Hosting, mirroring or remote search services.
- Embeddings or vector search. This is unchanged from v2.
- A code-structure graph. P3 says structure is read from the code.

---

## 4. Phases and git coupling

### 4.1 The baseline: how we use git today

repo-memory already depends on git in these ways, and they are the baseline for phase A:
- `.memory/` is committed. The memory PR on `memory/pending`, built in a dedicated worktree, is the only way shared knowledge is published (SPEC §6.8).
- The repo ID is derived from the root commit (SPEC §4.6).
- Notes record the current branch and HEAD (v2 §5.1). A branch merging moves its notes from the local lane to the shared lane (v2 §6 step 7).
- Anchors are checked against the working tree (v2 §8.1).

### 4.2 The three phases

| Phase | Git coupling | Allowed git operations | What it adds |
|---|---|---|---|
| **A** | None beyond the baseline | The baseline, plus a read-only check that a commit object exists (`git cat-file -e`, used by A4 to recognise SHAs). A4 has a fallback for when git is unavailable. | Hardening, cost control, honest answers, skills, evaluation |
| **B** | **Reads history** | Read-only: `log`, `blame`, `rev-list`, `patch-id`, `reflog`, `rev-parse`. Optionally `gh` reads. All results go into the rebuildable local index. | Provenance (A1) and code-to-reasoning answers (A2). Merges and reverts become outcomes we can attribute (P5). |
| **C** | **Writes state to git** outside the working tree | Notes and custom refs written with plumbing; explicit fetch and push of our namespaces; one local config key; an optional `prepare-commit-msg` hook | Links and evidence shared between machines and developers |

**Boundary rule.** Code for a later phase must not be needed by an earlier phase's features. Each phase is behind its own feature flag: `git.read_history`, then `git.write_state`. Turning a flag off returns repo-memory to the previous phase's behaviour, and nothing breaks.

### 4.3 Entry criteria

| Phase | Starts when | Exit gate |
|---|---|---|
| **A** | Now. Most of it lands with PR-07 (§15). A7 additionally waits for X6 and Y6. | Y4 and Y5 pass |
| **B** | v2's harvest path exists (PR-07), so `observed` links can be recorded when a note is harvested | Y1, Y2b and Y7 pass |
| **C** | **Need is shown with data.** Phase B instruments `memory.stats` with: evidence IDs that couldn't be resolved locally, `why` answers missing links that exist only on another machine, and the number of machines or developers publishing to the memory PR. Phase C starts when a second developer or machine publishes regularly, and those counts are non-trivial. | Y2c and Y3 pass |

As a solo user on one machine you stay in phase B indefinitely, and lose nothing.

---

# Phase A: no new git coupling

## 5. A4: Layered scrubber

### 5.1 Current state

`internal/core/scrub` is an empty package. Scrubbing is a private function in `internal/core/pipeline/compact/compact.go` (`scrubSecrets`) with four regexes. Two of them are harmful:
- **every UUID** is replaced, and that includes conversation IDs;
- **every hex string of 32 or more characters** is replaced, and that includes full git SHAs.

That damages evidence today, and phase B depends on both kinds of ID surviving.

### 5.2 Design

There is one package, `internal/core/scrub`, with one entry point:

```go
func Scrub(text string, c Context) (Result, error)
// Context: write boundary, repo ID, known-ID allow-set.
// Result: scrubbed text, plus counts by label (never values).
```

**Write boundaries.** It runs everywhere data crosses a boundary:
- episode compaction;
- note harvest;
- input prepared for the lean agent (v2 §5.6);
- op publish (the existing second scan, SPEC §11.3).

In phase C it also runs on evidence objects and commit-link notes.

**Layers**, applied in order:

| # | Layer | Detects |
|---|---|---|
| 1 | **Structural** | Private-key blocks; credentialed URLs (`scheme://user:pass@host`); database connection strings; `Authorization:` headers; assignments to secret-like names (`*_TOKEN`, `*_SECRET`, `*_KEY`, `PASSWORD`, …); provider key formats with fixed prefixes. The provider list is our own table, maintained in the package. |
| 2 | **Entropy** | Unlabelled tokens of 24 or more characters over base64 or hex alphabets, above an entropy threshold. **Allowed through:** UUIDs and ULIDs in the known-ID allow-set (conversations, steps, notes, entries); hex strings of 7–40 characters that resolve as commit objects in this repository (`git cat-file -e`, batched); content from files matched by `scrub.skip_paths` (for example `go.sum` and lockfiles). **If git is unavailable,** a SHA-shaped string is kept only when it appears in a git context (git command output, `commit`, `sha` or `rev` labels), and is otherwise redacted. |
| 3 | **PII** (opt-in) | Email addresses and phone numbers |
| 4 | **Repo rules** | Label-to-regex rules, plus allow patterns, from `.memory/scrub.yaml` |
| 5 | **Blobs** | Inline base64 images and binaries over 1 KiB, replaced by `«blob:image/png 23KiB sha256:ab12…»` |

**Placeholders.** The existing format `«secret:<label>»` is kept. Optionally it gains a short keyed hash, as in `«secret:github-token#3f9a»`. The hash is an HMAC with a per-machine key, so repeats of one secret de-duplicate without revealing anything.

### 5.3 Policy layering

| Layer | File | Can do |
|---|---|---|
| **Shared** | `.memory/scrub.yaml` (reviewed in a PR) | Turn layers on, add rules, add allow patterns |
| **Local** | `<data-dir>/<repo-id>/scrub.local.yaml` | **Add** rules only. It can't disable a layer or a shared rule, and it can't add allow patterns. |

The shared file decides what may enter shared lanes. One developer's local settings therefore can't weaken it.

### 5.4 Failing closed

- **If any layer fails** (a bad regex in repo rules, a timeout, an error from a `git cat-file` batch), the write is refused.
  - **Tick jobs** retry and then dead-letter, as in SPEC §4.3.
  - **Note harvest**, which runs in a hook, writes the raw note to a private `quarantine/` directory instead of the inbox and never blocks the agent. The tick re-scrubs it later.
- **Hooks never wait on the scrubber** beyond the harvest budget: 2 ms for a 600-token note (Y4).
- **`repo-memory doctor scrub`** reports counts by label and boundary, never values.

### 5.5 Test corpora

| Corpus | Purpose |
|---|---|
| `testdata/scrub/planted/` | Secrets of every supported format, planted in realistic transcript steps |
| `testdata/scrub/benign/` | Look-alikes that must survive: SHAs, conversation UUIDs, ULIDs, `go.sum` lines, ESP-IDF hex dumps, MAC addresses, partition tables and flash offsets |

---

## 6. A5: Spend guards and dry runs

### 6.1 Dry runs

Every job kind that spends tokens supports `--dry-run --json`. That covers the fallback, consolidation, backfill and gardening.

```
repo-memory backfill --dry-run --json
repo-memory consolidate --dry-run --json
```

The estimate gives:
- items (conversations or notes);
- calls;
- input tokens per pool (chars/4, our convention), plus the lean agent's measured overhead (X8);
- expected output;
- the share of each pool's remaining weekly and 5-hour quota from `/usage` (X3).

**Calibration.** Each real run stores its estimate next to the actual usage reported in `agy`'s JSON output. The estimator scales by the rolling median of actual over estimate, and `memory.stats` reports that ratio.

### 6.2 Guards

A job runs only when **every** guard passes:

| Guard | Scope | Persisted in |
|---|---|---|
| Quota floor | Per pool (existing, v2 §5.4) | Read live from `/usage` |
| Daily note cap | Per repository (existing, v2 §5.1) | Job table |
| **Minimum interval** | Per job kind | Job table: last start time, which survives restarts |
| **Maximum items per run** | Per job kind | Configuration; the cursor in the job table |

Frequency and volume are separate on purpose. A minimum interval alone lets one run take everything. A volume cap alone lets many runs follow each other.

They are set per machine, because quota is per user:

```yaml
# <data-dir>/config.yaml
spend:
  quota_floor: 0.25
  jobs:
    fallback:      {min_interval: 1h,   max_items: 10}  # conversations
    consolidation: {min_interval: 24h,  max_items: 40}  # notes
    backfill:      {min_interval: 6h,   max_items: 25}  # conversations
    gardening:     {min_interval: 168h, max_items: 20}  # dossiers
```

### 6.3 Nothing spends without consent

`repo-memory init` never starts a token-spending job. It prints the backfill dry-run estimate and the command to start it. Backfill order follows SPEC §14: configured priority areas first, then the highest-churn areas.

---

## 7. A6: Freshness, gaps and short-term recall

### 7.1 Freshness record

After each run, the tick writes `<data-dir>/<repo-id>/state/freshness.json`:
- the op-log position the index was built from, and whether a rebuild is pending;
- the HEAD the index was built at, and whether HEAD has moved since. That's a baseline git read;
- the size of the inbox and the time of the last consolidation;
- the count of entries marked `suspect` (v2 §8.1).

### 7.2 Effect on delivery

- **Push.** If the index is unreadable or older than the op log, hooks push nothing (degraded mode, SPEC §7.9). That beats pushing from a stale index. The anchor check at delivery still guards each individual entry.
- **Pull.** Every `memory.brief`, `memory.why` and `memory.search` answer starts with one state line, for example:

  > `memory: index current to op log · HEAD moved since build · 4 notes not yet consolidated · 2 entries suspect`

  In phase B the line can say how many commits behind HEAD the index is.

### 7.3 Gaps

`memory.brief` ends with up to three gap lines. A gap is either:
- a dossier matched for the task whose coverage (SPEC §9.1) is below `gaps.threshold`; or
- a path in the task's scope that no dossier covers.

For example:

> `No memory covers internal/core/scrub. Absence of memory is not evidence; read the code.`

Gaps are **pull-only by default**. `gaps.push: true` adds at most one gap line to the task-start push, only when the task's main scope has no dossier. This keeps to v2 §8.1.6: no overviews in pushes.

### 7.4 Short-term recall

Harvested notes are useful before consolidation runs, which can be a day later.
- **Indexed within a tick.** The tick indexes inbox notes in full-text search, with status `unconsolidated` and their branch scope.
- **Pull only.** `memory.search`, `memory.why` and `memory.brief` return them, labelled. They are **never pushed**.
- **Removed when consolidated,** once ops reference them.

This keeps P11: hooks only spool, and the tick indexes within a minute.

---

## 8. A8: Managed instruction blocks (`v2`)

This replaces SPEC §4.6 step 2 ("adds the map include line"). `repo-memory enroll` (alias `init`) writes a delimited `v2` block into the harness's instruction file: `GEMINI.md`, and **[R2]** `CLAUDE.md`.

```markdown
<!-- repo-memory:begin v2 — managed by repo-memory; edits inside are overwritten -->
@[memory atlas](.memory/ATLAS.md)

## Repository Memory (`repo-memory`)
- **Pushed context (`[repo-memory ...]`):** Hooks automatically inject verified invariants, gotchas, failure signatures, and ruled-out approaches when you start a task or touch anchored files. Treat them as repository notes verified against the code; do not repeat or echo them as new discoveries.
- **Pull for depth (MCP):** Use `memory_brief` (module dossier), `memory_why` (entry/file/line rationale), `memory_history` (evolution), and `memory_search` (FTS5 search) when exploring unfamiliar subsystems.
- **Distillation:** End-of-turn capture happens automatically via the `Stop` hook on milestone turns (no action required). Use `memory_note` mid-turn only when you want to record a critical discovery or dead end immediately.
<!-- repo-memory:end -->
```

- **Strictly machine-managed (`VD19`):** Root `GEMINI.md` (or the delimited block inside an existing `GEMINI.md`) is managed exclusively by `repo-memory enroll` (`init`) and `repo-memory unenroll` (`leave`), never hand-populated with static v1 `@[memory map](.memory/views/MAP.md)` pointers.
- **Creating:** if the file doesn't exist, `enroll` (`init`) creates it and records that in the registry. Otherwise the block is appended.
- **Updating:** only the text between the markers is ever rewritten. The version tag (`v1` $\to$ `v2`) allows automatic in-place upgrades from `views/MAP.md` to `.memory/ATLAS.md`.
- **Leaving:** `unenroll` (`leave`) removes the block. It deletes the file only if we created it and it is now empty.
- **Duplicate or unbalanced markers:** `enroll` refuses, and `doctor` reports them.
- **`AGENTS.md`** is still never touched by default (SPEC §12.2).

---

## 9. A10: Evaluation discipline

This applies to X2, X6, Eval v2 (2×2 forward-task design), and B1–B5. It should land before X6 runs.

- **Paired design.** Every task runs under every arm (no memory, PA documents, dossiers) from the same commit, with the same model. Each arm runs at least 3 times, because agent runs vary. Per-task differences are the unit of analysis.
- **Pre-registration.** The dataset file declares the arms, metrics, minimum effect sizes and the kill criterion **before** the first run. `repo-memory eval` refuses to compare runs whose dataset changed after the runs started.
- **Statistics:**
  - a Wilcoxon signed-rank test on per-task differences, or a paired t-test when differences are near-normal;
  - Holm correction across metrics (tests passed, steps, tokens, repeats of known dead ends);
  - each result reported as an effect size with a 95% confidence interval.
- **Claims.** A metric counts as improved only if its corrected p < 0.05 **and** its effect is above the declared minimum.
- **Report section.** Every evaluation report has a section titled **"What this does not show"**.
- **Tooling.** `repo-memory eval compare <a.json> <b.json>` in `internal/eval` does the pairing, tests and correction.

X6's kill criterion ("if dossiers don't beat no memory, stop") is evaluated this way.

---

## 10. A7: Procedures become skills (`procedural` lifecycle)

In `v2.2` (design Revision 4 §5.2), `recipe` (alias `procedure`) and `failure-signature` entries belong to the **`procedural` cognitive lifecycle**, which is coupled to **execution outcomes** rather than static code anchors alone.

### 10.1 Execution-Outcome Tracking (`PostToolUse`) & Build-Manifest Freshness

1. **Command outcome tracking (`success_count` / `failure_count`):** Whenever `PostToolUse` observes a shell command execution (`run_command` / `bash`) whose normalized command prefix matches an active `recipe` / `procedure` entry's `commands` list:
   - exit code `0` increments the entry's `success_count` and records `last_succeeded_at` and the conversation ID;
   - a non-zero exit code increments `failure_count` and flags the entry `stale_suspect: true` if two consecutive sessions fail on the recipe's command.
2. **Build-manifest sensitivity:** If a commit or working-tree diff modifies a repository build or environment manifest (`Makefile`, `CMakeLists.txt`, `platformio.ini`, `sdkconfig*`, `go.mod`, `package.json`, `Taskfile.yml`, `.github/workflows/*.yml`) anchored to a `recipe`, `tick` flags that recipe `stale_suspect: true` until its command next succeeds (`exit 0`).

### 10.2 Graduation to `.agents/skills/rm-<slug>/SKILL.md`

A `recipe` / `procedure` entry graduates to an on-demand harness skill when **either**:
- it carries an explicit `render_as_skill: rm-<slug>` attribute approved in a memory PR; **or**
- it has **2 or more steps**, **`success_count >= 2` across 2 or more distinct conversations**, `failure_count == 0` since its last amendment, and is not `stale_suspect`.

**How graduation renders:**
- **Deterministic rendering (no LLM):** It is a pure projection of the op log, like a dossier:
  - **Frontmatter:** `name: rm-<slug>`, `description: <entry title + relevant_when>`, and `generated_by: repo-memory` (`source_entry_id: <id>`). Both `name` and `description` are required so Antigravity indexes the skill (X8).
  - **Body:** the verified steps/commands, `Why`, anchor paths, evidence links, and a footer: `"Generated from [<id>] by repo-memory. Edit the dossier entry or op log, not this file."`
- **Location:** `.agents/skills/rm-<slug>/SKILL.md` on Antigravity, and **[R2]** `.claude/skills/rm-<slug>/SKILL.md` on Claude Code.
- **CI lint** (SPEC §11.4) checks that every file with `generated_by: repo-memory` under `.agents/skills/rm-*/SKILL.md` matches what deterministic regeneration produces.
- **Lifecycle:** when the underlying `recipe` entry is superseded, retired, or flagged `stale_suspect`, the renderer updates or removes `.agents/skills/rm-<slug>/SKILL.md` on the next render pass.
- **Delivery:** while a graduated skill exists, `first-touch` pushes for that recipe shrink to a 1-line pointer (`"skill rm-<slug> covers this procedure"`), saving context tokens while keeping discovery push-first.

### 10.3 Caution

Generated always-on instruction files have been measured to hurt (v2 §2.2), and passive unprompted skills have a 1–8% read rate when agents do not know they exist (v2 §2.4). Pairing on-demand `.agents/skills/rm-<slug>/SKILL.md` files with a 1-line `first-touch` pointer gives agents push-first awareness without loading the full multi-step recipe until needed. Y6 gates this item.

---

## 11. A9: Repository (`DocSeedSource`) and external capture sources

### 11.1 Priority 0 Built-in Source: `DocSeedSource` (`repo-memory seed-docs` / `source: doc-seed`)

Our 77-session `esp32` audit (v2 §2.4) showed that curated human-authored markdown guides (`docs/developer/`, `.agents/skills/`, architecture docs) go 92–99% unread unless pushed via hooks. Revision 3 promotes in-repo documentation to **Priority 0 (`DocSeedSource`)**, ahead of external session importers:

1. **Ingestion (`repo-memory seed-docs`):** Discovers curated markdown files configured under `doc_seeds` in `.memory/config.yaml` (defaulting to `docs/developer/**/*.md` and hand-written `.agents/skills/**/SKILL.md` files that do not have `generated_by: repo-memory`). Each file is passed through the preparation layer and `repo-memory-distiller` to extract concrete `invariant`, `contract`, `gotcha`, `recipe`, and `failure-signature` notes anchored to the source files and symbols they govern.
2. **Provenance metadata (`source: doc-seed`):** Every note and resulting op-log entry carries:
   - `source: "doc-seed"`
   - `source_doc_path`: repo-relative path to the markdown file (e.g., `docs/developer/build-and-flash.md`)
   - `source_doc_blob_sha`: git blob SHA (`git hash-object <path>`) of the markdown file at distillation time
3. **Async Doc Freshness ("Dreaming" in `tick`):** On each background `tick`, the daemon checks whether any `source_doc_path`'s current git blob SHA differs from `source_doc_blob_sha` (or if the file was deleted). When a seeded doc changes in `git diff`, `tick` queues a background re-distillation job ("dreaming") that reconciles the updated markdown against the existing `source: doc-seed` entries for that file and emits `amend`, `supersede`, or `retire` ops.

### 11.2 External session capture sources (`CaptureSource`)

Some users also run another tool that captures agent sessions. Importing those sessions as events extends coverage (SPEC G9) and gives the fallback more input.

```go
// internal/harness — beside Harness, not part of it
type CaptureSource interface {
    Name() string
    Discover(ctx context.Context, repoRoot string) ([]SessionRef, error)
    Read(ctx context.Context, ref SessionRef) (episode.Raw, error) // → normal compaction
}
```

- **Phase split.** The interface, `DocSeedSource`, PA work-order reports (`source: pa`), and file-based sources are phase A. Sources that store sessions in git refs or branches are read-only git reads, so they wait for phase B (`git.read_history`).
- **Treated as import events** (SPEC §14) with `provenance: capture:<source>`. They are processed by the fallback (v2 §5.4) and the scrubber like everything else.
- **De-duplicated:** if our own capture already holds the same harness conversation ID, our copy wins.
- **Echo safety:** our injection ledger covers any conversation we delivered to, whoever captured it. A session with no ledger is marked `exposure: unknown`. Consolidation treats an item that repeats an existing entry from such a session as an echo (a no-op), which errs on the safe side.
- **Read-only:** importers never write to another tool's storage.
- **Clean-room:** each importer is written from that tool's published format documentation, pinned to a format version, with a fixture test (§2).
- **Existing sources** already fit this interface: harness transcripts, PA files (`import-pa` / `seed`), and curated repo docs (`seed-docs`).

---

# Phase B: reading git history

Everything in this phase is **read-only**. Results go into the rebuildable local index (`index.sqlite`), and nothing is written to the repository, its refs or its config. It is gated by `git.read_history` (default on once phase B ships).

## 12. A1: Commit links (derived, local)

### 12.1 Problem

Notes record the branch and head commit at harvest (v2 §5.1), so we know which commit a conversation **started from**. We don't know which commits it **produced**. That has two consequences:
- `memory.why` can't get from code to the reasoning behind it.
- **The P5 outcome signals can't be attributed.** SPEC P5 counts merges and reverts as outcomes, but we can't tell which conversation's work was merged or reverted.

### 12.2 Link record

There is one row per link in the `commit_links` table:

| Field | Example |
|---|---|
| `commit` | full SHA |
| `conversation` | conversation ID |
| `harness` | `antigravity` |
| `strength` | `observed` |
| `steps` | `[120, 188]` |
| `notes` | `["note-01M4…"]` |
| `files` | `["internal/core/scrub/scrub.go"]` |
| `branch` | `pr-07-scrub` |
| `linked_at` | `2026-10-06T14:02:11Z` |

`strength` is one of:

| Strength | Meaning | How it's derived |
|---|---|---|
| `observed` | The agent made this commit | The transcript has a commit tool call, and its output names the new commit. The short SHA is resolved with `git rev-parse`. The milestone gate (v2 §5.1) already detects commits, so it records this link at harvest. |
| `inferred` | A human committed work the agent did | See below |
| `merged-from` | A merge or squash commit that carries linked commits | See §12.3 |

A fourth strength, `declared`, arrives in phase C (§14.3).

**Inferred links.** The tick reads new commits since its cursor (`git rev-list`). For each commit with no `observed` link, it computes for each conversation:

> overlap = |files in the commit ∩ files the conversation edited| / |files in the commit|

It then links every conversation where all of these hold:
- overlap ≥ 0.5;
- the conversation's last edit to those files came before the commit time, and within `links.window` (default 24 h);
- no later conversation edited the same files before the commit.

Several conversations can link to one commit.

### 12.3 Surviving history rewrites, read-only

Phase B writes nothing to git, so it can't use `notes.rewriteRef`. Instead it re-finds rewritten commits by their content.

| Rewrite | Handling |
|---|---|
| Local `rebase`, `commit --amend`, cherry-pick | When the tick sees a new commit with no link, it compares that commit's `git patch-id` with the patch-ids of linked commits that are no longer reachable from any branch (found through the reflog). On a match, the link is copied to the new SHA with the same strength. An amend that changed the diff falls back to inferred linking. |
| Merge commit | A `merged-from` link on the merge commit lists the linked commits it brings in |
| Squash merge, rebase-merge on a host | The tick already watches branches merging, for lane promotion (v2 §6 step 7). When a branch with linked commits disappears or is merged, the tick compares `git patch-id` of the branch's combined diff with each new commit on the target branch. A match adds a `merged-from` link. |
| Squash merge with conflicts (patch-ids differ) | If `gh` is available and the subject ends in `(#N)`, the PR's commit list is read (read-only). Otherwise the links stay on the branch commits and are reported as unresolved in `memory.stats`. |

**Rebuilding.** The table can be rebuilt from this machine's transcripts plus git. Links for work done on other machines aren't visible until phase C.

### 12.4 Outcomes for consolidation

Commit links turn git history into outcome evidence (P5):

| Git event | Evidence for the linked conversation's notes |
|---|---|
| The linked commit (or its `merged-from` successor) reaches the default branch | `merged`: human acceptance |
| The linked commit is reverted (`This reverts commit <sha>`, or a patch-id inverse) | Refutation of note items anchored to the reverted files (SPEC §14 already treats reverts as refutation) |
| The commit is still unmerged when its branch is deleted | No outcome: neither supports nor refutes |

The P13 echo rule still applies. A merge counts as support for a repeated entry only if the conversation changed one of the entry's anchored files and then passed a check.

---

## 13. A2: Line-level `why`

### 13.1 Behaviour

`memory.why` (SPEC §7.7) accepts `path:line`, `path:start-end` and `path#symbol`, as well as its current targets. The CLI equivalent is `repo-memory why <target>`.

1. `git blame --porcelain` over the range gives the commits that last touched those lines.
2. `commit_links` gives the conversations behind those commits, together with their strengths.
3. From there it collects:
   - notes from those conversations whose anchors cover the path;
   - entries whose evidence cites those notes;
   - current entries anchored to the path or symbol.
4. For `path#symbol`, `git log -L :<symbol>:<path>` adds the symbol's earlier commits, where git's function matching supports the language. `memory.history(entity)` uses the same lookup.

**Output** is bounded, at 800 tokens by default. Per commit it gives the short SHA, subject, date, link strength, conversation title, note item titles and entry IDs. After that come the current entries for the target, ranked as in v2 §8.3.

**Commits with no link** still appear, with their subject and date. Git history alone is often a useful answer.

### 13.2 Stated limits

Every answer carries these as a single line:
- **Links are per commit, not per line.** A line hand-edited inside an agent commit is still linked to that conversation.
- **Blame follows rewrites.** After a squash, the answer comes through `merged-from` links.
- **Uncommitted lines** are reported as "uncommitted (this working tree)", with the current conversation if it edited them.
- **Quoted notes are historical evidence.** Verify them against the code before acting on them.

### 13.3 Why this matters

Today `memory.why` goes from entry to evidence. A2 adds the opposite direction, from code to reasoning. That's the question people actually ask: "why is this line like this?" Y7 measures it.

---

# Phase C: writing state to git

Phase C exists only to **share** phase B's links and our evidence across machines and developers. It is gated by `git.write_state` (default off), and starts only when the criteria in §4.3 are met.

**What it may write:**
- notes under `refs/notes/repo-memory`;
- refs under `refs/repo-memory/`;
- one local config key (`notes.rewriteRef`);
- optionally, a `prepare-commit-msg` hook.

**What it never touches:** branches, the working tree, the index, HEAD, and the user's commits. The memory PR remains the only route for **knowledge**. Phase C carries only provenance and evidence, which are never delivered.

## 14. A11: Shared commit links

### 14.1 Storage

- **Git notes** under `refs/notes/repo-memory`, one JSON line per link (the §12.2 fields).
  - Notes hold IDs and strengths only, never transcript text, and they pass the scrubber (A4).
  - Notes don't change commit SHAs.
  - Notes aren't fetched or pushed unless a refspec asks for them, so other people's clones are unaffected.
- **Rewrites.** `git.write_state` sets `notes.rewriteRef = refs/notes/repo-memory` in the repository's **local** config, so git copies notes onto rebased and amended commits. Phase B's patch-id remapping (§12.3) stays as the fallback.
- **On read,** the local `commit_links` table merges its own rows with rows from notes. Notes add links for work done on other machines.

### 14.2 Sync

Sync follows the setting in §15.4.
- **Fetch:** into `refs/notes/repo-memory-remote/<remote>`.
- **Merge:** with `git notes merge -s cat_sort_uniq`, which is a clean union for line-oriented notes.
- **Push:** the merged ref is pushed.

### 14.3 Optional trailer

The trailer is off by default (`links.trailer: off`). When it's on, `repo-memory init` installs a `prepare-commit-msg` git hook. The hook adds:

```
Repo-Memory-Conversation: <conversation id>
```

It does this only when exactly one conversation edited the staged files within the window. Links from the trailer have strength `declared`.
- It is **never interactive**: prompts break agent commits and GUI git clients.
- If our binary isn't on `PATH`, the hook does nothing.
- Its value is that the trailer survives squash merges that phase B can't match (§12.3, last row).

---

## 15. A3: Shared evidence

### 15.1 Problem

Entries cite evidence such as `note-01M4…` or `E09`. Notes live in each developer's private evidence store (v2 §7.1), so on a teammate's machine:
- `memory.why` dead-ends at an ID it can't resolve;
- consolidation can't see the other machine's exposure lists, which weakens the P13 echo filter across machines.

Phase B's `memory.stats` counters (§4.3) measure how often this happens.

### 15.2 What can be shared

`evidence.share` is set in `.memory/config.yaml`. It is a team decision.

| Level | Shared | Default |
|---|---|---|
| `none` | Nothing. This is today's behaviour. | ✅ |
| `notes` | Harvested notes, already scrubbed and linted (v2 §5.1), with their metadata: conversation ID, steps covered, model, harness version, branch, head commit, anchor check results, exposure list | |
| `notes+summaries` | As `notes`, plus platform checkpoint summaries (v2 §5.3), after a second scrub | |

Raw transcripts and episodes are **never** shared, at any level. SPEC §11.1 is unchanged.

**Evidence isn't memory.** Nothing in the evidence refs is ever delivered. Delivery reads only the op log, which still goes through the memory PR. Sharing evidence therefore doesn't bypass P12 for knowledge. It does publish content without review, which is why the default is `none` and the scrubber fails closed.

### 15.3 Storage

There is one ref per object:

```
refs/repo-memory/evidence/<last two chars of ULID>/<ULID>
```

- Each ref points to a parentless commit whose tree holds `note.json`, and `summary.md` at `notes+summaries`.
- Objects are written with plumbing only: `hash-object`, `mktree`, `commit-tree` and `update-ref`. The working tree, the index and HEAD are never touched.
- **Immutable:** an object is never rewritten. A correction is a new object with `supersedes: <ULID>`.
- **Why one ref per object:** no two machines ever write the same ref, so pushes never fail as non-fast-forward. Objects can also be fetched and pushed one at a time.
- **Scale:** a few thousand objects a year is a few thousand refs. Git handles that comfortably with packed refs. Y3 checks it.

### 15.4 Sync

Sync is set per machine, in `<data-dir>/config.yaml`. The same settings cover commit-link notes (§14.2).

| Setting | Values | Default |
|---|---|---|
| `evidence.sync` | `manual` (`repo-memory evidence push` and `pull`) or `tick` (at most hourly) | `manual` |
| `evidence.remote` | A remote name, or a separate repository URL | Must be set explicitly when sharing is turned on |

- **No fetch refspec is added.** The tick fetches with an explicit refspec (`'refs/repo-memory/*:refs/repo-memory/*'`) and never adds one to the remote's config. A user's `git fetch` is unaffected.
- **No git hooks** are used for sync.
- **Separate repository.** A URL keeps evidence out of a public code repository.
- **Turning sharing on.** `repo-memory evidence enable --remote <r>` prints exactly what would be pushed and asks for confirmation. `repo-memory evidence pending` lists unpushed objects at any time.
- **Failures** are reported in `memory.stats` and never block other work.

**Retracting.** `repo-memory evidence retract <ULID>` deletes the local ref and pushes the deletion. The command warns that clones which already fetched the object keep their copy.

### 15.5 Reading

- **The index** includes fetched evidence objects.
- **`memory.why`** resolves an evidence ID from the local store first, then the refs.
- **Consolidation** reads exposure lists from shared notes, so the echo filter works across machines.

---

## 16. Changes to existing sections

| Phase | Section | Change |
|---|---|---|
| A | SPEC §4.6 step 2 | Replaced by managed blocks (A8) |
| A | SPEC §7.7 pull tools | All pull answers start with a state line. `memory.brief` ends with gap lines (A6). |
| A | SPEC §7.9 degraded modes | A stale or unreadable index means no pushes (A6) |
| A | SPEC §10.3 benchmarks | Paired, pre-registered design and reporting (A10) |
| A | SPEC §11.3 secrets | Replaced by the layered scrubber at every write boundary, failing closed (A4) |
| A | SPEC §11.4 integrity | The lint also checks generated skills against regeneration (A7), and `.memory/scrub.yaml` for valid patterns (A4) |
| A | SPEC §14 migration | Import sources generalised as `CaptureSource` (A9). Backfill never starts without consent (A5). |
| A | SPEC §18.4 libraries | Secret scanning is the `scrub` package's own layers. Any third-party library is an optional layer-1 provider behind the same interface. |
| A | v2 §5.1 harvest | Scrubs the note, or quarantines it (A4) |
| A | v2 §5.4 fallback, v2 §6 consolidation | Subject to A5 guards |
| A | v2 §6 gardening | May propose `render_as_skill` (A7) |
| B | SPEC §7.7 pull tools | `memory.why` accepts `path:line`, `path:start-end` and `path#symbol`. `memory.history` uses `git log -L` (A2). |
| B | v2 §5.1 harvest | Records `observed` commit links (A1) |
| B | v2 §6 consolidation | Merged and reverted linked commits count as outcomes, under P13 (A1) |
| B | SPEC §10.4 observability | `memory.stats` counts unresolved evidence IDs, links missing because they exist only on other machines, and publishing machines. These are the phase C entry signals. |
| C | SPEC §4.6 enrolment | With `git.write_state`, `init` sets `notes.rewriteRef` in local config, and optionally installs the trailer hook (A11) |
| C | SPEC §11.1 privacy | Unchanged by default. `evidence.share` is an explicit, team-level exception that never covers transcripts or episodes (A3). |

**Storage additions:**

```
Phase A
  .memory/scrub.yaml                            shared scrub rules (A4)
  .agents/skills/rm-<slug>/SKILL.md             graduated procedural recipes (A7)
  <data-dir>/repos/<repo-id>/scrub.local.yaml   local additions only (A4)
  <data-dir>/repos/<repo-id>/quarantine/        notes that failed scrubbing (A4)
  <data-dir>/repos/<repo-id>/state/freshness.json (A6)
  index.db (via index.IndexPath): + inbox FTS   (A6)
Phase B
  index.db: + commit_links, patch-id cache      (A1)
Phase C
  refs/notes/repo-memory                        commit → conversation links (A11)
  refs/repo-memory/evidence/<xx>/<ULID>         shared evidence objects (A3)
  index.db: + evidence_objects                  (A3)
```

**New config keys:**

| Phase | Shared (`.memory/config.yaml`) | Machine (`<data-dir>/config.yaml`) |
|---|---|---|
| A | `scrub.*`, `gaps.threshold`, `gaps.push`, `skills.enabled`, `doc_seeds` | `spend.*` |
| B | `links.window` | `git.read_history` |
| C | `links.trailer`, `evidence.share` | `git.write_state`, `evidence.sync`, `evidence.remote` |

---

## 17. Phasing and validation

### 17.1 Schedule

| Phase | PR | Items | Why then |
|---|---|---|---|
| A | PR-06 / PR-07 / PR-08 | A4, A5, A6, A8, A10 | Layered scrubber (A4), spend guards (A5), managed instruction blocks (A8), freshness/gaps (A6), and paired eval harness (A10) landed with the v2 foundation and delivery engine. |
| A | PR-10a / PR-10b / PR-11 | A7 (`procedural` lifecycle & skill graduation), A8 (`v2` block), A9 (`DocSeedSource` / `seed-docs` + async "dreaming") | Codified in `PR-10a`, core delivery/storage fixes in `PR-10b`, and `seed-docs` + `procedural` lifecycle validation in `PR-11` ahead of the full 2×2 forward-task evaluation (`PR-10-eval-v2` WO4–WO7). |
| B | Post-Eval v2 | A1, A2, git-backed A9 sources | Extends `memory_why` to line-level git blame once v2.2 forward-task evaluation completes. |
| C | When §4.3's criteria are met | A11, A3 | Only needed for more than one machine or developer. |

### 17.2 Experiments

| Phase | # | Experiment | Pass criterion |
|---|---|---|---|
| A | Y4 | **Scrubber.** Run the planted and benign corpora (§5.5), plus 200 sampled transcript steps, with and without git available. | 100% recall on planted; ≤ 1% false positives on benign and the sample; ≤ 2 ms per 600-token note |
| A | Y5 | **Estimator.** Compare dry-run estimates with actual usage over 20 jobs, after calibration. | Within ±30% |
| A | Y6 | **Skills.** Replay 4 procedure-heavy tasks: entry pushes against the generated skill (A10 method). | The skill arm is no worse on success or steps |
| B | Y1 | **Link accuracy.** Hand-label the conversation(s) behind 40 commits across this repository and esp32-app-foundation. | `observed` precision ≥ 0.95; `inferred` precision ≥ 0.80; recall reported |
| B | Y2b | **Read-only rewrite survival.** On a scratch repository: amend, rebase, cherry-pick, merge, a clean squash merge and a squash merge with conflicts, without notes. | Links survive the first five. The sixth is reported as unresolved, or resolved through `gh`. |
| B | Y7 | **Line-level `why`.** 20 "why is this line like this?" questions taken from history: `memory.why` against `git log` and blame alone. | `memory.why` is correct on more questions, with a significant paired effect |
| C | Y2c | **Notes survival.** Repeat Y2b with `notes.rewriteRef` and the trailer, across two clones syncing notes. | No links lost; notes merge without conflicts |
| C | Y3 | **Evidence ref scale.** 10,000 synthetic evidence refs: time to fetch and push, `git gc`, and the effect on a plain `git fetch` from a clone without the refspec. | Fetch and push of 50 new objects ≤ 5 s; plain `git fetch` unaffected |

---

## 18. Risks and open questions

| Phase | Risk | Mitigation |
|---|---|---|
| A | The scrubber removes IDs we rely on | The known-ID allow-set and SHA resolution, with a fallback for when git is unavailable (§5.2), checked by the benign corpus |
| A | Scrubber false negatives | Layered detectors; shared rules; second scan before publish; the Y4 recall gate |
| A | Generated skills hurt agents | Always reviewed; on-demand loading; gated by X6 and Y6 |
| A | Importer formats drift | Version-pinned importers with fixture tests. An unknown version is skipped and reported. |
| B | Inferred links attach the wrong conversation | Strength is shown on every answer. The overlap and window thresholds are tuned in Y1. `inferred` alone never counts as an outcome under P5. |
| B | Patch-id remapping links the wrong commit (identical diffs on two branches) | Remap only to commits on the same branch, or its merge target; ambiguous matches are skipped and reported |
| B | Squash merges on hosts lose links | `patch-id` and `gh` fallbacks; unresolved links counted. Phase C's trailer is the durable fix if it matters. |
| C | Shared evidence leaks something sensitive | Off by default. Notes are short and already distilled. The scrubber fails closed. A separate-repository option. `pending` and `retract` commands. |
| C | Ref and notes clutter in users' clones | Nothing is fetched without our explicit refspec. Git config isn't changed beyond `notes.rewriteRef`. |
| C | Hosts reject or hide custom refs and notes | Y3 is run against the host in use, and the separate-repository option is available. Hosting UIs not showing notes is accepted: we read them, people don't need to. |

**Questions for reviewers**
1. **Phase C criterion** (§4.3). Is "a second developer or machine publishes regularly, and the stats counters are non-trivial" the right trigger? Should a solo user with two machines enter phase C?
2. Should inferred links require the commit author to match the conversation's machine user?
3. Is git notes the right shared home for links, given that hosting UIs don't show notes? The alternative is evidence objects, keyed by commit.
4. Is `evidence.share: none` the right default, or should `notes` be the default for the `team` review profile?
5. Should gap lines ever be pushed (`gaps.push`), or should they stay pull-only?
6. Are the A7 thresholds (2 steps, `success_count >= 2` across 2 conversations, 0 failures since last amendment) too strict or too loose?

---

## 19. Considered and not adopted

| Idea | Why not |
|---|---|
| Hosted mirroring, remote search, a web UI | Server infrastructure is a SPEC non-goal (§1.2). Dossiers are markdown and readable anywhere. |
| Embeddings and hybrid lexical-plus-vector ranking | A v2 non-goal. Revisit only if B2 shows full-text recall is the bottleneck. |
| A code-structure graph and impact analysis | P3: structure is read from the code. A code-graph tool can sit alongside us. |
| Cross-repository ("global") facts | SPEC non-goal (§1.2). Harness-native personal memory covers it. |
| Webhooks on memory changes | No consumer. The memory PR is the change feed. |
| Interactive "link this commit?" prompts during `git commit` | They break agent commits and GUI git clients. Links are derived instead (§12.2). |
| Line-level AI or human authorship tags | Not a memory goal, and fragile under rewrites |
| Working-tree snapshots at every turn | We aren't a rewind tool. Harnesses already provide checkpoints. |
| Starting backfill automatically at setup | It spends quota without consent (§6.3) |
| Writing git notes or refs in phase B, to survive rebases | Patch-id remapping achieves the same read-only (§12.3). Writing waits for a sharing need (phase C). |
| Harness adapters as separate executables found on `PATH` | One binary (SPEC §18.1). Revisit for third-party harnesses after Release 2. |
