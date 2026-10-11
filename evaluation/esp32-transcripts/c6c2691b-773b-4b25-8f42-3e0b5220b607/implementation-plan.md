# Implementation Plan: Addressing Architect Review Changes for PR #268

## Overview

PR #268 (`fix/issue-254-remove-remaining-stored-std-function`) removes remaining stored `std::function` members across `DeadlineToken`, `IPreconditionRegistry`, `OTAUpdateManager`, `DisplayServiceHelper`, and `DeepStackExecutor` to eliminate heap allocation during steady-state ticks.

The Architect reviewed PR #268 and requested four architectural changes:
1. **Broken Consumer Templates & Sample Applications**: Adapt `template-app/src/MyApp.cpp`, `sample-app/src/WeatherModule.cpp`, and `sample-app/test/test_unit/test_weather/test_main.cpp` to the new `IPreconditionRegistry::add` non-capturing function pointer signature.
2. **Rebase Conflict & Compilation Breakage with `origin/main`**: Rebase onto `origin/main` and adapt `modules/Application/RedactorPreconditionContributor.hpp` to the new callback signatures.
3. **Pointer Stability & Dynamic Allocation in `BLEManager`**: Replace `std::vector<ClientProviderPrecContext>` in `modules/BLEManager/BLEManager.hpp` with a fixed-size `std::array<ClientProviderPrecContext, 4>` for absolute pointer stability and zero heap allocations.
4. **Missing Ergonomic Overload on `IPreconditionRegistry`**: Add an inline convenience overload in `include/interfaces/IPreconditionRegistry.hpp` allowing `(name, role, evaluate, context, edges, edgeCount, ...)` without requiring an explicit `nullptr` for `describe`.

---

## Safety & In-Flight Agent Check (Rule 01)

### Current Repository State
- **Current Branch:** `fix/issue-255-convert-bare-new-to-nothrow`
- **Dirty Working Tree:** `modules/BLEManager/BLEStack.cpp` has uncommitted modifications.
- **Active In-Flight Session:** Conversation `6261db5e-06ea-484d-8f92-ff45b5a88dd5` is actively working on Issue #255 and holding the PlatformIO lock.
- **Precondition to Execution:** We MUST NOT switch branches or modify files until session `6261db5e` completes and commits its changes, leaving the working tree clean and the PlatformIO lock released.

---

## Detailed Implementation Steps

### 1. Branch Checkout & Rebase
- Verify session `6261db5e` has finished.
- Check out `fix/issue-254-remove-remaining-stored-std-function`.
- Fetch `origin/main` and rebase: `git fetch origin main && git rebase origin/main`.

### 2. Finding 4: Ergonomic Overload on `IPreconditionRegistry`
**File:** [`include/interfaces/IPreconditionRegistry.hpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/include/interfaces/IPreconditionRegistry.hpp)
- Add inline overload:
  ```cpp
  void add(const char* name,
           PreconditionRole role,
           EvalFn evaluate,
           void* context,
           const Edge* edges = nullptr,
           uint8_t edgeCount = 0,
           const DampingPolicy* damping = nullptr,
           const RetryPolicy* retry = nullptr) {
      add(name, role, evaluate, nullptr, context, edges, edgeCount, damping, retry);
  }
  ```
- Update [`modules/TimeManager/TimePreconditionContributor.hpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/TimeManager/TimePreconditionContributor.hpp#L72-L79):
  Use the new overload to eliminate the `nullptr` placeholder.

### 3. Finding 2: Adapt `RedactorPreconditionContributor.hpp` Post-Rebase
**File:** [`modules/Application/RedactorPreconditionContributor.hpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/Application/RedactorPreconditionContributor.hpp)
- Update `registerPreconditions`:
  - `EvalFn`: `[](void*) -> PreconditionStatus { ... }`
  - `DescribeFn`: `[](Cause c, char* buf, size_t len, void*) { ... }`

### 4. Finding 1: Fix Consumer Templates & Sample Applications
**Files:**
- [`template-app/src/MyApp.cpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/template-app/src/MyApp.cpp#L224-L230):
  Update `registerPreconditions`:
  ```cpp
  void MyApp::registerPreconditions(IPreconditionRegistry& reg) {
      reg.add("MyApp Ready", PreconditionRole::Operational,
          [](void* ctx) -> PreconditionStatus {
              auto* self = static_cast<MyApp*>(ctx);
              return self->_ready
                  ? PreconditionStatus{true, Cause::Met}
                  : PreconditionStatus{false, Cause::Connecting};
          },
          this);
  }
  ```
- [`sample-app/src/WeatherModule.cpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/sample-app/src/WeatherModule.cpp#L576-L672):
  - "Weather API Key": Pass `this` as context; update `EvalFn` to access `self->_keyManager`; update `DescribeFn` to accept `(Cause c, char* buf, size_t len, void*)`.
  - "Weather Location": Pass `this` as context; update `EvalFn` to access `self->_locationConfigured`; update `DescribeFn` to accept `(Cause c, char* buf, size_t len, void*)`.
  - "Weather API": Pass `this` as context; update `EvalFn` to access `self->_hasData` and `self->_lastFetchStatus`; update `DescribeFn` to accept `(Cause c, char* buf, size_t len, void*)`.
- [`sample-app/test/test_unit/test_weather/test_main.cpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/sample-app/test/test_unit/test_weather/test_main.cpp#L156-L272):
  - Update `MockPreconditionRegistry::add` override signature to accept `void* context = nullptr`, `damping = nullptr`, `retry = nullptr`.
  - Store `context` in `Spec`. Add `eval()` and `desc()` methods on `Spec` that invoke `evaluate(context)` and `describe(c, buf, len, context)`.
  - Update unit test calls to use `reg.get(i).eval()` and `reg.get(i).desc(...)`.

### 5. Finding 3: Pointer Stability & Zero Dynamic Allocation in `BLEManager`
**Files:**
- [`modules/BLEManager/BLEManager.hpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/BLEManager/BLEManager.hpp#L270-L276):
  Include `<array>` and replace `std::vector<ClientProviderPrecContext>` with:
  ```cpp
  static constexpr size_t MAX_CLIENT_PRECONDITIONS = 4;
  std::array<ClientProviderPrecContext, MAX_CLIENT_PRECONDITIONS> _clientProviderPrecContexts{};
  size_t _clientProviderPrecCount = 0;
  ```
- [`modules/BLEManager/BLEManager.cpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/BLEManager/BLEManager.cpp#L1751-L1815):
  Reset `_clientProviderPrecCount = 0;`.
  For each provider, check bounds (`_clientProviderPrecCount < MAX_CLIENT_PRECONDITIONS`).
  Assign to `_clientProviderPrecContexts[idx]` and pass `&_clientProviderPrecContexts[idx]` as context to `registry.add`.
  Guarantees zero heap allocation and absolute pointer stability.

---

## Verification & Testing Gate
1. **Compilation Check**:
   - Compile `native` test suites (`pio test -e native` / MCP `run_tests`).
   - Verify sample-app and template-app compile without errors.
2. **Leak Check**:
   - Run `python3 .agents/plugins/bug-fixer-loop-plugin/scripts/pre_tool_gate.py --scan-only .`.
3. **Commit & Push**:
   - Commit changes to `fix/issue-254-remove-remaining-stored-std-function`:
     `[BugFixer] Address PR #268 review feedback: update consumer apps, rebase on main, array context in BLEManager, add registry overload`
   - Push to `origin fix/issue-254-remove-remaining-stored-std-function`.
