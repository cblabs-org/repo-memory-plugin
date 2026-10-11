# X6-lite Evaluation — Full Status Report

**Date:** 2026-10-11T10:33 BST  
**Branch:** `pr-09-v2-x6-lite/wo4-runs`  
**Model:** `gemini-3.8-flash-high`  
**PIN_COMMIT:** `3036eb46c45a` (contains all 3 product fixes)

---

## 1. What We're Doing

X6-lite measures whether **repo-memory dossier injection** helps an AI coding agent implement work orders more correctly. The experiment compares two conditions:

- **(a) no_memory** — agent gets the WO prompt only, memory plugin disabled
- **(c) dossiers** — agent gets the same WO prompt + live memory injections via PreInvocation hooks

Each run clones the repo at the WO setup commit, gives the agent the task, then scores against hidden oracle tests that the agent never sees.

**7 work orders** span the repo-memory-plugin codebase (ops, dossiers, config, index, delivery, gates, MCP tools). **50 hidden tests** total.

---

## 2. Infrastructure Journey

### Product Fixes (committed, in PIN)

| Fix | Commit | What |
|-----|--------|------|
| Delivery channel | `8d7ad7b` | `systemMessage` → `ephemeralMessage` (agy CLI doesn't support systemMessage) |
| Tool call parser | `d57f561` | Antigravity native format (`{name, args}`) instead of OpenAI format |
| Junk entry filter | `3036eb4` | `isJunkText()` guard in seeder — removes `None.`, `(pending)`, `N/A` placeholders |

### Eval Harness Fixes (committed, in runner)

| Fix | Commit | What |
|-----|--------|------|
| W3: Test-set replacement | `a4f3ecc` | Delete agent `_test.go` before copying oracle tests (prevents `redeclared` errors) |
| W5: Agent diff archive | `a4f3ecc` + `c4b9967` | Save `agent-diff.patch`, `agent-diff-stat.txt`, `agent-untracked.txt` per run |
| W5: Analysis set field | `a4f3ecc` | `analysis_set` in result JSONL to distinguish pilot/batch/re-run rows |
| W6: API contract appendix | `a4f3ecc` | Append exact identifier names to WO prompt (same for both conditions) |

---

## 3. Run History

### Block 1 Batch (now superseded)
- **14 runs** (7 WOs × 2 conditions), all valid
- **Result: 12 of 14 compile-fail** against hidden tests (0% pass rate)
- Only wo4-gates scored: **5/5 both conditions** (ceiling effect)
- wo6-pull-tools: 0/8 → 3/8 with dossiers (later shown to be naming coincidence, not memory effect)
- **Total tokens:** 4.84M
- **Verdict:** INCONCLUSIVE — scoring measured name-guessing, not implementation capability

### Root Cause (reviewer W1-W6)
The oracle tests used exported identifiers (`OpAdd`, `NewBatchID`, `WithOpsDir`, `ErrNotFound`, etc.) that were **never specified in the WO text**. Agents couldn't compile because they chose different names. One WO's spec even contradicted the oracle test signature (`GateEntries` 2-param vs 3-param).

### W6 Remediation Pilot (current)
- **4 runs** on 2 WOs (wo4-index-migration + wo1-op-log, both conditions)
- API contracts appended to prompt, test-set replacement fixed
- **All 4 valid** (1 contamination rerun succeeded)

---

## 4. Current Results (W6 Pilot)

```
WO                       no_memory        dossiers         Δ tokens
─────────────────────── ──────────────── ──────────────── ─────────
wo4-index-migration      9/9  (100.0%)    8/9  ( 88.9%)   −8.6%
wo1-op-log              10/13 ( 76.9%)   10/13 ( 76.9%)  −13.1%
```

### Before vs After (same WOs)

| WO | Block 1 (both) | W6 Pilot (a) | W6 Pilot (c) |
|---|:---:|:---:|:---:|
| wo4-index-migration | 0/9 compile-fail | **9/9** ✅ | **8/9** ✅ |
| wo1-op-log | 0/13 compile-fail | **10/13** ✅ | **10/13** ✅ |

> [!IMPORTANT]
> The API contract fix transformed scores from **0% → 77-100%**. The eval now measures implementation quality, not name-guessing.

### Token Efficiency

| WO | no_memory | dossiers | Δ |
|---|---:|---:|---:|
| wo4-index-migration | 357,174 | 326,413 | **−8.6%** |
| wo1-op-log | 315,892 | 274,516 | **−13.1%** |
| **Mean** | **336,533** | **300,465** | **−10.7%** |

Both WOs used fewer tokens with dossiers. Suggestive at n=1.

### Delivery Metrics

| WO | Injections | Index Entries |
|---|---:|---:|
| wo4-index-migration | 36 | 155 |
| wo1-op-log | 28 | 147 |

Delivery is working correctly. `first_touch` moments fire alongside `task_start`.

---

## 5. What We Know

### ✅ Confirmed Working
- **Clone isolation** — git-filter-repo removes future commits, contamination checks catch leaks
- **Delivery pipeline** — enrollment → index build → PreInvocation hooks → ephemeral injection
- **Moment system** — `task_start` + `first_touch` fire correctly (29-60 injections per run)
- **Junk filter** — seeder `isJunkText()` removes placeholder entries at source
- **API contract approach** — agents can compile against hidden tests with exact identifier names
- **Test-set replacement** — agent test files deleted before oracle copy, no more `redeclared`
- **Agent diff archival** — `.patch`, `.txt` files preserved for re-scoring

### ⚠️ Not Yet Determined
- **Memory effect on test scores** — n=1 per cell, both pilot WOs show 0 difference (tied scores)
- **Token efficiency** — consistent −9% to −13% with dossiers, but n=1 noise
- **5 remaining WOs** — untested with W6 fixes
- **Replicates** — needed to assess variance and statistical significance

### ❌ Known Limitations
- **`about_to_edit` never fires** — model decides and edits in same turn (product limitation)
- **wo4-gates ceiling** — 5/5 both conditions in Block 1, likely same with W6 fix
- **No `--seed` flag** — can't control model randomness, variance is inherent

---

## 6. Token Budget

| Phase | Tokens | Status |
|-------|-------:|--------|
| Block 1 batch (14 runs) | 4,841,146 | ✅ Complete (superseded) |
| W6 pilot (4 runs) | 1,274,095 | ✅ Complete |
| Block 1 re-run (14 runs) | ~4,800,000 est. | ⏳ Pending |
| Replicates (14 × 2 more) | ~9,600,000 est. | 📋 Planned |
| **Total spent** | **~6,115,000** | |
| **Total remaining** | **~14,400,000** | |

Ultra 200 quota: ~20× Pro. At ~350K tokens/run, each 5h window supports ~15-20 runs.

---

## 7. Files & Artifacts

### Key Files
- [`evaluation/scripts/x6_runner_v3.sh`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/scripts/x6_runner_v3.sh) — Main runner (~877 lines)
- [`evaluation/scripts/x6_batch.sh`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/scripts/x6_batch.sh) — Batch orchestrator (7×2, interleaved)
- [`evaluation/results/x6-lite-results-v3.jsonl`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/results/x6-lite-results-v3.jsonl) — 34 rows (22 valid)
- [`evaluation/oracle/`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/oracle/) — 7 WO oracle dirs with API_CONTRACT.md, test files, calibrated totals

### Commit Chain
```
c4b9967  eval: archive .patch/.txt files (W5 agent diff)
a4f3ecc  eval: W3/W5/W6 fixes + API contracts for all 7 WOs
3036eb4  fix(seed): filter placeholder entries at source          ← PIN
d57f561  fix(delivery): parse Antigravity native tool call format
8d7ad7b  fix(delivery): switch injection channel to ephemeralMessage
```

### Open Issues
- [#16](https://github.com/cblabs-org/repo-memory-plugin/issues/16) — Migration: clean junk entries from existing ops logs
- `about_to_edit` product limitation — needs its own issue

---

## 8. Next Steps

1. **Full Block 1 re-run** — 14 runs (7 WOs × 2 conditions) with all W6 fixes. ~4.8M tokens, ~3h wall clock.
2. **Apply pre-registered kill-gate** — if mean(dossiers) − mean(no_memory) < 15%, stop. Otherwise proceed to replicates.
3. **Replicates** — n=3 per cell (14 × 2 more runs = 28 runs, ~9.6M tokens) for variance estimation.
4. **Product PR** — move the 3 product fixes (`8d7ad7b`, `d57f561`, `3036eb4`) to a proper product branch.
