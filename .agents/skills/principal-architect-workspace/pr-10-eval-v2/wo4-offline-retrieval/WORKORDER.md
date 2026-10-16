---
name: pr-10-eval-v2-wo4
description: Handoff instructions for Work Order 4 — Stage 1 & Layer 2 Offline Retrieval and Write-Side Micro-Probes
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective

Execute Stage 1 (offline retrieval quality check and instruction-layer pull-rate audit) and the Layer 2 read-side and write-side micro-probes for `repo-memory v2.2` against the frozen forward evaluation tasks (`Task 1: DS18B20 Temperature Sensor Module` and `Task 2: Bitcoin Price Module`) on `esp32-app-foundation`, verifying that all read-side and write-side readiness gates pass before spending live agent tokens in Stage 2a/2b (`WO5`–`WO6`).

## Context & Provenance

- Parent Project: [PROJECT.md](../PROJECT.md) (`pr-10-eval-v2`)
- Handover Context: [chief-architect-handover.md](../chief-architect-handover.md)
- Evaluation Proposal: [docs/evaluations/memory-value-evaluation-proposal.md](../../../../docs/evaluations/memory-value-evaluation-proposal.md)
- Frozen Forward Tasks:
  - `evaluation/eval-v2/tasks/task-1-temperature-module.txt`
  - `evaluation/eval-v2/tasks/task-2-bitcoin-price-module.txt`
- Micro-Probe Scripts & Reports (`PR-14`):
  - `evaluation/eval-v2/scripts/build_index.sh` (`evaluation/eval-v2/SEED_REPORT.md`)
  - `evaluation/eval-v2/scripts/run_read_probe.sh` (`docs/evaluations/v2-2-read-side-micro-probe.md`)
  - `evaluation/eval-v2/scripts/run_write_probe.sh` (`docs/evaluations/v2-2-write-side-micro-probe.md`)
  - `evaluation/eval-v2/scripts/run_pilot_verification.sh` (`docs/evaluations/v2-2-micro-probe-verdict.md`)

## Strict Boundary Box

- **In Scope:**
  - `evaluation/eval-v2/SEED_REPORT.md`
  - `docs/evaluations/v2-2-read-side-micro-probe.md`
  - `docs/evaluations/v2-2-write-side-micro-probe.md`
  - `docs/evaluations/v2-2-micro-probe-verdict.md`
  - `.agents/skills/principal-architect-workspace/pr-10-eval-v2/wo4-offline-retrieval/WORKORDER.md`
- **Out of Scope:**
  - Do NOT modify the frozen task prompt files in `evaluation/eval-v2/tasks/`.
  - Do NOT push or interact with GitHub.

## Tooling Constraints

Use `~/.local/bin/repo-memory`, `./evaluation/eval-v2/scripts/{build_index.sh,run_read_probe.sh,run_write_probe.sh,run_pilot_verification.sh}`, and `go test`.

## Line-Item Instructions

- [x] **1. Audit static documentation voluntary pull rates across 77 historical `esp32-app-foundation` sessions:**
  - Measure voluntary agent read rates on `.agents/skills/framework-expert/SKILL.md` (`5%`), `docs/developer/*.md` (`8%`), `creating-a-module` (`2%`), and `docs/developer/README.md` (`1%`).
  - Confirm that curated documentation behaves as a pull system (`1%–8%` voluntary uptake), motivating `repo-memory seed-docs` (`DocSeedSource`) in `v2.2`.

- [x] **2. Rebuild and verify the combined `v2.2` seed + `doc-seed` index (`evaluation/eval-v2/scripts/build_index.sh`):**
  - Combine `repo-memory seed` (`256` PA workorder entries) and `repo-memory seed-docs` (`1,027` curated developer guide & skill entries) into **`1,283` entries** across **`3,203` verified file anchors** and **`54` subsystem dossiers** (`0` `ops gc` drops).

- [x] **3. Execute the Layer 2 Read-Side Multi-Invocation Micro-Probe (`evaluation/eval-v2/scripts/run_read_probe.sh`):**
  - Verify `MomentTaskStart` Precision@10 (`>= 80%` threshold) on both Task 1 and Task 2.
  - Verify multi-invocation `UserMessage` persistence across Invocations `0..3`, `CHECKPOINT` compaction (`BuildReinjection`), and `/rewind` (`DetectRewind`).
  - Record results in `docs/evaluations/v2-2-read-side-micro-probe.md`.

- [x] **4. Execute the Layer 2 Write-Side Distillation & Consolidation Micro-Probe (`evaluation/eval-v2/scripts/run_write_probe.sh`):**
  - Measure `Stop` milestone selectivity across 25 real transcripts (`83` assistant turns), Stage 1b zero-token harvest, Stage 1c `PrepareTranscript` byte reduction, `ValidateDistillerOutput`, `scrub.Scrub`, and Stage 2 consolidation (`P13` echo filter, `VD18` corroboration gate, Five-Tier routing, and `memory/pending` worktree publish).
  - Record results in `docs/evaluations/v2-2-write-side-micro-probe.md`.

- [x] **5. Execute the Compiled-Binary 1-Task Pilot Verification (`evaluation/eval-v2/scripts/run_pilot_verification.sh`) and synthesize verdict:**
  - Record the consolidated Layer 2 GO / NO-GO gate tables in `docs/evaluations/v2-2-micro-probe-verdict.md`.

## Verification Requirement

1. `./evaluation/eval-v2/scripts/build_index.sh` exits `0` (`1,283` entries, `3,203` anchors, `54` dossiers).
2. `./evaluation/eval-v2/scripts/run_read_probe.sh` exits `0` (`90%–100%` Precision@10, `71`/`86` persistent entries at edit time).
3. `./evaluation/eval-v2/scripts/run_write_probe.sh` exits `0`.
4. `./evaluation/eval-v2/scripts/run_pilot_verification.sh` exits `0`.

## Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
complete

### Changes Made
- `evaluation/eval-v2/SEED_REPORT.md` — Updated with `v2.2` combined index metrics (`1,283` total entries = `256` `seed` + `1,027` `doc-seed`, `3,203` verified file anchors, `54` projected subsystem dossiers, `0` `ops gc` drops).
- `docs/evaluations/v2-2-read-side-micro-probe.md` — Empirical read-side micro-probe report across Task 1 (`DS18B20`) and Task 2 (`Bitcoin`) verifying `90%–100%` Precision@10 on `MomentTaskStart`, `71` and `86` persistent rules retained in context at edit time via `UserMessage`, `6/6 (100%)` subsystem rubric contract coverage, `30`/`39` entries restored post-`CHECKPOINT`, and `68`/`82` entries re-armed post-`/rewind`.
- `docs/evaluations/v2-2-write-side-micro-probe.md` — Empirical write-side micro-probe report across 25 real transcripts (`83` assistant turns) verifying `21.69%` (`18/83`) `Stop` milestone selectivity, `0`-token Stage 1b harvest (`6` items, `100%` idempotent), `62.30%` corpus / `74.20%` top-session / `93.96%` PA-session `PrepareSession` byte reduction, `4/4` adversarial validator rejections, `4/4` secret classes redacted, and Stage 2 consolidation (`P13` echo filtering, `VD18` corroboration gating, Five-Tier routing, and `memory/pending` worktree commits).
- `docs/evaluations/v2-2-micro-probe-verdict.md` — Consolidated GO / NO-GO gate matrix (`5/5` Gate 1 criteria passed, `5/5` Gate 2 criteria passed; **`VERDICT: GO`** for Layer 3 workstation evaluation).

### Decisions Made
- Retained natural templates without artificial "read the docs" prompts so Stage 2 evaluates real-world push (`repo-memory v2.2`) vs. pull (`PA` workspace and static docs), where historical voluntary pull rates across 77 sessions were `1%–8%`.
- Upgraded the evaluation index from PA-only (`256` entries) to the `v2.2` combined `seed` + `seed-docs` corpus (`1,283` entries) so all 6 target `esp32-app-foundation` subsystems have file-anchored semantic, episodic, and procedural coverage.

### Dead Ends & Warnings
- In `v2.1`, single-turn `EphemeralMessage` delivery combined with session-long `ledger.InjectedSet` deduplication left `0` entries in context by Invocation $\ge 2$; switching persistent moments to `UserMessage` (`PR-10b`) resolved this completely.

### Out-of-Scope Findings
(none)

### Blockers
(none)
