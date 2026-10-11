# W6 Remediation Pilot Report

**Date:** 2026-10-11  
**Fixes tested:** W3 (test-set replacement), W5 (agent diff, analysis_set), W6 (API contract)  
**Runs:** 4 (3 valid, 1 contamination)

---

## Results

| # | WO | Condition | Hidden | Tokens | Delivery | Status |
|---|---|---|:---:|---:|---:|---|
| 1 | wo4-index-migration | no_memory | —/9 | 532,536 | — | ❌ contamination |
| 2 | wo4-index-migration | dossiers | **8/9** | 326,413 | 36 | ✅ |
| 3 | wo1-op-log | no_memory | **10/13** | 315,892 | — | ✅ |
| 4 | wo1-op-log | dossiers | **10/13** | 274,516 | 28 | ✅ |

### Before vs After (same WOs from Block 1 batch)

| WO | Block 1 (a) | Block 1 (c) | W6 Pilot (a) | W6 Pilot (c) |
|---|:---:|:---:|:---:|:---:|
| wo4-index-migration | 0/9 ❌ | 0/9 ❌ | contamination | **8/9** ✅ |
| wo1-op-log | 0/13 ❌ | 0/13 ❌ | **10/13** ✅ | **10/13** ✅ |

> [!TIP]
> **Scores went from compile-fail (0%) to 77-89%.** The API contract fix works — agents can now compile against hidden tests because they have the exact identifier names.

---

## Fix Validation

### W3: Test-set replacement ✅
Log shows `Cleared agent test files from internal/core/index/` and `Cleared agent test files from internal/core/ops/` — agent `_test.go` files are deleted before oracle tests are copied. No more `redeclared` errors.

### W5: Agent diff archive ⚠️ (partially working)
The diff is written to `${RUN_ROOT}/agent-diff.patch` but the archive glob only copied `*.json` and `*.log`. **Fixed:** added `*.patch` and `*.txt` to the archive copy step. Committed as `c4b9967`.

### W5: Analysis set ✅
All 4 rows in JSONL have `"analysis_set": "w6-pilot"` — distinct from prior batch rows.

### W6: API contract ✅
Log shows `API contract appended (now 235 lines)` for wo4-index-migration and `API contract appended (now 217 lines)` for wo1-op-log. Same contract applied to both conditions.

---

## Contamination Issue (Run 1)

**Cause:** The oracle files (`evaluation/oracle/*/API_CONTRACT.md` etc.) were untracked, making `git status --porcelain` unstable. If the tick sidecar or anything touched the main repo between the before/after snapshots, the porcelain output changed.

**Fix:** Committed all oracle files and scripts as `a4f3ecc`. Working tree is now clean.

---

## Token Efficiency Signal

| WO | no_memory | dossiers | Δ% |
|---|---:|---:|---:|
| wo4-index-migration | 532,536 | 326,413 | **−39%** |
| wo1-op-log | 315,892 | 274,516 | **−13%** |

Both WOs used fewer tokens with dossiers. At n=1 this is suggestive, not conclusive.

---

## Remaining Items for Full Re-run

1. ~~Commit oracle files~~ → Done (`a4f3ecc`)
2. ~~Fix archive glob for .patch/.txt~~ → Done (`c4b9967`)
3. Re-run wo4-index-migration/no_memory to replace contaminated result
4. Full Block 1 re-run: 7 WOs × 2 conditions = 14 runs (~4.8M tokens)
