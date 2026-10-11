# X6-lite Execution Plan — WO4

## Overview

Execute the X6-lite evaluation: 8 work orders × 3 conditions × 2 seeds = **48 baseline runs** (up to 72 with tiebreakers).

## Run Matrix

| # | Work Order | Source PR | Condition (a) | Condition (b) | Condition (c) |
|---|---|---|---|---|---|
| 1 | `wo1-op-log` | PR-07 | no_memory | pa_documents | dossiers |
| 2 | `wo2-dossier-schema-renderer` | PR-07 | no_memory | pa_documents | dossiers |
| 3 | `wo3-scopes-aliases` | PR-07 | no_memory | pa_documents | dossiers |
| 4 | `wo4-index-migration` | PR-07 | no_memory | pa_documents | dossiers |
| 5 | `wo6-seeding` | PR-07 | no_memory | pa_documents | dossiers |
| 6 | `wo1-ledger-engine` | PR-08 | no_memory | pa_documents | dossiers |
| 7 | `wo4-gates` | PR-08 | no_memory | pa_documents | dossiers |
| 8 | `wo6-pull-tools` | PR-08 | no_memory | pa_documents | dossiers |

**Seeds:** 42, 137 (baseline). Seed 256 added as tiebreaker where results disagree.

## Budget

| Metric | Budget | Estimated Per-Run |
|---|---|---|
| Total runs | ≤72 | — |
| Total tokens | ≤7M | ~100K/run |
| Wall clock | ~4.5–7 hrs | ~5–10 min/run |

## Execution Mechanics

Each run invokes:
```
agy --agent Execution-Worker -p <WO_TEXT> --output-format json --project repo-memory-<ID> --dangerously-skip-permissions
```

The harness (`x6_replay.go`) handles:
1. **Checkout** the parent commit of the target WO
2. **Inject** the condition (disable memory / copy PA docs / copy dossiers)
3. **Run** the worker via `agy` headless CLI
4. **Collect** pass/fail, steps, tokens

## Proposed Batch Strategy

> [!IMPORTANT]
> Each `agy` run is a real LLM invocation consuming real quota. We need to batch carefully.

### Batch 1: Pilot (2 runs) — ~20 min
Run one WO under conditions (a) and (c) with seed 42 only to validate the harness end-to-end and calibrate per-run cost.

**Pilot WO:** `wo1-op-log` (negative control, expected to be cheapest)

### Batch 2: Full Wave 1 (16 runs) — ~2.5 hrs
All 8 WOs × condition (a) no_memory × seed 42 and 137.

### Batch 3: Full Wave 2 (16 runs) — ~2.5 hrs
All 8 WOs × condition (c) dossiers × seed 42 and 137.

### Batch 4: PA Comparison (16 runs) — ~2.5 hrs
All 8 WOs × condition (b) pa_documents × seed 42 and 137.

### Batch 5: Tiebreakers (0–8 runs) — ~1 hr
For any WO/condition where seeds 42 and 137 disagree, add seed 256.

## Results Storage

Results will be written to `evaluation/results/x6-lite-results.jsonl` — one JSON line per run:
```json
{
  "work_order": "wo1-op-log",
  "condition": "no_memory",
  "seed": 42,
  "status": "pass",
  "steps_taken": 15,
  "total_tokens": 98000,
  "error": "",
  "timestamp": "2026-10-09T18:30:00Z"
}
```

## Key Questions for You

1. **Pilot first?** Should we run the 2-run pilot to validate the harness before committing to the full 48-run batch?
2. **Parallelism:** How many concurrent `agy` runs can we safely execute? (1 serial is safest for quota tracking, but 2–4 parallel would cut wall time significantly.)
3. **Batch approval:** Are you comfortable with me executing batches autonomously, or do you want to approve each batch before I launch it?
