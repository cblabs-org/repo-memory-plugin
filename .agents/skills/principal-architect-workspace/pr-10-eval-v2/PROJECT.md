# PR-10-eval-v2: Memory Value Evaluation v2

> **Programme:** repo-memory
> **PR:** pr-10-eval-v2
> **Base branch:** main (at `208b0e0`, post PR-09b merge)
> **Status:** 🔄 Active — WO4 complete (Layer 2 GO); WO5 ready for workstation Layer 3 execution
> **Human approval:** Decomposition approved 2026-10-11T20:24 by the project owner.
> **Design ref:** [memory-value-evaluation-proposal.md](../../../docs/evaluations/memory-value-evaluation-proposal.md) (v2, approved)
> **Handover ref:** [chief-architect-handover.md](chief-architect-handover.md)

## Objective

Execute the approved evaluation v2 design to answer the programme's central questions:

1. **Is repo-memory better than nothing?**
2. **Is repo-memory better than the PA skill's memory approach?**

This PR covers Stages 0b through 3 of the evaluation proposal. Stage 0 (product fixes) is already complete (PR-09b). The original PR-10 (distillation) and all subsequent PRs are gated on evaluation results — they proceed only if the evaluation shows value.

## Programme Context

- X6-lite (PR-09): **STOP verdict** — 84% no-memory baseline on spec-style tasks, no headroom. Evaluation v2 pivots to esp32 with short task prompts.
- PA retrieval study: 1.3% MEMORY.md uptake across 231 transcripts. Pull-based memory effectively unused.
- PR-09b (Stage 0): product fixes shipped — canonical IndexPath, ranker improvements (bm25 relevance, per-session dedup, relevance threshold), binary rebuilt.
- **Chief Architect §12 decisions:** forward-task 2×2 approved, §9/§10 approved, 7.5M cap approved, Stage 0 outside budget confirmed, c2 deferred.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | [wo1-template-build](wo1-template-build/WORKORDER.md) | Build the esp32 evaluation template per §9: rsync working tree with exclusions, verify build (`pio run` + `pio test -e native`), squash history, create condition templates (tpl-a/b/c/d). Produce the template verification script. | standard | ✅ Complete |
| 2 | [wo2-protection-harness](wo2-protection-harness/WORKORDER.md) | Implement §10 protection: env vars (`GH_TOKEN=invalid`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_TERMINAL_PROMPT=0`), one-time verification check, BugFixer/sidecar disable instructions, run-copy procedure with APFS clonefile. Produce `eval_run.sh` orchestration script. | standard | ✅ Complete |
| 3 | [wo3-index-build](wo3-index-build/WORKORDER.md) | Build the esp32 index from the PA workspace at HEAD. Seed dossiers from the 105 PA project dirs / 435 workorders. Verify non-trivial entry count and FTS search works. | standard | ✅ Complete |
| 4 | [wo4-offline-retrieval](wo4-offline-retrieval/WORKORDER.md) | Stage 1: Offline retrieval quality check. Queries = task prompts + sample historical issue texts. Rate top-k injected entries for relevance. Compare against plain search over PA docs and code. Pre-set precision@k thresholds. | interactive | ✅ Complete |
| 5 | [wo5-pilot](wo5-pilot/WORKORDER.md) | Stage 2a: Pilot run. 1 task × (a),(c) × 1 run. Validate harness (contamination check, protection, archival). Measure cost per run to set 2b's size. | interactive | ☐ Ready (Workstation Layer 3) |
| 6 | [wo6-main-evaluation](wo6-main-evaluation/WORKORDER.md) | Stage 2b: Main evaluation. 2-3 tasks × conditions × 2 replicates, interleaved and pre-registered. Final Q1/Q2 verdicts. Rubric grading, token accounting, cost/quality frontier. | interactive | ☐ Planned |
| 7 | [wo7-report](wo7-report/WORKORDER.md) | Final evaluation report: aggregate results, cost/quality frontier, Q1/Q2 verdicts, confidence intervals, honesty clause, recommendations for the programme (continue to distillation / redirect / stop). | standard | ☐ Planned |

## Dependencies

- **Base:** main at `208b0e0` (post PR-09b, Stage 0 complete)
- **esp32 repo:** `~/Documents/Projects/esp32-app-foundation` (working tree, not a fresh clone — see handover §5)
- **Evaluation root:** `~/Documents/Projects/repo-memory-plugin/evaluation/` (pre-existing from X6-lite)
- **Binary:** `~/.local/bin/repo-memory` (rebuilt in PR-09b)

## Human Tasks (from handover §4)

1. **Task authoring (before WO5):** Write 2-3 forward task prompts for esp32 (areas where history matters: LVGL/CrowPanel display, OTA, portal). Frozen before Stage 2.
2. **Rubric authoring (before WO5):** Conventions/pitfalls checklist per task, blind to condition.
3. **Stage 1 relevance labelling (WO4):** Hand-label top-k retrieval results.

## Exit Criteria

- §10 protection verified: `gh api user` and `git push` both fail in the evaluation env
- Template builds: `pio run` and `pio test -e native` pass
- esp32 index has non-trivial entry count from PA workspace seeding
- Stage 1: offline retrieval meets pre-set precision@k thresholds (or ranker fixed and repeated)
- Stage 2a: pilot harness clean, cost measured
- Stage 2b: Q1/Q2 verdicts reached with directional confidence
- Total agent tokens for Stages 2–3 ≤ 7.5M
- Final report committed with honesty clause

## Risks

| Risk | Mitigation |
|---|---|
| esp32 template doesn't build (gitignored generated files) | Working tree copy, not fresh clone; verify with `pio run` before squashing; capture fix steps as script |
| Too few tasks to generalise | Results reported as directional; tasks chosen where memory should matter; honesty clause |
| Runs cost more than ~450K each | Pilot first; shrink order: keep (a)+(c); then (b); then (c3); then (d); cut replicates before tasks |
| Agent reaches GitHub or real checkout | §10 protection + transcript scan + no-remote template |
| Template becomes stale during eval | One template per batch, verified by script, archived with sha256 manifest |

## Session Log

### 2026-10-11 — Project Creation (Chief Architect handover)
- New Chief Architect session. Ingested handover from session `a0c33c40`.
- Programme at 14/19 PRs. This PR replaces the original PR-10 (distillation) in the immediate sequence.
- Original PR-10 through PR-15 are renumbered and gated on evaluation results.
- 7 WOs decomposed: template build → protection harness → index build → offline retrieval → pilot → main eval → report.
- WOs 1-3 are Stage 0b (mechanical/standard, automatable). WO4 is Stage 1 (interactive, needs human labels). WO5-6 are Stages 2a/2b (interactive, human tasks gating). WO7 is the final report.
### 2026-10-11 — WO1 Complete (template build)
- esp32 working tree rsynced from `45d4b5d` with exclusions applied. All generated build files preserved.
- `pio test -e native`: **989/989 tests passed** (0 failures, 01:19). No template fixups required.
- History squashed to single `baseline` commit. 4 condition templates created via APFS clonefile.
- tpl-a/tpl-c: PA workspace removed, `disabled-skills.txt` override installed.
- tpl-b/tpl-d: PA workspace retained as-is.
- Verification script and MANIFEST.md created. platformio.ini SHA-256: `d4e548eb...`.
- No out-of-scope findings. No dead ends (fsmonitor socket warning resolved by .git removal).
- Branching WO2 for protection harness.

### 2026-10-11 — WO2 Complete (protection harness)
- `eval_env.sh`: sets `GH_TOKEN=invalid`, `GIT_CONFIG_NOSYSTEM=1`, `GIT_TERMINAL_PROMPT=0`, unsets `SSH_AUTH_SOCK`.
- `verify_protection.sh`: one-time check — `gh api user` fails (Bad credentials), `git push` fails (terminal prompts disabled). Both verified.
- `eval_run.sh`: full orchestration — APFS clonefile copy, per-run plugin/data dirs for conditions c/d, safety prompt appended, archive phase (git diff, untracked files, transcript, ledger).
- `check_contamination.sh`: transcript scanner for real checkout path, `git push`, `gh` writes, cross-run references. Tested on clean and contaminated inputs.
- `PROTECTION.md`: threat model, scope/limitations, BugFixer/sidecar disable/restore procedures.
- No out-of-scope findings. No blockers.
- Branching WO3 for index build.

### 2026-10-11 — WO3 Complete (index build) — Stage 0b Done
- Seeded **260 entries from 79 workorders** (of 435 total; 317 pending/unexecuted, 39 template-only). 0 parse errors.
- Entry breakdown: 157 decisions, 58 ruled-out/dead-ends, 45 open-issues/findings, 2,206 entry anchors.
- SQLite index: 492 KB, 260 rows in `entries`, FTS5 indexed.
- FTS verification: display(13), portal(19), OTA(11), WiFi(13), LVGL(17), compound(38).
- Hermetic rebuild via ephemeral staging repo — templates untouched.
- `build_index.sh` repeatable automation with `--clean` flag.
- `SEED_REPORT.md` complete.
- **Stage 0b complete.** All 3 infrastructure WOs done. Next: WO4 (Stage 1, offline retrieval) — blocked on human task prompts.

### 2026-10-12 — Stage 1: Offline Retrieval & Doc-Read-Rate Analysis
- **Tasks frozen:** 2 forward tasks — (1) DS18B20 temperature module (module + U8G2 + portal + MCP), (2) Bitcoin price module (HTTP API + DataManager + U8G2 + portal + MCP).
- **Retrieval quality:** FTS against seeded index shows 70-90% precision@10 for both tasks. DataManager entries fire via anchor-based injection on file touch. No ranker fixes needed.
- **Instruction layer audit (critical finding):**
  - esp32 ships ~30K lines of curated content: framework-expert skill (8,813), developer guides (8,749), design spec (3,022), sample/template apps (1,950), rules (439).
  - Historical read rate across 77 non-PA sessions: **framework-expert 5%, developer guides 8%, creating-a-module 2%, docs/developer/README 1%**.
  - Curated documentation is a pull system; agents almost never find it. Memory is a push system; it injects at the moment of relevance.
  - **Decision: test real-world scenario.** No "read the docs" rule added to templates. This tests push vs pull, not memory vs docs.
- **Product direction surfaced:** Doc-seeded dossiers (ingest curated docs as entries) + async freshness ("dreaming" — watch for doc changes, re-process dossiers). Gated on positive evaluation verdict. Added to programme plan backlog.
- Stage 1 gate: PASS pending user confirmation.

### 2026-10-16 — WO4 Complete & Layer 3 Workstation Handover Prepared (PR-14 / PR-15 WO1)
- **v2.2 Combined Index (`PR-14/WO1`):** Combined `repo-memory seed` (`256` PA workorder entries) + `repo-memory seed-docs` (`1,027` curated developer guide & skill entries) into **`1,283` entries**, **`3,203` verified file anchors**, and **`54` projected subsystem dossiers** (`0` `ops gc` drops).
- **Read-Side Multi-Invocation Micro-Probe (`PR-14/WO2`, `docs/evaluations/v2-2-read-side-micro-probe.md`):** Achieved **`90%–100%` Precision@10** on `MomentTaskStart`; persistent `UserMessage` delivery retained **`71` active entries** (Task 1) and **`86` active entries** (Task 2) at code edit time (covering `6/6` subsystem rubric contracts), restored `30`/`39` entries post-`CHECKPOINT`, and re-armed `68`/`82` entries post-`/rewind`.
- **Write-Side Distillation & Consolidation Micro-Probe (`PR-14/WO3`, `docs/evaluations/v2-2-write-side-micro-probe.md`):** Verified **`21.69%` (`18/83`)** `Stop` milestone selectivity across 25 real transcripts, `0`-token Stage 1b harvest (`6` items, `100%` idempotent), **`62.30%` corpus / `74.2%–94.0%` top-session `PrepareSession` byte reduction**, `4/4` adversarial validator rejections, `4/4` secret classes redacted, and Stage 2 consolidation (`P13` echo filter + `VD18` corroboration gate + Five-Tier routing + `memory/pending` worktree commit).
- **Compiled-Binary 1-Task Pilot (`PR-14/WO4`, `docs/evaluations/v2-2-micro-probe-verdict.md`):** Verified §10 protection, Condition A (`0/5` architectural compliance) vs Condition C (`5/5` architectural compliance + `Stop` note harvest + warm consolidation), `0` contamination violations, and clean archival (**`VERDICT: GO`**).
- **Layer 3 Workstation Handover (`PR-15/WO1`):** Committed `chief-architect-handover.md`, `wo4-offline-retrieval/WORKORDER.md` through `wo7-report/WORKORDER.md`, blind grading rubrics and automated scorer (`evaluation/eval-v2/rubrics/`), and copy-pasteable workstation bridge prompt (`evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`). Ready for live `pio` + `agy` execution starting at `WO5`.
