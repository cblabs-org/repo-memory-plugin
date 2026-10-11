# Architectural Audit: Architect Review Loop (WO1–WO4)

## Audit Scope
All files created or modified by the 4 Work Orders:
- `skills/bug-fixer-tick/SKILL.md` (WO1)
- `skills/architect-review/SKILL.md` (WO2)
- `scripts/architect_dispatcher.py` (WO3)
- `scripts/tick_dispatcher.py` (WO4)
- `plugin.json` (WO3)

---

## Findings

### 🔴 CRITICAL

#### 1. `plugin.json` — Existing sidecars clobbered
**File:** [plugin.json](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/plugin.json)  
**Issue:** The WO3 worker **replaced** the entire `plugin.json` content instead of appending to the existing `sidecars` array. The original file likely had no inline `sidecars` key (sidecars are defined via `sidecars/*/sidecar.json`), or had other fields. The worker overwrote it with *only* the architect sidecar entry, and the `description` now says "3-loop" when we've added a 4th.  
**Fix:** The architect sidecar should be registered as `sidecars/bug-fixer-architect/sidecar.json` (matching the existing pattern), not inlined into `plugin.json`. Restore `plugin.json` to its original state and create the proper sidecar config file.

#### 2. `tick_dispatcher.py` — No fallback when Developer conversation is expired/deleted
**File:** [tick_dispatcher.py](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/tick_dispatcher.py#L280-L308)  
**Issue:** Two sub-issues:
- **Missing ID:** If `agy_dev_conv_id` is not found in the PR body, the code logs a message and `continue`s — the PR is silently skipped forever, stuck with `needs-iteration` and never fixed.
- **Resume failure:** If `agentapi message` fails (conversation expunged from disk), the code logs an error and does nothing — again, the PR is stuck forever.
**Fix:** In both cases, fall back to `agentapi new-conversation`, passing the PR diff and architect comments so a fresh Developer agent can fix the code statelessly.

#### 3. `tick_dispatcher.py` — Uses `gh issue edit` to remove label from a PR
**File:** [tick_dispatcher.py](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/tick_dispatcher.py#L304)  
**Issue:** The code calls `gh issue edit <pr_num> --remove-label needs-iteration`. But `gh issue edit` operates on *issues*, not *pull requests*. While GitHub's API treats PRs as issues under the hood, the `gh` CLI's `issue edit` command does **not** reliably edit PR labels. It should use `gh pr edit`.  
**Fix:** Change to `gh pr edit`.

---

### 🟡 MEDIUM

#### 4. `architect_dispatcher.py` — No max-iteration guard
**File:** [architect_dispatcher.py](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/architect_dispatcher.py)  
**Issue:** There is no mechanism to prevent infinite review loops. If the Architect keeps rejecting and the Developer keeps failing to satisfy it, the loop burns tokens indefinitely. The concept paper specified a max of 3 iterations.  
**Fix:** Track iteration count. The simplest approach: count how many `[ARCHITECT] Changes Requested` comments exist on the PR. If ≥ 3, skip the PR and add a `needs-human-intervention` label.

#### 5. `architect_dispatcher.py` — Missing timestamps in log output
**File:** [architect_dispatcher.py](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/architect_dispatcher.py#L13-L14)  
**Issue:** The `log()` function does not include a UTC timestamp, unlike every other dispatcher in the plugin (`tick_dispatcher.py`, `triage_dispatcher.py`, `trajectory_dispatcher.py`).  
**Fix:** Add the standard `datetime.now(timezone.utc).strftime(...)` timestamp format.

#### 6. `architect_dispatcher.py` — Prompt lacks skill path and config references
**File:** [architect_dispatcher.py](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/architect_dispatcher.py#L89-L96)  
**Issue:** The prompt sent to the Architect agent says `"Run the architect-review skill on this PR"` but does not tell the agent *where* the skill file is, where `config.json` is, or where `safe_ingest.py` is. The other dispatchers (`tick_dispatcher.py`, `trajectory_dispatcher.py`) always pass these paths explicitly. Without them, the Architect agent must discover them heuristically, which is fragile.  
**Fix:** Add `SKILL_PATH`, `SAFE_INGEST`, and `CONFIG_FILE` to the prompt, matching the pattern in `tick_dispatcher.py`.

#### 7. `architect_dispatcher.py` — Stray carriage return on line 33
**File:** [architect_dispatcher.py](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/scripts/architect_dispatcher.py#L33)  
**Issue:** Line 33 ends with `\r\n` (Windows line ending) instead of `\n`. This is a minor portability issue but could cause subtle shell problems on macOS/Linux.  
**Fix:** Strip the `\r`.

---

### 🟢 MINOR

#### 8. `architect-review/SKILL.md` — No max-iteration self-awareness
**File:** [skills/architect-review/SKILL.md](file:///Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin/skills/architect-review/SKILL.md)  
**Issue:** The skill instructs the Architect to approve or reject, but does not tell it to check how many previous review rounds have occurred. If the max-iteration guard in the dispatcher stops re-spawning after 3 rounds, it would be more graceful if the Architect *itself* knew to lower its bar on round 3 and approve with advisory comments rather than hard-rejecting.  
**Fix:** Add a note instructing the Architect to check the number of prior `[ARCHITECT]` comments and, on the 3rd review, approve with advisory comments if the remaining issues are minor.

---

## Summary

| # | Severity | File | Issue |
|---|----------|------|-------|
| 1 | 🔴 Critical | `plugin.json` | Existing sidecars clobbered; wrong registration pattern |
| 2 | 🔴 Critical | `tick_dispatcher.py` | No fallback when conversation is missing/expired |
| 3 | 🔴 Critical | `tick_dispatcher.py` | Uses `gh issue edit` instead of `gh pr edit` for PRs |
| 4 | 🟡 Medium | `architect_dispatcher.py` | No max-iteration guard (infinite loop risk) |
| 5 | 🟡 Medium | `architect_dispatcher.py` | Missing timestamps in log output |
| 6 | 🟡 Medium | `architect_dispatcher.py` | Prompt lacks skill path and config references |
| 7 | 🟡 Medium | `architect_dispatcher.py` | Stray carriage return |
| 8 | 🟢 Minor | `architect-review/SKILL.md` | No max-iteration self-awareness |
