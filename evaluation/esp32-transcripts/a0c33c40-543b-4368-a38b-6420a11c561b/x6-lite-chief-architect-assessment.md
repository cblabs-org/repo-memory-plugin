# Chief Architect Assessment — X6-lite Evidence Package

**Date:** 2026-10-11
**Assessor:** Chief Architect (session `a0c33c40`)
**Source:** WO4 execution session `7f162d28`

---

## TL;DR

The X6-lite evaluation went through **three iterations** (v1 invalidated → v2 invalidated → v3 ran cleanly) and has produced a qualified but genuine result. The infrastructure is validated. The kill gate **cannot be called yet** — the result is INCONCLUSIVE due to scoring problems, but the trajectory is promising and the fix is clear.

---

## Document Assessment

### 1. `x6-lite-results.md` — Initial Post-Mortem ⚠️ HISTORICAL
The first run (52 runs, 11.3M tokens) was completely invalidated due to 3 critical harness defects:
- Agents ran in main checkout, not worktree
- `REPO_MEMORY_HOUSEKEEPING=1` disabled delivery in ALL conditions
- Exit tests ran against untouched worktree

**Assessment:** Correctly invalidated. The diagnostic work was thorough and all 3 defects are real. This document is evidence of a failed run, not a result. **Keep as historical evidence.**

### 2. `x6-lite-v3-implementation-plan.md` — v3 Harness Design ✅ GOOD
Comprehensive redesign addressing all v1/v2 defects. 10 design decisions documented. Hooks-only delivery, pinned build, interleaved scheduling, API contracts, hidden test isolation.

**Assessment:** Sound engineering. The harness is now production quality. The decision to drop wo6-seeding (PA workspace leakage) was correct.

### 3. `x6-lite-pilot-report.md` — v3 Results (14 runs) 🔑 KEY DOCUMENT
The actual data: 7 WOs × 2 conditions × 1 replicate = 14 valid runs, 4.84M tokens.

**Key findings:**
- **5 of 7 WOs at floor** (0/0 both conditions) — agents can't compile against hidden tests in either condition
- **1 WO at ceiling** (wo4-gates: 5/5 both) — too easy
- **1 WO shows signal** (wo6-pull-tools: 0→3/8) — but attributed to naming coincidence by reviewer

**Assessment:** The data is real but the measurement is compromised. The floor effect (12/14 runs fail to compile) means the experiment measures API name-guessing, not task capability. The reviewer (W2, W4) is correct that the wo6-pull-tools signal is a naming coincidence, not attributable to memory.

### 4. `x6-lite-remediation-plan.md` — Fix Plan ✅ GOOD
All 6 reviewer items verified. Clear fixes: API contract appendix (W6), test-set replacement (W3), agent diff archival (W5).

**Assessment:** The remediation is straightforward and correct. Estimated cost: ~6h + 4.8M tokens for re-run.

### 5. `x6-memory-retrieval-and-scope-report.md` — Strategic Analysis 🔑 STRATEGIC

This is the most important document in the set. It reframes the entire product question:

> [!IMPORTANT]
> **"The open question is no longer 'does content reach the model' but 'is the right content selected.'"**

Key strategic insights:
1. **X6-lite can't measure the benefits that grow with codebase size** (navigation, finding relevant knowledge among thousands of entries)
2. **Push ≠ RAG** — precision matters more than recall because injections are unrequested interruptions
3. **Retrieval quality is measurable offline** (cheap, fast iteration) before expensive agent runs
4. **The current ranker discards relevance** — bm25 score isn't carried into the final ranking
5. **35-60% duplicate injections** across moments
6. **Zero MCP pull-tool usage** in any run
7. **Task-start ranking effectively orders by entry type**, not relevance

The proposed Layer 1/2/3 evaluation pyramid is sound RAG methodology applied to push-based memory.

**Assessment:** This analysis correctly identifies that the scoring layer, not the infrastructure, is the bottleneck. It also correctly identifies that X6-lite's scope (small codebase, spec-style tasks) structurally suppresses the benefits memory should provide. The 3-layer eval proposal is the right approach.

---

## Kill Gate Status

**INCONCLUSIVE — measurement invalid.** Per the pre-registered protocol, the gate cannot be called because:
1. The scoring layer measures name-guessing (API contract problem)
2. n=1 per cell (insufficient replicates)
3. The pre-registered analysis rule wasn't formally applied

**This is NOT a fail.** The infrastructure works (isolation, delivery, moments). The fix is in the scoring (API contracts), not the product.

## Recommended Path Forward

1. **Apply W6 fixes** (API contracts, test-set replacement, agent diff archival) — ~2h work
2. **Re-run Block 1** (14 runs, ~4.8M tokens) — this completes WO4
3. **Apply pre-registered rule** — this completes WO5 and determines the gate
4. **Log retrieval inputs/outputs** during the re-run (§7 of scope report) — enables Layer 1 later
5. **File the scope statement** (§2 of scope report) with whatever verdict the gate produces

## Programme Impact

If the re-run gate passes: proceed to PR-10 (distillation) as planned.
If the re-run gate fails or is inconclusive again: the scope report's Layer 1/2 approach gives a cheaper path to improving retrieval before committing to full distillation.
If budget is exhausted: report as "harness validated, scoring invalidated, infrastructure works" and decide whether to redirect to ranker improvements before more agent runs.

---

> The evaluation has been more expensive than planned (~16M tokens across 3 iterations vs 7M budgeted), but the diagnostic rigour is high. The product question has sharpened from "does memory help?" to "does the ranker select the right content?" — that's genuine progress.
