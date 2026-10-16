---
name: pr-10-eval-v2-wo6
description: Handoff instructions for Work Order 6 — Stage 2b Pre-Registered 2x2 Factorial Evaluation
complexity: interactive
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Execute the **Stage 2b Pre-Registered 2×2 Factorial Evaluation** across `Task 1` (`evaluation/eval-v2/tasks/task-1-temperature-module.txt`) and `Task 2` (`evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`), Conditions `a`, `b`, `c`, `d` (plus optional `c3` advertised-pull variant if within the 7.5M token cap confirmed in `WO5`), across **2 interleaved replicates**. For every run, execute `pio test -e native`, verify zero transcript contamination (`check_contamination.sh`), archive all artifacts (`eval_run.sh --archive`), and compute blind rubric scorecards (`evaluation/eval-v2/rubrics/grade_run.sh`).

## Context & Provenance

- Parent Project: [PROJECT.md](../PROJECT.md) (`pr-10-eval-v2`)
- Handover Context: [chief-architect-handover.md](../chief-architect-handover.md)
- Pilot Work Order: [wo5-pilot/WORKORDER.md](../wo5-pilot/WORKORDER.md)
- Workstation Setup & Bridge Prompt: [evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md](../../../../evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md)
- Blind Grading Rubrics:
  - `evaluation/eval-v2/rubrics/task-1-temperature-module.yaml`
  - `evaluation/eval-v2/rubrics/task-2-bitcoin-price-module.yaml`
  - `evaluation/eval-v2/rubrics/grade_run.sh`

## Strict Boundary Box

- **In Scope:**
  - `evaluation/eval-v2/runs/run-t{1,2}-{a,b,c,d}-rep{1,2}/`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo6-main-evaluation/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md` (Session Log update upon completion)
- **Out of Scope:**
  - Do NOT modify any `repo-memory` Go source code, templates, or frozen task/rubric files once Stage 2b begins.
  - Do NOT push or interact with GitHub during agent runs.
  - Ensure all git commits use author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Tooling Constraints

Use `evaluation/eval-v2/scripts/eval_run.sh`, `evaluation/eval-v2/scripts/check_contamination.sh`, `evaluation/eval-v2/rubrics/grade_run.sh`, `pio`, and `agy`.

## Line-Item Instructions

- [ ] **1. Pre-Registered Interleaved Execution Schedule:**
  - Execute the 16 runs (or the budget-adjusted subset from `WO5` if per-run cost $> 450\text{K}$ tokens) in the following pre-registered counterbalanced order to eliminate time-of-day, caching, or quota drift bias:
    - **Replicate 1 (`rep1`): `[t1-a, t1-c, t1-b, t1-d, t2-c, t2-a, t2-d, t2-b]`**
      1. `run-t1-a-rep1` (`--condition a --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      2. `run-t1-c-rep1` (`--condition c --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      3. `run-t1-b-rep1` (`--condition b --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      4. `run-t1-d-rep1` (`--condition d --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      5. `run-t2-c-rep1` (`--condition c --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      6. `run-t2-a-rep1` (`--condition a --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      7. `run-t2-d-rep1` (`--condition d --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      8. `run-t2-b-rep1` (`--condition b --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
    - **Replicate 2 (`rep2`, Reversed): `[t2-b, t2-d, t2-a, t2-c, t1-d, t1-b, t1-c, t1-a]`**
      9. `run-t2-b-rep2` (`--condition b --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      10. `run-t2-d-rep2` (`--condition d --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      11. `run-t2-a-rep2` (`--condition a --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      12. `run-t2-c-rep2` (`--condition c --task evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`)
      13. `run-t1-d-rep2` (`--condition d --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      14. `run-t1-b-rep2` (`--condition b --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      15. `run-t1-c-rep2` (`--condition c --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
      16. `run-t1-a-rep2` (`--condition a --task evaluation/eval-v2/tasks/task-1-temperature-module.txt`)
    - *(Optional `c3` Advertised-Pull Variant)*: If total tokens after the 16 runs remain well below the `7.5M` cap, run `run-t1-c3-rep1` and `run-t2-c3-rep1` with `hooks.json` disabled (`"enabled": false`) and only `mcp_config.json` active to measure advertised-pull uptake.

- [ ] **2. Per-Run Execution, Native Test Verification, Contamination Check & Archival Protocol:**
  - For each `<RUN_ID>` in the schedule above:
    ```bash
    # 1. Stage run
    ./evaluation/eval-v2/scripts/eval_run.sh \
      --condition <COND> \
      --task <TASK_FILE> \
      --run-id <RUN_ID>

    # 2. Launch agent
    ./evaluation/eval-v2/runs/<RUN_ID>/run_agent.sh

    # 3. Run native unit tests before archiving
    (cd evaluation/eval-v2/runs/<RUN_ID>/repo && pio test -e native) \
      > evaluation/eval-v2/runs/<RUN_ID>/native_test_result.txt 2>&1 || true

    # 4. Archive run (deletes repo/ working copy, preserves diff + untracked + transcript + ledger)
    ./evaluation/eval-v2/scripts/eval_run.sh \
      --archive \
      --run-id <RUN_ID> \
      --conversation-id <CONV_ID>
    cp evaluation/eval-v2/runs/<RUN_ID>/native_test_result.txt \
      evaluation/eval-v2/runs/<RUN_ID>/archive/native_test_result.txt

    # 5. Check transcript contamination
    ./evaluation/eval-v2/scripts/check_contamination.sh --run-id <RUN_ID> \
      evaluation/eval-v2/runs/<RUN_ID>/archive/transcript.jsonl

    # 6. Blind rubric grading
    ./evaluation/eval-v2/rubrics/grade_run.sh \
      --run-dir evaluation/eval-v2/runs/<RUN_ID>/archive \
      --task auto
    ```

- [ ] **3. Cumulative Budget Tracking:**
  - Track cumulative agent tokens after each run. Halt additional runs if cumulative Stage 2a + Stage 2b usage approaches the **7.5M token cap**.

## Verification Requirement

1. Every completed run directory `evaluation/eval-v2/runs/<RUN_ID>/archive/` contains `git-diff-baseline.patch`, `transcript.jsonl`, `native_test_result.txt`, `scorecard.json`, and `scorecard.md`.
2. `check_contamination.sh` exits `0` across all archived runs (any contaminated run is tagged `invalid_contamination` and excluded from primary aggregates).
3. Total Stage 2a + Stage 2b token expenditure is $\le 7.5\text{M}$ tokens.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

```markdown
---
## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete | partial | blocked

### Changes Made
- `path/to/file` — what changed and why

### Decisions Made
- Decision description and rationale

### Dead Ends & Warnings
- What was attempted, why it failed, and what to avoid

### Out-of-Scope Findings
- Finding description and suggested action

### Blockers
(none)
```
