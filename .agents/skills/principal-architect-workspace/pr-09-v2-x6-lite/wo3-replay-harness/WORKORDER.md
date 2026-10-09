---
complexity: standard
role: Test Harness Engineer
objective: Build a replay harness that checks out parent commits and runs workers on WO text under each condition.
boundary_box: `internal/core/eval/` or a dedicated test harness directory. Do not run the full evaluation suite yet.
tooling: Go standard library, existing PR-03 eval harness components.
---

# WO3: Build Replay Harness

You are the Test Harness Engineer. Your task is to build the replay harness for the X6-lite evaluation.

## Context
We are executing PR-09-v2-x6-lite to determine if delivered dossiers improve agent performance. WO1 has defined the protocol, and WO2 is building the data. Your job is to build the machinery to run the tests.

## Instructions

1. **Review Existing Harness:** Look at the eval harness built in PR-03 (`internal/core/eval/`).
2. **Build Replay Harness:** Write a harness (e.g. a Go script or test file) that can:
   - Check out the parent commit of a given Work Order.
   - Inject the specific condition: (a) no memory, (b) time-cut PA documents, or (c) time-cut dossiers.
   - Run a worker (using the headless executor or a similar test driver) on the Work Order's text.
   - Collect and parse the exit test results (pass/fail).
   - Collect secondary metrics (steps taken, total tokens used).
3. **Condition Injection:** 
   - For condition (b), the harness should place the time-cut PA documents in the workspace.
   - For condition (c), the harness should configure the delivery engine to use the time-cut dossiers.
4. **Output Format:** Ensure the harness outputs results in a structured format (e.g. JSON or CSV) that can be easily analyzed in WO5.

## Verification
- Run a dry-run or unit test of the harness to ensure it correctly checks out commits and swaps configurations without executing expensive LLM calls.

## Completion Report Template

```markdown
## Completion Report: WO3 Replay Harness

### Decisions Made
- Harness entry point: ...
- Injection mechanics: ...

### Dead Ends & Warnings
- [List any issues encountered]
```

## Completion Report: WO3 Replay Harness

### Decisions Made
- Harness entry point: `internal/eval/x6_replay.go`, providing a `ReplayHarness` struct with methods `CheckoutParent`, `InjectCondition`, and `RunWorker`.
- Injection mechanics: Modifying `.memory/config.yaml` to disable memory for condition `a`. For conditions `b` and `c`, memory is enabled and we perform file copies to the expected directories (repo root for PA docs, data dir for dossiers) to simulate the presence of time-cut memory context. `RunWorker` invokes `agy --agent Execution-Worker` via headless `exec.CommandContext` to process the target Work Order autonomously.

### Dead Ends & Warnings
- Encountered a worktree lock while trying to checkout `pr-09-v2-x6-lite/wo3-replay-harness`. Decided to build the harness directly in the active subagent worktree; changes will be committed and merged to the target branch.
