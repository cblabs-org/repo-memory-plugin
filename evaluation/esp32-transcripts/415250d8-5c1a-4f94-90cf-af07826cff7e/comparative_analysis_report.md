# ESP32 App Foundation — Comparative Agent Build Analysis

**Evaluator Role:** Senior Embedded Systems Architect & OOP Expert  
**Date:** 2026-05-22  
**Spec Evaluated Against:** [PROGRAM-PLAN.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/skills/principal-architect-workspace/esp32-app-foundation/PROGRAM-PLAN.md) + [design_specification.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/docs/design_specification.md)

---

## Builds Under Review

| Label | Path | Agent Stack | Role |
|-------|------|-------------|------|
| **T1** | [esp32-app-foundation](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation) | Principal Architect Skill + Claude Opus 4.6 / Gemini Pro 3.1 | Multi-agent orchestrated (PA → Workers) |
| **T2** | [esp32-app-foundation-t2](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation-t2) | Antigravity Teamwork-Preview + Gemini Flash 3.5 High | Teamwork collaborative agents |

---

## Executive Summary

Both implementations deliver a functional ESP32 application framework, but they differ significantly in **depth, completeness, and engineering rigour**. **T1 is the substantially stronger build** — it delivers more code, more tests, better spec compliance, and more mature embedded patterns. T2 is a credible implementation that nails several architectural decisions and introduces some interesting design divergences, but it has notable gaps in project hygiene, test coverage, and spec fidelity.

| Dimension | T1 | T2 | Winner |
|-----------|----|----|--------|
| Total Production Code | ~16,600 LoC | ~11,100 LoC | T1 |
| Test Code | ~3,974 LoC (24 files, 15 suites) | ~1,060 LoC (11 files, 5 suites) | T1 |
| Spec Compliance | High (WiFi TODO noted) | Moderate (structural gaps) | T1 |
| Architecture & OOP | Excellent | Good | T1 |
| Embedded Best Practices | Strong | Good with caveats | T1 |
| Documentation | Outstanding | Very Good | T1 |
| Innovation/Divergence | Conservative, spec-faithful | More creative adaptations | Mixed |

---

## 1. Project Structure & Completeness

### Spec-Required Artefacts Comparison

| Artefact | Spec Requirement | T1 | T2 |
|----------|-----------------|----|----|
| `src/main.cpp` | Framework entry point | ✅ [main.cpp](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/src/main.cpp) | ❌ **Missing** — only in `template/` |
| `library.json` | PlatformIO manifest | ✅ [library.json](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/library.json) (1.4KB) | ❌ **Missing** at root |
| `.gitignore` | Repo hygiene | ✅ Present | ❌ **Missing** |
| `README.md` | Project docs | ✅ [README.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/README.md) | ❌ **Missing** |
| `CONTRIBUTING.md` | Developer guide | ✅ [CONTRIBUTING.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/CONTRIBUTING.md) | ❌ **Missing** |
| `sdkconfig.defaults` | ESP-IDF config | ✅ At root (797B) | ⚠️ Only inside `template/` |
| `sdkconfig.defaults.esp32s3nano` | S3 variant | ✅ At root (763B) | ⚠️ Only inside `template/` |
| `CMakeLists.txt` | ESP-IDF compat | ✅ Present | ❌ Missing |
| `include/portal_assets/` | PROGMEM gzip output | ✅ Generated | ❌ Not generated |
| Git repository | Initialized repo | ✅ `.git/`, `.github/` | ❌ **Not a git repo** |

> [!IMPORTANT]
> T2 has **no root `src/main.cpp`**, meaning the framework cannot be compiled standalone from root. The entry point exists only in `template/src/main.cpp`. This fundamentally changes the project topology from a buildable library+app to a library-only layout. The spec explicitly requires a root entry point per the Hollywood Principle.

### Module Presence

| Module | Spec | T1 | T2 |
|--------|------|----|----|
| `modules/application/` | Kernel | ✅ | ✅ |
| `modules/configManager/` | Kernel | ✅ | ✅ |
| `modules/keyManager/` | Kernel | ✅ | ✅ |
| `modules/wifiManager/` | Kernel | ✅ | ✅ |
| `modules/timeManager/` | Kernel | ✅ | ✅ |
| `modules/otaUpdateManager/` | Kernel | ✅ | ✅ |
| `modules/eventBus/` | Kernel | ✅ | ✅ |
| `modules/dataManager/` | Kernel | ✅ | ✅ |
| `modules/webServer/` | Kernel | ✅ | ✅ |
| `modules/mcpServer/` | Kernel | ✅ (6 files, 1,226 LoC) | ✅ |
| `modules/u8g2Display/` | Optional | ✅ (38.6KB) | ✅ (13.2KB) |
| `modules/scheduler/` | Optional | ❌ Not implemented | ✅ **Bonus** |
| `lib/boardLED/` | Optional | ❌ Not implemented | ✅ **Bonus** |
| `lib/buttonHandler/` | Optional | ❌ Not implemented | ✅ **Bonus** |

**Verdict:** T1 delivers all spec-required scaffolding (git, library.json, README, CONTRIBUTING, root entry point). T2 omits critical project infrastructure but compensates with three additional optional modules (`scheduler`, `boardLED`, `buttonHandler`).

---

## 2. Architecture & OOP Evaluation

### 2.1 Application Hub — Composition Root

Both builds correctly implement the Application hub as a **composition root** using the **Hollywood Principle** (framework owns `setup()`/`loop()`). Both implement the **two-tier vector model** with `_compactAndLock()`. Key differences:

#### T1 — [Application.hpp](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/application/Application.hpp) (258 lines) + [Application.cpp](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/modules/application/Application.cpp) (388 lines)

**Strengths:**
- **Return-value registration**: All `register*()` methods return `bool` — callers know if registration succeeded or was rejected
- **Strict null-check + lock-check guard** on every registration method
- **All core services owned by value** (EventBus, DataManager, WebServerManager, McpServer) — no heap allocation
- **No `virtual` keyword on the class itself** — Application is final by design, correctly not inheritable
- **Step-by-step boot documented** with 12 numbered steps in comments and Doxygen
- `begin()` returns `bool` for fatal error signalling

**Issues:**
- WiFiManager, ConfigManager, KeyManager, TimeManager, OtaUpdateManager are **not wired into the hub** — `begin()` has TODOs. These services exist as standalone modules but aren't composed
- No configManager/keyManager accessors exposed

#### T2 — [Application.hpp](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation-t2/modules/application/Application.hpp) (205 lines) + [Application.cpp](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation-t2/modules/application/Application.cpp) (202 lines)

**Strengths:**
- **All 9 kernel services owned by value** and fully wired — ConfigManager, KeyManager, WifiManager, TimeManager, OtaUpdateManager are all composed and booted
- **Complete service accessor coverage** (`getConfigManager()`, `getKeyManager()`, `getWifiManager()`, `getTimeManager()`, `getOtaUpdateManager()`)
- **Soft-reset mechanism is more sophisticated**: deferred via `requestSoftReset()` flag, prevents mid-tick structural mutation, supports `onRegister()` callback and function pointer for re-registration
- **Two registration tiers**: structural (locked) vs lightweight (always-open) — a creative extension of the spec

**Issues:**
- Registration methods return `void`, not `bool` — callers cannot detect rejection. Uses `assert()` instead, which is a **hard crash** in embedded contexts (extremely dangerous on production hardware)
- `virtual ~Application() = default` + `protected onRegister()` — Application is designed as an inheritable base class, which **violates** the spec's "Composition over Inheritance" rule (CA-02)
- `begin()` returns `void`, not `bool` — no way to signal fatal boot failure
- Constructor pre-registers kernel managers as config consumers — coupling at construction time

> [!WARNING]
> T2's use of `assert()` for post-lock registration violations is a **critical embedded anti-pattern**. In production firmware, assertions typically abort the program. The spec calls for graceful error logging and rejection, which T1 implements correctly.

### 2.2 Interface Design Comparison

| Interface | T1 Approach | T2 Approach | Verdict |
|-----------|------------|------------|---------|
| `ITickable::tick()` | `void tick()` — no args | `void tick(uint32_t currentMillis)` — receives timestamp | **T2 better** — avoids each module calling `millis()` |
| `iConfigurable` | 3 methods: `loadConfig()`, `saveConfig()`, `resetConfig()` — self-managed | 1 method: `reapplyConfig(const FrameworkConfig&)` — push-based | **T1 closer to spec** — §6 defines module-owned `/mod/*.json` files |
| `iDataSource` | 4 pure virtuals: clean, minimal contract | 10+ methods incl. `serializeData()`, `lockData()`, `testConnection()`, hash helpers | **T1 better** — T2 is over-specified, couples to ArduinoJson in interface |
| `IWebContributor` | `registerRoutes()` + optional `getTabName()` | `registerRoutes()` + `getTabName()` + `getTabEndpoint()` | **T2 richer** — better SPA extension model |
| `IStatusReporter` | `reportStatus(msg, progress)`, `reportError(msg)`, `clearStatus()` | `reportMessage(category, msg)`, `reportProgress(op, %)`, `reportError(op, msg)` | **T2 slightly better** — categorized reporting |

> [!NOTE]
> T2's `iDataSource` interface includes `ArduinoJson.h` — this is an **interface-level dependency** that violates the Dependency Inversion Principle. Interfaces should depend on abstractions, not on concrete serialization libraries. T1 keeps `iDataSource` free of library dependencies.

### 2.3 Encapsulation & SOLID Compliance

| Principle | T1 | T2 |
|-----------|----|----|
| **SRP** | ✅ Clean separation. Each module has one responsibility | ✅ Good separation. Scheduler is a clean addition |
| **OCP** | ✅ Extension via interface registration | ✅ Extension via interface registration |
| **DIP** | ✅ Interfaces define contracts, concrete types injected | ⚠️ `iDataSource` pulls in `ArduinoJson.h` and `JsonListener` |
| **Encapsulation** | ⚠️ `GithubClient` exposes public fields | ⚠️ `u8g2Display` system boards are public members |
| **No Cross-Module Includes** | ✅ Optional modules don't include each other | ✅ Optional modules don't include each other |
| **Kernel Knows Nothing** | ✅ Kernel headers never import module headers | ✅ Maintained |
| **Composition over Inheritance** | ✅ No `BaseApp`, no virtual Application | ❌ `Application` has `virtual ~Application()` and `protected onRegister()` |

---

## 3. Embedded Systems Best Practices

### 3.1 Memory Management

| Practice | T1 | T2 |
|----------|----|----|
| Two-tier vector model | ✅ All 5 vectors compacted + locked | ✅ 3 structural + 2 lightweight vectors |
| `shrink_to_fit()` post-lock | ✅ On all vectors | ⚠️ Only on 3 structural vectors |
| Heap reporting | ✅ `heap_caps_get_free_size` + min watermark | ✅ `ESP.getFreeHeap()` (less detailed) |
| EventBus allocation | ✅ Fixed 32-slot array (`Subscription[]`) | ✅ Fixed 32-slot array |
| No dynamic alloc in tick | ✅ Verified | ✅ Verified |
| `StaticJsonDocument` vs `DynamicJsonDocument` | ⚠️ Mixed v6/v7 API in ConfigManager | ✅ ArduinoJson v7 used throughout |
| Stack vs heap objects | ✅ Static `Application app` in main.cpp | ✅ Template follows same pattern |

### 3.2 RTOS Patterns

| Pattern | T1 | T2 |
|---------|----|----|
| FreeRTOS task for DataManager | ✅ 8KB stack, priority 5, pinned to Core 0 | ✅ Background thread with mutex |
| RAII MutexLock | ✅ DataManager has `MutexLock` helper | ⚠️ Uses raw `xSemaphoreTake`/`xSemaphoreGive` |
| Tick budget (≤10ms) | ✅ Documented in ITickable contract | ⚠️ Not documented in ITickable, but implemented |
| `NATIVE_BUILD` guards | ✅ Consistent across all modules | ✅ Consistent, plus `__has_include` fallback |

### 3.3 Platform Abstraction

| Pattern | T1 | T2 |
|---------|----|----|
| NATIVE_BUILD compile guards | ✅ All ESP-IDF calls wrapped | ✅ All ESP-IDF calls wrapped |
| Native mock infrastructure | ⚠️ `String` stub duplicated in 4+ files | ✅ Centralised mock directory: [test/mocks/](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation-t2/test/mocks) |
| Native test framework | GoogleTest 1.15.2 | ArduinoFake 0.4.0 + custom test_main |
| C++ standard | `-std=gnu++17` | `-std=gnu++14` (hardware) |

> [!TIP]
> T2's centralised [test/mocks/](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation-t2/test/mocks) directory with mock headers for `FS.h`, `WiFi.h`, `Preferences.h`, `U8g2lib.h`, and FreeRTOS stubs is a **better pattern** than T1's repeated inline `String` stub duplication. T1 should extract these to a shared mock header.

### 3.4 Build Configuration

| Aspect | T1 | T2 |
|--------|----|----|
| Base config inheritance | ✅ `[env:base_esp32]` + `extends` — DRY | ❌ Full config duplicated between esp32dev/s3nano |
| Include paths | ✅ Single `-I include`, PIO resolves rest | ⚠️ **17 explicit `-I` flags** — fragile, requires manual update per module |
| Platform source | `espressif32` (official) | `pioarduino` GitHub fork (non-standard) |
| C++ standard | C++17 (both HW and native) | C++14 native / unspecified HW |
| `lib_compat_mode` | `strict` | Not specified in HW envs |
| S3 partition table | ✅ LittleFS SubType correct | ❌ **Uses `spiffs` SubType** with LittleFS code |
| `CONFIG_AUTOSTART_ARDUINO` | ✅ In sdkconfig.defaults | ⚠️ In template only, not at root |
| Build scripts | 5 scripts (portal, fonts, timestamp, layouts, lint) | 1 script (portal only) |

> [!CAUTION]
> T2's S3 partition table declares SubType `spiffs` but the codebase uses `LittleFS` — this will cause a filesystem mount failure on the S3 target. T2's 17 explicit `-I` flags in `platformio.ini` is also a maintenance hazard; adding a new module requires updating 3 environment sections.

---

## 4. Test Coverage & Quality

### 4.1 Quantitative Comparison

| Metric | T1 | T2 |
|--------|----|----|
| Total test code | **3,974 LoC** | **1,060 LoC** |
| Test files | 24 | 11 |
| Test suites | 15 | 5 native + 1 E2E |
| Test framework | GoogleTest 1.15.2 | Custom test_main + ArduinoFake |
| Modules with tests | 15/15 (100%) | 5/12 (42%) |
| E2E test | Empty (`.gitkeep`) | ✅ Playwright web portal tests |

### 4.2 Module-Level Coverage

| Module | T1 Tests | T2 Tests |
|--------|---------|---------|
| Application | ✅ 26 tests, 5 groups | ❌ None |
| EventBus | ✅ ~10 tests | ✅ 4 tests |
| ConfigManager | ✅ ~15 tests | ✅ 5 tests |
| KeyManager | ✅ ~12 tests | ✅ 4 tests |
| DataManager | ✅ ~12 tests | ❌ None |
| WebServer | ✅ ~8 tests | ❌ None |
| McpServer | ✅ **20+ tests** (full JSON-RPC) | ❌ None |
| WifiManager | ✅ ~10 tests | ❌ None |
| TimeManager | ✅ ~6 tests | ✅ 3 tests |
| OtaUpdateManager | ✅ ~8 tests | ❌ None |
| u8g2Display | ✅ **8 test files** (~900 LoC) | ✅ 4 carousel tests |
| DeviceCrypto | ✅ Tests present | ❌ None |
| Logger | ✅ Tests present | ❌ None |
| Scheduler | N/A | ❌ None |

### 4.3 Quality Assessment

**T1 Test Quality**: ★★★★★
- Proper GoogleTest fixtures with `SetUp()`/`TearDown()`
- Hand-rolled mocks are clean and minimal
- Tests cover positive paths, error paths, edge cases (null pointers, post-lock rejection)
- MCP server tests exercise full JSON-RPC protocol including malformed input
- Display tests cover carousel rotation, board lifecycle, sleep timeout

**T2 Test Quality**: ★★★★☆
- Good mock infrastructure (centralised, well-structured)
- Realistic filesystem simulation via `MockLittleFS`
- **Playwright E2E web tests** are a standout feature T1 lacks — tests the portal SPA end-to-end
- Coverage is too thin — only 5 of 12 modules have any native tests

> [!IMPORTANT]
> T1 has **3.75× more test code** than T2 and covers **100% of modules** vs T2's **42%**. T2's gap is particularly concerning for safety-critical modules like `WifiManager`, `OtaUpdateManager`, and `DataManager` — all untested.

---

## 5. Spec Compliance Audit

### 5.1 Non-Negotiable Architectural Rules

| Rule | Description | T1 | T2 |
|------|-------------|----|----|
| 1 | Hollywood Principle — framework owns `setup()`/`loop()` | ✅ | ⚠️ No root main.cpp |
| 2 | Composition over Inheritance | ✅ | ❌ Application is inheritable |
| 3 | No Cross-Module Includes | ✅ | ✅ |
| 4 | Kernel Knows Nothing About Modules | ✅ | ✅ |
| 5 | Two-Tier Vector Model (no `MAX_*`) | ✅ | ✅ |
| 6 | LittleFS Directory Convention | ✅ | ✅ |
| 7 | House Style Documentation | ✅ Outstanding | ✅ Very Good |
| 8 | `tick()` ≤ 10ms | ✅ Documented in interface | ⚠️ Not documented |
| 9 | Static module instances | ✅ | ✅ |
| 10 | No `DynamicJsonDocument` | ⚠️ v6 API leak in ConfigManager | ✅ |
| 11 | EventBus: static fn-ptr + `void* ctx` | ✅ | ✅ |
| 12 | `WiFiClientSecure` heap-allocated in `executeFetch()` only | ✅ | ✅ |

**T1 compliance: 11/12** (one v6/v7 API inconsistency)  
**T2 compliance: 9/12** (Hollywood Principle gap, inheritance violation, tick budget undocumented)

### 5.2 Confirmed Architecture Decisions (CAs)

| CA | Decision | T1 | T2 |
|----|----------|----|----|
| CA-01 | Monorepo | ✅ | ✅ |
| CA-02 | No separate `esp32-modules` repo | ✅ | ✅ |
| CA-03 | Git submodule consumption (no `lib_deps` registry) | ✅ library.json configured | ❌ No library.json |
| CA-04 | `template/` folder inside repo | ✅ | ✅ |
| CA-05 | `std::vector` two-tier (no `MAX_*`) | ✅ | ✅ |
| CA-06 | Dual framework `arduino + espidf` | ✅ | ✅ |

### 5.3 Discovered Standards (D-01, D-02)

| Standard | T1 | T2 |
|----------|----|----|
| D-01: `CONFIG_AUTOSTART_ARDUINO=y` in sdkconfig.defaults | ✅ | ⚠️ Only in template |
| D-02: `lib_extra_dirs = modules` in both base and native | ✅ | ⚠️ Uses explicit `-I` instead |

---

## 6. Code Quality Deep Dive

### 6.1 Documentation Quality

| Metric | T1 | T2 |
|--------|----|----|
| File headers (license + module + description) | ✅ 100% coverage, standardised | ✅ 100% coverage, standardised |
| Doxygen on public methods | ✅ Comprehensive | ✅ Comprehensive |
| Inline member comments | ✅ `///< suffix` on all members | ✅ `///< suffix` on all members |
| Exported Functions/Classes block | ✅ Every file | ✅ Every file |
| README / CONTRIBUTING | ✅ Both present | ❌ Both missing |
| Step-by-step boot comments | ✅ 12 numbered steps in begin() | ✅ 6 numbered steps in begin() |

Both builds demonstrate **excellent house-style compliance**. T1 edges ahead with more detailed boot-sequence documentation and project-level documentation files.

### 6.2 Error Handling

| Pattern | T1 | T2 |
|---------|----|----|
| Registration rejection | Returns `false` + error log | `assert()` crash |
| Boot failure | Returns `false` from `begin()` | No error return (`void`) |
| Null pointer guards | Explicit check + reject | Null check + silent return |
| LittleFS mount failure | Fatal return false + log | Not visible in root (template only) |

T1's error handling is **production-grade**. T2's use of `assert()` is appropriate for development but **dangerous in production firmware** where a graceful degradation or watchdog-assisted restart is preferred over a hard crash.

### 6.3 Notable Design Innovations

**T1 Innovations:**
- **RAII MutexLock** in DataManager — elegant scoped locking pattern
- **McpServer implements IWebContributor** — HTTP POST bridge for MCP tools, enabling browser-based device control
- **`lint_includes.py`** — automated kernel-module include boundary enforcement
- **`generate_layouts.py`** — display layout code generation from constraints

**T2 Innovations:**
- **Scheduler module** — generic time-based callback system with daily and periodic scheduling
- **Deferred soft-reset** via `requestSoftReset()` flag — prevents mid-tick structural mutations
- **Centralised mock directory** — cleaner test infrastructure
- **Playwright E2E tests** for web portal — real browser validation
- **`iDataSource` with FNV-1a hash helpers** — built-in change detection for data sources
- **Two-tier registration** (structural vs lightweight) — allows web/config contributors to register dynamically

---

## 7. Display Module Comparison

| Aspect | T1 | T2 |
|--------|----|----|
| Total display code | **38.6KB** | **13.2KB** |
| Widget toolkit | 9 widgets, 19 files | 9 widgets, 19 files |
| Board implementations | 6 boards + `iBoardLayout` interface | 6 boards + interface |
| `iGfxWidget` base class | ✅ 5.7KB — rich (position, size, visibility, tick/render) | ✅ 2.2KB — minimal |
| `drawingPrimitives` | ✅ 7.8KB — full text alignment/truncation system | ✅ 5.1KB — basic |
| `U8g2StateSaver` RAII | ✅ Present | ❌ Not found |
| Font management | ✅ Dedicated fonts/ dir | ✅ Dedicated fonts/ dir |
| Display config file | ✅ `/mod/display.json` | ⚠️ Stale `mod_display.json` at root |
| Frame pacing | ✅ 20 FPS (50ms) gate | ✅ Present |
| Sleep/screensaver | ✅ Timeout → screensaver → OLED off | ✅ SleepClockBoard |

T1's display module is **nearly 3× the size** of T2's, with significantly richer widget implementations (e.g., `scrollingTextWidget` at 8.1KB vs 4.6KB). T1 includes `U8g2StateSaver` — an RAII scope guard for U8G2 graphics state that prevents font/clip/color leaks during nested renders. This is a production-quality pattern missing from T2.

---

## 8. Web Portal Comparison

| Aspect | T1 | T2 |
|--------|----|----|
| Architecture | **6 separate files** (HTML + CSS + 4 JS modules) | **Single 28KB monolithic HTML** |
| Tabs | Config, WiFi, System, OTA | WiFi Setup, System Info, OTA Update |
| API request queuing | ✅ Sequential XHR (protects ESP32 socket pool) | ✅ `ApiQueue` with 50ms cooldown |
| Dynamic extension tabs | ✅ Via `/api/tabs` | ✅ Via `/api/tabs` |
| Portal builder | ✅ Minify + per-file gzip → PROGMEM headers | ✅ Strip + compress → single PROGMEM array |
| Styling | Clean responsive CSS (2.3KB) | Dark glassmorphic theme, CSS custom properties |
| E2E tests | ❌ None | ✅ **Playwright** 5 test cases |
| Total size | ~10.5KB source | ~28KB source |

T2's **Playwright E2E tests** for the web portal are a genuine standout — this is a testing dimension T1 completely lacks. T2's dark glassmorphic theme is also more visually polished. However, T1's modular 6-file structure is better for maintainability and incremental PROGMEM serving.

---

## 9. Build Pipeline Comparison

| Script | T1 | T2 |
|--------|----|----|
| `portalBuilder.py` | ✅ 76 lines, per-file gzip | ✅ 149 lines, single-file compress |
| `build_fonts.py` | ✅ 7KB — U8G2 font compilation | ❌ Missing |
| `build_timestamp.py` | ✅ 2KB — `BUILD_TIME` macro | ❌ Missing |
| `generate_layouts.py` | ✅ 10KB — display layout constants | ❌ Missing |
| `lint_includes.py` | ✅ 4.6KB — include boundary enforcement | ❌ Missing |

T1 has a **5× richer build toolchain**. The `lint_includes.py` script is particularly valuable — it enforces the "Kernel Knows Nothing About Modules" rule at the CI level.

---

## 10. Summary Scorecard

| Dimension | T1 | T2 | Notes |
|-----------|:---:|:---:|-------|
| **Project Structure** | 9/10 | 5/10 | T2 missing git, README, library.json, root entry point |
| **Interfaces** | 9/10 | 7/10 | T2 couples iDataSource to ArduinoJson; naming inconsistency shared |
| **Application Hub** | 8/10 | 8/10 | T1 safer (bool returns). T2 more complete (all services wired) |
| **Kernel Services** | 8/10 | 9/10 | T2 has all services integrated end-to-end. T1 has WiFi TODO |
| **Libraries** | 7/10 | 8/10 | T2 has bonus boardLED/buttonHandler; T1 duplicates String stubs |
| **Build Config** | 9/10 | 5/10 | T2 duplicated config, wrong S3 SubType, 17 manual `-I` flags |
| **Tests** | 9/10 | 6/10 | T1 has 3.75× more tests, 100% module coverage |
| **Display Module** | 9/10 | 7/10 | T1 3× more code, U8g2StateSaver RAII, richer widgets |
| **Web Portal** | 7/10 | 8/10 | T2 has Playwright E2E, glassmorphic styling |
| **Build Pipeline** | 9/10 | 4/10 | T1 has 5 scripts vs T2's 1 |
| **Documentation** | 10/10 | 8/10 | Both have house-style; T1 has README + CONTRIBUTING |
| **Embedded Practices** | 9/10 | 7/10 | T2's assert(), void returns, and S3 partition bug are concerns |
| **OOP Compliance** | 9/10 | 7/10 | T2's inheritable Application and DIP violation in iDataSource |
| **TOTAL** | **112/130** (86%) | **89/130** (68%) | |

---

## 11. Detailed Findings by Team

### T1 — Key Strengths
1. **Exemplary test coverage** — 15 suites, 3,974 LoC, covers every single module
2. **Production-grade error handling** — `bool` returns, error logging, graceful degradation
3. **Outstanding documentation** — README, CONTRIBUTING, house-style on every file, numbered boot steps
4. **Rich build toolchain** — 5 Python scripts including include-boundary linter
5. **MCP server sophistication** — 1,226 LoC, full JSON-RPC 2.0 with HTTP bridge
6. **U8g2StateSaver** RAII pattern for graphics state management

### T1 — Key Weaknesses
1. **WiFiManager not integrated** — `begin()` has TODO, most critical service is disconnected
2. **String stub duplication** — copy-pasted in 4+ headers, should be a shared mock
3. **Mixed ArduinoJson v6/v7** API usage in ConfigManager
4. **No E2E or hardware tests** — E2E directory is empty
5. **EventBus has legacy copyright header** (still says "Departures Board")

### T2 — Key Strengths
1. **All kernel services fully wired** — ConfigManager, KeyManager, WiFi, Time, OTA all integrated and booting
2. **Centralised mock infrastructure** — clean `test/mocks/` with comprehensive stubs
3. **Playwright E2E tests** — real browser-based validation of the web portal
4. **Scheduler module** — bonus generic callback system the spec listed as optional
5. **Deferred soft-reset** pattern — prevents mid-tick structural corruption
6. **Polished web portal** — glassmorphic dark theme with API request queuing

### T2 — Key Weaknesses
1. **Missing project infrastructure** — no git repo, README, CONTRIBUTING, library.json, .gitignore
2. **`assert()` for registration errors** — hard crash instead of graceful rejection
3. **`begin()` returns `void`** — no fatal error signalling
4. **S3 partition SubType mismatch** — `spiffs` declared but `LittleFS` used
5. **iDataSource couples to ArduinoJson** — violates DIP at the interface level
6. **58% of modules have zero tests** — WifiManager, DataManager, OTA, WebServer, MCP, Scheduler all untested
7. **Application is inheritable** — violates "Composition over Inheritance" spec rule
8. **17 manual `-I` flags** in platformio.ini — maintenance hazard

---

## 12. Recommendations

### For T1 (to reach production-ready)
1. **Wire WiFiManager into Application.begin()** — this is the single biggest gap
2. **Extract String stubs** to a shared `test/mocks/NativeBuildStubs.hpp`
3. **Resolve ArduinoJson v6/v7 API inconsistency** in ConfigManager
4. **Add Playwright E2E tests** for the web portal (learn from T2's approach)
5. **Update EventBus copyright header** to match project

### For T2 (to reach production-ready)
1. **Create root `src/main.cpp`** — framework must be standalone-buildable
2. **Add `library.json`, `README.md`, `CONTRIBUTING.md`, `.gitignore`**
3. **Initialize git repository** with proper `.gitignore`
4. **Replace `assert()` with `bool` returns** in all registration methods
5. **Fix S3 partition table** — change SubType from `spiffs` to `littlefs`
6. **Remove ArduinoJson dependency from `iDataSource.hpp`**
7. **Add tests for remaining 7 modules** (WiFi, Data, OTA, Web, MCP, Scheduler, Application)
8. **Remove `virtual` from Application** — enforce composition over inheritance
9. **Extract base config** in `platformio.ini` to eliminate duplication
10. **Use `-std=gnu++17`** consistently (currently gnu++14 for native)

---

## 13. Conclusion

**T1 is the clear winner** on engineering rigour, spec compliance, test coverage, and production readiness. Its multi-agent orchestration (PA → Workers) approach with mandatory skill loading produced a more disciplined, thorough, and better-documented codebase. The 3.75× test advantage is particularly significant for an embedded framework where bugs have real-world hardware consequences.

**T2 demonstrates genuine architectural creativity** — the deferred soft-reset, centralized mocks, Playwright E2E tests, and full kernel service wiring are all areas where it surpasses T1. Its implementation of three bonus optional modules (scheduler, boardLED, buttonHandler) shows broader functional ambition.

The quality gap likely reflects the **agent orchestration model**: T1's Principal Architect pattern with explicit work orders, mandatory skill boot sequences, and gate reviews naturally produces more rigorous output. T2's collaborative teamwork model delivered faster but with less structural discipline around testing and project hygiene.

Both builds would benefit from **cross-pollination**: T1 should adopt T2's centralized mocks and Playwright tests; T2 should adopt T1's error handling patterns, build toolchain, and testing discipline.
