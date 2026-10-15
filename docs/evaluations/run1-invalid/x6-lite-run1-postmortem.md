# X6-lite Results — Post-Mortem Investigation

> [!CAUTION]
> **RESULTS INVALIDATED.** The second-opinion critique is correct on all major claims. The X6-lite evaluation harness had three critical defects that render the results meaningless. A re-run is required.

## Investigation Findings

### Defect 1: Agents Ran in Main Checkout, Not Worktree ✅ CONFIRMED

**Evidence:** ``x6_runner.sh` line 177` launches `agy` without `cd "${WORKTREE_DIR}"` first. The `--project` flag only names the agy project — it doesn't set CWD.

```bash
# What it does (WRONG):
REPO_MEMORY_HOUSEKEEPING=1 agy --agent "Execution-Worker" -p "${WO_TEXT}" ...

# What it should do:
cd "${WORKTREE_DIR}"
REPO_MEMORY_HOUSEKEEPING=1 agy --agent "Execution-Worker" -p "${WO_TEXT}" ...
```

**Consequence:** Every agent saw the finished main checkout (which contains all completed WO code, tests, and completion reports). Most agents just verified existing code rather than implementing from scratch.

**Verification:** The pilot run's agent output references `<repo-root>/` 7 times, the worktree only once.

**Collateral damage:** The main checkout now has 1,229 lines of uncommitted changes across 18 files left by eval agents editing concurrently.

### Defect 2: Memory Delivery Disabled in ALL Conditions ✅ CONFIRMED

**Evidence:** `REPO_MEMORY_HOUSEKEEPING=1` causes early return in all three hooks:
- ``preinvocation.go:55``: `return []byte("{}"), nil`
- ``pretooluse.go:89``: `return []byte("{}"), nil`
- ``posttooluse.go:61``: `return []byte("{}"), nil`

**Consequence:** The dossier condition injected files into `.memory/dossiers/` in the worktree, but the hooks that would *deliver* that data to the agent were disabled. Even if the agent had been in the worktree, it would never have received the dossier content via the memory system.

**The correct approach for no_memory:** Use `REPO_MEMORY_DISABLED=1` (which also returns early but is the correct env var for "user opted out") and DON'T set HOUSEKEEPING for conditions (b) and (c).

### Defect 3: Exit Tests Ran Against Untouched Worktree ✅ CONFIRMED

**Evidence:** ``x6_runner.sh` line 201` does `cd "${WORKTREE_DIR}"` for exit tests. But since the agent worked in the main checkout, the worktree was never modified.

**Per-WO impact (verified from exit test logs):**

| WO | What Decided the Result | Passes Without Agent? |
|---|---|---|
| wo1-ledger-engine | `[no test files]` — no tests exist at parent commit | ✅ Yes — auto-pass |
| wo4-index-migration | Existing tests pass on parent commit | ✅ Yes — auto-pass |
| wo6-seeding | Existing tests pass on parent commit | ✅ Yes — auto-pass |
| wo6-pull-tools | Existing tests pass on parent commit | ✅ Yes — auto-pass |
| wo2-dossier-schema-renderer | `internal/core/dossiers/` doesn't exist at parent | ❌ Always fails |
| wo4-gates | `TestSessionAttributor_Integration` is clock-dependent and flaky | 🎲 Random |
| wo3-scopes-aliases | Runner overwrites `.memory/config.yaml`, breaking `TestAssignSubsystem`. Seed 42 "passed" via Go test cache (`(cached)`) | ❌ Fails (1 cached fluke) |
| wo1-op-log | `internal/core/ops/` exists with passing tests at parent | ✅ Yes — auto-pass |

**This means 24 of 35 passes were pre-determined by the base commit state, independent of any agent action.** The remaining passes come from test caching (wo3) and flaky tests (wo4-gates).

## Summary of Invalid Results

The aggregate numbers (no_memory 65%, dossiers 67%, pa_documents 71%) measure **the base commit's test state** plus noise from test caching and flaky tests. They tell us nothing about whether memory helps agents complete work orders.

## Required Fixes Before Re-Run

| # | Fix | Priority |
|---|---|---|
| 1 | **`cd "${WORKTREE_DIR}"` before launching agy** | Critical |
| 2 | **Use `REPO_MEMORY_DISABLED=1` only for no_memory; omit for (b) and (c)** | Critical |
| 3 | **Merge into `.memory/config.yaml` instead of overwriting** (preserves subsystems block) | Critical |
| 4 | **Add calibration runs:** "no agent" must fail, "oracle commit" must pass — validates the test actually measures agent work | Critical |
| 5 | **Fail any run where `git status` in main checkout changes** | High |
| 6 | **Score quota failures (agy exit 3) as invalid, not as pass/fail** | High |
| 7 | **Randomize condition order** to decorrelate from quota pressure | Medium |
| 8 | **Use hidden acceptance tests from the real completion commit** | Medium |
| 9 | **Increase sample size or reduce WO count** for statistical power | Medium |

## Token Cost of Invalidated Run

| Item | Value |
|---|---|
| Total tokens consumed | 11,373,208 |
| Wall clock time | ~4 hours (+ 3 hours quota waits) |
| Valid data produced | **None** |

## Recommendation

**A full re-run is required.** The harness defects are fundamental — every single result is unreliable. The fixes above (especially #1, #2, #3, #4) must be implemented before re-running.

Before re-running, also clean up the main checkout:
```bash
git checkout -- .   # Discard eval-agent edits (except WORKORDER.md)
```
