---
name: pr-10-eval-v2-wo7
description: Handoff instructions for Work Order 7 — Final Evaluation v2 Report and Cost/Quality Frontier
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Synthesize all archived Stage 2a (`WO5`) and Stage 2b (`WO6`) run scorecards (`scorecard.json`), native test results (`native_test_result.txt`), delivery ledgers (`ledger/*.json`), and token accounting into the final evaluation report at `docs/evaluations/eval-v2-report.md` in strict compliance with Addendum A §9 (`A10: Evaluation discipline`) and the [Evaluation v2 Proposal](../../../../docs/evaluations/memory-value-evaluation-proposal.md).

## Context & Provenance

- Parent Project: [PROJECT.md](../PROJECT.md) (`pr-10-eval-v2`)
- Handover Context: [chief-architect-handover.md](../chief-architect-handover.md)
- Addendum A §9 (A10): [docs/addendum-a-provenance-and-hardening.md](../../../../docs/addendum-a-provenance-and-hardening.md)
- Evaluation v2 Proposal: [docs/evaluations/memory-value-evaluation-proposal.md](../../../../docs/evaluations/memory-value-evaluation-proposal.md)
- Layer 2 Micro-Probe Verdict: [docs/evaluations/v2-2-micro-probe-verdict.md](../../../../docs/evaluations/v2-2-micro-probe-verdict.md)

## Strict Boundary Box

- **In Scope:**
  - `docs/evaluations/eval-v2-report.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo7-report/WORKORDER.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`
- **Out of Scope:**
  - Do NOT modify any `repo-memory` Go source code or alter archived run scorecards retroactively.
  - Ensure all git commits use author `matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>`.

## Tooling Constraints

Use `read_file`, `write_file`, `edit_file`, `python3` (for statistical pairing, Wilcoxon signed-rank / paired t-test, Holm correction, and 95% confidence intervals), and `git`.

## Line-Item Instructions

- [ ] **1. Aggregate All Run Scorecards & Token Ledgers:**
  - Parse `evaluation/eval-v2/runs/*/archive/scorecard.json`, `native_test_result.txt`, and `ledger/*.json` across all valid Stage 2a and Stage 2b runs.
  - Verify that at least 25% of blind rubric scorecards are spot-checked against the raw `git-diff-baseline.patch` and `untracked/` source files per proposal §5.

- [ ] **2. Compute Paired Differences, Statistical Tests & Holm Correction (Addendum A §9 A10):**
  - Treat `task × replicate` as the paired unit of analysis (`Condition C - Condition A` for `Q1`; `Condition C - Condition B` and `Condition D - Condition C` for `Q2`).
  - Compute mean paired differences and **95% confidence intervals** (bootstrap / t-interval) across:
    1. Blind rubric score (`0–100` points)
    2. Objective native test pass rate (`pio test -e native`)
    3. Total read-side tokens (including carried context / `cache_read` tokens)
    4. Tool steps and build/test revision cycles
  - Run Wilcoxon signed-rank test and paired t-test on per-task differences and apply **Holm step-down multiple-testing correction** across the primary metrics.

- [ ] **3. Construct the Read-Side + Write-Side Token Accounting & Cost/Quality Frontier:**
  - Tabulate read-side tokens (input + output + carried `cache_read` tokens) and amortised write-side maintenance tokens (PA workspace authoring cost vs `repo-memory` deterministic seeding + warm-cache `Stop` distillation cost).
  - Present the **Cost/Quality Frontier** table comparing Conditions `A`, `B`, `C`, and `D` (and `C3` if executed).

- [ ] **4. Author `docs/evaluations/eval-v2-report.md`:**
  - Include:
    1. Executive Summary & Explicit Verdicts for **`Q1`** (`Condition C` vs `Condition A`) and **`Q2`** (`Condition C` vs `Condition B` and `Condition D`).
    2. Layer 2 (`PR-14`) + Layer 3 (`Stage 2a/2b`) End-to-End Summary Table.
    3. Per-Run Raw Scorecard & Token Ledger Table (`Task 1` and `Task 2`, Replicates 1 and 2).
    4. Paired Differences, Wilcoxon / Paired t-test p-values, Holm-Corrected p-values, and 95% Confidence Intervals.
    5. Push Precision (`Condition C`/`D` injected entry relevance and 0% harm rate) vs Pull Uptake (`Condition B` PA workspace reads / `Condition C3` MCP tool calls).
    6. Cost/Quality Frontier (Read-Side + Amortised Write-Side Tokens vs Rubric & Test Score).
    7. **Mandatory Section (`## What this does not show`):** Explicitly state the directional nature of 2 forward tasks $\times$ 2 replicates on a single C++ embedded repository (`esp32-app-foundation`), what cannot be extrapolated, and prospective `c2` (self-built memory over multi-week organic capture) follow-up.

- [ ] **5. Restore Workstation Daemons & Update `pr-10-eval-v2/PROJECT.md`:**
  - Re-enable `repo-memory-daemon` (`~/.gemini/config/plugins/repo-memory-daemon/config.json` $\to$ `"enabled": true`) and `BugFixer` daemon per `evaluation/eval-v2/PROTECTION.md` §5.
  - Mark `WO5`, `WO6`, and `WO7` complete in `.agents/skills/principal-architect-workspace/pr-10-eval-v2/PROJECT.md`.

## Verification Requirement

1. `docs/evaluations/eval-v2-report.md` exists, is complete, and includes the mandatory `## What this does not show` section.
2. All paired comparisons, 95% confidence intervals, Holm-corrected p-values, and read+write token tables are internally consistent with `evaluation/eval-v2/runs/*/archive/scorecard.json`.

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
