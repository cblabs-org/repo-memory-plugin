# Build / Flash / Soak Test Plan — `esp32s3nano`

> **Target:** `esp32s3nano` environment on `/dev/cu.usbmodem101`
> **Build from:** `main` branch (HEAD: `e4f08be4`)
> **Last tag:** `v0.0.12-dev.83` — **36 commits / 23 merged PRs** since then

---

## Merged PRs Since Last Tag — Categorized

### 🔧 Runtime / Memory Safety (must verify on hardware)

| PR | Issue | Summary | Hardware Signal |
|----|-------|---------|----------------|
| #268 | #254 | Remove remaining stored `std::function` | No crash, no heap fragmentation |
| #267 | #255 | Convert bare `new` to `nothrow` | No alloc panic, graceful degradation |
| #266 | #256 | Audit missing `nothrow` | Same as above |
| #243 | #135 | Harden OTA screenshot mutex | No deadlock during OTA check |

### 📡 OTA / Security (must verify on hardware)

| PR | Issue | Summary | Hardware Signal |
|----|-------|---------|----------------|
| #272 | #252 | OTA token TLS fix | OTA check completes without TLS error |
| #273 | #251 | Unsigned OTA default | OTA verifier initializes correctly |
| #270 | #226 | OTA defer boot | Boot completes before OTA check starts |

### 🌐 Portal / UI (verify via serial + HTTP)

| PR | Issue | Summary | Hardware Signal |
|----|-------|---------|----------------|
| #271 | #80 | Portal branding FOUC fix | No flash of unstyled content |
| #260 | #233 | Portal precondition health review | Precondition endpoint responds |
| #250 | #140 | Portal SVG icon stroke fix | Icons render correctly |
| #248 | #141 | Password input eye button fix | Password toggle works |
| #247 | #105 | Link portal version to release notes | Version displayed on portal |
| #264 | #234 | Move Bluetooth button after WiFi | Button order correct |

### 📊 Observability / Diagnostics

| PR | Issue | Summary | Hardware Signal |
|----|-------|---------|----------------|
| #258 | #213 | Surface redactor saturation | Redactor health in serial logs |
| #265 | #241 | Sample app location gate fix | Location gate doesn't false-positive |

### 🔵 BLE / WiFi Coexistence

| Commit | Summary | Hardware Signal |
|--------|---------|----------------|
| `6397199d` | BLE TX power +9 dBm for WiFi coex | WiFi + BLE stable simultaneously |

### 📝 Docs-only / Tooling (no hardware verification needed)

| PR | Issue | Summary |
|----|-------|---------|
| #274 | #201 | Remove test ignore directive |
| #259 | #202 | Document OTA findings |
| #261 | — | Webserver startup rationale |
| #262 | #183 | Document setInsecure rationale |
| #263 | #223 | Compile-check fenced C++ in docs |
| #257 | #231 | Rename framework-design |
| #244 | #242 | Fix native test LVGL stubs |

---

## Execution Plan — 5 Phases

### Phase 0: Pre-flight (read-only, no approval needed)
1. Checkout `main` branch (working tree is clean except for untracked bug-fixer dispatch files)
2. Resolve target: `agent_resolve_target` for `esp32s3nano`
3. Run `agent_safe_pin_audit` for strapping pin check
4. Verify PIO lock is free

### Phase 1: Tag → Build → Flash
1. **Tag:** `v0.0.12-dev.84` (36 untagged commits with new functionality → Flash Gate requires tag)
2. **Build:** `agent_build_diagnose` for `esp32s3nano` — record RAM/flash usage
3. **Flash:** `agent_flash_monitor_verify` with boot markers

**Expected boot markers:**
- `[AppFoundation] Boot complete`
- `[WiFi]` initialization
- `[BLE]` initialization
- `[WebServer]` listening
- `[OTA]` check deferred / scheduled

**Rejected patterns:**
- `panic`, `Guru Meditation`, `watchdog`, `brownout`, `reboot`, `abort()`

### Phase 2: Post-Boot Verification (serial + HTTP, ~2 min)
1. `capture_serial_window` — 30s stability window, confirm no crash/restart
2. HTTP health check via `curl` to device IP:
   - `GET /api/system` → verify `firmware_version` matches `v0.0.12-dev.84`
   - `GET /api/health` → all subsystems healthy
   - `GET /api/preconditions` → precondition engine responding (#233)
3. Verify BLE advertising active in serial logs (#239)
4. Check for redactor saturation warnings (#213)

### Phase 3: Soak Test — 15 min runtime stability
1. Start serial monitor
2. Set a 15-minute timer
3. Every 5 minutes, capture a serial window and check:
   - **No crash / reboot** — device uptime monotonically increasing
   - **No heap degradation** — free heap not declining more than 5% from baseline
   - **WiFi remains connected** — no disconnection events
   - **BLE remains advertising** — no BLE stack errors
   - **No OTA deadlock** — OTA check should complete cleanly (#135, #226)
   - **No `std::function` related crash** (#254)
   - **No `new` allocation panic** (#255, #256)
4. After 15 mins, run final `agent_monitor_health` assessment

### Phase 4: Results Summary
1. Compile pass/fail for each hardware-relevant PR
2. Record heap baseline vs final
3. Record WiFi/BLE stability metrics
4. Flag any anomalies for investigation

---

## Autonomy Boundaries

| Action | Autonomous? | Notes |
|--------|------------|-------|
| Checkout `main` | ✅ | Read-only git operation equivalent |
| Tag `dev.84` | ✅ | Per versioning rule: dev tags are autonomous |
| Build | ✅ | No hardware risk |
| **Flash** | ⚠️ **Needs approval** | Per skill rules: ask before flashing |
| Monitor / soak | ✅ | Read-only serial observation |
| HTTP health checks | ✅ | Read-only device queries |

> [!IMPORTANT]
> **I will need your explicit approval once to flash the device.** After that, the entire soak test runs autonomously.

---

## Current State Summary

- **Branch:** `pr-281-latest` (4 commits ahead of main — unmerged #280 bug-fixer work)
- **Device:** ESP32-S3 Nano on `/dev/cu.usbmodem101` ✅
- **PIO Lock:** Free ✅
- **Working tree:** Clean (only untracked dispatch files) ✅
- **Stash:** 4 entries (from previous sessions, safe to ignore)
