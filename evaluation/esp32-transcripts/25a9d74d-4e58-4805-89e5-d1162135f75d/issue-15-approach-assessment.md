# Issue #15 — Approach Assessment

**Issue:** [BugFixer-Health] 24h Trajectory Review: Dispatcher Churn, MCP Lock Contention, and Uncommitted Dispatcher Fix

---

## Sub-Items Status

| # | Sub-Item | Verdict | Action |
|---|----------|---------|--------|
| 1 | Dispatcher re-spawn churn (`status:awaiting-review` exclusion) | ✅ Already fixed | Verify only |
| 2 | PlatformIO MCP lock contention | ⛔ Out of scope | None — maintainer confirmed this belongs in the client repo |
| 3 | AgentAPI offline → label churn | 🔧 **Needs fix** | Add agentapi pre-flight check to all dispatchers |

---

## Sub-Item 1: Dispatcher Re-Spawn Churn Loop

### Analysis

The issue reports that `tick_dispatcher.py` repeatedly claimed issues that already had open PRs because it only excluded `status:in-progress` but not `status:awaiting-review`.

The current code in [`engineering_dispatcher.py` lines 576–579](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py#L576-L579) already has the fix:

```python
eligible = [
    i for i in issues
    if not any(l.get("name") in ("bug-fixer:engineer:in-progress", "bug-fixer:architect:awaiting-review") for l in i.get("labels", []))
]
```

Both `bug-fixer:engineer:in-progress` **and** `bug-fixer:architect:awaiting-review` are excluded. The hot-patch mentioned in the issue body has been committed.

### Recommendation

**No code change needed.** The fix is already in place. Close this sub-item.

---

## Sub-Item 2: PlatformIO MCP Lock Contention

### Analysis

Maintainer explicitly commented:

> "No need to fix the PIO stuff in the bug-fixer — it should be in the client repo where that is handled."

### Recommendation

**No action.** Out of scope.

---

## Sub-Item 3: AgentAPI Offline → Label Churn

### Problem

When `agentapi` is unreachable (e.g., during unattended cron ticks while the IDE is closed), the engineering dispatcher:

1. **Adds `bug-fixer:engineer:in-progress`** to an issue (line 605–606)
2. **Attempts to spawn** via `agentapi new-conversation` — which fails
3. **Removes the label** on failure (line 654–655)
4. **Next tick** (10 min later): repeats steps 1–3

This creates rapid label add/remove churn that trips the circuit breaker across multiple cooldown cycles. The issue reports 5 separate 1-hour cooldowns during a single 6-hour window.

### Current State

- **No dispatcher** (engineering, architect, triage, trajectory) performs any agentapi health check before starting work
- The engineering dispatcher is the worst affected because it **mutates labels before spawning** — the others either don't mutate labels (triage, trajectory) or have lower-consequence failures (architect)
- The circuit breaker in engineering (3 failures → 1hr cooldown) partially mitigates this but doesn't prevent the label churn itself — it just delays the next batch of churn

### Proposed Approach

Add an **agentapi pre-flight health check** as a reusable utility, and gate all dispatcher cycles on it.

#### Implementation Design

```python
# In a shared utility (e.g., liveness.py or a new agentapi_health.py)

def agentapi_is_reachable(timeout: float = 5.0) -> bool:
    """Check if agentapi daemon is responsive.
    
    Since agentapi has no dedicated health endpoint, we probe it by
    running a lightweight command and checking for a timely response.
    `agentapi --help` is side-effect-free and exercises the binary path.
    """
    try:
        r = subprocess.run(
            ["agentapi", "--help"],
            capture_output=True, text=True, timeout=timeout
        )
        return r.returncode == 0
    except (FileNotFoundError, subprocess.TimeoutExpired, OSError):
        return False
```

> [!NOTE]
> `agentapi --help` is the lightest available probe — it verifies the binary is on PATH, the daemon socket is reachable, and it responds within the timeout. There's no `agentapi status` or health endpoint. A failed `--help` reliably predicts that `new-conversation` will also fail.

#### Where to add the gate

| Dispatcher | File | Priority | Rationale |
|---|---|---|---|
| **Engineering** | [`engineering_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py#L212) | **High** | Mutates labels before spawning — label churn is the core symptom |
| **Architect** | [`architect_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/architect_dispatcher.py) | Medium | Spawns conversations but doesn't mutate labels in the dispatcher |
| **Triage** | [`triage_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/triage_dispatcher.py) | Low | No label mutation, just a wasted process |
| **Trajectory** | [`trajectory_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/trajectory_dispatcher.py) | Low | No label mutation, just a wasted process |

#### Placement in engineering_dispatcher.py

Add the check **early in `main()`**, after the cooldown check but before any GitHub interaction:

```python
# Line ~237, after cooldown check, before "Step 0: Sweep dispatches"

# Pre-flight: verify agentapi is responsive before doing any work
if not agentapi_is_reachable():
    log("ENGINEERING_EXIT: agentapi is unreachable — skipping this cycle. "
        "No labels will be modified.")
    sys.exit(0)
```

> [!IMPORTANT]
> The check must be **before Step 0** (dispatch sweeps) because sweep_dispatches also calls `agentapi send-message` for nudges. Running sweeps when agentapi is down would log false "nudge failed" warnings.

#### Should the health check failure count toward the circuit breaker?

**No.** The circuit breaker tracks "spawn failures with eligible work" — an agentapi-down exit is a clean skip, not a failure. The dispatcher should exit with code 0 (success) to avoid polluting failure counts.

### Risk Assessment

**Low risk:**
- The `agentapi --help` probe is side-effect-free
- A false positive (agentapi reports healthy but fails on `new-conversation`) falls through to existing error handling
- A false negative (agentapi is healthy but `--help` times out) just skips one cycle — the sidecar retries in 10 minutes
- No behavioral change when agentapi is working normally

### Files to Modify

| File | Change |
|---|---|
| `scripts/liveness.py` (or new `scripts/agentapi_health.py`) | Add `agentapi_is_reachable()` utility |
| `scripts/engineering_dispatcher.py` | Import + gate at top of `main()` |
| `scripts/architect_dispatcher.py` | Import + gate at top of `main()` |
| `scripts/triage_dispatcher.py` | Import + gate at top of `main()` (optional, low priority) |
| `scripts/trajectory_dispatcher.py` | Import + gate at top of `main()` (optional, low priority) |
| `tests/` | Unit test for the health check function |

### Alternative Considered: Check Before Each `agentapi` Call

Instead of a single pre-flight check, we could wrap every `agentapi` invocation with a health gate. This was rejected because:
- It adds complexity to every call site (6+ locations across dispatchers)
- The failure mode is already handled at each call site (check return code → clean up)
- The real problem is label mutation *before* the agentapi call, not the call itself
- A single early-exit is simpler, more predictable, and easier to test

---

## Summary Recommendation

1. **Sub-item 1:** ✅ Already fixed — close.
2. **Sub-item 2:** ⛔ Out of scope — close.
3. **Sub-item 3:** Implement `agentapi_is_reachable()` + early-exit gates. Start with `engineering_dispatcher.py` (where label churn occurs), then extend to other dispatchers.

The total change is ~20 lines of production code + tests. Low risk, high impact on operational stability during unattended periods.
