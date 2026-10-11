# PR Memory Allocation Review — Second Opinion

> [!NOTE]
> All three PRs are BugFixer-authored, all tests pass (949–950 cases), all leak scans clean. 
> This review focuses on **gotchas and correctness**, not style.

---

## PR #267 — Convert bare `new` to `std::nothrow` (issue #255)

### 🔴 CRITICAL BUG: Use of undefined variable `prev`

In [`BLEStack.cpp`](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/BLEManager/BLEStack.cpp) the original code was:

```cpp
auto* prev = chr->getCallbacks();
chr->setCallbacks(new ChainingCharCallbacks(prev, s_chainingStack, s_chainingQueue));
```

The PR **deleted the `auto* prev = chr->getCallbacks();` line** but **still references `prev`** in the replacement:

```cpp
auto* cb = new (std::nothrow) ChainingCharCallbacks(prev, s_chainingStack, s_chainingQueue);
```

**`prev` is now an undeclared identifier.** This will be a **compile error** on ESP32. The fact that the PR body says it compiled successfully is suspicious — this either compiled against a stale object, or the test environment didn't rebuild this translation unit.

**Fix:** The deleted line needs to be restored:
```cpp
auto* prev = chr->getCallbacks();
auto* cb = new (std::nothrow) ChainingCharCallbacks(prev, s_chainingStack, s_chainingQueue);
```

### ✅ `executeCentralConnect` — Good

The `ClientCallbacks` allocation failure path correctly:
1. Deletes the `NimBLEClient` to prevent leak
2. Posts a failure event 
3. Returns `false`

Clean error handling.

### ⚠️ DataManager — Partial degradation, but continues with null clients

```cpp
_secureClient = new (std::nothrow) FrameworkSecureClient();
if (_secureClient) {
    _secureClient->useDefaultCertBundle();
} else {
    ESP_LOGE(TAG, "Failed to allocate FrameworkSecureClient");
}
_plainClient = new (std::nothrow) WiFiClient();
```

If `_secureClient` or `_plainClient` is null, `begin()` continues. Downstream callers (e.g., `FetchGuard`) will receive null client pointers. **This is safe only if all downstream code null-checks before use.** Given this is a boot-time allocation, OOM here likely means the device is in a catastrophically low-memory state anyway, so logging + limping is a reasonable approach — but worth verifying that `FetchGuard` doesn't blindly dereference.

**Verdict: Acceptable risk** — boot-time OOM is effectively a dead device regardless.

### ✅ ButtonModule — Clean

All three `ShimContext` allocations gracefully `continue` on failure, skipping callback registration. No leaks, no null dereference risk.

---

## PR #268 — Remove stored `std::function` members (issue #254)

### ✅ DeadlineToken — Clean

`std::function<uint32_t()>` → raw function pointer `TimeProviderFn` + `void* ctx`. The default `nullptr` falls back to `millis()`. No gotchas.

### ✅ IPreconditionRegistry / PreconditionEngine — Clean

The `void* context` parameter is inserted **before** the existing defaulted parameters (`edges`, `edgeCount`, `damping`, `severity`), which is correct since `context` also defaults to `nullptr`. The call sites in tests that pass `edges` now correctly pass `nullptr` for `context` before `edges`:

```cpp
engine.add("C", PreconditionRole::Operational, [](void*) { ... }, nullptr, nullptr, edges, 1);
//                                                          ^describe  ^context  ^edges
```

This is correct.

### ⚠️ BLEManager `_clientProviderPrecContexts` — Pointer stability concern

```cpp
_clientProviderPrecContexts.push_back({this, p->getProviderId()});
auto* precCtx = &_clientProviderPrecContexts.back();
```

This stores a pointer to an element in a `std::vector`. **If the vector reallocates during a subsequent `push_back`, all previously stored pointers become dangling.** This is a classic vector invalidation bug.

**However**, looking at the usage pattern: `registerPreconditions()` iterates over `_clientProviders` and pushes one context per provider. The `precCtx` pointer is immediately passed to `registry.add()` which stores it. After the loop completes, no more `push_back` occurs, so the final vector layout is stable.

**The danger is during the loop itself**: if there are N client providers, the first N-1 `precCtx` pointers could be invalidated when the Nth `push_back` triggers reallocation.

**But this is actually safe** because the `precCtx` is only **read** when the precondition engine calls `evaluate(context)`, which happens in `PreconditionEngine::tick()` — well after `registerPreconditions()` returns and the vector is stable. The pointer is stored by the engine, not used during registration.

**Verdict: Safe in current usage**, but fragile. A `reserve(_clientProviders.size())` before the loop would make this bulletproof and self-documenting.

### ⚠️ BLEManager `const char* providerId` — Lifetime concern

```cpp
struct ClientProviderPrecContext {
    BLEManager* mgr;
    const char* providerId;  // ← raw pointer
};
```

`p->getProviderId()` — what does this return? If it returns a `const char*` pointing to a stable member (e.g., a string literal or a member buffer), this is fine. If it returns `.c_str()` of a `std::string` that could be moved/destroyed, this is a dangling pointer waiting to happen.

Given this is stored alongside `_clientProviders` (which are long-lived registered modules), this is **almost certainly safe** — the provider outlives the precondition context. But worth a quick sanity check on `getProviderId()`'s return type.

### ✅ DeepStackExecutor — Clean and clever

The backwards-compatible `std::function` wrapper is well done:

```cpp
bool DeepStackExecutor::run(const std::function<void()>& work, uint32_t timeoutMs) {
    auto invoker = [](void* ctx) {
        auto* fn = static_cast<const std::function<void()>*>(ctx);
        if (fn && *fn) { (*fn)(); }
    };
    return run(invoker, const_cast<std::function<void()>*>(&work), timeoutMs);
}
```

The `const_cast` is safe because `invoker` only reads through the pointer (casts back to `const`). The `std::function` lifetime is guaranteed by the caller's stack frame since `run()` blocks until completion. No storage of the closure.

### ✅ DisplayServiceHelper — Clean

`_isOtaActive` default changed from `[]{ return false; }` to `nullptr` with a `isOtaActive()` helper that returns `false` when null. Functionally identical, no heap allocation.

### ✅ OTAUpdateManager — Clean

Trivial `std::function<void()>` → function pointer conversion.

---

## PR #266 — Release audit script false positive fixes (issue #256)

### ✅ Scope refinement — Good

Adding `test/`, `tests/`, `tmp/`, `managed_components/`, `stubs/`, `portal_assets/`, `node_modules/` to exclusion lists is correct — these aren't authored framework code.

### ⚠️ `eval "$filter_chain"` pattern — Shell injection surface

The filter chain is built by string concatenation and evaluated with `eval`:
```bash
local filter_chain="grep -v '/\.pio/' | grep -v ..."
grep_out=$(echo "$grep_out" | eval "$filter_chain" || true)
```

`$SUBMODULE_EXCLUDE` is interpolated into the filter chain string. If `SUBMODULE_EXCLUDE` contained shell metacharacters, this would be a command injection. **In practice**, this variable comes from the script's own configuration (the submodule path), not user input, so the risk is theoretical. But it's a code smell — a series of piped greps would be safer than `eval`.

**Verdict: Acceptable** — the variable is internal. But if this script ever becomes externally configurable, the `eval` needs to go.

### ✅ String literal stripping — Correct

```bash
grep_out=$(echo "$grep_out" | sed -E 's/"([^"\\]|\\.)*"/""/g' || true)
```

Correctly handles escaped quotes inside strings. This kills false positives from `ESP_LOGD("CRYPTO", "Generated new MAC-derived...")`.

### ✅ Strict allocation regex — Correct

```bash
grep -E "\bnew[[:space:]]+[A-Za-z_][A-Za-z0-9_:<>]*[[:space:]]*[({[;]"
```

Matches `new Type(`, `new Type{`, `new Type[`, `new Type;` — all allocation forms. Won't match prose like "new line" or "new features".

---

## Summary

| PR | Verdict | Gotchas |
|----|---------|---------|
| **#267** | 🔴 **Has a compile-breaking bug** | `prev` variable deleted but still referenced in `BLEStack.cpp`. Won't compile. |
| **#268** | ✅ **Good to merge** (with minor suggestion) | Vector pointer stability is safe but fragile — consider `reserve()`. Check `getProviderId()` lifetime. |
| **#266** | ✅ **Good to merge** | `eval` pattern is a minor code smell but safe in context. |

> [!CAUTION]
> **PR #267 must not be merged as-is.** The `BLEStack.cpp` change will fail compilation. The `auto* prev = chr->getCallbacks();` line was accidentally deleted.
