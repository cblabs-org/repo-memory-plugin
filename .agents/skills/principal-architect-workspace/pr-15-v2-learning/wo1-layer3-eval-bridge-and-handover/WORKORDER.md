---
name: pr-15-v2-learning-wo1
description: Handoff instructions for Work Order 1 — Layer 3 (PR-10-eval-v2) Portable Bridge Prompt, Work Orders WO4–WO7, and Blind Grading Rubrics
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Make Layer 3 (`PR-10-eval-v2` Stage 2a/2b live evaluation) 100% self-contained and ready to pull and run on a separate macOS workstation where PlatformIO (`pio`) and the Antigravity CLI (`agy`) are installed. Commit the `chief-architect-handover.md` document, the remaining `WO4`–`WO7` `WORKORDER.md` files for `pr-10-eval-v2`, update `pr-10-eval-v2/PROJECT.md`, create the blind grading rubrics and automated scorer in `evaluation/eval-v2/rubrics/`, and author the copy-pasteable workstation bridge prompt at `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`.

## Context & Provenance

- Parent Project: [PROJECT.md](../PROJECT.md) (`pr-15-v2-learning`, 5-WO decomposition approved by the project owner in `PROJECT.md` §Human Approval).
- Evaluation Proposal: [docs/evaluations/memory-value-evaluation-proposal.md](../../../../../docs/evaluations/memory-value-evaluation-proposal.md)
- Layer 2 Micro-Probe Reports (completed in `PR-14`):
  - [docs/evaluations/v2-2-read-side-micro-probe.md](../../../../../docs/evaluations/v2-2-read-side-micro-probe.md)
  - [docs/evaluations/v2-2-write-side-micro-probe.md](../../../../../docs/evaluations/v2-2-write-side-micro-probe.md)
  - [docs/evaluations/v2-2-micro-probe-verdict.md](../../../../../docs/evaluations/v2-2-micro-probe-verdict.md)
- Existing `pr-10-eval-v2` files:
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo1-template-build/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo2-protection-harness/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo3-index-build/WORKORDER.md`
  - `evaluation/eval-v2/tasks/task-1-temperature-module.txt`
  - `evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`
  - `evaluation/eval-v2/scripts/{prepare_templates.sh,build_index.sh,verify_protection.sh,verify_template.sh,eval_run.sh,check_contamination.sh}`

## Strict Boundary Box

- **In Scope:**
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/chief-architect-handover.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo4-offline-retrieval/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo5-pilot/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo6-main-evaluation/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo7-report/WORKORDER.md`
  - `evaluation/eval-v2/rubrics/task-1-temperature-module.yaml`
  - `evaluation/eval-v2/rubrics/task-2-bitcoin-price-module.yaml`
  - `evaluation/eval-v2/rubrics/grade_run.sh`
  - `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`
  - `.agents/skills/principal-architect-workspace/pr-15-v2-learning/wo1-layer3-eval-bridge-and-handover/WORKORDER.md` (Completion Report)
- **Out of Scope:**
  - Do NOT modify any Go source code in `cmd/` or `internal/` (that belongs to `WO2`–`WO5`).
  - Do NOT create new git branches or push to remote.
  - **Repository Hygiene:** Never include any proprietary internal identifiers, internal issue/changelist prefixes, internal domain emails, or absolute local machine home paths in any repository files (use portable `$HOME` or `~/` paths in scripts and documentation).
  - Ensure all git commits use author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Tooling Constraints

Use `read_file`, `write_file`, `edit_file`, and `run_command` for file creation, script testing, and git commits.

## Line-Item Instructions

- [x] **1. Create `.agents/skills/principal-architect-workspace/pr-10-eval-v2/chief-architect-handover.md`:**
  - Document the complete `v2.2` evaluation architecture and handover context referenced by `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`:
    1. **Executive Overview & Research Questions (`Q1` & `Q2`):**
       - `Q1`: Is `repo-memory v2.2` better than no memory (`Condition C` vs `Condition A`) on quality, architectural compliance, and token efficiency?
       - `Q2`: Is `repo-memory v2.2` better than the Principal Architect (`PA`) skill's manual workspace memory (`Condition C` vs `Condition B`) on the cost/quality frontier (including read-side carried tokens + amortised write-side maintenance cost), and are they additive (`Condition D`)?
    2. **Completed Foundations (Stages 0, 0b, 1, and Layer 2 Micro-Probes in `PR-10a`–`PR-14`):**
       - Summarize Stage 0b (`WO1`–`WO3`), the `v2.2` upgrades (`PR-10a` through `PR-13`: persistent `UserMessage` delivery, `DetectRewind`, `seed-docs` + `repoSymbolIndex`, 3 cognitive lifecycles, Stage 1 warm-cache `Stop` continuation + free harvest, Stage 2 `stream-json` consolidation + `P13` echo filter + `VD18` corroboration gate), and the Layer 2 Micro-Probe & 1-Task Pilot results from `PR-14` (`1,283` entries = `256` `seed` + `1,027` `doc-seed`, `3,203` anchors, `54` dossiers, `90%–100%` Precision@10, `71`/`86` active rules in context at edit time).
    3. **Workstation Execution Sequence (`WO5` $\to$ `WO6` $\to$ `WO7`):**
       - Detail how the macOS workstation operator or agent runs `WO5` (Stage 2a live 1-task pilot with `pio test -e native` and `agy` CLI), `WO6` (Stage 2b pre-registered 2×2 factorial evaluation across Task 1 and Task 2, Conditions A, B, C, D, 2 interleaved replicates, blind rubric grading via `evaluation/eval-v2/rubrics/grade_run.sh`), and `WO7` (final statistical synthesis in `docs/evaluations/eval-v2-report.md` per Addendum A §9 A10).

- [x] **2. Create the 4 remaining `WORKORDER.md` files under `.agents/skills/principal-architect-workspace/pr-10-eval-v2/`:**
  - **`wo4-offline-retrieval/WORKORDER.md`:**
    - Frontmatter: `name: pr-10-eval-v2-wo4`, `description: Handoff instructions for Work Order 4 — Stage 1 & Layer 2 Offline Retrieval and Write-Side Micro-Probes`, `complexity: standard`.
    - Document the Stage 1 offline retrieval check, the 77-session static doc read-rate audit (1–8% voluntary pull rate), and the Layer 2 read-side and write-side micro-probes executed in `PR-14`.
    - Include a completed `## Completion Report` (`status: complete`) referencing `docs/evaluations/v2-2-read-side-micro-probe.md`, `docs/evaluations/v2-2-write-side-micro-probe.md`, and `docs/evaluations/v2-2-micro-probe-verdict.md` (`90%–100%` Precision@10 on `MomentTaskStart`, `71` and `86` persistent rules at edit time, `6/6` subsystem rubric coverage, `VERDICT: GO`).
  - **`wo5-pilot/WORKORDER.md`:**
    - Frontmatter: `name: pr-10-eval-v2-wo5`, `description: Handoff instructions for Work Order 5 — Stage 2a Live 1-Task Workstation Pilot (Condition A vs Condition C)`, `complexity: interactive`.
    - Step-by-step instructions for running the Stage 2a live workstation pilot on `evaluation/eval-v2/tasks/task-1-temperature-module.txt` under Condition A (`pilot-t1-a-live`) and Condition C (`pilot-t1-c-live`) using `evaluation/eval-v2/scripts/eval_run.sh`, running `pio test -e native` inside the run workspace before archival, archiving with `eval_run.sh --archive`, running `check_contamination.sh`, grading with `evaluation/eval-v2/rubrics/grade_run.sh`, and measuring per-run token cost (including carried cache-read tokens) to confirm the Stage 2b replicate schedule within the 7.5M token cap.
    - Include the blank `## Completion Report` template at the bottom.
  - **`wo6-main-evaluation/WORKORDER.md`:**
    - Frontmatter: `name: pr-10-eval-v2-wo6`, `description: Handoff instructions for Work Order 6 — Stage 2b Pre-Registered 2x2 Factorial Evaluation`, `complexity: interactive`.
    - Step-by-step instructions for running the pre-registered interleaved 2×2 factorial matrix across `task-1-temperature-module.txt` and `task-2-bitcoin-price-module.txt`, Conditions `a`, `b`, `c`, `d` (plus optional `c3` advertised-pull variant if within the 7.5M token cap), 2 replicates per cell (interleaved schedule: Rep 1 `[t1-a, t1-c, t1-b, t1-d, t2-c, t2-a, t2-d, t2-b]`, Rep 2 reversed), running `pio test -e native`, `check_contamination.sh`, `eval_run.sh --archive`, and blind grading via `evaluation/eval-v2/rubrics/grade_run.sh`.
    - Include the blank `## Completion Report` template at the bottom.
  - **`wo7-report/WORKORDER.md`:**
    - Frontmatter: `name: pr-10-eval-v2-wo7`, `description: Handoff instructions for Work Order 7 — Final Evaluation v2 Report and Cost/Quality Frontier`, `complexity: standard`.
    - Step-by-step instructions for synthesizing all archived Stage 2a/2b run scorecards, token ledgers, and test results into `docs/evaluations/eval-v2-report.md` following Addendum A §9 (A10): paired differences per task × replicate, Wilcoxon signed-rank / paired t-test, Holm step-down multiple-testing correction, 95% confidence intervals, read-side + write-side token accounting, cost/quality frontier table, explicit `Q1` and `Q2` verdicts, and the mandatory `## What this does not show` honesty clause.
    - Include the blank `## Completion Report` template at the bottom.

- [x] **3. Update `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`:**
  - Update the top `> **Status:**` line to `🔄 Active — WO4 complete (Layer 2 GO); WO5 ready for workstation Layer 3 execution`.
  - In the `## Work Order Ledger` table, mark `wo4-offline-retrieval` as `✅ Complete` and `wo5-pilot` as `☐ Ready (Workstation Layer 3)`.
  - Append a `### 2026-10-16 — WO4 Complete & Layer 3 Workstation Handover Prepared (PR-14 / PR-15 WO1)` entry to the `## Session Log` summarizing the `v2.2` combined index (`1,283` entries, `3,203` anchors, `54` dossiers), the Read-Side (`90%–100%` Precision@10, `71`/`86` persistent entries) and Write-Side (`21.69%` milestone selectivity, `74.2%–94.0%` `PrepareSession` reduction) micro-probes, the compiled-binary 1-task pilot (`VERDICT: GO`), and the committed `WO4`–`WO7` workorders, blind rubrics (`evaluation/eval-v2/rubrics/`), and `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`.

- [x] **4. Create blind grading rubrics and scorer in `evaluation/eval-v2/rubrics/`:**
  - **`evaluation/eval-v2/rubrics/task-1-temperature-module.yaml`:**
    - Structured YAML blind convention & pitfall rubric for `task-1-temperature-module.txt` (Waterproof DS18B20-Compatible Temperature Sensor Module), authored blind to condition from `esp32-app-foundation` project rules and interfaces:
      1. `id: t1_data_source_and_config` (`weight: 20`): Implements `IDataSource` (`fetchUpdate(const FetchContext&)` or `fetchUpdate`) and `IConfigurable` storing update interval in `/mod/<ns>.json` clamped to `[1s, 15m]` (`1000`–`900000` ms or `1`–`900` s).
      2. `id: t1_u8g2_carousel_board` (`weight: 20`): Implements `IDisplayBoard` (`render(U8G2&)`, `isExpired()` returning `false` for permanent carousel residency, `canAdvance()`).
      3. `id: t1_web_portal_top_level_tab` (`weight: 20`): Implements `IWebContributor` (`registerRoutes`, `getApiPrefix()`) and registers a top-level navigation tab via `ITabRegistry::addTab` with `parent = nullptr` and bare 24x24 inner SVG geometry (no outer `<svg>` wrapper tag).
      4. `id: t1_mcp_contributor_stateless` (`weight: 20`): Implements `IMCPContributor::registerMCPTools(MCPToolRegistry&)` using a stateless non-capturing lambda (`[]`) and explicit `mutatesState` flag (`false` for read telemetry, `true` for config mutation).
      5. `id: t1_application_unified_registration` (`weight: 10`): Declares the module at static/member scope (ADR-16, no raw `new` heap leak without ownership) and wires it via `Application::registerModule()` (and `#ifdef WITH_DISPLAY_U8G2` guard where applicable).
      6. `id: t1_native_tests_and_non_blocking` (`weight: 10`): Includes unit tests under `test/`, passes `pio test -e native`, and avoids blocking `delay()` on the main loop.
  - **`evaluation/eval-v2/rubrics/task-2-bitcoin-price-module.yaml`:**
    - Structured YAML blind convention & pitfall rubric for `task-2-bitcoin-price-module.txt` (Bitcoin Price Module via CoinMarketCap API once per hour):
      1. `id: t2_data_source_and_hourly_tier` (`weight: 20`): Implements `IDataSource` (`fetchUpdate(const FetchContext&)`) with `PriorityTier::Low` (or 1-hour / `3600s` / `3600000ms` cadence) and returns accurate `UpdateStatus` (`Success`, `RateLimited`, etc.).
      2. `id: t2_tls_and_streaming_json_safety` (`weight: 20`): Uses `FetchContext` shared transport without calling `setInsecure()` and without allocating a class-member/stack `WiFiClientSecure`; parses payload safely (e.g. `JsonStreamingParser` or bounded parser without unbounded heap allocation).
      3. `id: t2_u8g2_carousel_board` (`weight: 15`): Implements `IDisplayBoard` (`render(U8G2&)`, `isExpired() == false`, non-blocking native U8G2 draw calls).
      4. `id: t2_web_portal_contributor` (`weight: 15`): Implements `IWebContributor` (`registerRoutes(PsychicHttpServer&)`, `getApiPrefix()`) and exposes Bitcoin price status in the portal.
      5. `id: t2_mcp_contributor_stateless` (`weight: 15`): Implements `IMCPContributor::registerMCPTools(MCPToolRegistry&)` with stateless `[]` lambda and `mutatesState = false` for price read tool.
      6. `id: t2_application_and_native_tests` (`weight: 15`): Wires module via `Application::registerModule()` at static scope (ADR-16), adds unit tests under `test/`, and passes `pio test -e native` with no `delay()` calls.
  - **`evaluation/eval-v2/rubrics/grade_run.sh` (`chmod +x`):**
    - Executable bash script: `grade_run.sh --run-dir <path-to-run-or-archive> [--task {1,2,auto}] [--json <out.json>] [--md <out.md>]`
    - Must operate **blind to condition** (do NOT inspect `condition` in `run_metadata.json` when computing rubric points; only record `run_id` and `task`).
    - Combines `git-diff-baseline.patch` (or `git-diff.patch`) and all files under `untracked/` (or `repo/` if still present) into a unified source inspection corpus.
    - Evaluates each rubric check deterministically for Task 1 or Task 2, checks for `native_test_result.txt` / `pio test -e native` pass status if recorded, checks `check_contamination.sh` on `transcript.jsonl` if present, extracts token usage from `transcript.jsonl` if present, and outputs a clean JSON scorecard (`scorecard.json`) and Markdown report (`scorecard.md`).
    - Test `grade_run.sh` against the archived pilot runs (`evaluation/eval-v2/runs/pilot-t1-a-v22/archive` and `evaluation/eval-v2/runs/pilot-t1-c-v22/archive` if present, or a synthetic test fixture in `/tmp`) to verify it runs cleanly with exit code `0`.

- [x] **5. Create `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md`:**
  - Author a comprehensive, copy-pasteable workstation guide and agent bridge prompt containing:
    1. **Workstation Prerequisites:** macOS (or Linux) machine with `git`, `go` ($\ge 1.22$), `sqlite3`, `python3`, PlatformIO CLI (`pio`), and Antigravity CLI (`agy`) installed, plus network access to clone `matthewmcneill/esp32-app-foundation` during template staging (`prepare_templates.sh`).
    2. **Step-by-Step Terminal Pre-Flight Commands (copy-pasteable block):**
       ```bash
       git fetch origin
       git checkout pr-15-v2-learning/wo5-verification-and-skill-lint # or current branch
       go build -o ~/.local/bin/repo-memory ./cmd/repo-memory
       export PATH="$HOME/.local/bin:$PATH"
       bash evaluation/eval-v2/scripts/prepare_templates.sh
       bash evaluation/eval-v2/scripts/ verify_template.sh evaluation/eval-v2/template
       bash evaluation/eval-v2/scripts/build_index.sh --clean
       bash evaluation/eval-v2/scripts/verify_protection.sh
       ```
       (Note: make sure there is no space after `scripts/` in `bash evaluation/eval-v2/scripts/verify_template.sh evaluation/eval-v2/template`!)
    3. **Sidecar / Global Plugin Isolation Checklist:** Exact commands to pause global `repo-memory-daemon` sidecar and `BugFixer` during evaluation batch runs (and restore them afterwards) per `evaluation/eval-v2/PROTECTION.md`.
    4. **Copy-Pasteable Agent Bridge Prompt (````markdown ... ``` `):**
       - A self-contained prompt that the project owner can paste directly into a new Antigravity / Claude Code session on the workstation to act as the Principal Architect / Evaluation Executor for `pr-10-eval-v2` starting at `WO5` (`wo5-pilot/WORKORDER.md`) $\to$ `WO6` (`wo6-main-evaluation/WORKORDER.md`) $\to$ `WO7` (`wo7-report/WORKORDER.md`).
       - Must list all file paths to page in (`PROJECT.md`, `chief-architect-handover.md`, `wo5-pilot/WORKORDER.md`, `wo6-main-evaluation/WORKORDER.md`, `wo7-report/WORKORDER.md`, `evaluation/eval-v2/rubrics/grade_run.sh`, and `docs/evaluations/v2-2-micro-probe-verdict.md`).

## Verification Requirement

1. Run `evaluation/eval-v2/rubrics/grade_run.sh --help` and test `grade_run.sh` on both a Condition A archive and a Condition C archive (verifying blind scoring produces valid JSON and Markdown).
2. Run a repository hygiene check confirming zero internal identifiers or hardcoded local home paths exist in any newly created/modified files.
3. Run `go test ./cmd/... ./internal/...` to ensure everything remains green.
4. Commit your changes on branch `pr-15-v2-learning/wo1-layer3-eval-bridge-and-handover` (author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`), then **STOP and send a message back to the Principal Architect** confirming verification and awaiting approval to append the Completion Report.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `.agents/skills/principal-architect-workspace/pr-10-eval-v2/chief-architect-handover.md` — Authored comprehensive `v2.2` Layer 3 evaluation architecture and handover context (`Q1` & `Q2`, 2×2 factorial matrix, Stage 0/0b/1 foundations, `PR-10a`–`PR-13` `v2.2` upgrades, `PR-14` Layer 2 micro-probe & compiled-binary pilot verdict, and `WO5` $\to$ `WO6` $\to$ `WO7` workstation execution sequence).
- `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo4-offline-retrieval/WORKORDER.md` — Authored `WO4` workorder with completed `## Completion Report` (`status: complete`) referencing `docs/evaluations/v2-2-read-side-micro-probe.md`, `docs/evaluations/v2-2-write-side-micro-probe.md`, and `docs/evaluations/v2-2-micro-probe-verdict.md`.
- `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo5-pilot/WORKORDER.md` — Authored `WO5` workorder for Stage 2a live 1-task workstation pilot (`pilot-t1-a-live` vs `pilot-t1-c-live`) with `pio test -e native`, `eval_run.sh --archive`, `check_contamination.sh`, `grade_run.sh`, and 7.5M token cap sizing.
- `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo6-main-evaluation/WORKORDER.md` — Authored `WO6` workorder for Stage 2b pre-registered counterbalanced interleaved 2×2 factorial evaluation across Task 1 and Task 2 (Replicate 1 & Replicate 2 reversed).
- `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo7-report/WORKORDER.md` — Authored `WO7` workorder for final statistical synthesis (`docs/evaluations/eval-v2-report.md`) per Addendum A §9 (A10), including Wilcoxon / paired t-test, Holm step-down correction, 95% confidence intervals, read+write cost/quality frontier, and mandatory `## What this does not show` section.
- `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md` — Updated status to `🔄 Active — WO4 complete (Layer 2 GO); WO5 ready for workstation Layer 3 execution`, updated Work Order Ledger (`wo4-offline-retrieval` `✅ Complete`, `wo5-pilot` `☐ Ready (Workstation Layer 3)`), and appended the `2026-10-16` Session Log entry.
- `evaluation/eval-v2/rubrics/task-1-temperature-module.yaml` — Created blind 6-criterion (100-point) architectural & convention rubric for `task-1-temperature-module.txt`.
- `evaluation/eval-v2/rubrics/task-2-bitcoin-price-module.yaml` — Created blind 6-criterion (100-point) architectural & convention rubric for `task-2-bitcoin-price-module.txt`.
- `evaluation/eval-v2/rubrics/grade_run.sh` — Created executable (`chmod +x`) blind grading script that inspects `git-diff-baseline.patch` and untracked source files (excluding `.memory/`, `.agents/`, `GEMINI.md`, `CLAUDE.md`, and `AGENTS.md`), checks `native_test_result.txt` / transcript test outcomes, runs `check_contamination.sh`, extracts token telemetry, and writes `scorecard.json` and `scorecard.md`.
- `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md` — Authored workstation prerequisites, copy-pasteable terminal pre-flight commands, background daemon disable/restore procedures, and the self-contained agent bridge prompt for `WO5` $\to$ `WO6` $\to$ `WO7`.

### Decisions Made
- Excluded `.memory/`, `.agents/`, `GEMINI.md`, `CLAUDE.md`, and `AGENTS.md` from `grade_run.sh`'s source inspection corpus so that seeded dossiers and managed instruction blocks in Condition C/D archives never leak into source-code rubric evaluation, preserving strict blindness to condition.
- Stripped C/C++ line and block comments (`strip_cpp_comments`) before evaluating forbidden patterns (`<svg[\s>]`, `\bdelay\s*\(`, `\bsetInsecure\s*\(`) so explanatory comments (e.g., `// no outer <svg> tag`) do not trigger false-positive penalties.

### Dead Ends & Warnings
- In Python, `rel_path.lstrip("./")` strips any leading `.` or `/` characters, turning `".memory/ATLAS.md"` into `"memory/ATLAS.md"` and bypassing a `".memory/"` prefix filter. Used explicit prefix stripping (`while rel.startswith("./"): rel = rel[2:]`) in `grade_run.sh` to ensure dot-directories (`.memory/`, `.agents/`, `.git/`, `.pio/`) are reliably excluded.

### Out-of-Scope Findings
- Pre-existing `evaluation/eval-v2/PROTECTION.md` line 70 (authored in `pr-10-eval-v2/wo2-protection-harness`) contains a hardcoded `/Users/...` example path alongside the portable `~/Documents/Projects/esp32-app-foundation` path; left untouched as `PROTECTION.md` is outside `WO1`'s Strict Boundary Box.

### Blockers
(none)
