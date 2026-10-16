# repo-memory — Programme Plan (v2.2)

> **Programme:** repo-memory
> **Specification:** [SPEC.md](../../../../SPEC.md) v1.0 + [v2 Design](../../../../docs/design-v2-distillation-retrieval.md) rev 4 / v2.2 + [Addendum A](../../../../docs/addendum-a-provenance-and-hardening.md) rev 3
> **Release:** 1 (Antigravity)
> **Created:** 2026-10-01 · **Revised:** 2026-10-04 (v2 pivot) · **v2.1:** 2026-10-04 (plan review) · **v2.2:** 2026-10-15 (post-X6-lite & Stage 1 synthesis, session 21)
> **Total PRs:** 22 (19 complete, 1 ready to resume at WO4/7, 2 planned).
> **Base branch:** `pr-13-v2-consolidation/wo6-tick-orchestration-and-verification` (ahead of `pr-12-v2-distillation/wo6-managed-instruction-block-and-verification` `e6ee499`, `pr-11-v2-doc-seeding-and-lifecycles/wo5-dogfood-doc-seed-and-verification` `860610f`, `pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification` `4abdc55`, `pr-10a-v2-2-docs-and-specs/wo5-verification-and-handover` `723593b`, `pr-10-eval-v2/wo3-index-build` `ec96755`, and `main` `83edd25`).
> **Dependency model:**
> - **PR-10a-v2-2-docs-and-specs** upgrades `SPEC.md` (v1.0), `docs/design-v2-distillation-retrieval.md` (Rev 4), `docs/addendum-a-provenance-and-hardening.md` (Rev 3), `docs/antigravity-hooks-reference.md`, and `README.md` first.
> - **PR-10b-delivery-and-storage-fixes** fixes the `8d7ad7b`+`d3ab994` `EphemeralMessage` $\to$ `UserMessage` persistence regression, wires `DetectRewind`, and resolves open storage/seeder issues (`#10`–`#13`, `#16`) **before** resuming `PR-10-eval-v2` at `WO4` (`wo4-dry-run`) and `WO5` (`wo5-pilot`).

---

## Changes in v2.2 (post-X6-lite & Stage 1 synthesis, 2026-10-15)

| # | Change | Why |
|---|---|---|
| 1 | **PR-10a: Upgrade Specs, Design Docs & Reference Docs first** | `SPEC.md` (v0.10) still describes the retired v1 4-stage pipeline and `.memory/claims/`. `design-v2-distillation-retrieval.md` (Rev 3) and `addendum-a-provenance-and-hardening.md` (Rev 2) predate the X6-lite STOP verdict, the Stage 1 77-session audit, and the 6 v2.2 architectural pillars. |
| 2 | **PR-10b: Fix `8d7ad7b`+`d3ab994` `EphemeralMessage` $\to$ `UserMessage` regression + Open Issues `#10`–`#13`, `#16` before `PR-10` Stage 2** | Commit `8d7ad7b` switched `task-start`, `first-touch`, `first-entry`, and `re-injection` to `EphemeralMessage` while `d3ab994` added per-session `InjectedSet` deduplication, causing injected memories to vanish after 1 tool call. `DetectRewind` is also unwired in `preinvocation.go`. Fixing this before `PR-10` WO4/WO5 is required for a valid Stage 2 evaluation. |
| 3 | **Doc-Seeded Dossiers (`repo-memory seed-docs`) & Async Doc Freshness ("Dreaming") (`VD16`, `PR-11`)** | The 77-session `esp32` audit proved agents read static repo skills/guides only 1–8% of the time (`framework-expert` 5%, `creating-a-module` 2%). `DocSeedSource` distills curated docs (`docs/developer/`, `.agents/skills/`) into anchored dossier entries pushed on `first-touch`, with async git-diff re-distillation in `tick`. |
| 4 | **Typed Cognitive Memory Lifecycles (`semantic`, `procedural`, `episodic`) & `{section?}` Rendering (`VD17`, `PR-11`)** | Replaces uniform entry handling with type-driven invalidation, delivery, and skill graduation, and omits empty markdown sections (`{section?}`) until populated. |
| 5 | **Two-Stage 5-Minute Warm-Cache Distillation (`VD14`, `VD15`, `PR-12`, `PR-13`)** | Stage 1 Primary is 100% automatic end-of-turn `Stop` (`NO_TOOL_CALL`, `executionNum == 0`) continuation on milestone turns + free `CHECKPOINT`/`brain/` harvest (preserving `G1`/`P1` zero cooperation), with `memory_note` as an optional mid-turn override that deduplicates against `Stop`. Stage 2 uses a multi-turn `stream-json` `repo-memory-distiller` session per tick to share the warm prefix cache. |
| 6 | **State Scope Hierarchy (`temp:`/`session:`/`user:`/`branch:`/`repo:`) & Pre-Consolidation Corroboration Gate (`VD18`, `PR-13`)** | Isolates workstation-specific facts in gitignored `user-dossiers/`, keeps unmerged branch anchors in `.memory/local/`, and requires a corroborating tool/test pass, verified `HEAD` anchor, or human confirmation before promoting hypotheses to active `do:` rules. |
| 7 | **Remove stale v1 `E02` `GEMINI.md` / `.memory/views/` stub (`VD19`, `PR-10a`)** | Root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) is a leftover v1 `E02` test fixture; per Addendum A8, `GEMINI.md` is strictly machine-managed by `repo-memory init` / `leave`. |

---

## Changes in v2.1 (plan review, 2026-10-04)

| # | Change | Why |
|---|---|---|
| 1 | v2 is built on `main`, not PR-04's tip (PD18) | PR-05a is already merged to `main`, with later fixes on top (fail-open `PreToolUse` `3c02240`, FTS `fdb1f69`). The PR-05b branch holds the executor fixes and all v2 documents. Branching from PR-04 would lose all of these, along with the reusable executor, job chaining and local publish. |
| 2 | New **PR-05-v2-foundation** (PD19) | Fixes defects in infrastructure the plan treated as unchanged: the capture→episode break, hooks disabled in the live install, empty cost telemetry, 1.5 GB of probe logs, and the 57 ms non-enrolled hook path. Adds artifact capture, because Antigravity now prunes to about 500 conversations. |
| 3 | The spike is a prototype spike (PD20) | X1, X5, X8 and X9 need prototype code in live hooks. X2 moves to PR-10, where real work orders exist. The X3 method is corrected. X8 adds agent discovery from the runtime working directory. |
| 4 | Read side first, with an early kill gate (PD21, PD22) | Storage → delivery → **X6-lite** using time-cut seeded dossiers, then distillation and consolidation. Whether delivered dossiers help is tested after 3 implementation PRs instead of 5. |
| 5 | X6 protocol fixed (PD23) | Time-cut dossiers and PA docs, so there is no leakage. A pre-registered metric, seeds, work-order selection and quota budget. |
| 6 | Hook latency exit criteria (PD24) | Delivery adds work on every `PreInvocation` |
| 7 | Turn-note safety (PD25) | An off switch, defined milestone signals, the interaction with PA sessions, and opt-out by role |
| 8 | v1 events store retired explicitly (PD26) | `.memory/events/` is replaced by `.memory/ops/` |

---

## Repositories in Scope

| Repository | Owns | Programme work there |
|---|---|---|
| **repo-memory-plugin** (this repo) | SPEC, the `repo-memory` Go binary, Antigravity plugin packaging, CI and releases | All PRs. Also enrolled as the **dogfood** repository from PR-02b. |
| **esp32-app-foundation** (pilot) | Its `.memory/` store, `GEMINI.md` map include, PA workspace | Phase 2 capture enrolment after PR-05-v2-foundation validates capture (PD17). Full enrolment at PR-12-v2-pilot. PA workspace migration at PR-14-v2-pa-integration. Each is a PR in that repo. |
| **mm-skills/principal-architect** | The PA skill | The PA change request at PR-14-v2-pa-integration, raised through that repo's own process |

---

## Dependency Graph

```
Phase 1: Infrastructure (COMPLETE)
PR-01 → PR-01b → PR-02a → PR-02b → PR-03 → PR-04                    ✅

Phase 2: v1 Pipeline
PR-05a  v1 Pipeline Code          ✅ merged to main (stages removed in PR-07-v2)
PR-05b  Dogfood Pilot             ✅ complete; branch closed onto main in PR-05-v2 wo1
  │
  ▼  (main)
Phase 3: v2 Distillation & Retrieval
PR-05-v2   Foundation (close-out, defect fixes, artifact capture, telemetry)
  │
  ▼
PR-06-v2   Prototype Spike (X1, X3, X5, X8, X9)
  │
  ▼  ═══════ X-gate ═══════
PR-06b-v2  Daemon + Workspace Bundle Architecture
  │
  ▼
PR-07-v2   Storage (op log, dossiers, index, v1 removal, seeding)
  │
  ▼
PR-08-v2   Delivery (channels, ledger, re-injection, pull tools)
  │
  ▼
PR-09-v2   X6-lite: B1 replay + 231-session PA retrieval study        ✅ (STOP on spec-style WOs → pivoted to Eval v2)
  │
  ▼
PR-09b     Stage 0 Product Fixes (IndexPath, bm25, dedup)              ✅
  │
  ▼
PR-10-eval-v2 (WO1..WO3: Stage 0b templates/seeding + Stage 1 audit)   ✅ (Paused at WO3/7)
  │
  ▼
PR-10a     v2.2 Specs, Design (Rev 4), Addendum A (Rev 3) & Info Docs  ✅ COMPLETE (PR #17)
  │
  ▼
PR-10b     Delivery Persistence (UserMessage, Rewind) & Issues #10-#16 ✅ COMPLETE (PR #18)
  ├───► PR-10-eval-v2 (Resume WO4..WO7: Dry-Run, Stage 2a/2b, Report)  🔄 READY NEXT
  └───► PR-11-v2      Doc-Seeded Dossiers, Lifecycles & {section?}     ✅ COMPLETE (PR #19)
          │
          ▼  (joins PR-10 + PR-11)
        PR-12-v2      Two-Stage Warm-Cache Distillation                ✅ COMPLETE (PR #20)
          │
          ▼
        PR-13-v2      Warm Batch Consolidation & Scope Hierarchy       ✅ COMPLETE (PR #21)
          │
          ▼
        PR-14-v2      End-to-End Distillation & Consolidation Eval     🔄 READY NEXT
          │
          ▼
        PR-15-v2      Procedural Validation, Skill Graduation & Learn  ☐
          │
          ▼
        PR-16-v2      PA Integration & Full Repository Rollout         ☐
```

---

## PR-10a-v2-2-docs-and-specs: v2.2 Specifications, Design & Reference Docs

**Base:** `pr-10-eval-v2/wo3-index-build` (`07fda90`).
**Objective:** Align all canonical specification, design, reference, and programme documents with the `v2.2` (Revision 4) architecture and empirical findings before mutating Go code in `PR-10b`.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-design-v2-rev4` | Upgrade `docs/design-v2-distillation-retrieval.md` from Revision 3 to **Revision 4 (`v2.2`)**: empirical grounding from `PR-09` & `PR-10` Stage 1; 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`) & `{section?}` optional section rendering; Two-Stage 5-minute warm-cache distillation (automatic milestone-gated `Stop` `NO_TOOL_CALL` continuation primary + free `CHECKPOINT`/`brain/` harvest + optional `memory_note` mid-turn dedup + multi-turn `stream-json` consolidation); 5-tier state scope hierarchy & Pre-Consolidation Corroboration Gate; Doc-Seeded Dossiers (`repo-memory seed-docs`) & Async Doc Freshness ("Dreaming"); persistent `userMessage` vs transient `ephemeralMessage` delivery contract & `DetectRewind` wiring. | architect |
| 2 | `wo2-addendum-a-rev3` | Upgrade `docs/addendum-a-provenance-and-hardening.md` from Revision 2 to **Revision 3**: A7 `procedural` execution-outcome validation (`PostToolUse` exit-code tracking, `success_count`/`failure_count`, deterministic `.agents/skills/rm-<slug>/SKILL.md` graduation); A8 managed instruction block refinement; A9 `DocSeedSource` (`source: doc-seed` + git-diff freshness). | standard |
| 3 | `wo3-spec-v1-0` | Upgrade `SPEC.md` from `Draft v0.10` to **`v1.0`**, retiring v1 4-stage pipeline & `.memory/claims/` sections, incorporating all items from `docs/experiments/SPEC-CHANGES.md` and `design-v2-distillation-retrieval.md` Rev 4 while preserving `G1`/`P1` zero-cooperation capture and delivery. | architect |
| 4 | `wo4-info-docs-and-cleanup` | Update `docs/antigravity-hooks-reference.md` with verified `agy 1.3.3` `userMessage` vs `ephemeralMessage` lifetime, `Stop` `NO_TOOL_CALL`, `invoke_subagent` hook ordering, and plugin split; update `README.md` to match `SPEC.md v1.0`; remove the stale v1 `E02` root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` artifacts. | standard |
| 5 | `wo5-verification-and-handover` | Cross-check all internal markdown links, section references, and external GitHub hygiene across `SPEC.md`, `docs/`, and `.agents/skills/principal-architect-workspace/`; finalize `PR-10a` Session Log. | mechanical |

### Exit Criteria
- `docs/design-v2-distillation-retrieval.md` is at Revision 4 (`v2.2`), `docs/addendum-a-provenance-and-hardening.md` is at Revision 3, and `SPEC.md` is at `v1.0` with zero contradictions between them.
- `docs/experiments/SPEC-CHANGES.md` items are marked applied in `SPEC.md v1.0`.
- Stale v1 `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/` are removed.

---

## PR-10b-delivery-and-storage-fixes: Delivery Persistence & Storage/Seeder Fixes

**Base:** `PR-10a-v2-2-docs-and-specs`.
**Objective:** Fix the `8d7ad7b`+`d3ab994` "One-Invocation Vanishing Memory" bug, wire `DetectRewind`, and resolve open storage/seeder defects (`#10`–`#13`, `#16`) before resuming `PR-10-eval-v2` Stage 2 (`wo4-dry-run` / `wo5-pilot`).

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-persistent-usermessage-and-rewind` | In `internal/harness/antigravity/preinvocation.go` and `compaction.go`: switch `MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, and `BuildReinjection` from `EphemeralMessage` to `UserMessage` (keeping `EphemeralMessage` strictly for 1-step `about-to-edit`, `error-match`, and search warnings). Wire `DetectRewind(steps, ledger.LastKnownStepCount)` into `HandlePreInvocation` before moment evaluation. Add regression tests verifying persistent vs ephemeral channel assignment and rewind re-arming. | standard |
| 2 | `wo2-index-and-projector-fixes` | Fix GitHub Issue **#13** (`internal/core/index/entries.go`: delete existing child rows in `entries_fts`, `entry_anchors`, and `error_patterns` before `INSERT OR REPLACE` on `entries`) and Issue **#11** (`internal/core/dossiers`: deduplicate entries by `ID` across scopes in the projector when an entry is re-added or moved to a new scope). | standard |
| 3 | `wo3-seeder-fixes` | Fix GitHub Issue **#10** (`internal/core/seed/convert.go`: validate extracted backtick anchor candidates as real repository file paths / symbols rather than arbitrary code tokens) and Issue **#12** (`internal/core/seed`: stamp `--before` seeded ops with the workorder/cutoff commit's timestamp and SHA rather than `HEAD`). | standard |
| 4 | `wo4-ops-cleanup-and-rebuild` | Fix GitHub Issue **#16** (clean junk placeholder ops from `.memory/ops/`, re-render `.memory/dossiers/`), rebuild `~/.local/bin/repo-memory`, and re-seed/rebuild the `esp32-app-foundation` evaluation template index (`tpl-c`, `tpl-d`) so `PR-10` WO4/WO5 runs against clean anchors and persistent `UserMessage` delivery. | standard |
| 5 | `wo5-dry-run-verification` | Run a live multi-invocation verification confirming that `task-start`, `first-touch`, and `first-entry` injections emitted as `UserMessage` remain in the conversation context across subsequent tool calls while `InjectedSet` suppresses duplicates. | standard |

### v2 Experiments and the Work They Inform

| Experiment | Runs in | Informs |
|---|---|---|
| X1 `Stop` continue, gate, harvest, PA-session interaction | PR-06 | PR-10 turn notes |
| X3 Quota accounting | PR-06 | Gate thresholds; fallback and consolidation budgets |
| X5 Delivery channels (`systemMessage`/`userMessage` persistence, checkpoints) | PR-06; confirmed in PR-08 | PR-08 delivery |
| X8 Lean agent (overhead, discovery, hook recursion, `--json-schema`) | PR-06 | PR-07 seeding, PR-10 fallback, PR-11 consolidation |
| X9 Hook exit codes | PR-06 | All hook handlers |
| X11 Compaction re-injection | PR-08 | PR-08 delivery |
| **X6-lite** B1 replay with seeded dossiers | PR-09 | **Early kill gate** |
| X2 Note quality vs PA reports | PR-10 | Request wording |
| X10 Staying on task | PR-10, PR-11 | Preparation layer |
| X4 Consolidation quality | PR-11 | Consolidation |
| X6 B1 replay with distilled dossiers | PR-12 | Confirmatory gate |
| X7 Knowledge Items (optional) | PR-08 | Optional channel |

### X6 Protocol (shared by X6-lite and X6), PD23

The protocol is written down and approved before any run.

1. **Work orders.**
   - 6–8 completed work orders that have executable exit tests.
   - Chosen where earlier knowledge plausibly matters: they depend on decisions, dead ends or external behaviour found in earlier work orders.
   - Include at least two where a no-memory run is expected to struggle.
2. **Time cut.** For each work order, dossiers are built only from sessions, PA reports and commits **before that work order's parent commit**. PA documents are also taken as of the parent commit. Nothing from the target work order's own sessions, or anything later, may appear in any condition.
3. **Conditions:**
   - (a) no memory;
   - (b) PA documents as of the parent commit;
   - (c) time-cut dossiers delivered through the PR-08 mechanism.
4. **Seeds.** At least 2 per work order and condition. A third is added where results disagree.
5. **Metrics.**
   - Primary: the share of exit tests passing.
   - Secondary: steps and total tokens.
   - The pass threshold is fixed in advance: (c) beats (a) on the primary metric, aggregated over work orders. "Match PA at fewer tokens" is reported, not gating, for X6-lite.
6. **Budget.** Estimate the quota per run, and schedule runs across the 5-hour and weekly pools. The user approves the budget.
7. **Snapshot.** Copy the candidate work orders' transcripts and PA files into the evidence store immediately, before pruning removes them (PD17).

---

## Completed PRs (Phase 1 + Phase 2)

PRs 01 through 05b are complete. Their PROJECT.md files and session logs are preserved in the PA workspace.
- PRs 01–05a are on `main`.
- PR-05b's branch (`pr-05b-dogfood-pilot/wo4-b5-evaluation`) is closed onto `main` by PR-05-v2-foundation wo1.

---

## PR-05-v2-foundation: Close-out, Defect Fixes & Capture

**Base:** `main`.
**v2 Design Sections:** §2.3, §5.3, §9; SPEC §4.6, §6.2.

**Objective:** Put the retained infrastructure into a known-good state before any v2 work. Close PR-05b onto `main`, fix the defects found in the plan review, and start capturing what v2 needs before pruning removes it.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-close-pr05b` | Merge the PR-05b branch's commits to `main`: the executor stdin fix (`4163dd5`), the robust JSON extraction and 429/exit-code-3 handling (`960d852`), and all docs (dogfood evaluation with Appendix C, the v2 design rev 1–3). Mark PR-05b complete in its PROJECT.md. Archive `.memory/claims/` to `.memory/archive/v1-claims/`. No other code changes. | mechanical |
| 2 | `wo2-capture-to-evidence` | Fix the broken link between capture and episodes. `convertCapturesToTriage` (`internal/core/tick/pipeline.go`) queues triage by conversation ID, but nothing parses or compacts the captured transcript into the episode store: 124 captured, 0 processed. Build episodes, or evidence records, from captured transcripts incrementally. Parse `CHECKPOINT` steps into structured sections (User Knowledge, Model Knowledge, Work Accomplished, …). Backfill the 124 already captured. Remove the v1 capture→triage hand-off. | standard |
| 3 | `wo3-artifact-capture` | Copy conversation artifacts (`brain/<id>/*.md` and `.metadata.json`) into `<data-dir>/<repo-id>/evidence/artifacts/`, at `Stop` and during the sweep, for enrolled repos. Deterministic. Add a `snapshot` command that copies the named conversations' transcripts and artifacts now, for X6 candidates (PD17). | standard |
| 4 | `wo4-hooks-restore` | The installed `~/.gemini/config/plugins/repo-memory/hooks.json` has been `{}` since 2026-10-02 22:45, so no hooks run and capture relies on the sweep. Record why it was emptied. Make `install`/`upgrade` write the hook set safely, with fail-open behaviour verified (empty stdout or `{"decision":"allow"}`; never `{}`). Add the non-enrolled fast path (E05: 57 ms → ≤ 5 ms via a marker or registry cache). Delete the 1.5 GB of probe logs and add rotation for logs and probes. | standard |
| 5 | `wo5-cost-telemetry` | Executor results are stored with `Cost{}`. Record `usage` from `agy` JSON output (input, output, thinking, cache read) on every job. Add a read-only `gen_metadata` reader, marked experimental and undocumented, for the uncached, cached and output tokens of IDE turns. X1 and X3 need it. Add a `/usage` reader (`agy -p "/usage" --output-format json`, zero tokens) as a shared quota-guard library. | standard |

### Exit Criteria
- `main` contains PR-05a, its two follow-up fixes, the PR-05b fixes and all v2 docs. The PR-05b branch is closed.
- The 124 captured transcripts are readable as episodes or evidence; checkpoints are parsed; new captures flow through.
- Artifacts are captured for enrolled repos; `snapshot` works.
- Hooks are installed and firing in the dogfood repo; `PreToolUse` fails open; non-enrolled hook p95 ≤ 5 ms; probe logs are removed and rotation is in place.
- Job costs are recorded; the `gen_metadata` reader returns per-turn usage; the quota-guard library works.

---

## PR-06-v2-spike: Prototype Spike

**Base:** PR-05-v2-foundation.
**v2 Design Sections:** §2.3, §5.1, §5.4, §8.2–8.4, §9, §12.

**Objective:** Validate the platform mechanics v2 relies on.
- **Prototype code** (the `Stop` gate and harvest, `systemMessage` injection, the lean agent) lives on a spike branch behind a feature flag (`REPO_MEMORY_V2_SPIKE=1`), installed locally, with a documented rollback.
- **What merges:** only the experiment reports in `docs/experiments/` and the proposed SPEC edits. Production versions are built in PR-07 onward.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-stop-continue` | **X1.** A prototype `Stop` gate: `terminationReason` equals `NO_TOOL_CALL` (case-insensitive), `fullyIdle`, `executionNum == 0`; continue with the draft request; harvest at `executionNum >= 1`. Measure per note from `gen_metadata`: uncached, cached and output tokens. Also measure user visibility; whether a user message can arrive mid-continuation; and `max_stop_hook_continuations`. **Include PA and worker sessions:** check that a continuation at a PA milestone or worker completion doesn't disrupt the PA loop or completion reports. Sample: ≥ 20 requests. | interactive |
| 2 | `wo2-hook-contracts` | **X9.** For every hook type, test exit 1, exit 2, empty output and `{}`. Document fail-open versus fail-closed per type, and update the hook handlers' contract tests to match. | standard |
| 3 | `wo3-channels` | **X5.** Do `systemMessage` and `userMessage` persist across invocations and turns? Do they keep the cache? Do they survive a `CHECKPOINT`? Produce a checkpoint deliberately: a long synthetic session, or a scripted large-context run, described in the report. Compare a first-touch push plus an edit pointer against deny-once on edit: what the agent does next, and how many extra steps. Cross-reference issue #2 Q44–Q49. | interactive |
| 4 | `wo4-lean-agent` | **X8.** Build the `repo-memory-distiller` Markdown agent (`excludeDefaultComponents: true`, `tools: []`, `inheritCustomizations: false`, `inheritMcp: false`, `hidden: true`, `mainAgent: true`). Find where it must be installed to resolve from the **runtime working directory `<data-dir>/runs/<repo-id>/`** (user-level `~/.gemini/config/agents/`, or inside the plugin if plugins can ship agents). Measure input tokens, single-shot and over `stream-json`. Check whether our plugin hooks fire inside it, and whether `--json-schema` applies. | interactive |
| 5 | `wo5-quota-and-reconciliation` | **X3.** `/usage` reports `remaining_fraction` (about 0.1% resolution) only for the cap nearest to being reached (`window` is null). Run on an otherwise idle pool, in batches large enough to move the reading, at least twice. Compare turn notes (cached context) with lean-agent calls. Then summarise X1, X3, X5, X8 and X9. List every v2 design section confirmed or changed, and propose SPEC v1.0 edits for user approval before PR-07. | standard |

### Exit Criteria (X-gate)
- **X1:** harvest succeeds on ≥ 95% of ≥ 20 requests; median uncached input ≤ 10k per note; no interleaving; no disruption to PA or worker sessions, or an opt-out by role defined.
- **X9:** behaviour documented per hook type; contract tests updated.
- **X5:** persistence across invocations, turns and checkpoints confirmed, or the delivery design revised.
- **X8:** overhead measured; agent discoverable from the runtime working directory; no hook recursion; output-format strategy chosen.
- **X3:** the cost of cached tokens against quota understood well enough to set the gate and budgets.
- SPEC v1.0 edits proposed and approved.

---

## PR-06b-v2-plugin-split: Daemon + Workspace Bundle Architecture

**Base:** `main`. **Interstitial** between PR-06 and PR-07.
**v2 Design Sections:** §2.3 (Mechanics — plugin loading)

**Objective:** Split the single global `repo-memory` plugin into a daemon (sidecar only) and workspace bundle (hooks/MCP/rules/skills loaded per-repo via `.agents/plugins.json`). Eliminates hook overhead in non-enrolled repos. Renames `init`→`enroll`, `leave`→`unenroll`.

Motivated by X1 finding (plugin enablement bug) and X9 finding (bimodal hook contracts). Platform assumptions validated by Phase 0 experiment.

---


## PR-07-v2-storage: Op Log, Dossiers, Index, v1 Removal & Seeding

**v2 Design Sections:** §5.5, §7.1–7.5, §6 (splits/aliases)

**Objective:** Build the storage layer and a way to produce dossiers **without** turn notes or consolidation. That way the read side can be tested (X6-lite) before distillation is built.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-op-log` | `internal/core/ops/`: op types (`add`, `amend`, `supersede`, `retire`, `move`, `split`), op file schema (one JSON file per batch, named by ULID), and the layout `.memory/ops/<yyyy-mm>/<ulid>.json`. Writer, reader and iterator; validation rules per op type. **Retire the v1 events store** (`.memory/events/`, PR-04 wo7). Migrate anything worth keeping into ops; otherwise archive it (PD26). | standard |
| 2 | `wo2-dossier-schema-renderer` | `internal/core/dossiers/`: dossier kinds; the entry schema (ID, the eight types, required fields per type, anchors, evidence links, verified stamp, `closes_when`); required sections (Purpose and boundaries; Invariants; Ruled out, failure signatures and gotchas) plus optional sections omitted when empty; a cap of about 3k tokens. Render dossiers and `ATLAS.md` from the op log, fully or incrementally; superseded and retired entries go to `.history.md`. Detect hand edits and convert them to ops. | architect |
| 3 | `wo3-scopes-aliases` | `internal/core/config/scopes.go`: declared scopes (topics, external systems, procedures) in `.memory/config.yaml`; derived module scopes (package directory rolled up to a parent until there are enough entries, using the entity extractor); aliases from `split` ops; alias resolution for `memory.brief`. | standard |
| 4 | `wo4-index-migration` | `internal/core/index/`: replace the claims, anchors and relations tables with tables for entries, anchors (with fingerprints), error patterns (command prefix, pattern, dossier scope), aliases and history. Add FTS5 over entries, synopses and checkpoint summaries. Rebuild from the op log via pointer-file swap; update `repo-memory reindex`. | standard |
| 5 | `wo5-v1-removal` | Remove the v1 stages (`internal/core/pipeline/triage`, `extract`, `reconcile`, `verify`), the views generator and their tick wiring. **Keep** the headless executor, job chaining, local-publish worktree and 429 handling from PR-05a/05b, generalised for the lean agent. No broken imports; tests green. | standard |
| 6 | `wo6-seeding` | A path to dossiers without turn notes. (a) A deterministic PA completion-report parser ("Decisions Made", "Dead Ends & Warnings" → notes, `source: pa`). (b) A one-off lean-agent seeding pass over evidence (checkpoint summaries, artifacts, compacted transcripts) that emits ops directly, with output validated in Go. (c) A **`--before <commit>` time cut** that excludes all evidence at or after a commit, as X6 requires. Seeded entries are tagged `source: seed` and reviewed by a human before use. | architect |

### Exit Criteria
- Op files round-trip: write → read → render → re-render gives identical dossiers.
- History files, the atlas and cap linting work; hand edits become ops.
- The index rebuilds from the op log; anchor lookups and FTS work; `reindex` works.
- Scope derivation gives sensible module assignments on the dogfood repo.
- v1 stages removed and tests green; the executor and publish code are reused.
- Time-cut seeded dossiers produced for the dogfood repo and reviewed by the user.

---

## PR-08-v2-delivery: Channels, Moments & Injection Ledger

**v2 Design Sections:** §8.1–8.6

**Objective:** Deliver dossier entries precisely and keep them in context:
- persistent `systemMessage` for knowledge that lasts;
- `ephemeralMessage` for hints at the moment of need;
- deny-once gates;
- the injection ledger, with compaction and rewind re-injection;
- the pull tools.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-ledger-engine` | `internal/core/delivery/`. **Ledger:** entry IDs, channel, step index, content hash and tags (`[repo-memory <id>]`), stored with the conversation's mark state. **Engine:** a fast anchor check (failures marked `suspect` and not pushed); ranking (exact anchor > directory; invariant > failure-signature > ruled-out > rest; verified > unverified; most evidence); budget enforcement per channel. The exposure list (entries pushed or pulled) is recorded here, so the echo filter has data from day one. | architect |
| 2 | `wo2-persistent-pushes` | `PreInvocation` as `systemMessage`, or as `userMessage` if X5 says so. **Task start** (`initialNumSteps <= 1`): relevant dossiers from prompt terms and branch diff paths, ≤ 500 tokens. **First entry** into a dossier: module-wide invariants, ≤ 150 tokens, once per dossier per conversation. **First touch** of an anchor: file- or symbol-specific entries, ≤ 150 tokens, once per anchor per conversation. Branch diff paths are precomputed in the tick, not computed in the hook. A one-line correction is posted when the agent's own edit breaks an injected entry's anchor. | architect |
| 3 | `wo3-ephemeral-and-errors` | **About-to-edit pointer**, ≤ 50 tokens. **Error matching:** on `PostToolUse` for a failed command, read the output from the transcript's `GENERIC` step at `stepIdx`. A match needs the error pattern **and** either the command prefix or an active dossier scope. Deliver the fix as `ephemeralMessage`, ≤ 200 tokens. | standard |
| 4 | `wo4-gates` | `PreToolUse` deny-once for entries marked `gate: edit` ("[REPO-MEMORY `<id>`] why / do. Re-issue the edit after checking this."), acknowledged per conversation and entry. `force_ask` for invariants that need a human. Risky-action gates (SPEC §7.5) carried forward. Follows the X9 contract. | standard |
| 5 | `wo5-compaction-reinjection` | On every `PreInvocation`, read the transcript from the last offset. Detect compaction (a `CHECKPOINT` after an injection's step) and rewind (transcript shorter, or the step's timestamp changed). Re-inject one consolidated `systemMessage` of still-relevant entries, ≤ 1k tokens; skip tags that survive in the checkpoint text. Re-arm first-entry and first-touch dedupe. **X11.** | architect |
| 6 | `wo6-pull-tools` | MCP server tools: `memory.brief` (dossier-based, resolving aliases), `memory.why` (entry with short evidence spans), `memory.history`, `memory.search` (FTS over entries), `memory.note` (to the inbox), `memory.stats`. Pulls are recorded in the exposure list. | standard |

### Exit Criteria
- X5 confirmed in production code; **X11:** detection on ≥ 95% of checkpoints, with re-injected entries present afterwards.
- All moments deliver correctly, budgets are respected, ranking is correct, and suspect entries are not pushed.
- Gates deny once, then allow the retry.
- **Hook latency (PD24):** with all delivery features on, `PreInvocation`/`PreToolUse`/`PostToolUse` p95 ≤ 60 ms when nothing is injected and ≤ 150 ms when injecting, measured on the dogfood repo.
- All MCP tools work against dossiers; exposure lists are populated from pushes and pulls.

---

## PR-09-v2-x6-lite: Early Kill Gate

**v2 Design Sections:** §12 (X6)

**Objective:** Answer the project's central question before building distillation and consolidation: **do delivered dossiers make agents better on this repository's work?** It uses time-cut dossiers from PR-07 seeding, delivered by PR-08, under the X6 protocol above.

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-protocol` | Write the pre-registered protocol: work-order selection with reasons, time cuts, conditions, seeds, primary and secondary metrics, threshold, and quota budget and schedule. The user approves it before any run. | interactive |
| 2 | `wo2-dossier-build` | For each selected work order, build time-cut dossiers with `--before <parent commit>`, and time-cut PA documents. The user reviews the dossiers for leakage and accuracy. | standard |
| 3 | `wo3-replay-harness` | A harness that checks out the parent commit, runs a worker on the work order's text under each condition, and collects test results, steps and tokens. Reuse the PR-03 eval harness. | standard |
| 4 | `wo4-runs` | Execute the runs to the schedule and budget. | interactive |
| 5 | `wo5-report-decision` | Report the results against the pre-registered threshold. Recommend continue, redirect (for example, delivery-only with PA-sourced dossiers) or stop. The user decides. | standard |

### Exit Criteria
- **Early kill gate:** time-cut dossiers beat no-memory on the primary metric. If not, the project stops or is redirected by user decision.
- Results, including the PA comparison and token costs, recorded in `docs/experiments/X6-lite.md`.

---

## PR-10-v2-distillation: Turn Notes, Fallback & Preparation

**v2 Design Sections:** §5.1–5.6

**Objective:** Produce knowledge continuously:
- turn notes at milestones;
- marks;
- the subagent registry;
- the preparation layer;
- the lean-agent fallback;
- note quality validated on real work (X2).

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-stop-gate-harvest` | The production `Stop` gate and harvest, per v2 §5.1 and the X1/X9 findings. **Milestone signals defined:** commit (`git commit` succeeded); PR opened (`gh pr create` succeeded); work order complete (PA completion report written); error resolved (the same command prefix failed, then passed, after ≥ 15 steps); ≥ 2 files edited followed by a build or test. Cooldown 30 minutes, waived for commits and PRs; daily cap 20 per repository. **Settings:** `turn_notes: on \| fallback-only \| off`, and opt-out by role (PA and worker sessions, per X1). The "autonomous goal" condition applies only if X1 found a detectable signal; otherwise it is dropped. Loop protection as in the design. | architect |
| 2 | `wo2-marks` | `internal/core/state/`: per-conversation marks (step index, byte offset, timestamp), clamped on rewind, unmoved by checkpoints. The exposure list from PR-08 is attached to each note at harvest. Previous note titles are listed in the request after a checkpoint. | standard |
| 3 | `wo3-subagent-registry` | A SQLite table mapping child conversation → parent, populated from `PostToolUse` on `invoke_subagent` (`toolCall` is present). Used to exclude subagents from turn notes and to link them for the fallback. | standard |
| 4 | `wo4-preparation-layer` | `internal/core/pipeline/prepare/`: the §5.6 defences. Strip file reads, our own injections, `Stop` reasons, user-input wrappers and subagent prompts; reframe checkpoint headers; produce JSON records in delimited containers with framing; validate output (citations to agent, tool or human steps; rejection of copied instructions; random-token echo; detection of task-doing). **X10** test set of instruction-heavy sessions. | architect |
| 5 | `wo5-sidecar-fallback` | For conversations idle ≥ 2 hours with uncovered milestones: quota guard (PR-05 library); the lean agent from its X8-verified install location; input = latest checkpoint + tail since the later of checkpoint and mark + artifacts, prepared by wo4; batched prompts or `stream-json`. Notes tagged `source: sidecar`. Reuse the PR-07 seeding prompt where applicable. | architect |
| 6 | `wo6-note-quality` | **X2.** Run turn notes on this PR's own work orders and the next PR's, alongside the PA. Compare notes with the matching completion-report sections, blind. Tune the request wording; record model, length and types; cover Flash and Pro sessions. | interactive |

### Exit Criteria
- Harvest succeeds on ≥ 95% of ≥ 30 milestone requests; non-milestone turns stay silent; the settings work.
- Marks are correct through rewinds and checkpoints; subagents are excluded.
- **X10:** derailed outputs ≤ 2% on the test set, none reaching the inbox.
- The fallback produces notes within the quota budget.
- **X2:** notes judged at least as useful in ≥ 50% of pairs, at ≤ ⅓ of the length.

---

## PR-11-v2-consolidation: Echo Filter, LLM Ops & Publish

**v2 Design Sections:** §6, §5.6

### Work Orders

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-echo-filter` | P13 echo filter (v2 §6 step 1), using exposure lists from PR-08. A repeat of an exposed entry is a no-op, unless the session changed an anchored file and then passed a build, test or the entry's check. | standard |
| 2 | `wo2-routing-dedup-anchors` | Route by anchors and scopes (derived scopes proposed through the memory PR); merge exact duplicates; flag existing entries whose anchors no longer resolve, for the LLM step. | standard |
| 3 | `wo3-llm-decide` | The lean agent decides the ops; the input is JSON records (§5.6); several dossiers per prompt, or `stream-json`. The LLM never rewrites a page. | architect |
| 4 | `wo4-validate-record` | Validate every op in Go; append to the op log; re-render; lint caps. | standard |
| 5 | `wo5-publish` | Two-lane publish (SPEC §6.8), reusing the PR-05a local-publish worktree. Branch scoping: notes from unmerged branches stay local and move to shared on merge, using the anchor checks recorded at collection. | architect |
| 6 | `wo6-orchestration` | Wire into the tick: run on ≥ 10 notes or daily; skip on the daily cap, or when the quota guard reports the pool below threshold. **X4** over one week of notes, every op reviewed. | standard |

### Exit Criteria
- **X4:** ≥ 80% of ops accepted; no echo counted as confirmation; caps respected.
- **X10:** holds for consolidation inputs.
- Two-lane publish works; branch-scoped notes stay local until merge.

---

## PR-12-v2-pilot: Pilot & Confirmatory X6

**v2 Design Sections:** §12 (X6)

### Work Orders

| # | WO | Repository | Scope | Complexity |
|---|---|---|---|---|
| 1 | `wo1-dogfood-run` | repo-memory-plugin | Full v2 on this repo: turn notes, fallback, consolidation. Measure cost per note, per fallback call and per consolidation batch from the telemetry. | standard |
| 2 | `wo2-pilot-enrolment` | esp32-app-foundation | Full enrolment with a reviewed scope config, as a PR in that repo. Run the fallback and seeding over captured evidence (available since the PD17 capture-only enrolment). | standard |
| 3 | `wo3-dossier-review` | repo-memory-plugin | Review dossiers from both repos against ground truth. Score precision per type, missed knowledge and stale entries. | interactive |
| 4 | `wo4-x6-confirmatory` | repo-memory-plugin | Re-run the X6 protocol with time-cut dossiers produced by the full pipeline instead of seeding. | interactive |
| 5 | `wo5-cost-report` | repo-memory-plugin | Cost per session (turn notes, fallback, consolidation, delivery), quota impact, and comparison with v1 (~112k per episode) and the PA (~5k to produce + ~9k to load). | standard |

### Exit Criteria
- **Confirmatory X6:** distilled dossiers beat no-memory and match PA docs at fewer tokens. If not, stop or fall back to seeded or PA-sourced dossiers.
- Precision ≥ 0.8 for pushed entries; cost within budget; pilot dossiers reviewed by the user.

---

## PR-13-v2-learning: Close Conditions, Coverage & Metrics

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-close-conditions` | Evaluate `closes_when` in the tick: issue closed, branch merged, work order completed, command passes, path or symbol exists or is absent, file contains or lacks a pattern. Retire the entry with the reason. Entries without a condition are pull-only. | standard |
| 2 | `wo2-gardening` | Weekly, gated job with every op reviewed in the memory PR: propose splits near the cap, merge near-duplicates, retire entries with gone anchors. | standard |
| 3 | `wo3-coverage` | Coverage and gap scores per dossier and per type | standard |
| 4 | `wo4-elicitation` | 1–3 questions from the highest gaps, written to the weekly review and `QUESTIONS.md`. Answers become notes tagged `source: human`. | standard |
| 5 | `wo5-session-metrics` | Push precision (pushed entries referenced later), tokens pushed, hit rate, fallback rate. Stored in `metrics/*.jsonl`. | standard |
| 6 | `wo6-replay-tuning` | Offline replay of delivery policy on episode prefixes, labelled from suffixes. **Exit:** one policy change that improves push precision on the replay set without lowering hit rate. | architect |

### Exit Criteria
- Close conditions fire correctly; gardening proposals are sensible; gap scores are meaningful.
- Questions appear in the review; metrics are collected; one policy change is accepted per wo6.

---

## PR-14-v2-pa-integration: Cross-Repo

| # | WO | Repository | Scope | Complexity |
|---|---|---|---|---|
| 1 | `wo1-episode-summaries` | repo-memory-plugin | `repo-memory summary <session>` (≤ 1.5k tokens) | standard |
| 2 | `wo2-workorder-briefs` | repo-memory-plugin | `repo-memory brief --workorder <path>` | standard |
| 3 | `wo3-pa-change-request` | mm-skills/principal-architect | Compatibility mode through that repo's process: when the repo is enrolled, the PA embeds generated briefs and reads generated summaries; turn-note interaction per the PR-10 role settings. | architect |
| 4 | `wo4-esp32-migration` | esp32-app-foundation | After the PA change ships: PROJECT.md holds intent, plan, ledger and approvals only; knowledge comes from dossiers. | standard |

### Exit Criteria
- Compatibility mode merged and verified on a non-enrolled project.
- **No hand-written memory files** for a full project cycle in esp32-app-foundation.
- B4 shows lower total cost than PA hand-written memory.

---

## PR-15-v2-backfill: Full Import & Re-projection

| # | WO | Scope | Complexity |
|---|---|---|---|
| 1 | `wo1-transcript-import` | All captured evidence, through the fallback, throttled by the daily budget. Note: only transcripts captured before pruning exist (PD10, PD17). | standard |
| 2 | `wo2-doc-import` | Design decision logs, ADRs, guides and experiment reports, imported as notes | standard |
| 3 | `wo3-git-import` | Commits, PR descriptions, and reverts as refutation evidence | standard |
| 4 | `wo4-inferred-rationale` | A lean-agent pass for subsystems with no rationale coverage. Results are tagged `hypothesis`/`inferred`. | standard |
| 5 | `wo5-reprojection` | `repo-memory reproject`: an op-log replay under a new pipeline version, producing a diff memory PR. Human decisions are re-applied by entry ID and content hash. | architect |

### Exit Criteria
- Every subsystem has a dossier; the coverage report is published; historical defects end retired or superseded; re-projection preserves every human decision.

---

## Human Tasks

| When | Task | Needed by |
|---|---|---|
| After PR-05-v2-foundation | **Phase 2 capture:** choose and enrol a second repository (esp32-app-foundation or another active repo) once capture is validated here | PR-12 pilot |
| PR-05-v2 | Confirm why the installed `hooks.json` was emptied on 2026-10-02 | wo4 |
| PR-06-v2 | Observe the interactive experiments (X1, X3, X5, X8); judge turn-note visibility; approve the SPEC v1.0 edits | X-gate |
| PR-07-v2 | Review the time-cut seeded dossiers | PR-07 exit |
| PR-09-v2 | Approve the X6 protocol and quota budget; review dossiers for leakage; decide continue, redirect or stop | Early kill gate |
| PR-10-v2 | Blind-judge the X2 note pairs; confirm the role opt-out settings | PR-10 exit |
| PR-12-v2 | Review pilot dossiers; review the esp32 scope proposal; decide at the confirmatory gate | PR-12 exit |
| PR-11 onward | Review memory PRs; answer weekly review reports | Ongoing |
| PR-14-v2 | Approve the PA compatibility-mode change | PR-14 |

---

## Risk Register

| Risk | Mitigation | Owner |
|---|---|---|
| `Stop` continue doesn't behave as documented | X1 with a prototype before production; fallback-only mode as backstop | PR-06 |
| Turn notes disrupt PA or worker sessions | X1 includes PA sessions; opt-out by role | PR-06, PR-10 |
| Quota counts cached tokens at full rate | Milestone-only gate; the fallback carries more load; X3 measures | PR-06 |
| Users find end-of-turn notes noisy | Milestone gate, cooldown, cap; `turn_notes` setting | PR-10 |
| Agents write wrong or self-serving notes | Evidence field; `verified` only from tests, commands, merges or a human; memory-PR review; anchor checks | PR-10, PR-11 |
| Processing models follow instructions in their input | §5.6 defences; lean agent with no tools; X10 | PR-10, PR-11 |
| Echoes come back as confirmation | Exposure lists from PR-08; echo filter | PR-08, PR-11 |
| Injections lost to compaction or rewind | Ledger and re-injection; X11 | PR-08 |
| **Dossiers don't improve outcomes** | **Early kill gate (X6-lite) at PR-09**; confirmatory X6 at PR-12 | PR-09, PR-12 |
| **X6 leakage** (dossiers contain the replayed work order's own knowledge) | Time cut by parent commit for dossiers and PA docs; user leakage review | PR-09 |
| X6 too small to detect real effects | Pre-registered metric; ≥ 2 seeds; work orders chosen for headroom | PR-09 |
| **Conversation pruning** (~500 kept, ~19 days) destroys pilot, backfill and X6 inputs | Artifact capture and `snapshot` in PR-05; PD17 capture-only enrolment of esp32 | PR-05, user |
| **Hooks disabled in the live install** | Safe re-enable in PR-05 wo4; fail-open verified; install writes the hook set | PR-05 |
| Hook latency exceeded | Budgets as exit criteria (PD24); precompute in the tick; non-enrolled fast path | PR-05, PR-08 |
| Lean agent not discoverable from the runtime working directory | X8 tests install location | PR-06 |
| Lean-agent overhead too high | X8 measures before reliance | PR-06 |
| Several developers publish at once | Op log never conflicts; dossiers re-render | PR-07 |
| Notes from a branch that never merges | Kept local, scoped to that branch | PR-11 |
| Platform behaviour changes | Recorded probes; experiments re-run on version changes | Programme |
| Antigravity turns on Knowledge Items | Detect; publish into them or switch off overlap | PR-08 |
| PA change disrupts other projects | Compatibility mode; through the PA repo's process | PR-14 |

---

## Decision Log

### v1 Decisions (retained)

| # | Decision | Date | Rationale |
|---|---|---|---|
| PD1 | Decompose SPEC P0 across PR-01 and the capture PRs | 2026-10-01 | P0 too large for one PR |
| PD3 | Experiments as a docs-only spike before implementation | 2026-10-01 | No rebase churn. (v2 spike is a prototype spike; see PD20.) |
| PD4 | Release 2 (Claude Code) excluded from this plan | 2026-10-01 | Per SPEC §15.2 |
| PD8 | Cross-repo changes through each repository's own process | 2026-10-01 | Separate repos |
| PD9 | Enrolment, kill switch, install lifecycle and CI in PR-01 | 2026-10-01 | Global plugin acts only on enrolled repos |
| PD10 | No emergency preservation of **old** transcripts | 2026-10-01 | User decision. Near-term v2 inputs: see PD17. |
| PD14 | Enrol repo-memory-plugin as dogfood from PR-02b | 2026-10-01 | Real test corpus |
| PD16 | Read one transcript source in priority order | 2026-10-01 | Verified on 537 conversations |

### v2 Decisions

| # | Decision | Date | Rationale |
|---|---|---|---|
| VD1 | ~~Archive PR-05a/05b; branch from PR-04's tip~~ **Superseded by PD18** | 2026-10-04 | — |
| VD2 | Validate platform assumptions before implementation | 2026-10-04 | v2 relies on unverified behaviour |
| VD3 | Op log replaces claims store | 2026-10-04 | Same event-sourcing pattern. The v1 events store is retired (PD26). |
| VD4 | Kill criterion on B1 replay | 2026-10-04 | Now applied early (PD22) and confirmed later |
| VD5 | Entity extractor carries forward | 2026-10-04 | Feeds derived scopes |
| VD6 | Lean Markdown agent replaces the default `agy -p` executor | 2026-10-04 | Measured in X8, including install location |
| VD7 | Turn notes via `Stop` continue, not `/btw` | 2026-10-03 | Plugins can't trigger `/btw` (Q25) |
| VD8 | Persistent `systemMessage`; hints via `ephemeralMessage` | 2026-10-03 | E01; confirmed in X5 |
| VD9 | Op log as source of truth; dossiers rendered | 2026-10-03 | No git conflicts |
| VD10 | Echo filter with exposure lists | 2026-10-03 | P13 |
| VD11 | Anchor checks at collection and delivery | 2026-10-03 | No LLM verify stage |
| VD12 | Preparation layer for processing input | 2026-10-03 | §5.6 |

### v2.1 Decisions (plan review, 2026-10-04)

| # | Decision | Rationale |
|---|---|---|
| PD17 | **Decided:** two-phase capture. Phase 1: fix and validate capture in repo-memory-plugin (PR-05-v2-foundation). Phase 2: enrol esp32-app-foundation or another active repo once capture is proven stable — not before. X6 candidates come from this repo only. | User decision (2026-10-04). Accepts the pruning risk: esp32 transcripts may be lost before enrolment. The pilot (PR-12) works with whatever remains plus newly captured evidence. |
| PD18 | v2 is built on `main`, after closing PR-05b onto it | PR-05a is already on `main`, with later fixes; PR-05b holds the executor fixes and all v2 docs; the executor, job chaining and local publish are reused |
| PD19 | PR-05-v2-foundation fixes defects before v2 work | Capture→episode break, hooks disabled, empty cost telemetry, probe logs, non-enrolled latency |
| PD20 | PR-06 is a prototype spike: code on a flagged spike branch; only reports and SPEC edits merge | X1/X5/X8/X9 need live hook code; X2 needs real work orders, so it moves to PR-10 |
| PD21 | Read side first: storage → delivery → X6-lite → distillation → consolidation | The read side decides outcomes (VibeMemBench) and is what X6 tests; exposure lists exist before the echo filter needs them |
| PD22 | Early kill gate (X6-lite) at PR-09 with time-cut seeded dossiers; confirmatory X6 at PR-12 | Tests the central question after 3 implementation PRs instead of 5 |
| PD23 | Pre-registered X6 protocol with time cut, seeds, primary metric and budget | Prevents leakage; makes the gate meaningful |
| PD24 | Hook latency budgets are exit criteria for delivery | Delivery adds work to every `PreInvocation` |
| PD25 | Turn-note settings, defined milestone signals, opt-out by role | Safety in PA-driven repositories |
| PD26 | v1 events store retired in favour of `.memory/ops/` | VD3 said "adapts" without a work order |

### v2.2 Decisions (post-X6-lite & Stage 1 synthesis, 2026-10-15)

| # | Decision | Date | Rationale |
|---|---|---|---|
| VD13 | Persistent push moments (`task-start`, `first-touch`, `first-entry`, `re-injection`) use `UserMessage` where `systemMessage` is rejected; `EphemeralMessage` is strictly for 1-step hints; `DetectRewind` is wired into `PreInvocation` | 2026-10-15 | Commit `8d7ad7b` + `d3ab994` (`EphemeralMessage` + `InjectedSet` dedup) evicted memories after 1 invocation. `UserMessage` is a plain string that persists across invocations until `CHECKPOINT` compaction. |
| VD14 | Stage 1 Primary is 100% automatic end-of-turn `Stop` (`NO_TOOL_CALL`, `executionNum == 0`) continuation on milestone turns + free `CHECKPOINT`/`brain/` harvest; `memory_note` is an optional mid-turn override that deduplicates against `Stop` | 2026-10-15 | Preserves `G1`/`P1` (zero user/agent cooperation required). Empirical studies (`PR-09`, `PR-10` Stage 1) proved agents do not reliably invoke optional memory tools unprompted. |
| VD15 | Stage 2 batch consolidation runs a single multi-turn `stream-json` `repo-memory-distiller` session per tick | 2026-10-15 | Allows dossiers $2..N$ in a tick batch to share the warm 5-minute prefix cache (`ATLAS.md` + consolidator instructions). |
| VD16 | Add `DocSeedSource` (`repo-memory seed-docs`) and async doc-diff freshness ("Dreaming") in `tick` | 2026-10-15 | `PR-10` Stage 1 77-session audit showed static repo guides/skills (`framework-expert` 5%, dev guides 8%, `creating-a-module` 2%) are almost never pulled by coding agents unless converted to anchored dossier entries and pushed. |
| VD17 | Govern the 8 entry types by 3 cognitive lifecycles (`semantic`, `procedural`, `episodic`) and use optional `{section?}` dossier rendering | 2026-10-15 | `semantic` validates via code anchors; `procedural` validates via `PostToolUse` exit codes and graduates to `.agents/skills/rm-<slug>/SKILL.md` (Addendum A7); `episodic` is append-only rationale. `{section?}` eliminates empty heading bloat. |
| VD18 | Enforce 5-tier state scopes (`temp:`, `session:`, `user:`, `branch:`, `repo:`) and the Pre-Consolidation Corroboration Gate | 2026-10-15 | Isolates workstation facts (`user-dossiers/`) and unmerged branch anchors (`.memory/local/`); prevents uncorroborated intermediate hypotheses (`verified: null`) from becoming active `do:` imperatives. |
| VD19 | Remove stale v1 `E02` root `GEMINI.md` (`@[memory map](.memory/views/MAP.md)`) and `.memory/views/`; keep `GEMINI.md` strictly machine-managed by `repo-memory init` (Addendum A8) | 2026-10-15 | Static session-start map includes were retired in v2 (§15). |
