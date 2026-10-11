# Principal Architect Review — `needs-approach-review` Issues

> **Date:** 2026-10-01  
> **Reviewer:** Principal Architect (session cfd5743d)  
> **Scope:** All 9 open issues labelled `needs-approach-review`  
> **Context:** PA memory paged (97 completed projects, 3 active). Module Index and framework source reviewed.

---

## Summary Table

| # | Title | Severity | Verdict | Key Concern |
|---|-------|----------|---------|-------------|
| 253 | DataManager `std::function` heap churn | GRAVE-05 | ⚠️ **Approach needs rework** | Function-pointer + `void*` breaks modern C++ contract; `std::function` SBO already avoids heap for small captures |
| 252 | OTA bearer token over `setInsecure()` TLS | P1/Security | ✅ **Approach sound, needs detail** | Redirect chain validation on real CDN is the hard part; cert bundle RAM cost ~1.5 KiB |
| 251 | OTA unsigned-by-default | P1/Security | ✅ **Approach sound, one concern** | `otaAllowUnsigned` opt-in flag creates a "documented insecurity" — consider compile-time-only |
| 226 | OTA boot-time auto-check ignores `checkHour` | P1 | ✅ **Approach sound** | Part 2 (deferral veto) needs careful API design to avoid the Orchestrator Anti-Pattern |
| 209 | Wire DataSource health to LVGL subjects | Enhancement | ⚠️ **Approach too narrow** | Bug-fixer proposed binding to a specific display layout; this needs to be consumer-agnostic |
| 204 | AuditModule runtime-configurable budget | Enhancement | 🔴 **Resolved — close** | Conversation already concluded: `build_flags` is sufficient, owner agreed to close |
| 201 | Remove OTA tests from `test_ignore` | Todo | ✅ **Approach sound** | Straightforward, but must ensure mocks compile under the current native test harness |
| 193 | MCP tool dispatch on main loop | Enhancement | ⚠️ **Approach needs refinement** | Option 2 (MainLoopDispatcher) is best, but the response model (sync vs async JSON-RPC) is underspecified |
| 80 | Portal branding FOUC | P3 | ✅ **Approach sound (existing comment is excellent)** | Colour FOUC already fixed; title FOUC fix + readiness marker is clean |

---

## Detailed Reviews

### #253 — DataManager `std::function` heap churn (GRAVE-05)

**Bug-fixer proposed:** Replace `std::function<void(const FetchContext&)>` with `void(*)(const FetchContext&, void* ctx)` function pointer + context, plus a preallocated ring buffer.

**Architect's assessment: ⚠️ Needs rework.**

The diagnosis is correct — heap allocation per job is undesirable in a long-running embedded system. However, the proposed fix has problems:

1. **`std::function` SBO already avoids heap for small captures.** On ESP-IDF's libstdc++, `std::function` uses Small Buffer Optimisation for captures ≤16 bytes (2 pointers). Most DataManager job callbacks capture only `this` (8 bytes), so they never hit the heap. The GRAVE-05 classification may be overstated — need to measure actual allocation counts.

2. **Function-pointer + `void*` is a regression.** The framework's design philosophy (SOLID, modern C++ with `std::function` throughout interfaces) would be violated. Every `IDataSource` consumer would need to rewrite their fetch callbacks. The API break cascades to all consumer apps.

3. **The real fix is the ring buffer, not the function pointer.** Replace `std::deque<DataJobEntry>` with a fixed-capacity ring (the `JOB_QUEUE_DEPTH` is already bounded at 16). Each slot in the ring can hold the `DataJobEntry` including its `std::function` — the point is to eliminate the deque's per-node `new`/`delete`, not to eliminate `std::function`.

4. **Consider `std::array<DataJobEntry, JOB_QUEUE_DEPTH>` + head/tail indices.** Zero heap allocations, zero changes to the public API signature, same bounded behaviour.

**Questions for human:**
- Has anyone profiled the actual heap fragmentation from the deque? The GRAVE-05 severity seems speculative.
- Are there any DataManager consumers with callbacks capturing more than 16 bytes (which would defeat SBO)?

---

### #252 — OTA bearer token over `setInsecure()` TLS (P1/Security)

**Bug-fixer proposed:** Replace `setInsecure()` with ESP-IDF cert bundle, strip auth on cross-host redirects, restrict fallback.

**Architect's assessment: ✅ Sound approach, needs implementation detail.**

The approach is correct and well-scoped. Considerations:

1. **Cert bundle RAM cost:** `esp_crt_bundle_attach()` uses ~1.5 KiB heap for the bundle index. This is acceptable but should be documented in the memory budget.

2. **GitHub CDN redirect chain:** GitHub releases redirect through `objects.githubusercontent.com` (different host). The proposal correctly identifies stripping `Authorization` on cross-host redirects, but the implementation needs to handle the redirect *manually* — `HTTPClient` auto-follows redirects by default and will send the header to the CDN host. This is the hardest part of the fix.

3. **Fallback semantics:** The issue says "restrict TLS fallback to genuine certificate-verification errors rather than any negative HTTP code." This is correct — the current code treats *any* `httpCode < 0` as "try insecure", which catches DNS failures, timeouts, etc. The fix should distinguish `MBEDTLS_ERR_X509_*` / `MBEDTLS_ERR_SSL_*` certificate errors from transport errors.

4. **Pair with #251:** These two issues are tightly coupled. The cert bundle change should land in the same PR as the unsigned-OTA fix to avoid a state where we have verified TLS but accept unsigned firmware, or vice versa.

**Questions for human:**
- Should #251 and #252 be bundled as a single PA project (2-3 WOs), or kept as separate PRs?
- Is there a test endpoint (non-production) we can use to validate the redirect chain without consuming a real OTA release?

---

### #251 — OTA unsigned-by-default (P1/Security)

**Bug-fixer proposed:** Fail-closed when `otaPublicKey` is `nullptr`, add `otaAllowUnsigned` opt-in flag.

**Architect's assessment: ✅ Sound, with one concern.**

1. **Fail-closed is the right default.** The framework should refuse OTA operations when no public key is configured. This is a breaking change for consumers who haven't set a key, but that's the correct forcing function.

2. **`otaAllowUnsigned` as a runtime flag is dangerous.** If this is a `FrameworkConfig` member, consumers can set it in `main.cpp` and forget about it. Worse, it could be persisted to config and toggled via the portal. **Recommendation:** Make this a **compile-time-only** flag (`-D OTA_ALLOW_UNSIGNED`), not a runtime `FrameworkConfig` member. Development builds opt in explicitly at build time; production builds cannot accidentally enable it. This follows the same pattern as `BOARD_AUDIT_BYTES`.

3. **`handleSignedUpload` path:** The issue correctly identifies that the signed-upload HTTP endpoint needs the same treatment. Currently if no key is configured, a raw binary upload via the portal would be accepted without signature verification.

4. **Migration path:** Consumers who rely on unsigned OTA for development need clear documentation: "add `-D OTA_ALLOW_UNSIGNED` to your dev environment's `build_flags`, remove it for production."

**Questions for human:**
- Compile-time-only flag vs runtime flag — which do you prefer? I strongly recommend compile-time-only.
- Should the dev-mode flag also emit a persistent precondition warning (via `IPreconditionContributor`) so the portal shows "⚠️ OTA signature verification disabled"?

---

### #226 — OTA boot-time auto-check ignores `checkHour` (P1)

**Bug-fixer proposed:** Add `checkOnBoot` boolean setting (default `true`), wire through config/API/MCP/portal.

**Architect's assessment: ✅ Sound for Part 1.**

Part 1 is well-scoped and follows the existing config pattern (`autoCheck`, `checkHour`). The default of `true` preserves backward compatibility. No concerns on this part.

**Part 2 (deferral veto) — needs careful design:**

The suggested `setDeferralCheck(std::function<bool()>)` is reasonable but risks the **Orchestrator Anti-Pattern** — the OTA manager polling an app-provided callback is tightly coupling the OTA lifecycle to app state. Better alternatives:

1. **`IPreconditionContributor`-style approach:** The OTA manager publishes a `"ota.update.pending"` precondition check before starting. If any contributor vetoes, the update is deferred. This uses the existing precondition framework and is decoupled — the OTA manager doesn't know *who* is busy, just whether it's safe to proceed.

2. **EventBus veto pattern:** OTA publishes an `"ota.check.request"` event; any listener can respond with a deferral. This is more complex but more extensible.

Option 1 fits the framework's existing patterns better. The precondition engine is already there.

**Questions for human:**
- Is Part 2 (deferral veto) actually needed now, or is `checkOnBoot=false` sufficient for the echenon use case?
- If needed, do you prefer the precondition-contributor approach or the `std::function` callback?

---

### #209 — Wire DataSource health to LVGL subjects (Enhancement)

**Bug-fixer proposed:** Define `app_datasource_*` subjects in `globals.xml` for the CrowPanel display, bind to a health widget.

**Architect's assessment: ⚠️ Too narrow — needs rethinking.**

1. **Display-layout coupling:** The bug-fixer proposes changes to a specific display layout (`uidesign/crowpanel-128-rotary/lvglpro/`). This is a *consumer-specific* display project, not a framework concern. The framework's `LVGLDisplayModule` should publish subjects in a consumer-agnostic way; the display *design* is the consumer's responsibility.

2. **Framework responsibility vs consumer responsibility:**
   - **Framework should:** Replace `(void)updates;` with `lv_subject_set_*()` calls using well-defined `sys_datasource_*` subject naming conventions (following the existing `sys_*` pattern from PR #208).
   - **Consumer should:** Decide how to display them (which screen, what widget, what layout).

3. **Subject naming convention:** The issue proposes `app_datasource_*` but the framework convention from the SLS→LVGL Pro migration (PR #208) is `sys_*` for framework-managed subjects. Data source health should be `sys_ds_<name>_state`, `sys_ds_<name>_error`, etc.

4. **Multi-source challenge:** A device may have N data sources. The subject system doesn't natively support dynamic arrays of subjects — you'd need a bounded set (e.g., `sys_ds_0_name`, `sys_ds_0_state`, ..., `sys_ds_3_name`, `sys_ds_3_state`) or a serialized JSON subject.

**Questions for human:**
- Is the intent here to do the framework plumbing (subject publishing) only, or also to design a specific display panel?
- For multi-source support: bounded array of subjects (simpler, limited to N sources) or a single JSON subject (flexible, harder to bind in LVGL Pro)?
- Should this be scoped as a PA project given it touches the LVGL Pro subject convention, or is it small enough for a single PR?

---

### #204 — AuditModule runtime-configurable budget (Enhancement)

**Architect's assessment: 🔴 Should be closed.**

The conversation in the thread has already reached resolution:
- The maintainer (matthewmcneill) confirmed that `build_flags = -D BOARD_AUDIT_BYTES=131072` is sufficient for the Drayton consumer migration.
- The architectural distinction between build-time constants (budget, partition sizes) and runtime knobs (enabled, rate_limit) was agreed upon.
- The only remaining action item is improving documentation discoverability of `BOARD_AUDIT_BYTES` in the audit developer guide.

The bug-fixer triage then re-classified this as T1 (auto-fixable), apparently ignoring the conversation above that concluded with "happy to close this as won't fix." This is a triage error.

**Recommendation:** Close this issue. Open a separate documentation-only issue (T1) for improving `BOARD_AUDIT_BYTES` discoverability in the audit guide, if desired.

---

### #201 — Remove test_otaUpdateManager from test_ignore (Todo)

**Bug-fixer proposed:** Remove from `test_ignore` in `platformio.ini`, fix mocks if needed.

**Architect's assessment: ✅ Sound, with a caveat.**

The approach is straightforward. Considerations:

1. **Mock completeness:** The OTA test suite was likely ignored because it depends on mocks for `WiFiClientSecure`, `HTTPClient`, `Update`, etc. that may not have been maintained. The worker needs to verify the mocks compile and the tests pass under the current native test framework before un-ignoring.

2. **CI gate impact:** Since PR #197 made native tests a merge gate, un-ignoring a broken test suite would block all PRs. The worker should un-ignore, run `pio test -e native`, and only submit if green.

3. **Scope creep risk:** If the mocks are significantly out of date, the scope could balloon from "remove from test_ignore" to "rewrite OTA test mocks." If that happens, this should be re-classified as T2 (needs-approach-review) with a proper scope estimate.

**No questions for human — this is straightforward if the mocks are healthy.**

---

### #193 — MCP tool dispatch on main loop (Enhancement)

**Bug-fixer proposed:** Introduce an optional dispatch mechanism with `executeOnMainLoop` flag, drain pending calls during `Application::tick()`.

**Architect's assessment: ⚠️ Approach needs refinement — Option 2 with sync response model.**

Of the three options in the original issue, **Option 2 (MainLoopDispatcher utility)** is the best fit:

1. **Option 1 (framework-level tick drain):** Would require `Application` to know about MCP tool dispatch, creating a coupling between the app lifecycle and the MCP transport layer. The `Application` class is already complex.

2. **Option 2 (SPSC queue utility):** The framework provides the primitive (`MainLoopDispatcher<T>`), the module drains it from its own `tick()`. This keeps the dispatch logic in the module that understands its own state, and the queue primitive is reusable.

3. **Option 3 (esp_event_post):** Introduces a dependency on the ESP-IDF event loop task, which may not be the main loop task. The task affinity is configurable and varies by consumer. Too fragile.

**The underspecified part is the response model:**

The MCP tool handler needs to return a JSON-RPC response. If the handler enqueues work and returns immediately, the response is `{"queued": true}` — the client doesn't know the outcome. This is what the Wiser Controller does today, and it works because the MCP client (an AI agent) can poll for state changes.

But a proper framework solution should support both:
- **Fire-and-forget** (current pattern): handler enqueues, returns `{"queued": true}`, client polls separately.
- **Synchronous-appearing** (better DX): handler enqueues, *blocks the web-server task* on a `xSemaphoreGive` from the main loop, and returns the actual result. This has latency implications (web-server task blocked for up to one tick period, ~50ms) but gives the client a proper response.

The framework should support both via a dispatch mode enum:
```cpp
enum class MCPDispatch { INLINE, MAIN_LOOP_ASYNC, MAIN_LOOP_SYNC };
```

**Questions for human:**
- Is the async (fire-and-forget) model sufficient for all known use cases, or do you need synchronous dispatch for some tools?
- Should this be a PA project (it touches MCPToolRegistry, IMCPContributor, and potentially Application)? I'd estimate 2-3 WOs.

---

### #80 — Portal branding FOUC (P3)

**Bug-fixer proposed (latest comment):** Option A + C with early `/api/app` fetch.

**Architect's assessment: ✅ The existing maintainer comment (#80, matthewmcneill, 2026-08-20) is an excellent technical analysis and the approach is sound.**

The maintainer's own comment already:
- Confirmed that colour FOUC is fixed (PR #136).
- Diagnosed the remaining title FOUC precisely.
- Proposed the clean three-step fix (early title apply → readiness marker → update test helpers).
- Explained *why* it was deliberately left unfixed (test coupling).

The bug-fixer's later comment proposes implementing the same thing, which is correct. The only thing I'd add:

1. **The readiness marker should be a CSS class, not a data attribute.** `document.body.classList.add('portal-ready')` is easier to target in both test selectors and CSS (for hiding content until ready).

2. **Consider a micro-flash suppression:** If the `/api/app` response is slow (e.g., the server is still booting), the title will still flash briefly. A `<style>` block in `index.html` that hides `#portal-title` until a JS class is applied (`body:not(.portal-ready) #portal-title { visibility: hidden; }`) would eliminate even the degenerate case.

**No questions for human — the path is clear.**

---

## Cross-Cutting Observations

1. **#251 and #252 should be a single PA project.** They are companion issues (both OTA security), touch the same files, and the fixes are interdependent. Suggested structure: WO1 = cert bundle + redirect handling, WO2 = fail-closed unsigned OTA + compile-time opt-in, WO3 = documentation + test coverage.

2. **#204 should be closed immediately.** The conversation is already resolved; the bug-fixer triage re-opened a settled discussion.

3. **The bug-fixer triage classifications are partially wrong.** #204 was incorrectly classified as T1 after the maintainer agreed to close it. #209 was classified T1 but requires design decisions (subject naming, multi-source representation) that make it T2 at minimum. #193 was classified with a vague approach that underspecifies the response model.

4. **#253's GRAVE-05 severity may be inflated.** The `std::function` SBO likely avoids the heap for most callbacks. Recommend profiling before investing in a ring-buffer rewrite.
