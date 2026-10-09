---
complexity: interactive
role: Evaluation Executor
objective: Execute the X6-lite runs using the replay harness and collect results.
boundary_box: `internal/eval/x6_replay.go`, execution of tests, parsing results.
tooling: Use the Go test runner/eval harness to run the configurations.
---

# WO4: Execute X6-lite Runs

You are the Evaluation Executor for an interactive task. Your job is to run the pre-registered X6-lite evaluation protocol.

## Context
WO2 generated the time-cut evaluation data, and WO3 built the replay harness. We are now ready to run the evaluation.

## Instructions
1. Review the generated harness (`internal/eval/x6_replay.go`) and the evaluation data in `evaluation/data/`.
2. Schedule and execute the runs according to the budget approved in WO1 (≤72 runs, ≤7M tokens, ~4.5–7 hours).
3. For each of the 8 WOs, run the conditions (a) no memory, (b) time-cut PA documents, and (c) time-cut dossiers across the specified seeds.
4. If a disagreement occurs between the 2 seeds for a WO/condition, execute the 3rd tiebreaker seed.
5. Collect the results: pass/fail on the exit tests, steps taken, and total tokens used.
6. Present the raw results back to the user to confirm completion.
7. Note any anomalies, API errors, or quota issues during the runs.

## Verification
- Work interactively with the user if runs hit quota limits or require batching.
- Ensure the results are safely stored (e.g. in a CSV or JSON file) so WO5 can analyze them.

## Completion Report Template

```markdown
## Completion Report: WO4 Runs

### Decisions Made
- Total runs executed: ...
- Total tokens consumed: ...
- Results format: ...

### Dead Ends & Warnings
- [List any issues encountered during execution]
```
