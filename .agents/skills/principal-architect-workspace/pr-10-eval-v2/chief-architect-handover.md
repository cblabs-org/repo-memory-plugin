# Chief Architect Handover: `PR-10-eval-v2` (`repo-memory v2.2` Layer 3 Live Evaluation)

**Date:** 2026-10-16  
**Project:** `pr-10-eval-v2`  
**Target Evaluation Repository:** `matthewmcneill/esp32-app-foundation` (frozen baseline commit `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`)  
**Current Stage:** `WO1`–`WO4` Complete (**Layer 2 Verdict: GO**); Ready for `WO5` (Stage 2a Live Workstation Pilot) $\to$ `WO6` (Stage 2b Pre-Registered 2×2 Main Evaluation) $\to$ `WO7` (Final Evaluation v2 Report)

---

## 1. Executive Overview & Research Questions (`Q1` & `Q2`)

This handover governs the execution of **Layer 3 (`PR-10-eval-v2` Stages 2a, 2b, and Final Report)** on a macOS (or Linux) workstation equipped with PlatformIO (`pio`) and the Antigravity CLI (`agy`).

The evaluation answers the programme's two central empirical questions using short, realistic issue-style forward tasks at `HEAD` on `esp32-app-foundation` (~85K LOC C/C++, 387 source files, 187 native test files, 989 unit tests):

1. **`Q1` (`Condition C` vs `Condition A` — Value over No Memory):**
   - *Is `repo-memory v2.2` (push-based architectural & procedural memory) better than no memory (`Condition A`) on build/test quality, blind architectural compliance, revision cycles, and total read-side token efficiency?*
2. **`Q2` (`Condition C` vs `Condition B` & `Condition D` — Value over Manual PA Memory & Additivity):**
   - *Is `repo-memory v2.2` (`Condition C`) better than the Principal Architect (`PA`) skill's manual workspace memory (`Condition B`: 105 project dirs, 435 workorders, ~1.2M tokens) on the **cost/quality frontier** (accounting for both read-side carried cache tokens and amortised write-side maintenance tokens), and are `repo-memory` + `PA` additive (`Condition D`)?*
   - *(Optional Variant `Condition C3`)*: If budget permits within the 7.5M token cap, how does advertised pull-only MCP memory (`Condition C3`) compare against push delivery (`Condition C`) in uptake rate and precision?

### The 2×2 Factorial Conditions

| Condition | Template | `.agents/skills/principal-architect-workspace/` | `repo-memory v2.2` Enrolment & Push Hooks | Purpose |
| :---: | :---: | :---: | :---: | :--- |
| **`a`** | `tpl-a` | Removed (`disabled-skills.txt` disables PA skill) | Off (`REPO_MEMORY_DISABLED=1`) | **No-Memory Baseline** |
| **`b`** | `tpl-b` | Present (105 PA projects, 435 workorders) | Off (`REPO_MEMORY_DISABLED=1`) | **PA Manual Workspace Memory Only** |
| **`c`** | `tpl-c` | Removed (`disabled-skills.txt` disables PA skill) | On (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop` hooks + `v2.2` seed index) | **`repo-memory v2.2` Push Memory Only** |
| **`d`** | `tpl-d` | Present (105 PA projects, 435 workorders) | On (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop` hooks + `v2.2` seed index) | **Combined (`PA` + `repo-memory v2.2`)** |

---

## 2. Completed Foundations (Stages 0, 0b, 1, and Layer 2 Micro-Probes in `PR-10a`–`PR-14`)

### 2.1 Stage 0 & Stage 0b (`PR-09b` & `PR-10-eval-v2` `WO1`–`WO3`)
- **`WO1` (`wo1-template-build`):** Copied `esp32-app-foundation` working tree at `45d4b5d` preserving gitignored generated build artifacts (`managed_components/`, `include/portal_assets/`, `include/publicKey.hpp`, `modules/LVGLDisplay/ui/`), verified `pio test -e native` (**989/989 tests passing**), squashed git history to a single `baseline` commit with zero remotes, and created neutral condition templates (`tpl-a`, `tpl-b`, `tpl-c`, `tpl-d`) verified by `evaluation/eval-v2/scripts/verify_template.sh`.
- **`WO2` (`wo2-protection-harness`):** Implemented the 4-layer local protection harness (`evaluation/eval-v2/PROTECTION.md`, `eval_env.sh`, `verify_protection.sh`, `eval_run.sh`, `check_contamination.sh`). Verified that `GH_TOKEN=invalid`, `GIT_CONFIG_NOSYSTEM=1`, and `GIT_TERMINAL_PROMPT=0` deterministically block both `gh api user` and `git push`.
- **`WO3` (`wo3-index-build`):** Built the hermetic seed pipeline (`evaluation/eval-v2/scripts/build_index.sh`).

### 2.2 `v2.2` Architectural Upgrades (`PR-10a` through `PR-13`)
Stage 1 root-cause analysis exposed two critical bottlenecks in `v2.1`:
1. **The One-Invocation Vanishing Memory Bug (`8d7ad7b` + `d3ab994`):** `v2.1` emitted `MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, and `BuildReinjection` via single-turn `EphemeralMessage` while `ledger.InjectedSet` permanently blocked re-delivery—leaving **0 entries** in the LLM prompt context by Invocation $\ge 2$ when the agent edited code.
2. **The 1–8% Voluntary Pull Rate on Curated Documentation:** Across 77 non-PA sessions in `esp32-app-foundation`, agents voluntarily read `framework-expert/SKILL.md` in only **5%** of sessions, `docs/developer/*.md` in **8%**, and `creating-a-module` in **2%**, while `v2.1`'s PA-only workorder index (`256` entries) had **0 entries** in 4 of the 6 core framework subsystems (`modules/DataManager`, `modules/WebServer`, `modules/MCPServer`, `modules/ConfigManager`).

To resolve both bottlenecks before running expensive live multi-replicate evaluations, `PR-10a` through `PR-13` shipped `repo-memory v2.2`:
- **`PR-10a` (`v2.2` Specs & Design Rev 4 / Addendum A Rev 3):** Codified the 3 cognitive lifecycles (`semantic`, `episodic`, `procedural`), persistent `UserMessage` delivery, `DocSeedSource` (`seed-docs`), warm-cache `Stop` distillation, and Stage 2 consolidation.
- **`PR-10b` (Delivery & Storage Fixes):** Switched persistent moments (`MomentTaskStart`, `MomentFirstTouch`, `MomentFirstEntry`, `BuildReinjection`) to `UserMessage` (`channel: "userMessage"`), added `DetectRewind` to prune rewound ledger records and re-arm dedup after `/rewind`, fixed `ResolveScope` against `pr-*` workorder scope pollution, and added `repoSymbolIndex` to anchor unanchored PA entries to verified repo files.
- **`PR-11` (`seed-docs` & Cognitive Lifecycles):** Implemented `repo-memory seed-docs` (`DocSeedSource`) to extract anchored `invariant`, `contract`, `gotcha`, `recipe`, and `failure-signature` entries from `docs/developer/**/*.md` and hand-written `.agents/skills/**/SKILL.md`, plus async doc freshness ("dreaming") in `tick`.
- **`PR-12` (Stage 1 Warm-Cache Distillation & Preparation Layer):** Implemented `Stop` hook milestone continuation (`executionNum == 0` $\to$ `decision: "continue"`, `executionNum >= 1` $\to$ harvest `repo-memory-note` YAML with `ExposureList` and `AnchorChecks`), zero-token Stage 1b harvest of `CHECKPOINT` summaries and `brain/` artifacts, Stage 1c `PrepareSession` (stripping `[repo-memory]` exposures and stubbing instruction reads), `ValidateDistillerOutput`, and the 4-layer secret scrubber (`scrub.Scrub`).
- **`PR-13` (Stage 2 Warm Batch Consolidation & Five-Tier Publishing):** Implemented `P13` Exposure-Aware Echo Filtering (`FilterEchoes`), `VD18` Pre-Consolidation Corroboration Gating (`EvaluateCorroboration`), exact-duplicate coalescing (`OpAmend`), Five-Tier routing (`ephemeral:`, `session:`, `user:`, `branch:`, `repo:`), and isolated two-lane publishing to the `memory/pending` git worktree.

### 2.3 Layer 2 Micro-Probes & Compiled-Binary Pilot Verdict (`PR-14` / `WO4` — `VERDICT: GO`)
In `PR-14` (`WO1`–`WO4`), all `v2.2` subsystems passed empirical Layer 2 micro-probes and compiled-binary pilot verification:
- **Combined `esp32-app-foundation` Corpus (`SEED_REPORT.md`):** **`1,283` total entries** (`256` `seed` + `1,027` `doc-seed`), **`3,203` verified file anchors**, **`54` projected subsystem dossiers** (`957` `semantic`, `214` `episodic`, `112` `procedural`), and **`0` `ops gc` drops**.
- **Read-Side Micro-Probe ([`docs/evaluations/v2-2-read-side-micro-probe.md`](../../../../docs/evaluations/v2-2-read-side-micro-probe.md)):**
  - **Precision@10 on `MomentTaskStart`:** **`90.0%` (9/10)** on Task 1 (`DS18B20`) and **`90.0%`–`100.0%`** on Task 2 (`Bitcoin`).
  - **Multi-Invocation Persistence:** Retains **`71` active entries** (`14` TaskStart + `57` FirstTouch/FirstEntry, `0` duplicates) in context at edit time on Task 1 and **`86` active entries** (`14` + `72`) on Task 2, covering **6/6 (100%)** of the blind rubric subsystem contracts (`IDataSource`/`FetchContext`, `IConfigurable`, `IDisplayBoard`, `IWebContributor`/`ITabRegistry`, `IMCPContributor` stateless `[]` lambda, and `Application::registerModule()` static scope).
  - **Compaction & Rewind Resilience:** `BuildReinjection` restores **`30` / `39` entries** (`999` / `1,000` tokens) post-`CHECKPOINT`, and `DetectRewind` re-arms **`68` / `82` entries** post-`/rewind`.
- **Write-Side Micro-Probe ([`docs/evaluations/v2-2-write-side-micro-probe.md`](../../../../docs/evaluations/v2-2-write-side-micro-probe.md)):**
  - Across **25 real transcripts (`83` assistant turns)**, `Stop` milestone selectivity is **`21.69%` (`18/83` turns)** (`78.31%` routine turns skipped silently).
  - Stage 1b harvests **`6` items (`3` notes)** at **`0` LLM tokens** (`100%` idempotent).
  - Stage 1c `PrepareTranscript` achieves **`62.30%` corpus-wide byte reduction** (**`74.20%`** on top real sessions, **`93.96%`** on PA sessions) with **`100%`** exposure stripping, **`4/4`** adversarial output rejection, and **`4/4`** credential class redaction.
- **Compiled-Binary 1-Task Pilot ([`docs/evaluations/v2-2-micro-probe-verdict.md`](../../../../docs/evaluations/v2-2-micro-probe-verdict.md)):**
  - `evaluation/eval-v2/scripts/run_pilot_verification.sh` verified Condition A (`pilot-t1-a-v22`, `0/5` architectural compliance) vs Condition C (`pilot-t1-c-v22`, `5/5` architectural compliance + `Stop` note harvest + `consolidate` $\to$ `unconsolidated_notes: 0`), `0` contamination hits, and clean archival.

---

## 3. Workstation Execution Sequence (`WO5` $\to$ `WO6` $\to$ `WO7`)

When pulling this repository onto the macOS evaluation workstation where `pio` and `agy` CLI are installed, execute the remaining three work orders in strict sequence:

### Step 1: Workstation Pre-Flight & Isolation (Before `WO5`)
Follow `evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md` to:
1. Build `~/.local/bin/repo-memory` from HEAD (`go build -o ~/.local/bin/repo-memory ./cmd/repo-memory`).
2. Stage the evaluation templates (`evaluation/eval-v2/scripts/prepare_templates.sh`) from `~/Documents/Projects/esp32-app-foundation` (or existing `evaluation/eval-v2/template`) and verify SHA-256 manifests (`verify_template.sh`).
3. Build the `v2.2` combined index (`evaluation/eval-v2/scripts/build_index.sh --clean`, verifying `1,283` entries).
4. Verify local §10 protection (`evaluation/eval-v2/scripts/verify_protection.sh`).
5. Disable the background `repo-memory-daemon` tick sidecar and `BugFixer` daemon per `evaluation/eval-v2/PROTECTION.md` §5.

### Step 2: `WO5` — Stage 2a Live 1-Task Workstation Pilot ([`wo5-pilot/WORKORDER.md`](wo5-pilot/WORKORDER.md))
- Run **Task 1 (`evaluation/eval-v2/tasks/task-1-temperature-module.txt`)** live with `agy` CLI under:
  - `pilot-t1-a-live` (`Condition A`)
  - `pilot-t1-c-live` (`Condition C`)
- In each run workspace, before archiving, run `pio test -e native` and save the output to `native_test_result.txt` in the run root so it is preserved in `archive/`.
- Run `check_contamination.sh` and `eval_run.sh --archive`.
- Run blind grading via `evaluation/eval-v2/rubrics/grade_run.sh --run-dir evaluation/eval-v2/runs/<run-id>/archive --task 1`.
- Measure total read-side tokens (including `cache_read_input_tokens` / carried context) per run to confirm the Stage 2b replicate schedule within the **7.5M token budget cap** (shrink order if average run $> 450\text{K}$ tokens: keep `a` + `c`, then `b`, then `c3`, then `d`).

### Step 3: `WO6` — Stage 2b Pre-Registered 2×2 Factorial Evaluation ([`wo6-main-evaluation/WORKORDER.md`](wo6-main-evaluation/WORKORDER.md))
- Execute the pre-registered interleaved 2×2 schedule across **Task 1 (`task-1-temperature-module.txt`)** and **Task 2 (`task-2-bitcoin-price-module.txt`)**, Conditions `a`, `b`, `c`, `d`, across **2 replicates**:
  - **Replicate 1 Order:** `run-t1-a-rep1` $\to$ `run-t1-c-rep1` $\to$ `run-t1-b-rep1` $\to$ `run-t1-d-rep1` $\to$ `run-t2-c-rep1` $\to$ `run-t2-a-rep1` $\to$ `run-t2-d-rep1` $\to$ `run-t2-b-rep1`
  - **Replicate 2 Order (Reversed):** `run-t2-b-rep2` $\to$ `run-t2-d-rep2` $\to$ `run-t2-a-rep2` $\to$ `run-t2-c-rep2` $\to$ `run-t1-d-rep2` $\to$ `run-t1-b-rep2` $\to$ `run-t1-c-rep2` $\to$ `run-t1-a-rep2`
- For each run: execute `pio test -e native`, verify zero contamination via `check_contamination.sh`, archive via `eval_run.sh --archive`, and score blind to condition via `evaluation/eval-v2/rubrics/grade_run.sh`.

### Step 4: `WO7` — Final Evaluation v2 Report ([`wo7-report/WORKORDER.md`](wo7-report/WORKORDER.md))
- Synthesize all archived scorecards, test pass rates, revision cycles, and read/write token ledgers into `docs/evaluations/eval-v2-report.md` following Addendum A §9 (A10):
  - Paired per-task differences (`Condition C - Condition A`, `Condition C - Condition B`, `Condition D - Condition C`).
  - Wilcoxon signed-rank test / paired t-test with Holm step-down multiple-testing correction across primary metrics and 95% confidence intervals.
  - Complete read-side (including carried context / cache reads) + amortised write-side token accounting and the **Cost/Quality Frontier** table.
  - Explicit `Q1` and `Q2` verdicts and the mandatory `## What this does not show` honesty clause.
- Re-enable the background `repo-memory-daemon` sidecar and `BugFixer` daemon per `evaluation/eval-v2/PROTECTION.md` §5.
