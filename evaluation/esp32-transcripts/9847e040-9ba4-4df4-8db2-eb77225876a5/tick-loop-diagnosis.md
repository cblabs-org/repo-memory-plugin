# 🔴 Tick Loop Diagnosis — Engineering Dispatcher Crash Loop

## Summary

**The engineering dispatcher is crash-looping every 10 minutes** due to a Python variable-shadowing bug. The other 3 loops are technically running but have no work to do.

## Root Cause: `import json` Inside `main()` Shadows the Module-Level Import

In [`engineering_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py#L259-L269), two redundant `import json` statements at **lines 259 and 268** were introduced inside the `main()` function (within the "Step 0.5: Sweep stale worktrees" section):

```python
# Line 259 and 268 — INSIDE main()
import json  # ← This makes Python treat `json` as a LOCAL variable
```

Python's scoping rule: if **any** assignment (including `import`) to a name exists anywhere in a function body, **all** references to that name in that function are treated as local. This means every `json.loads()` and `json.JSONDecodeError` reference that executes **before** line 259 will fail with:

```
UnboundLocalError: cannot access local variable 'json' where it is not associated with a value
```

The crash happens at **line 433** (`json.loads(r_prs.stdout)`) and cascades to line 434 (`except json.JSONDecodeError`), but the actual trigger is lines 259/268.

### Stack Trace (repeating every 10 minutes since at least 06:00 UTC)

```
File ".../engineering_dispatcher.py", line 433, in main
    needs_iteration_prs = json.loads(r_prs.stdout)
UnboundLocalError: cannot access local variable 'json' where it is not associated with a value

During handling of the above exception, another exception occurred:
    except json.JSONDecodeError:
UnboundLocalError: cannot access local variable 'json' where it is not associated with a value
```

## Status of All 4 Loops

| Loop | Sidecar | Status | Detail |
|------|---------|--------|--------|
| **Engineering** | `*/10 * * * *` | 🔴 **Crash-looping** | `UnboundLocalError: json` — every tick since ~06:00 UTC |
| **Triage** | `0 * * * *` | 🟡 Running, no work | "No untriaged issues" — correct behavior |
| **PR Review** | `*/15 * * * *` | 🟡 Running, no work | "No draft bug-fixer PRs found" — correct behavior |
| **Trajectory** | `0 18 * * *` | ⚪ Waiting | Scheduled for 18:00 UTC today — hasn't fired yet |

## Fix

Remove the two redundant `import json` lines (259 and 268) from inside `main()`. The module-level `import json` at line 15 is already correct and sufficient:

```diff
 # Line 257-263
             for lf in CLAIMS_DIR.glob("*.lock"):
                 try:
-                    import json
                     c = json.loads(lf.read_text())
 ...
 # Line 265-272
             for df in DISPATCHES_DIR.glob("*.json"):
                 try:
-                    import json
                     d = json.loads(df.read_text())
```

## Secondary Note: `plugin.json` has `"disabled": true`

[`plugin.json`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/plugin.json) (the source template) has `"disabled": true`, but the **installed workspace copy** at `.agents/plugins/bug-fixer-loop-workspace/plugin.json` has `"disabled": false` — so this is **not** blocking the sidecars. It's an intentional "ship disabled" design pattern per the git history (`d7bf546`).
