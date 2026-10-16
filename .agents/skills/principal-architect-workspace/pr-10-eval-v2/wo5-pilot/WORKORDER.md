---
name: pr-10-eval-v2-wo5
description: Handoff instructions for Work Order 5 — Stage 2a Live 1-Task Workstation Pilot (Condition A vs Condition C)
complexity: interactive
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Execute the **Stage 2a Live 1-Task Workstation Pilot** on a macOS (or Linux) workstation where PlatformIO (`pio`) and the Antigravity CLI (`agy`) are installed. Run `evaluation/eval-v2/tasks/task-1-temperature-module.txt` live under **Condition A (`pilot-t1-a-live`)** and **Condition C (`pilot-t1-c-live`)**, execute `pio test -e native` inside each run workspace before archival, verify zero contamination, archive artifacts, run blind grading via `evaluation/eval-v2/rubrics/grade_run.sh`, and measure per-run token cost (including carried `cache_read` tokens) to confirm the Stage 2b (`WO6`) replicate schedule within the **7.5M token cap**.

## Context & Provenance

- Parent Project: [PROJECT.md](../PROJECT.md) (`pr-10-eval-v2`)
- Handover Context: [chief-architect-handover.md](../chief-architect-handover.md)
- Workstation Setup & Bridge Prompt: [evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md](../../../../evaluation/eval-v2/LAYER3_BRIDGE_PROMPT.md)
- Layer 2 Micro-Probe Verdict: [docs/evaluations/v2-2-micro-probe-verdict.md](../../../../docs/evaluations/v2-2-micro-probe-verdict.md)
- Protection Spec: [evaluation/eval-v2/PROTECTION.md](../../../../evaluation/eval-v2/PROTECTION.md)

## Strict Boundary Box

- **In Scope:**
  - `evaluation/eval-v2/runs/pilot-t1-a-live/`
  - `evaluation/eval-v2/runs/pilot-t1-c-live/`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo5-pilot/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md` (Session Log update upon completion)
- **Out of Scope:**
  - Do NOT modify any `repo-memory` Go source code or the frozen task files (`evaluation/eval-v2/tasks/`).
  - Do NOT push or interact with GitHub during agent runs.
  - Ensure all git commits use author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Tooling Constraints

Use `evaluation/eval-v2/scripts/eval_run.sh`, `evaluation/eval-v2/scripts/verify_protection.sh`, `evaluation/eval-v2/scripts/check_contamination.sh`, `evaluation/eval-v2/rubrics/grade_run.sh`, `pio`, and `agy`.

## Line-Item Instructions

- [ ] **1. Pre-Flight & Daemon Isolation Check:**
  - Confirm `pio --version`, `agy --version`, and `~/.local/bin/repo-memory version` succeed.
  - Confirm background daemons (`repo-memory-daemon` sidecar and `BugFixer` daemon) are disabled per `evaluation/eval-v2/PROTECTION.md` §5.
  - Run `./evaluation/eval-v2/scripts/verify_protection.sh` and `./evaluation/eval-v2/scripts/verify_template.sh evaluation/eval-v2/template` and verify both report `PASS`.

- [ ] **2. Stage & Execute Live Pilot Run 1 — Condition A (`pilot-t1-a-live`):**
  - Stage the workspace:
    ```bash
    ./evaluation/eval-v2/scripts/eval_run.sh \
      --condition a \
      --task evaluation/eval-v2/tasks/task-1-temperature-module.txt \
      --run-id pilot-t1-a-live
    ```
  - Execute the live agent session using the generated launch script:
    ```bash
    ./evaluation/eval-v2/runs/pilot-t1-a-live/run_agent.sh
    ```
  - After the agent completes, run `pio test -e native` inside the run workspace and capture the result before archiving:
    ```bash
    (cd evaluation/eval-v2/runs/pilot-t1-a-live/repo && pio test -e native) \
      > evaluation/eval-v2/runs/pilot-t1-a-live/native_test_result.txt 2>&1 || true
    cp evaluation/eval-v2/runs/pilot-t1-a-live/native_test_result.txt \
      evaluation/eval-v2/runs/pilot-t1-a-live/archive/native_test_result.txt 2>/dev/null || true
    ```
  - Archive the run (passing `--conversation-id <CONV_ID>` or `--transcript <PATH>`):
    ```bash
    ./evaluation/eval-v2/scripts/eval_run.sh \
      --archive \
      --run-id pilot-t1-a-live \
      --conversation-id <CONV_ID_A>
    cp evaluation/eval-v2/runs/pilot-t1-a-live/native_test_result.txt \
      evaluation/eval-v2/runs/pilot-t1-a-live/archive/native_test_result.txt
    ```

- [ ] **3. Stage & Execute Live Pilot Run 2 — Condition C (`pilot-t1-c-live`):**
  - Stage the workspace:
    ```bash
    ./evaluation/eval-v2/scripts/eval_run.sh \
      --condition c \
      --task evaluation/eval-v2/tasks/task-1-temperature-module.txt \
      --run-id pilot-t1-c-live
    ```
  - Execute the live agent session:
    ```bash
    ./evaluation/eval-v2/runs/pilot-t1-c-live/run_agent.sh
    ```
  - Run `pio test -e native` inside the run workspace and capture the result before archiving:
    ```bash
    (cd evaluation/eval-v2/runs/pilot-t1-c-live/repo && pio test -e native) \
      > evaluation/eval-v2/runs/pilot-t1-c-live/native_test_result.txt 2>&1 || true
    ```
  - Archive the run:
    ```bash
    ./evaluation/eval-v2/scripts/eval_run.sh \
      --archive \
      --run-id pilot-t1-c-live \
      --conversation-id <CONV_ID_C>
    cp evaluation/eval-v2/runs/pilot-t1-c-live/native_test_result.txt \
      evaluation/eval-v2/runs/pilot-t1-c-live/archive/native_test_result.txt
    ```

- [ ] **4. Run Contamination Audit & Blind Rubric Grading:**
  - Verify zero contamination on both archived transcripts:
    ```bash
    ./evaluation/eval-v2/scripts/check_contamination.sh --run-id pilot-t1-a-live \
      evaluation/eval-v2/runs/pilot-t1-a-live/archive/transcript.jsonl
    ./evaluation/eval-v2/scripts/check_contamination.sh --run-id pilot-t1-c-live \
      evaluation/eval-v2/runs/pilot-t1-c-live/archive/transcript.jsonl
    ```
  - Grade both archives blind to condition using `grade_run.sh`:
    ```bash
    ./evaluation/eval-v2/rubrics/grade_run.sh \
      --run-dir evaluation/eval-v2/runs/pilot-t1-a-live/archive --task 1
    ./evaluation/eval-v2/rubrics/grade_run.sh \
      --run-dir evaluation/eval-v2/runs/pilot-t1-c-live/archive --task 1
    ```

- [ ] **5. Measure Per-Run Token Cost & Confirm Stage 2b (`WO6`) Sizing:**
  - Inspect `scorecard.json` in both archives to record total read-side tokens (including carried context / `cache_read` tokens), steps, and rubric scores.
  - Apply the proposal §8 sizing rule against the **7.5M token cap**:
    - If mean cost per run is $\le 450\text{K}$ tokens: proceed with the full 16-run 2×2 matrix in `WO6` (`2 tasks × 4 conditions {a,b,c,d} × 2 replicates`).
    - If mean cost per run exceeds $450\text{K}$ tokens: apply the pre-registered shrink order (`keep a + c` $\to$ `then b` $\to$ `then c3` $\to$ `then d`; cut replicates before cutting tasks).

## Verification Requirement

1. Both `pilot-t1-a-live/archive/` and `pilot-t1-c-live/archive/` contain `git-diff-baseline.patch`, `transcript.jsonl`, `native_test_result.txt`, `scorecard.json`, and `scorecard.md`.
2. `check_contamination.sh` exits `0` for both runs.
3. Per-run token cost and the confirmed Stage 2b schedule are recorded in the Completion Report below.

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
