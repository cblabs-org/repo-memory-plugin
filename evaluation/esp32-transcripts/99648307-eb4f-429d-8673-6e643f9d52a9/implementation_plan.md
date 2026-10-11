# Implementation Plan - Designing and Building the ESP32 App Foundation Framework

This implementation plan outlines the architecture, directory structure, and step-by-step phased approach for designing and writing the new **ESP32 App Foundation** framework from first principles in the empty `esp32-app-foundation-t2` workspace, utilizing details and design patterns mined from the `departures-board` reference repository.

## Audit Checklist
- [x] Reviewed by `house-style-docs` - passed (Standard camelCase file naming, license headers, and Doxygen method comments outlined)
- [x] Reviewed by `oo-expert` - passed (Composition over inheritance, pure interface segregation, and strict DIP patterns applied)
- [x] Reviewed by `embedded-web-designer` - N/A (No visual portal layout changes proposed in this plan)
- [x] Reviewed by `embedded-systems` - passed (Resource Impact Assessment for ESP32 memory, power, and security constraints completed)

---

## User Review Required

> [!IMPORTANT]
> **Key Architecture Decisions:**
> 1. **Hollywood Principle**: The framework kernel owns the `setup()` and `loop()` lifecycle methods, invoking the application and optional modules via pure virtual interface callbacks.
> 2. **Two-Tier Vector Model**: Structural arrays (`ITickable*`, `iDataSource*`, etc.) are built at boot/soft-reset and locked after `shrink_to_fit()`, preventing heap fragmentation in the steady-state `tick()` loop. Lightweight arrays (configurations, routes) are allowed to grow.
> 3. **No Cross-Module Direct Dependency**: Optional modules (e.g., `u8g2Display`, `scheduler`) never include each other. All inter-module communication is decoupled via the `EventBus` using payload-type-agnostic `const void*` publishing with static callback function pointers (no `std::function` heap overhead).
> 4. **Hardware Cryptography**: `DeviceCrypto` uses ESP-IDF/MbedTLS to derive a hardware-bound key from MAC/eFuse to encrypt credentials stored in `/etc/apikeys.json`. Plaintext keys on flash are strictly forbidden.
> 5. **Dual Framework Build**: Combines Arduino (for display/web servers) and ESP-IDF (for boot partition rollback, encryption, and IDF logging).

---

## Open Questions

> [!WARNING]
> 1. **Native Cryptography Fallback**: How should `DeviceCrypto` simulate key derivation under `NATIVE_BUILD`?
>    * *Proposed Solution*: Use a constant mock key generated from a simulated MAC address, allowing GoogleTest suites to run without real eFuse access.
> 2. **DNS/WebServer Mocking**: Should we mock `AsyncWebServer` and `DNSServer` classes in headers or provide a central stub file?
>    * *Proposed Solution*: Provide canonical stub headers in `test/mocks/` to avoid ODR (One Definition Rule) violations in native test builds.
> 3. **OTA Blessing Ceremony**: Should the framework automatically bless the current partition on the first successful data fetch, or should the app make the final blessing call?
>    * *Proposed Solution*: The `Application` hub will bless the partition after confirming WiFi connected, LittleFS mounted, and all core services initialized successfully, preventing bootloops.

---

## Proposed Changes

The implementation will be completed in 7 sequential phases:

### Phase 1: Foundation Kernel Interfaces & Base Constants
Create the pure abstract interfaces that decouple modules and drive the kernel.

#### [NEW] [ITickable.hpp](include/interfaces/ITickable.hpp)
* Interface for loop execution.
#### [NEW] [iConfigurable.hpp](include/interfaces/iConfigurable.hpp)
* Interface for config consumption and hot-reloading.
#### [NEW] [iDataSource.hpp](include/interfaces/iDataSource.hpp)
* Interface for background data fetching.
#### [NEW] [IStatusReporter.hpp](include/interfaces/IStatusReporter.hpp)
* Interface for cross-module boot/OTA/error progress updates.
#### [NEW] [IWebContributor.hpp](include/interfaces/IWebContributor.hpp)
* Interface for registering web routes and custom nav tabs.
#### [NEW] [IMcpContributor.hpp](include/interfaces/IMcpContributor.hpp)
* Interface for registering JSON-RPC tools with the MCP server.
#### [NEW] [AppFoundation.h](include/AppFoundation.h)
* Single-include façade for downstream applications.

---

### Phase 2: Core Utility Libraries
Implement low-level platform-agnostic wrappers in `lib/`.

#### [NEW] [deviceCrypto](lib/deviceCrypto/)
* ESP-IDF MbedTLS wrappers for hardware key derivation and AES-256 credential encryption.
#### [NEW] [logger](lib/logger/)
* Wrapper around ESP-IDF logger exposing standard log levels and the `LOG_SECURE()` redaction macro.
#### [NEW] [md5Utils](lib/md5Utils/)
* MbedTLS streaming MD5 implementation for incremental firmware download validation.
#### [NEW] [githubClient](lib/githubClient/)
* Lightweight HTTPS client for querying releases and downloading firmware binaries.

---

### Phase 3: Kernel Core
Write the central composition root, EventBus, and boot state machine.

#### [NEW] [AppState.hpp](include/AppState.hpp)
* System states: `BOOTING`, `WIFI_SETUP`, `APP_SETUP`, `RUNNING`, `WIFI_SHUTDOWN`.
#### [NEW] [FrameworkConfig.hpp](include/FrameworkConfig.hpp)
* Configuration structure for kernel settings (hostname, POSIX timezone, default brightness).
#### [NEW] [EventBus](modules/eventBus/)
* Static fn-ptr callback registry for synchronous decoupled publishing.
#### [NEW] [application](modules/application/)
* The `Application` hub that owns core components, schedules lifecycle phases, and locks structural vectors after `begin()`. Includes `softReset()` logic.

---

### Phase 4: Kernel Services
Create the core background workers and network management components.

#### [NEW] [configManager](modules/configManager/)
* JSON parser for `/etc/config.json` with schema migration and hot-reload.
#### [NEW] [keyManager](modules/keyManager/)
* Encryption-guarded API key store at `/etc/apikeys.json`.
#### [NEW] [wifiManager](modules/wifiManager/)
* Non-blocking WiFi connection manager, including captive portal fallback AP mode with 10-minute inactivity timeout.
#### [NEW] [timeManager](modules/timeManager/)
* Non-blocking NTP synchronization and POSIX timezone configuration.
#### [NEW] [dataManager](modules/dataManager/)
* Background FreeRTOS task handling priority-tiered scheduling for `iDataSource` clients.
#### [NEW] [otaUpdateManager](modules/otaUpdateManager/)
* GitHub HTTPS update engine with partition swap validation and automatic rollback.
#### [NEW] [webServer](modules/webServer/)
* ESPAsyncWebServer integration serving SPA PROGMEM gzip shell and calling `IWebContributor` endpoints.
#### [NEW] [mcpServer](modules/mcpServer/)
* JSON-RPC serial/HTTP executor exposing system configurations to developer agents.

---

### Phase 5: Display Module & Optional Modules
Write the monochrome graphic system and optional sensors/LED handlers.

#### [NEW] [u8g2Display](modules/u8g2Display/)
* Monochrome OLED board manager, custom Font definitions, and System boards (boot, AP setup, OTA progress, clock screensaver).
#### [NEW] [widgets](modules/u8g2Display/widgets/)
* 3-tier composable UI widget toolkit: Primitives (`iGfxWidget`), Generics (`labelWidget`), and Module-Aware (`wifiStatusWidget`).
#### [NEW] [scheduler](modules/scheduler/)
* Generic callback scheduler for time-based actions.
#### [NEW] [boardLED](lib/boardLED/)
* Decoupled hardware status LED handler.
#### [NEW] [buttonHandler](lib/buttonHandler/)
* Physical input handler with debounce and manual override triggers.

---

### Phase 6: Web Portal Frontend & Build Scripts
Package the front-end SPA and compiler pipeline scripts.

#### [NEW] [web/portal/](web/portal/)
* Vanilla HTML/CSS/JS single page application, including the request serialization `ApiQueue`.
#### [NEW] [portalBuilder.py](scripts/portalBuilder.py)
* Compiles, compresses, and generates `include/portal_assets/*.h` gzip data.
#### [NEW] [build_timestamp.py](scripts/build_timestamp.py)
* PIO hook injecting compile-time macros.
#### [NEW] [build_fonts.py](scripts/build_fonts.py) and [generate_layouts.py](scripts/generate_layouts.py)
* Standalone CLI compilers for BDF fonts and YAML layouts.

---

### Phase 7: Project Scaffolding & CI Configuration
Establish environments, linker scripts, and testing rigs.

#### [NEW] [platformio.ini](platformio.ini)
* Defines environments: `esp32dev`, `esp32s3nano`, and `native` (for GoogleTest builds).
#### [NEW] [partitions.esp32dev.csv](partitions.esp32dev.csv) and [partitions.esp32s3nano.csv](partitions.esp32s3nano.csv)
* Custom partition maps allocating double OTA sections and LittleFS space.
#### [NEW] [library.json](library.json)
* PlatformIO manifest for consumption.
#### [NEW] [test/](test/)
* Testing environment: `test_unit/` using GoogleTest on native host, and `test_hardware/` for physical verification.
#### [NEW] [template/](template/)
* Skeleton template app demonstrating standard integration.

---

## Resource Impact Assessment

### Memory Impact
* **Flash/ROM**: The total kernel binary is targeted to be `< 600KB` (excluding optional modules like display/fonts). All font tables, graphics assets, and SPA portal files will be declared `const` and stored in Flash/PROGMEM.
* **Static RAM**: Static RAM is budgeted at `< 40KB`. Module instances must be allocated statically at file scope (`static`) to prevent runtime allocation and scope-exit dangling pointer crashes in `Application` arrays.
* **Stack**: Heavy stack killers (`WiFiClientSecure`, `HTTPClient`, dynamic documents) are strictly offloaded to the heap via smart pointers:
  ```cpp
  std::unique_ptr<WiFiClientSecure> client(new (std::nothrow) WiFiClientSecure());
  ```
  FreeRTOS tasks will use a minimum stack size of 8KB.
* **Heap & Fragmentation**: `DynamicJsonDocument` is banned project-wide. All JSON parsing uses `StaticJsonDocument<N>` with pre-allocated capacities. The two-tier vector model compiles/compacts structural vectors via `shrink_to_fit()` once during boot, maintaining zero runtime allocations during the loop tick.

### Power Impact
* **Sleep/Active Profile**: The framework uses non-blocking polling and FreeRTOS event loops to allow optimal CPU sleep time. Display modules power down OLED/TFT matrices when clock/screensaver inactivity triggers.
* **Radio/Peripheral Activity**: Captive WiFi Portal times out after 10 minutes of user inactivity, transitioning to `WIFI_SHUTDOWN` and disabling the AP radio transmitter to conserve power.

### Security Impact
* **Attack Surface**: Inputs on API JSON handlers are checked against strict buffer limits (`len`/`total` in HTTP POST callback). Serial MCP server commands respect a `mutatesState` metadata field which is blocked when `mcpReadOnly` is active.
* **Data Protection**: AES-256 hardware encryption derived via MbedTLS guarantees credentials reside securely encrypted on LittleFS, protected from plaintext dumps.

---

## Verification Plan

### Automated Tests
1. **Host-Native Unit Tests**:
   * Execute `pio test -e native` to compile and run the GoogleTest suite verifying logic, EventBus subscriber execution, JSON serialization, and DataManager scheduling on the host CPU.
2. **Parity Builds**:
   * Build both targets: `pio run -e esp32dev` and `pio run -e esp32s3nano` to verify that include graph restrictions prevent leaks between optional modules and the kernel.
3. **Include Graph Audit**:
   * Run a custom script validating that no kernel component headers contain `#include` files pointing to optional modules.

### Manual Verification
1. **Portal Capture and AP Fallback**:
   * Flash a device, verify it boots to `WIFI_SETUP`, mounts the captive portal AP, accepts credential input, and successfully restarts into `RUNNING` mode.
2. **OTA update simulation**:
   * Deploy a dummy release version and test that the device downloads, checks MD5 chunk hashes, triggers the partition swap, restarts, and successfully executes the partition blessing.
