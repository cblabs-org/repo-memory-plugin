# Bug-Fixer Loop: Trajectory Investigation Report

**Date:** 2026-10-03  
**Conversations analyzed:** 8 (covering 4 issues across `matthewmcneill/esp32-app-foundation`)

---

## Executive Summary

The bug-fixer loop encountered a **re-dispatch amplification bug** where agents that hit terminal blockers (sandbox permission denials, forbidden commands, or file-not-found errors after context compaction) entered an **unrecoverable ERROR_MESSAGE loop**. The dispatcher's liveness system then detected these as "stalled/orphaned" and spawned **replacement agents for the same issue**, which in several cases hit the same blockers — creating a wasteful cycle of spawn → block → stall → re-spawn.

> [!CAUTION]
> **Root cause:** The system has no mechanism to distinguish between *transient* stalls (crash, timeout) and *structural* blockers (forbidden commands, sandbox denials) that will recur on every retry. The liveness sweep treats all stalls identically, leading to infinite re-dispatch loops for structurally blocked issues.

---

## Issue Chain Analysis

### Issue #245 — 3 conversations, 421 total steps

| Conv ID | Type | Steps | Terminal State | Root Cause |
|---------|------|-------|----------------|------------|
| `2f3a1e9b` | fresh_fix | 220 | ERROR_MESSAGE loop | `pio test` denied by user at sandbox prompt ("see rule 5") |
| `07eb7d91` | reattach | 115 | GENERIC (working) | Picked up work from `2f3a1e9b` |
| `dafaecfd` | reattach | 86 | **Completed** ✅ | Finished the PR successfully |

**What happened:** Agent tried to run `pio test -e native -f test_otaUpdateManager` with `BypassSandbox=true`. User denied permission with message "see rule 5". The agent then entered an ERROR_MESSAGE death spiral (steps 212–219 all empty ERROR_MESSAGE). The dispatcher detected it as stalled and reattached — twice — before one succeeded.

### Issue #278 — 3 conversations, 401 total steps  

| Conv ID | Type | Steps | Terminal State | Root Cause |
|---------|------|-------|----------------|------------|
| `b133049e` | fresh_fix | 140 | ERROR_MESSAGE loop | Sandbox/permission errors |
| `a2ab7702` | reattach | 116 | **Completed** ✅ | Finished the PR successfully |
| `950cd232` | reattach | 145 | **Completed** ✅ | Duplicate — also finished (redundant) |

**What happened:** Two reattach agents ran concurrently for the same issue. Both completed successfully, meaning one was entirely redundant — wasting tokens and a concurrency slot.

### Issue #279 — 1 conversation, 590 steps

| Conv ID | Type | Steps | Terminal State | Root Cause |
|---------|------|-------|----------------|------------|
| `d98f0d0f` | fresh_fix | 590 | ERROR_MESSAGE loop | File not found after context compaction |

**What happened:** Agent was doing massive member-rename work (165 `_camelCase` fixes). At step 573, context was compacted. The agent then tried to read `.agents/rules/01-architecture-boundary.md` which didn't exist in the worktree, causing an error. This triggered the ERROR_MESSAGE death spiral (steps 576–590). The liveness resume message was injected (step 585) but the agent couldn't recover from the error state.

### Issue #280 — 1 conversation, 571 steps

| Conv ID | Type | Steps | Terminal State | Root Cause |
|---------|------|-------|----------------|------------|
| `976b0c3b` | fresh_fix | 571 | ERROR_MESSAGE loop | Sandbox errors throughout, terminal error unclear |

**What happened:** Agent had repeated sandbox errors throughout the conversation (steps 2, 6, 8, 403, 405, 437, 449) — every time it tried to access files outside the sandbox boundary. It worked around them using `BypassSandbox=true` each time, but eventually entered the ERROR_MESSAGE loop. The liveness system sent an iteration message (step 563) but the agent was already in the unrecoverable state.

---

## Root Causes Identified

### 1. ERROR_MESSAGE Death Spiral (Critical)

**Pattern:** When an agent hits certain terminal errors (sandbox denial, file-not-found, permission check failure), it enters a state where it emits repeated empty `ERROR_MESSAGE` steps and cannot recover. Even when the liveness system injects a SYSTEM_MESSAGE (resume prompt), the agent continues emitting empty errors.

**Evidence:** All 4 "fresh_fix" conversations ended this way:
- `2f3a1e9b`: 8 consecutive empty ERROR_MESSAGE steps after sandbox denial
- `b133049e`: Similar pattern 
- `d98f0d0f`: 15+ empty ERROR_MESSAGE steps spanning the resume attempt
- `976b0c3b`: 16+ empty ERROR_MESSAGE steps spanning the resume attempt

**Impact:** The agent burns its entire step budget doing nothing, then the dispatcher sees it as "stalled" and spawns a replacement.

### 2. No Structural Blocker Detection in Dispatcher (Critical)

The dispatcher's liveness detection ([`liveness.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/liveness.py#L103-L120)) only checks:
- Is the transcript mtime recent? (< 300s)
- Is the last step a PLANNER_RESPONSE with no tool_calls?

It does **not** check:
- Whether the conversation ended in an ERROR_MESSAGE loop
- Whether the blocker was structural (forbidden command, sandbox denial)
- How many times this issue has already been dispatched and failed
- Whether the previous agent's transcript shows a pattern that will recur

### 3. Missing Per-Issue Dispatch Counter (High)

The circuit breaker in [`engineering_dispatcher.py`](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/engineering_dispatcher.py#L836-L855) only tracks **consecutive spawn failures** (line 837–855) — i.e., when `agentapi new-conversation` itself fails. It does **not** track:
- How many times an issue has been dispatched then stalled
- Whether a re-dispatch for the same issue is likely to hit the same blocker

### 4. Duplicate Reattach for Same Issue (#278)

Issue #278 got two concurrent reattach agents (`a2ab7702` and `950cd232`). Both ran to completion. This suggests the dispatcher's concurrency guard doesn't prevent two reattach dispatches for the same issue in the same tick cycle.

**Root:** The stale claim recovery loop (lines 430-548 in `engineering_dispatcher.py`) iterates over all `.lock` files and spawns reattach conversations in sequence. If two lock files exist for the same issue (shouldn't happen, but may via race conditions), or if the liveness check misreads the first reattach as stale, a duplicate can be spawned.

### 5. Sandbox Friction Pattern (Medium)

The agents running in `esp32-app-foundation` worktrees repeatedly hit sandbox restrictions accessing plugin scripts (e.g., `safe_ingest.py`) which are in the plugin directory outside the worktree's sandbox boundary. They worked around this with `BypassSandbox=true`, but this adds friction and sometimes the user denies the bypass.

---

## Recommended Fixes

### Fix 1: Per-Issue Dispatch Failure Tracking (Prevents re-dispatch loops)

Add a per-issue failure counter in the state directory. When an issue's agent stalls/errors out, increment the counter. If the counter reaches a threshold (e.g., 2), escalate to `bug-fixer:human:needs-intervention` instead of re-dispatching.

```python
# In engineering_dispatcher.py — sweep_dispatches recovery
def _record_issue_failure(issue_num: int) -> int:
    failures_file = STATE_DIR / "issue_failures.json"
    counts = json.loads(failures_file.read_text()) if failures_file.exists() else {}
    counts[str(issue_num)] = counts.get(str(issue_num), 0) + 1
    failures_file.write_text(json.dumps(counts))
    return counts[str(issue_num)]

MAX_ISSUE_DISPATCH_FAILURES = 2
```

### Fix 2: Detect ERROR_MESSAGE Death Spirals in Liveness

Enhance `is_stalled()` in `liveness.py` to detect the ERROR_MESSAGE death spiral:

```python
def is_in_error_loop(conv_id: str, threshold: int = 3) -> bool:
    """Detect conversations stuck in ERROR_MESSAGE loops."""
    p = transcript_path(conv_id)
    if not p.exists():
        return False
    try:
        with open(p, "rb") as f:
            f.seek(0, 2)
            read_size = min(f.tell(), 65536)
            f.seek(-read_size, 2)
            chunk = f.read(read_size).decode("utf-8", errors="replace")
            lines = chunk.strip().split("\n")
            # Check last N steps for consecutive ERROR_MESSAGE
            consecutive_errors = 0
            for line in reversed(lines):
                step = json.loads(line)
                if step.get("type") == "ERROR_MESSAGE":
                    consecutive_errors += 1
                else:
                    break
            return consecutive_errors >= threshold
    except (json.JSONDecodeError, OSError):
        return False
```

### Fix 3: Classify Blocker Type Before Re-dispatch

When recovering a stalled dispatch, classify the blocker as transient or structural:

- **Structural (do NOT re-dispatch):**
  - "tool call denied by pre-tool hook" → forbidden command
  - "user denied permission" → sandbox denial
  - ERROR_MESSAGE death spiral (≥3 consecutive)
  
- **Transient (safe to re-dispatch):**
  - Plan gate stall ("shall I proceed")
  - Generic timeout
  - Crash without error pattern

### Fix 4: Prevent Duplicate Reattach per Issue

Add a seen-set in the stale claim recovery loop:

```python
reattached_issues = set()

for lockfile in CLAIMS_DIR.glob("*.lock"):
    # ... existing logic ...
    issue_num = claim["issue"]
    
    if issue_num in reattached_issues:
        log(f"Issue #{issue_num} — already reattached this cycle, skipping.")
        continue
    
    # ... reattach logic ...
    reattached_issues.add(issue_num)
```

### Fix 5: Add Blocker Context to Dispatch Records

When `sweep_dispatches` detects a stalled conversation, record the blocker classification in the dispatch tracking file before recovery. This provides an audit trail and enables the per-issue failure counter to distinguish failure modes.

---

## Waste Analysis

| Issue | Conversations | Steps Wasted | Outcome |
|-------|--------------|--------------|---------|
| #245 | 3 (1 wasted) | ~220 | Eventually fixed after 2 retries |
| #278 | 3 (1 redundant) | ~145 | Fixed, but duplicate ran needlessly |
| #279 | 1 | ~15 (error loop) | Still in progress after compaction error |
| #280 | 1 | ~16 (error loop) | Still in progress, received iteration message in error state |

**Total wasted steps:** ~400+ steps across conversations that either failed terminally or were redundant.

---

## Pre-Tool Gate False Positive (Bonus Finding)

During this investigation, I triggered the `pre_tool_gate.py` RAW_GH_READ_REGEX myself by running a `grep` command that *searched for* the strings `gh issue list` and `gh pr list` in transcript files. The regex at [line 86](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/pre_tool_gate.py#L86) matches these strings even inside grep patterns, causing a false positive denial.

**The regex:**
```python
RAW_GH_READ_REGEX = re.compile(r"(?:^|[;&|]\s*|\$\(\s*|\s)gh\s+(?:issue|pr)\s+(?:list|view)\b")
```

**Should be updated** to exclude cases where `gh issue list` appears as an argument to `grep`, `cat`, `echo`, or inside quotes as a search pattern rather than an actual command invocation.
