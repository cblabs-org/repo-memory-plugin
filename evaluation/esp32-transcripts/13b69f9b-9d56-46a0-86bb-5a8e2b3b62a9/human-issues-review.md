# Bug Fixer: Human-Gated Issues — Status Report

> **Date:** 2026-10-03 · **Repository:** `matthewmcneill/esp32-app-foundation`
>
> All open issues carrying a `bug-fixer:human:*` label — grouped by tier, with current state analysis and what's needed to unblock each.

---

## Summary

| Tier | Label | Count | Meaning |
|------|-------|-------|---------|
| T2 | `bug-fixer:human:needs-approach-review` | 2 | Agent has a proposed fix; needs your sign-off before it can proceed |
| T4 | `bug-fixer:human:project-candidate` | 4 | Too large for the bug fixer — needs decomposition into a PA project |
| T5 | `bug-fixer:human:required` | 6 | Requires your direct involvement — the agent cannot resolve these autonomously |

**12 issues total.** Of these, the 2 T2 issues are the only ones you can unblock with a single comment (`approved`).

---

## T2 — Needs Approach Review (2 issues)

These have proposed approaches posted by the bug fixer or the Principal Architect. A reply of `approved` (or alternative direction) unblocks the pipeline.

---

### #209 · Wire data source health events to LVGL Pro subjects

| Field | Value |
|-------|-------|
| **Labels** | `enhancement`, `bug-fixer:human:needs-approach-review` |
| **Milestone** | None |
| **Created** | 2026-09-01 |
| **Last activity** | PA review (2026-10-01) |

**Current state:** The bug fixer proposed wiring `DataSourceHealthPayload` events to `app_datasource_*` subjects. The **Principal Architect reviewed and pushed back** — the fix conflates framework and consumer concerns. The PA identified three design questions that need human input:

1. **Framework vs consumer scope** — should this issue cover only the framework plumbing (`sys_ds_*` subject publishing), or also a specific display panel design?
2. **Multi-source subject strategy** — bounded array (`sys_ds_0_name` … `sys_ds_7_name`), serialized JSON subject, or summary-only subjects (`sys_ds_healthy_count`, `sys_ds_total_count`)?
3. **Should this be a PA project?** — it touches LVGL Pro subject conventions.

**To unblock:** Answer the three PA questions, then either `approve` the revised approach or re-scope as a PA project (T4).

<!-- MAINTAINER COMMENT: -->

---

### #253 · DataManager: job queue stores `std::function` per job — runtime heap churn (GRAVE-05)

| Field | Value |
|-------|-------|
| **Labels** | `P2-correctness`, `audit`, `tech-debt`, `bug-fixer:human:needs-approach-review` |
| **Milestone** | None |
| **Created** | 2026-09-26 |
| **Last activity** | PA review (2026-10-01) |

**Current state:** Bug fixer proposed replacing `std::function` with function pointer + `void*` context. The **Principal Architect disagreed** — the diagnosis is correct but the proposed fix is wrong:

- **The real issue is the container**, not the callable. `std::function` uses SBO on Xtensa GCC (≤16 bytes captures stay on the stack). The heap churn comes from `std::deque` doing per-node `new`/`delete`, not from `std::function`.
- **Correct fix:** Replace `std::deque<DataJobEntry>` with `std::array<DataJobEntry, JOB_QUEUE_DEPTH>` + ring buffer indices. ~30-line change, zero public API change.
- **PA raised two questions:**
  1. Has anyone profiled actual heap fragmentation from the DataManager job queue?
  2. Are there consumers with callbacks capturing > 16 bytes (defeating SBO)?

**To unblock:** Decide on the PA's recommended approach (ring buffer, keep `std::function`). Reply with `approved` if you agree with the PA's revised approach, or provide alternative direction. Consider whether profiling data should be gathered first.

> [!NOTE]
> This was the subject of the #277 churn alert — the bug fixer attempted this 4 times with the wrong approach before the PA review course-corrected. The ring buffer approach has not been attempted yet.

<!-- MAINTAINER COMMENT: -->

---

## T4 — Project Candidates (4 issues)

These are large-scope initiatives that need decomposition via `/principal-architect` into work orders before the bug fixer can tackle them.

---

### #225 · [BLEManager] RFC: New managed BLE module with IBLEServiceProvider and IBLEClientProvider interfaces

| Field | Value |
|-------|-------|
| **Labels** | `upstream-issue`, `P4-enhancement`, `found-by:echenon`, `bug-fixer:human:project-candidate` |
| **Milestone** | backlog |
| **Created** | 2026-09-17 |
| **Comments** | 10 (extensive RFC thread) |

**Current state:** RFC revision 2 was accepted (2026-09-18). The BLEManager itself was **already implemented** via PR #225's PA project. The issue body notes it's superseded but the issue remains open.

**To unblock:** Review whether this issue should be **closed** (BLEManager is shipped) or kept open for remaining follow-up work (e.g., `IBLEClientProvider` if that wasn't part of the delivered scope).

<!-- MAINTAINER COMMENT: -->

---

### #228 · feat(mcp): Add streaming serial log feed over WebSocket / SSE

| Field | Value |
|-------|-------|
| **Labels** | `P4-enhancement`, `bug-fixer:human:project-candidate` |
| **Milestone** | None |
| **Created** | 2026-09-22 |
| **Comments** | 1 (triage only) |

**Current state:** Feature request for real-time serial log streaming over WiFi (WebSocket or SSE). Triage classified it as T4 — multi-module, needs architectural decisions about transport, buffering, and memory impact.

**To unblock:** Decide whether this is worth pursuing now. If yes, use `/principal-architect` to decompose. If not, move to backlog milestone or close.

<!-- MAINTAINER COMMENT: -->

---

### #229 · template-app: add IDataSource skeleton with WiFi connectivity guard pattern

| Field | Value |
|-------|-------|
| **Labels** | `enhancement`, `bug-fixer:human:project-candidate` |
| **Milestone** | None |
| **Created** | 2026-09-22 |
| **Comments** | 1 (triage classified as T1 small-scope override, but label is T4) |

**Current state:** Triage initially classified this as T1 (template/docs update), but the label says T4 (project-candidate). There's a **classification mismatch** — the triage comment says "T1 (auto-fixable, small-scope override)" but the issue carries `bug-fixer:human:project-candidate`.

**To unblock:** Resolve the label conflict. If this is genuinely just adding a skeleton `IDataSource` to the template app, it may be T1 — remove `project-candidate` and add `bug-fixer:engineer:needs-fix`. If there are deeper design decisions (which guard patterns to demonstrate, API shape), keep it as T4.

<!-- MAINTAINER COMMENT: -->

---

### #230 · RFC: OTA Memory Headroom — Temporary Module Unloading During Firmware Updates

| Field | Value |
|-------|-------|
| **Labels** | `enhancement`, `bug-fixer:human:project-candidate` |
| **Milestone** | None |
| **Created** | 2026-09-23 |
| **Comments** | 1 (triage) |

**Current state:** Well-scoped RFC proposing an `IUnloadable` interface so modules (especially BLE) can release DRAM before OTA downloads. Triage correctly identified this as T4 — it spans 5+ modules, introduces a new lifecycle interface, and has open design questions (opt-in interface vs EventBus convention, DRAM threshold triggering, non-OTA use cases).

**To unblock:** Decide on the four open design questions in the RFC body, then use `/principal-architect` to decompose into work orders. This is a natural follow-on to the BLEManager (#225) and OTA security (#252) work.

<!-- MAINTAINER COMMENT: -->

---

## T5 — Human Required (6 issues)

These are either blocked by exclusion labels, require physical hardware, or involve scope too broad and deep for the bug fixer loop.

---

### #227 · [EventBus] Payload-lifetime contract is wrong on target — async pointer delivery; single-slot publishers race

| Field | Value |
|-------|-------|
| **Labels** | `documentation`, `P2-correctness`, `bug-fixer:human:required` |
| **Milestone** | `next-beta` |
| **Created** | 2026-09-18 |
| **Last activity** | Bug fixer skipped (overlap with PR #261, now merged) |

**Current state:** Core EventBus design flaw — payload pointers are delivered asynchronously, and single-slot publishers can overwrite the payload before subscribers read it. The BLEManager already has a `// FALLBACK(#227)` workaround (copy-on-first-event pattern).

The bug fixer noted an overlap with PR #261 (webserver startup docs), but that PR is now **merged** — the overlap reason is stale.

**Why T5:** This is a fundamental runtime contract change affecting every EventBus consumer. Changing payload lifetime semantics could break existing callback assumptions across the entire codebase.

**To unblock:** This is a core architecture decision. Options: (a) copy-into-queue semantics, (b) shared_ptr payloads, (c) document the current contract and make subscribers defensive. Recommend PA project decomposition if you choose (a) or (b).

<!-- MAINTAINER COMMENT: -->

---

### #232 · Device name inconsistency — app name doesn't propagate to WiFi AP SSID, BLE, or Home Assistant

| Field | Value |
|-------|-------|
| **Labels** | `P2-correctness`, `bug-fixer:human:required` |
| **Milestone** | `next-beta` |
| **Created** | 2026-09-23 |
| **Last activity** | Bug fixer skipped (overlap with PR #258, now merged) |

**Current state:** Extended audit found **17+ surfaces** where the device name is hardcoded instead of propagating from `APP_TITLE` / `getAppName()`. Includes correctness bugs:

- **TLS cert CN** hardcoded to `"esp32-device"` instead of using `getHostname()`
- **LVGL boot screen** uses static text instead of `bind_text="sys_app_title"`
- **mDNS** documented in README but **zero implementation** in code

The overlap PR #258 is now **merged** — the overlap blocker is stale.

**Why T5:** Touches 17+ surfaces across WiFi, BLE, TLS, LVGL, MCP, OTA, portal — a massive cross-cutting concern with correctness and security implications.

**To unblock:** This is a strong PA project candidate. Consider re-labelling to T4 (`bug-fixer:human:project-candidate`) and using `/principal-architect` to decompose into work orders (e.g., WO1: hostname unification, WO2: mDNS implementation, WO3: TLS cert CN fix, WO4: LVGL binding fix).

<!-- MAINTAINER COMMENT: -->

---

### #238 · WifiManager: silent link loss undetected — device thinks it's connected after router drops client

| Field | Value |
|-------|-------|
| **Labels** | `enhancement`, `bug-fixer:human:required` |
| **Milestone** | None |
| **Created** | 2026-09-25 |
| **Last activity** | Bug fixer posted approach proposal (2026-09-27) |

**Current state:** When a router silently drops the ESP32 (no deauth frame), the WifiManager state machine never transitions out of `Ready`. Proposed fix: periodic reachability check in `tick()` during `Ready` state. Bug fixer posted an approach but the issue is T5, so no work has started.

**Why T5:** Modifies the WiFi state machine and reconnect heuristics — core runtime behaviour change. Also noted in context of BLEManager/WiFi coexistence stabilisation.

**To unblock:** The bug fixer's proposed approach (periodic `WiFi.status()` check + optional gateway probe) is reasonable. Consider either: (a) approving the approach and reclassifying to T2 → T1, or (b) scoping as part of a broader WiFi resilience PA project.

<!-- MAINTAINER COMMENT: -->

---

### #245 · [Audit] Replace hardcoded hex colors with Design System tokens

| Field | Value |
|-------|-------|
| **Labels** | `bug-fixer:human:required` |
| **Milestone** | None |
| **Created** | 2026-09-26 |
| **Last activity** | Bug fixer skipped (overlap with PR #260, now merged) |

**Current state:** Audit found 146 hardcoded hex colors across CSS, JS, and LVGL files. Remediation is to replace with `var(--ds-color-...)` CSS variables from the design system token table.

The overlap PR #260 is now **merged** — the overlap blocker is stale.

**Why T5:** Large-scope (146 instances), touches portal CSS/JS extensively, requires design system knowledge to map each hex value to the correct token.

**To unblock:** This is potentially a good candidate for reclassification. If the design system token table is well-defined, this is actually a **mechanical batch replacement** (T1 via small-scope override) — each hex → token mapping is a lookup, not a design decision. Consider: (a) creating the hex → token mapping table, then reclassifying to T1, or (b) decomposing via PA if there are ambiguous mappings requiring design judgement.

<!-- MAINTAINER COMMENT: -->

---

### #246 · [Audit] Tech Debt: Documentation & House Style compliance cleanup

| Field | Value |
|-------|-------|
| **Labels** | `bug-fixer:human:required` |
| **Milestone** | None |
| **Created** | 2026-09-26 |
| **Last activity** | Bug fixer skipped (overlap with PR #258, now merged) |

**Current state:** Audit found: 73 missing Doxygen headers, 4 undocumented constants, 165 member naming violations, 49 directory/filename casing mismatches, missing test file headers.

The overlap PR #258 is now **merged** — the overlap blocker is stale.

**Why T5:** Very large scope (300+ individual items), but entirely mechanical.

**To unblock:** Similar to #245 — this is arguably a batch of mechanical T1 changes. Consider: (a) breaking into sub-issues by category (Doxygen headers, naming violations, casing mismatches) and reclassifying each as T1, or (b) decomposing via PA. The `house-style-documentation` skill can guide the agent.

<!-- MAINTAINER COMMENT: -->

---

### #277 · Bug Fixer Diagnostic Report: Churn Detected

| Field | Value |
|-------|-------|
| **Labels** | `bug-fixer-health`, `bug-fixer:human:required` |
| **Milestone** | None |
| **Created** | 2026-10-02 |

**Current state:** Trajectory review detected that #252 was attempted 4 times without success. Issue #252 is now **closed** (merged via PR #272). The churn was caused by the bug fixer repeatedly using the wrong approach (function pointer + `void*`), which was corrected by the PA review on #253.

**To unblock:** This is an informational alert. The underlying issue (#252) is resolved. **This can be closed.**

<!-- MAINTAINER COMMENT: -->

---

## Stale Overlap Blockers

Several issues had bug-fixer "skipped" comments referencing overlap with active PRs. All three of those PRs have since **merged**:

| Skipped Issue | Overlap PR | PR Status |
|--------------|-----------|-----------|
| #227 | PR #261 (webserver startup docs) | ✅ Merged |
| #232 | PR #258 (redactor saturation) | ✅ Merged |
| #245 | PR #260 (portal precondition health) | ✅ Merged |
| #246 | PR #258 (redactor saturation) | ✅ Merged |

These overlap blockers are now stale and no longer prevent work on these issues.

---

## Recommended Quick Wins

If you want to unblock the bug-fixer pipeline quickly, here are actions ordered by effort:

1. **Close #277** — informational, underlying issue resolved *(1 click)*
2. **Approve #253** with PA's ring-buffer approach — clear technical direction, ~30-line change *(1 comment)*
3. **Reclassify #229** — resolve T1/T4 label mismatch *(1 label edit)*
4. **Answer #209's three design questions** — unblocks data source health UI work *(1 comment)*
5. **Reclassify #238 to T2** and approve the proposed approach — periodic WiFi health check *(1 comment + label)*
6. **Reclassify #245 and #246** — if token mapping table and house style rules are clear, these are mechanical T1 batches *(label + optional sub-issue creation)*
7. **Reclassify #232 to T4** — strong PA project candidate, decompose into work orders *(label + PA session)*

