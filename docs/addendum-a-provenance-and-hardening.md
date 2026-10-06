# repo-memory Addendum A: Provenance, Sharing and Hardening

**Status:** Draft for review, revision 1
**Date:** 2026-10-06
**Applies to:** SPEC v0.10 together with the [v2 design](design-v2-distillation-retrieval.md), revision 3.

This addendum **adds** to those documents and replaces nothing in them. Where it changes an existing section, §14 lists the change.

**For reviewers:** the questions we most want answered are in §17.

---

## 1. Summary

A review of a comparable product (§2) found several techniques we don't have. The ones worth having are below. Each is designed here from scratch to fit our principles: P5 (truth from outcomes), P11 (never slow the loop), P12 (review gates sharing) and P13 (evidence independence).

| # | Item | What it gives us | Priority |
|---|---|---|---|
| A1 | **Commit links** | Every commit an agent session contributed to is linked to that conversation and its notes, without rewriting commits | High |
| A2 | **Line-level `why`** | `memory.why path:line` follows git blame to the commit, then the conversation, the notes and the entries | High |
| A3 | **Shared evidence (opt-in)** | Evidence IDs cited by entries resolve on every machine, not only the author's | Medium |
| A4 | **Layered scrubber** | Replaces today's four-regex stub. Shared rules can't be weakened locally. Any scanner failure blocks the write. | High |
| A5 | **Spend guards and dry runs** | Cost estimates before any token-spending job. Frequency and volume limits are separate knobs. | High |
| A6 | **Freshness, gaps and short-term recall** | Pull answers say how current memory is and where it has nothing. Unconsolidated notes are searchable at once. | Medium |
| A7 | **Procedures become skills** | Repeatedly verified procedures are rendered as on-demand skills | Low |
| A8 | **Managed instruction blocks** | Safe, reversible edits to `GEMINI.md` and **[R2]** `CLAUDE.md` | High (small) |
| A9 | **External capture sources** | Sessions captured by other tools can be imported as events | Low |
| A10 | **Evaluation discipline** | Paired, pre-registered comparisons for X6 and the B-series benchmarks | High |

Suggested phasing is in §15.

---

## 2. Clean-room provenance

**Input.** On 2026-10-06 we reviewed the public product documentation for Entire (docs.entire.io), including its "Brain" memory plugin's public README and command reference. We used it only to spot problem areas and general techniques. No third-party source code was read.

**Rules this addendum follows:**
- Every name, format, ref namespace, trailer, config key, command and data structure here is our own. None is copied from that product.
- The git mechanisms used are standard, documented git features: notes, custom refs, plumbing commands, `patch-id`, trailers and `notes.rewriteRef`.
- Where an idea matches something that product does, this document says so, and the design here is derived from our own requirements.

**Rules for implementers:**
1. Don't read third-party source code while implementing an A-item.
2. Implement A9 importers only from a tool's **published format documentation**. Record the document and its version in the importer's header comment.
3. Cite this addendum, not the third-party docs, as the design source in code and PRs.

---

## 3. Goals and non-goals

**Goals**
- **AG1 Traceability.** From any line, commit or entry, reach the conversations, notes and outcomes behind it in one call.
- **AG2 Portable evidence.** An entry's evidence can be checked on any machine that has the repository, when the team opts in.
- **AG3 Safe by construction.** Nothing leaves the private store without passing a scrubber that fails closed.
- **AG4 Predictable cost.** Every token-spending job can be estimated first, and is bounded by frequency, volume and quota.
- **AG5 Honest answers.** Pull answers state how fresh memory is and where it is silent.

**Non-goals** (these add to SPEC §1.2 and v2 §3)
- Rewriting the user's commits, or prompting during `git commit`.
- Installing git hooks by default.
- Line-level authorship attribution, such as "this line was written by AI".
- Hosting, mirroring or remote search services.
- Embeddings or vector search. This is unchanged from v2.
- A code-structure graph. P3 says structure is read from the code.

---

## 4. A1: Commit links

### 4.1 Problem

Notes record the branch and head commit at harvest (v2 §5.1), so we know which commit a conversation **started from**. We don't know which commits it **produced**. Without that, `memory.why` can't get from code to the reasoning behind it, and merges can't be attributed to the conversations that did the work.

### 4.2 Link record

```json
{
  "commit": "<full sha>",
  "conversation": "<conversation id>",
  "harness": "antigravity",
  "strength": "observed",
  "steps": [120, 188],
  "notes": ["note-01M4…"],
  "files": ["internal/core/scrub/scrub.go"],
  "branch": "pr-07-scrub",
  "linked_at": "2026-10-06T14:02:11Z"
}
```

`strength` is one of:

| Strength | Meaning | How it's derived |
|---|---|---|
| `observed` | The agent made this commit | The transcript has a commit tool call, and its output names the new commit. The short SHA is resolved with `git rev-parse`. |
| `declared` | A person or tool stated the link | Optional commit trailer (§4.4) |
| `inferred` | A human committed work the agent did | See below |
| `merged-from` | A merge or squash commit that carries linked commits | See §4.5 |

**Inferred links.** The tick reads new commits since its cursor. For each commit with no `observed` or `declared` link, it computes for each conversation:

> overlap = |files in the commit ∩ files the conversation edited| / |files in the commit|

It then links every conversation where all of these hold:
- overlap ≥ 0.5;
- the conversation's last edit to those files came before the commit time, and within `links.window` (default 24 h);
- no later conversation edited the same files before the commit.

Several conversations can link to one commit.

### 4.3 Storage

- **Local:** a `commit_links` table in the index. It can be rebuilt from transcripts and git.
- **Shared:** git notes under `refs/notes/repo-memory`, one JSON line per link. Notes hold IDs and strengths only, never transcript text, and they pass the scrubber (A4).
  - Notes don't change commit SHAs.
  - Notes aren't fetched or pushed unless a refspec asks for them, so other people's clones are unaffected.
- **Sync:** follows the evidence sync setting (§6.4).
  - **Fetch:** into `refs/notes/repo-memory-remote/<remote>`.
  - **Merge:** with `git notes merge -s cat_sort_uniq`, which is a clean union for line-oriented notes.
  - **Push:** the merged ref is pushed.

### 4.4 Optional trailer

The trailer is off by default (`links.trailer: off`). When it's on, `repo-memory init` installs a `prepare-commit-msg` git hook. The hook adds:

```
Repo-Memory-Conversation: <conversation id>
```

It does this only when exactly one conversation edited the staged files within the window. It is **never interactive**: prompts break agent commits and GUI git clients. If our binary isn't on `PATH`, the hook does nothing.

### 4.5 Surviving history rewrites

| Rewrite | Handling |
|---|---|
| Local `rebase`, `commit --amend` | `init` sets `notes.rewriteRef = refs/notes/repo-memory` in the repository's **local** config. Git then copies notes onto rewritten commits. |
| Merge commit | A `merged-from` link on the merge commit lists the linked commits it brings in |
| Squash merge, rebase-merge on a host | The tick already watches branches merging, for lane promotion (v2 §6 step 7). When a branch with linked commits disappears or is merged, the tick compares `git patch-id` of the branch's combined diff with each new commit on the target branch. A match adds a `merged-from` link. |
| Squash merge with conflicts (patch-ids differ) | If `gh` is available and the subject ends in `(#N)`, the PR's commit list is used. Otherwise the links stay on the branch commits and are reported as unresolved in `memory.stats`. |

### 4.6 Interaction with existing design

- **The milestone gate** (v2 §5.1) already detects commits in the transcript. It now records the `observed` link too.
- **Consolidation:** an entry whose evidence includes a note from a conversation linked to a merged commit gets that commit as further evidence. Merging is an outcome under P5. The P13 echo rule still applies: a linked commit only counts if the conversation changed an anchored file and passed a check.

---

## 5. A2: Line-level `why`

### 5.1 Behaviour

`memory.why` (SPEC §7.7) accepts `path:line`, `path:start-end` and `path#symbol`, as well as its current targets. The CLI equivalent is `repo-memory why <target>`.

1. `git blame --porcelain` over the range gives the commits that last touched those lines.
2. `commit_links` gives the conversations behind those commits, together with their strengths.
3. From there it collects:
   - notes from those conversations whose anchors cover the path;
   - entries whose evidence cites those notes;
   - current entries anchored to the path or symbol.
4. For `path#symbol`, `git log -L :<symbol>:<path>` adds the symbol's earlier commits, where git's function matching supports the language.

**Output** is bounded, at 800 tokens by default. Per commit it gives the short SHA, subject, date, link strength, conversation title, note item titles and entry IDs. After that come the current entries for the target, ranked as in v2 §8.3.

### 5.2 Stated limits

Every answer carries these as a single line:
- **Links are per commit, not per line.** A line hand-edited inside an agent commit is still linked to that conversation.
- **Blame follows rewrites.** After a squash, the answer comes through `merged-from` links.
- **Uncommitted lines** are reported as "uncommitted (this working tree)", with the current conversation if it edited them.
- **Quoted notes are historical evidence.** Verify them against the code before acting on them.

### 5.3 Why this matters

Today `memory.why` goes from entry to evidence. A2 adds the opposite direction, from code to reasoning. That's the question people actually ask: "why is this line like this?" Y7 measures it.

---

## 6. A3: Shared evidence (opt-in)

### 6.1 Problem

Entries cite evidence such as `note-01M4…` or `E09`. Notes live in each developer's private evidence store (v2 §7.1), so on a teammate's machine:
- `memory.why` dead-ends at an ID it can't resolve;
- consolidation can't see the other machine's exposure lists, which weakens the P13 echo filter across machines.

### 6.2 What can be shared

`evidence.share` is set in `.memory/config.yaml`. It is a team decision.

| Level | Shared | Default |
|---|---|---|
| `none` | Nothing. This is today's behaviour. | ✅ |
| `notes` | Harvested notes, already scrubbed and linted (v2 §5.1), with their metadata: conversation ID, steps covered, model, harness version, branch, head commit, anchor check results, exposure list | |
| `notes+summaries` | As `notes`, plus platform checkpoint summaries (v2 §5.3), after a second scrub | |

Raw transcripts and episodes are **never** shared, at any level. SPEC §11.1 is unchanged.

**Evidence isn't memory.** Nothing in the evidence refs is ever delivered. Delivery reads only the op log, which still goes through the memory PR. Sharing evidence therefore doesn't bypass P12 for knowledge. It does publish content without review, which is why the default is `none` and the scrubber fails closed.

### 6.3 Storage

There is one ref per object:

```
refs/repo-memory/evidence/<last two chars of ULID>/<ULID>
```

- Each ref points to a parentless commit whose tree holds `note.json`, and `summary.md` at `notes+summaries`.
- Objects are written with plumbing only: `hash-object`, `mktree`, `commit-tree` and `update-ref`. The working tree, the index and HEAD are never touched.
- **Immutable:** an object is never rewritten. A correction is a new object with `supersedes: <ULID>`.
- **Why one ref per object:** no two machines ever write the same ref, so pushes never fail as non-fast-forward. Objects can also be fetched and pushed one at a time.
- **Scale:** a few thousand objects a year is a few thousand refs. Git handles that comfortably with packed refs. Y3 checks it.

### 6.4 Sync

Sync is set per machine, in `<data-dir>/config.yaml`.

| Setting | Values | Default |
|---|---|---|
| `evidence.sync` | `manual` (`repo-memory evidence push` and `pull`) or `tick` (at most hourly) | `manual` |
| `evidence.remote` | A remote name, or a separate repository URL | Must be set explicitly when sharing is turned on |

- **No git config changes.** The tick fetches with an explicit refspec (`'refs/repo-memory/*:refs/repo-memory/*'`) and never adds one to the remote's config. A user's `git fetch` is unaffected.
- **No git hooks** are used for sync.
- **Separate repository.** A URL keeps evidence out of a public code repository.
- **Turning sharing on.** `repo-memory evidence enable --remote <r>` prints exactly what would be pushed and asks for confirmation. `repo-memory evidence pending` lists unpushed objects at any time.
- **Failures** are reported in `memory.stats` and never block other work.

**Retracting.** `repo-memory evidence retract <ULID>` deletes the local ref and pushes the deletion. The command warns that clones which already fetched the object keep their copy.

### 6.5 Reading

- **The index** includes fetched evidence objects.
- **`memory.why`** resolves an evidence ID from the local store first, then the refs.
- **Consolidation** reads exposure lists from shared notes, so the echo filter works across machines.

---

## 7. A4: Layered scrubber

### 7.1 Current state

`internal/core/scrub` is an empty package. Scrubbing is a private function in `internal/core/pipeline/compact/compact.go` (`scrubSecrets`) with four regexes. Two of them are harmful:
- **every UUID** is replaced, and that includes conversation IDs;
- **every hex string of 32 or more characters** is replaced, and that includes full git SHAs.

A1 and A2 depend on both kinds of ID surviving.

### 7.2 Design

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
- op publish (the existing second scan, SPEC §11.3);
- evidence object writes (A3);
- commit-link notes (A1).

**Layers**, applied in order:

| # | Layer | Detects |
|---|---|---|
| 1 | **Structural** | Private-key blocks; credentialed URLs (`scheme://user:pass@host`); database connection strings; `Authorization:` headers; assignments to secret-like names (`*_TOKEN`, `*_SECRET`, `*_KEY`, `PASSWORD`, …); provider key formats with fixed prefixes. The provider list is our own table, maintained in the package. |
| 2 | **Entropy** | Unlabelled tokens of 24 or more characters over base64 or hex alphabets above an entropy threshold. **Allowed through:** SHAs that resolve in this repository (`git cat-file -e`, batched); UUIDs and ULIDs present in the known-ID allow-set (conversations, steps, notes, entries); content from files matched by `scrub.skip_paths` (for example `go.sum` and lockfiles). |
| 3 | **PII** (opt-in) | Email addresses and phone numbers |
| 4 | **Repo rules** | Label-to-regex rules, plus allow patterns, from `.memory/scrub.yaml` |
| 5 | **Blobs** | Inline base64 images and binaries over 1 KiB, replaced by `«blob:image/png 23KiB sha256:ab12…»` |

**Placeholders.** The existing format `«secret:<label>»` is kept. Optionally it gains a short keyed hash, as in `«secret:github-token#3f9a»`. The hash is an HMAC with a per-machine key, so repeats of one secret de-duplicate without revealing anything.

### 7.3 Policy layering

| Layer | File | Can do |
|---|---|---|
| **Shared** | `.memory/scrub.yaml` (reviewed in a PR) | Turn layers on, add rules, add allow patterns |
| **Local** | `<data-dir>/<repo-id>/scrub.local.yaml` | **Add** rules only. It can't disable a layer or a shared rule, and it can't add allow patterns. |

The shared file decides what may enter shared lanes. One developer's local settings therefore can't weaken it.

### 7.4 Failing closed

- **If any layer fails** (a bad regex in repo rules, a timeout, an error from a `git cat-file` batch), the write is refused.
  - **Tick jobs** retry and then dead-letter, as in SPEC §4.3.
  - **Note harvest**, which runs in a hook, writes the raw note to a private `quarantine/` directory instead of the inbox and never blocks the agent. The tick re-scrubs it later.
- **Hooks never wait on the scrubber** beyond the harvest budget: 2 ms for a 600-token note (Y4).
- **`repo-memory doctor scrub`** reports counts by label and boundary, never values.

### 7.5 Test corpora

| Corpus | Purpose |
|---|---|
| `testdata/scrub/planted/` | Secrets of every supported format, planted in realistic transcript steps |
| `testdata/scrub/benign/` | Look-alikes that must survive: SHAs, conversation UUIDs, ULIDs, `go.sum` lines, ESP-IDF hex dumps, MAC addresses, partition tables and flash offsets |

The gates are in Y4.

---

## 8. A5: Spend guards and dry runs

### 8.1 Dry runs

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

### 8.2 Guards

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
    fallback:      {min_interval: 1h,  max_items: 10}   # conversations
    consolidation: {min_interval: 24h, max_items: 40}   # notes
    backfill:      {min_interval: 6h,  max_items: 25}   # conversations
    gardening:     {min_interval: 168h, max_items: 20}  # dossiers
```

### 8.3 Nothing spends without consent

`repo-memory init` never starts a token-spending job. It prints the backfill dry-run estimate and the command to start it. Backfill order follows SPEC §14: configured priority areas first, then the highest-churn areas.

---

## 9. A6: Freshness, gaps and short-term recall

### 9.1 Freshness record

After each run, the tick writes `<data-dir>/<repo-id>/state/freshness.json`:
- the op-log position the index was built from, and whether a rebuild is pending;
- the index's head commit and how many commits it is behind HEAD;
- the size of the inbox and the time of the last consolidation;
- the count of entries marked `suspect` (v2 §8.1).

### 9.2 Effect on delivery

- **Push.** If the index is unreadable or older than the op log, hooks push nothing (degraded mode, SPEC §7.9). That beats pushing from a stale index. The anchor check at delivery still guards each individual entry.
- **Pull.** Every `memory.brief`, `memory.why` and `memory.search` answer starts with one state line, for example:

  > `memory: index at 5ad059e (3 behind HEAD) · 4 notes not yet consolidated · 2 entries suspect`

### 9.3 Gaps

`memory.brief` ends with up to three gap lines. A gap is either:
- a dossier matched for the task whose coverage (SPEC §9.1) is below `gaps.threshold`; or
- a path in the task's scope that no dossier covers.

For example:

> `No memory covers internal/core/scrub. Absence of memory is not evidence; read the code.`

Gaps are **pull-only by default**. `gaps.push: true` adds at most one gap line to the task-start push, only when the task's main scope has no dossier. This keeps to v2 §8.1.6: no overviews in pushes.

### 9.4 Short-term recall

Harvested notes are useful before consolidation runs, which can be a day later.
- **Indexed within a tick.** The tick indexes inbox notes in full-text search, with status `unconsolidated` and their branch scope.
- **Pull only.** `memory.search`, `memory.why` and `memory.brief` return them, labelled. They are **never pushed**.
- **Removed when consolidated,** once ops reference them.

This keeps P11: hooks only spool, and the tick indexes within a minute.

---

## 10. A7: Procedures become skills

### 10.1 When

The weekly gardening job (v2 §6) proposes a skill for a `procedure` entry when **all** of these hold:
- it has 3 or more steps;
- it has 3 or more successful outcomes across 2 or more conversations, where the procedure's own commands passed. Under P13, running the procedure counts as a test, so exposed conversations count when the commands ran;
- it hasn't been superseded or amended in its meaning for 30 days.

### 10.2 How

- **The proposal is an `amend` op** setting `render_as_skill: rm-<slug>` on the entry. It is **always reviewed** in the memory PR, under every review profile.
- **Rendering is deterministic, with no LLM.** It's a view of the entry, like a dossier:
  - **frontmatter:** `name` and `description` (the entry's title plus its `relevant_when` line). Both are required, because Antigravity silently drops files without them (X8);
  - **body:** the steps, why, evidence links, and a footer: "Generated from `[proc-XXXX]` by repo-memory. Edit the entry, not this file."
- **Location** is chosen by harness capability: `.agents/skills/rm-<slug>/SKILL.md` on Antigravity, and **[R2]** `.claude/skills/` on Claude Code.
- **CI lint** (SPEC §11.4) checks that each skill matches what regeneration produces.
- **Lifecycle:** when the entry is superseded or retired, the next memory PR removes the skill.
- **Delivery:** while a skill exists, pushes for that entry shrink to a pointer: "skill `rm-release` covers this".

### 10.3 Caution

Generated always-on instruction files have been measured to hurt (v2 §2.2). Skills load on demand by description, which should be lower risk, but that is unproven. Y6 gates this item.

---

## 11. A8: Managed instruction blocks

This replaces SPEC §4.6 step 2 ("adds the map include line"). `init` writes a delimited block into the harness's instruction file: `GEMINI.md`, and **[R2]** `CLAUDE.md`.

```markdown
<!-- repo-memory:begin v1 — managed by repo-memory; edits inside are overwritten -->
@[memory atlas](.memory/ATLAS.md)
<!-- repo-memory:end -->
```

- **Creating:** if the file doesn't exist, `init` creates it and records that in the registry. Otherwise the block is appended.
- **Updating:** only the text between the markers is ever rewritten. The version tag allows upgrades, such as v2's move from `views/MAP.md` to `ATLAS.md`.
- **Leaving:** `leave` removes the block. It deletes the file only if we created it and it is now empty.
- **Duplicate or unbalanced markers:** `init` refuses, and `doctor` reports them.
- **`AGENTS.md`** is still never touched (SPEC §12.2).

---

## 12. A9: External capture sources

Some users already run another tool that captures agent sessions, for example tools that store transcripts alongside commits in git. Importing those sessions as events extends coverage (SPEC G9) and gives the fallback more input.

```go
// internal/harness — beside Harness, not part of it
type CaptureSource interface {
    Name() string
    Discover(ctx context.Context, repoRoot string) ([]SessionRef, error)
    Read(ctx context.Context, ref SessionRef) (episode.Raw, error) // → normal compaction
}
```

- **Treated as import events** (SPEC §14) with `provenance: capture:<source>`. They are processed by the fallback (v2 §5.4) and the scrubber like everything else.
- **De-duplicated:** if our own capture already holds the same harness conversation ID, our copy wins.
- **Echo safety:** our injection ledger covers any conversation we delivered to, whoever captured it. A session with no ledger is marked `exposure: unknown`. Consolidation treats an item that repeats an existing entry from such a session as an echo (a no-op), which errs on the safe side.
- **Read-only:** importers never write to another tool's storage.
- **Clean-room:** each importer is written from that tool's published format documentation, pinned to a format version, with a fixture test (§2).
- **Existing sources** already fit this interface: harness transcripts, and PA files (`import-pa`).

---

## 13. A10: Evaluation discipline

This applies to X2, X6 and B1–B5.

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

## 14. Changes to existing sections

| Section | Change |
|---|---|
| SPEC §4.6 step 2 | Replaced by managed blocks (A8). `init` also sets `notes.rewriteRef` in local config (A1). |
| SPEC §5.6 and v2 §7.1 storage | Add the items listed below this table |
| SPEC §7.7 pull tools | `memory.why` accepts `path:line`, `path:start-end` and `path#symbol` (A2). All pull answers start with a state line (A6). `memory.brief` ends with gap lines (A6). |
| SPEC §7.9 degraded modes | A stale or unreadable index means no pushes (A6) |
| SPEC §10.3 benchmarks | Paired, pre-registered design and reporting (A10) |
| SPEC §11.1 privacy | Unchanged by default. `evidence.share` is an explicit, team-level exception that never covers transcripts or episodes (A3). |
| SPEC §11.3 secrets | Replaced by the layered scrubber at every write boundary, failing closed (A4) |
| SPEC §11.4 integrity | The lint also checks generated skills against regeneration (A7), and `.memory/scrub.yaml` for valid patterns (A4) |
| SPEC §14 migration | Import sources generalised as `CaptureSource` (A9). Backfill never starts without consent (A5). |
| SPEC §18.4 libraries | Secret scanning is the `scrub` package's own layers. Any third-party library is an optional layer-1 provider behind the same interface. |
| v2 §5.1 harvest | Records `observed` commit links. Scrubs the note, or quarantines it. |
| v2 §5.4 fallback, v2 §6 consolidation | Subject to A5 guards. Linked merged commits count as evidence (A1, under P13). |
| v2 §6 gardening | May propose `render_as_skill` (A7) |

**Storage additions:**

```
.memory/
  scrub.yaml                                  shared scrub rules (A4)
refs/notes/repo-memory                        commit → conversation links (A1)
refs/repo-memory/evidence/<xx>/<ULID>         shared evidence objects (A3, opt-in)
<data-dir>/<repo-id>/
  scrub.local.yaml                            local additions only (A4)
  quarantine/                                 notes that failed scrubbing (A4)
  state/freshness.json                        (A6)
index.sqlite: + commit_links, evidence_objects, inbox FTS (A1, A3, A6)
```

**New `.memory/config.yaml` keys** (shared): `links.trailer`, `links.window`, `evidence.share`, `scrub.*`, `gaps.threshold`, `gaps.push` and `skills.enabled`.

**New machine keys:** `evidence.sync`, `evidence.remote` and `spend.*`.

---

## 15. Phasing

| PR | Items | Why then |
|---|---|---|
| PR-07 (with the v2 foundation) | A4, A8, A5, A10 | The scrubber must exist before any new write path. A10 must be in place before X6 runs. A8 and A5 are small. |
| PR-08 | A1, A2 | Needs the harvest path from PR-07 |
| With v2 delivery | A6 | Changes the hooks' push decision |
| After X6 passes | A3, A7, A9 | Only worth building if memory is shown to help |

---

## 16. Validation

| # | Experiment | Pass criterion |
|---|---|---|
| Y1 | **Link accuracy.** Hand-label the conversation(s) behind 40 commits across this repository and esp32-app-foundation. | `observed` precision ≥ 0.95; `inferred` precision ≥ 0.80; recall reported |
| Y2 | **Rewrite survival.** On a scratch repository: amend, rebase, merge, a clean squash merge and a squash merge with conflicts. | Links survive the first four. The fifth is reported as unresolved. |
| Y3 | **Evidence ref scale.** 10,000 synthetic evidence refs: time to fetch and push, `git gc`, and the effect on a plain `git fetch` from a clone without the refspec. | Fetch and push of 50 new objects ≤ 5 s; plain `git fetch` unaffected |
| Y4 | **Scrubber.** Run the planted and benign corpora (§7.5), plus 200 sampled transcript steps. | 100% recall on planted; ≤ 1% false positives on benign and the sample; ≤ 2 ms per 600-token note |
| Y5 | **Estimator.** Compare dry-run estimates with actual usage over 20 jobs, after calibration. | Within ±30% |
| Y6 | **Skills.** Replay 4 procedure-heavy tasks: entry pushes against the generated skill (A10 method). | The skill arm is no worse on success or steps |
| Y7 | **Line-level `why`.** 20 "why is this line like this?" questions taken from history: `memory.why` against `git log` and blame alone. | `memory.why` is correct on more questions, with a significant paired effect |

---

## 17. Risks and open questions

| Risk | Mitigation |
|---|---|
| Inferred links attach the wrong conversation | Strength is shown on every answer. The overlap and window thresholds are tuned in Y1. `inferred` alone never counts as an outcome under P5. |
| Shared evidence leaks something sensitive | Off by default. Notes are short and already distilled. The scrubber fails closed. A separate-repository option. `pending` and `retract` commands. |
| Ref and notes clutter in users' clones | Nothing is fetched without our explicit refspec. Git config isn't changed beyond `notes.rewriteRef`. |
| The scrubber removes IDs we rely on | The known-ID allow-set and SHA resolution (§7.2), checked by the benign corpus |
| Scrubber false negatives | Layered detectors; shared rules; second scan before publish; the Y4 recall gate |
| Generated skills hurt agents | Always reviewed; on-demand loading; gated by Y6 |
| Importer formats drift | Version-pinned importers with fixture tests. An unknown version is skipped and reported. |

**Questions for reviewers**
1. Is `evidence.share: none` the right default, or should `notes` be the default for the `team` review profile?
2. Should inferred links require the commit author to match the conversation's machine user?
3. Is git notes the right shared home for links, given that hosting UIs don't show notes? The alternative is evidence objects, keyed by commit.
4. Should gap lines ever be pushed (`gaps.push`), or should they stay pull-only?
5. Are the A7 thresholds (3 steps, 3 outcomes, 2 conversations, 30 days) too strict or too loose?

---

## 18. Considered and not adopted

| Idea | Why not |
|---|---|
| Hosted mirroring, remote search, a web UI | Server infrastructure is a SPEC non-goal (§1.2). Dossiers are markdown and readable anywhere. |
| Embeddings and hybrid lexical-plus-vector ranking | A v2 non-goal. Revisit only if B2 shows full-text recall is the bottleneck. |
| A code-structure graph and impact analysis | P3: structure is read from the code. A code-graph tool can sit alongside us. |
| Cross-repository ("global") facts | SPEC non-goal (§1.2). Harness-native personal memory covers it. |
| Webhooks on memory changes | No consumer. The memory PR is the change feed. |
| Interactive "link this commit?" prompts during `git commit` | They break agent commits and GUI git clients. Links are derived instead (§4.2). |
| Line-level AI or human authorship tags | Not a memory goal, and fragile under rewrites |
| Working-tree snapshots at every turn | We aren't a rewind tool. Harnesses already provide checkpoints. |
| Starting backfill automatically at setup | It spends quota without consent (§8.3) |
| Harness adapters as separate executables found on `PATH` | One binary (SPEC §18.1). Revisit for third-party harnesses after Release 2. |
