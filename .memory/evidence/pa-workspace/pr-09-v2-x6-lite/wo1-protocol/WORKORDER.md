---
complexity: interactive
role: Evaluation Protocol Designer
objective: Write the pre-registered X6-lite protocol and present it for human approval.
boundary_box: Write the protocol document only. Do not execute runs yet.
tooling: Read PR history, file history, and session logs to select appropriate candidate WOs.
---

# WO1: Write X6-lite Protocol

You are the Evaluation Protocol Designer. Your task is to write the pre-registered protocol for the X6-lite Early Kill Gate.

## Context
This is the most critical PR in the entire programme: it determines whether the project continues by testing whether delivered dossiers make agents better.

Read the X6 protocol definition from `PROGRAM-PLAN.md` lines 108-128.

## Instructions
1. Select 6–8 completed work orders that have executable exit tests. Choose WOs where earlier knowledge plausibly matters (e.g. they depend on decisions, dead ends, or external behaviour found in earlier WOs). Include at least two where a no-memory run is expected to struggle.
2. Define the exact time cut for each work order (its parent commit).
3. Specify the 3 conditions: (a) no memory, (b) PA documents as of the parent commit, (c) time-cut dossiers.
4. Specify the seeds (at least 2 per WO/condition, 3 if disagreement).
5. Specify the metrics: primary (share of exit tests passing), secondary (steps and total tokens). The pass threshold is fixed: (c) beats (a) on the primary metric.
6. Estimate the quota per run and define a schedule/budget.
7. Present this pre-registered protocol as a Markdown document to the user for explicit approval.

## Verification
- Do NOT begin the runs. This is purely to define the protocol.
- Present the protocol clearly to the user and ask for sign-off.

## Completion Report Template
(Fill this out once the user approves the protocol)

```markdown
## Completion Report: WO1 Protocol

### Decisions Made
- Selected WOs: ...
- Threshold: ...
- Budget: ...

### Dead Ends & Warnings
- None yet.
```


## Completion Report: WO1 Protocol

### Decisions Made
- Selected WOs: 8 work orders from PR-07 (5) and PR-08 (3): wo1-op-log, wo2-dossier-schema-renderer, wo3-scopes-aliases, wo4-index-migration, wo6-seeding, wo1-ledger-engine, wo4-gates, wo6-pull-tools.
- Negative control: wo1-op-log included as a calibration point (low prior-knowledge dependency; all conditions expected to perform similarly).
- Struggle candidates: wo6-seeding (novel time-cut + v1 dead ends) and wo4-gates (X9 hook contract, deny-once lifecycle).
- Threshold: condition (c) dossiers must strictly beat condition (a) no-memory on aggregate exit test pass rate. PA comparison (b) reported but not gating.
- Seeds: 2 per WO/condition (42, 137), 3rd tiebreaker (256) on disagreement.
- Budget: ≤72 runs, ≤7M tokens, ~4.5–7 hours across 1–2 days.
- Exit test paths corrected per actual WORKORDER.md contents: wo4-gates uses `./internal/harness/antigravity/...`, wo6-pull-tools uses `./internal/mcpserver/...`.
- Protocol approved by user on 2026-10-09.

### Dead Ends & Warnings
- PR-06b/wo2 and wo3 have unfilled completion report templates despite code being completed — excluded from selection.
- PR-08/wo3 (ephemeral-and-errors) also has pending template — excluded.
- Several WOs document pre-existing test failures (TestSessionAttributor_Integration) outside their boundary boxes — these must be excluded from pass/fail scoring during X6-lite runs.
- Early-PR candidates (pr-02a/wo4 commit-linkage, pr-05a/wo1 executors, pr-05b/wo1 subsystem-bootstrap) were considered as alternatives with concrete dead-end discrimination. Not selected because late-programme WOs have richer dossiers and tighter dependency chains, but documented as viable substitutions if needed.
