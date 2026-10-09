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
