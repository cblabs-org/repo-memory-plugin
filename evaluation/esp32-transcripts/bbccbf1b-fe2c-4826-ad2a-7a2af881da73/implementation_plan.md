# Phase 7 Implementation Plan — Build Infrastructure, Partitions, Host Test Suite, and Boilerplate Starter Template

## Audit Checklist
- [x] Reviewed by `house-style-documentation` - passed
- [x] Reviewed by `architectural-refactoring` - passed
- [x] Reviewed by `embedded-systems` - passed (Resource Impact Assessment included)

> [!IMPORTANT]
> **Host Testing Isolation & Compilation Portability**:
> Under the native build environment, standard hardware-dependent headers (such as `<WiFi.h>`, `<LittleFS.h>`, `<U8g2lib.h>`) do not exist. We resolve this by providing comprehensive mock headers in `test/mocks/` and utilising PlatformIO's `lib_ignore` to bypass compiling the actual hardware implementations of `wifiManager`, `webServer`, and `deviceCrypto` during native tests. All POSIX disk I/O calls inside the mock filesystem compile safely to host files inside `.agents/tmp/native_fs`.

---

## Goal Description
1. Configure **PlatformIO Configurations** (`platformio.ini`):
   - Setup environment `esp32dev` targeting standard ESP32 boards with custom partitions and pre-build Python build-timestamp and portal-assets hooks.
   - Setup environment `esp32s3nano` targeting Nano ESP32 boards with S3 custom partitions, CDC native USB serial logging overrides, and auto-reset touches.
   - Setup environment `native` targeting host compilation with `ArduinoFake` dependencies, custom test inclusion trees, and strict hardware library ignores.
2. Define **Partition Layout Tables**:
   - `partitions.esp32dev.csv`: Layout mapping 4MB Flash containing NVS, otadata, app0/app1 (1.9MB each), littlefs filesystem (896KB), and core dumps.
   - `partitions.esp32s3nano.csv`: Layout mapping 16MB Flash containing NVS, otadata, app0/app1 (4.5MB each), spiffs filesystem (6.5MB), and core dumps.
3. Establish **Host Mock Infrastructure** (`test/mocks/`):
   - Mocks for FreeRTOS tasks, queues, semaphores, and delay stubs.
   - Mock `LittleFS` that maps Arduino `FS` file calls directly to POSIX filesystem files under `.agents/tmp/native_fs`.
   - Mocks for `<WiFi.h>`, `<WiFiClientSecure.h>`, `<HTTPClient.h>`, `<U8g2lib.h>`, and ESP32 SDK functions (`esp_restart`, `esp_get_free_heap_size`, `esp_ota_ops`).
4. Implement **Host Test Suite** (`test/test_native/`):
   - `test_main.cpp`: Main setup using Unity to execute all native test modules.
   - `eventBusTests.cpp`: Assert subscription limits, dispatch safety, and parameter matching.
   - `configManagerTests.cpp`: Assert loading, defaults fallback, and schema migrations.
   - `keyManagerTests.cpp`: Assert encryption/decryption keys persistence.
   - `timeManagerTests.cpp`: Assert NTP mock statuses, scheduling ticks, and timezone formats.
   - `displayCarouselTests.cpp`: Assert display carousel rotation timings, wake/sleep screen contrast toggling, and override screens.
5. Create **Skeleton Application Template** (`template/`):
   - `template/platformio.ini`: Boilerplate configuration using framework core relative directories.
   - `template/src/main.cpp`: Application bootstrap setup and loop functions consuming `AppFoundation.h`.
   - `template/src/myApp.hpp` / `template/src/myApp.cpp`: Custom boilerplate class showing registration of display boards, custom web portal config tabs, and serial MCP tools.

---

## User Review Required

> [!WARNING]
> **Partition Table Flash Restrictions**:
> The `esp32dev` configuration allocates exactly 1.9MB (`0x1E0000`) for the application binary partition. If extensive application code is added alongside massive font arrays, the compiled binary size must be monitored to avoid partition overflow.

---

## Proposed Changes

### Build Configurations & Partitions
#### [NEW] [platformio.ini](platformio.ini)
- Contains env definitions for `esp32dev`, `esp32s3nano`, and `native` host testing.
- Uses `lib_extra_dirs` to link framework components dynamically.
- Configures pre-compilation script hooks for `portalBuilder.py`.

#### [NEW] [partitions.esp32dev.csv](partitions.esp32dev.csv)
- Partition table mapping 4MB memory layout.

#### [NEW] [partitions.esp32s3nano.csv](partitions.esp32s3nano.csv)
- Partition table mapping 16MB S3 memory layout.

### Host Mocks & Test Suite
#### [NEW] [test/mocks/freertos/FreeRTOS.h](test/mocks/freertos/FreeRTOS.h)
- Header stubbing FreeRTOS basic definitions and task structures.

#### [NEW] [test/mocks/freertos/queue.h](test/mocks/freertos/queue.h) / [semphr.h](test/mocks/freertos/semphr.h) / [task.h](test/mocks/freertos/task.h)
- Stub headers for scheduler functions.

#### [NEW] [test/mocks/FS.h](test/mocks/FS.h) / [LittleFS.h](test/mocks/LittleFS.h)
- Mocks representing Arduino FS and LittleFS storage interfaces.

#### [NEW] [test/mocks/MockLittleFS.hpp](test/mocks/MockLittleFS.hpp) / [MockLittleFS.cpp](test/mocks/MockLittleFS.cpp)
- Implements direct POSIX disk mapping under `.agents/tmp/native_fs` for host filesystem operations.

#### [NEW] [test/mocks/WiFi.h](test/mocks/WiFi.h) / [WiFiClient.h](test/mocks/WiFiClient.h) / [WiFiClientSecure.h](test/mocks/WiFiClientSecure.h)
- Mocks simulating WiFi status codes, connections, and secure client endpoints.

#### [NEW] [test/mocks/Preferences.h](test/mocks/Preferences.h)
- Mock for Arduino Preferences key-value persistence.

#### [NEW] [test/mocks/U8g2lib.h](test/mocks/U8g2lib.h)
- Mocks U8G2 rendering methods as no-ops.

#### [NEW] [test/mocks/Stubs.cpp](test/mocks/Stubs.cpp)
- Implements hardware-specific system restarts, MbedTLS encryption/decryption stubs, and SDK handlers.

#### [NEW] [test/test_native/test_main.cpp](test/test_native/test_main.cpp)
- Unity test suite coordinator executing native verification scenarios.

#### [NEW] [test/test_native/eventBusTests.cpp](test/test_native/eventBusTests.cpp)
- Logic assertions for EventBus messaging.

#### [NEW] [test/test_native/configManagerTests.cpp](test/test_native/configManagerTests.cpp)
- Schema and file migration test assertions.

#### [NEW] [test/test_native/keyManagerTests.cpp](test/test_native/keyManagerTests.cpp)
- Key cryptographic read/write assertions.

#### [NEW] [test/test_native/timeManagerTests.cpp](test/test_native/timeManagerTests.cpp)
- Clock ticks, scheduling callbacks, and timezone calculations assertions.

#### [NEW] [test/test_native/displayCarouselTests.cpp](test/test_native/displayCarouselTests.cpp)
- Carousel transitions, timeout expiries, and overriding diagnostic overlays assertions.

### Skeleton Boilerplate Template
#### [NEW] [template/platformio.ini](template/platformio.ini)
- Starter configuration linking app components to parent framework.

#### [NEW] [template/src/main.cpp](template/src/main.cpp)
- App setup and loop hooks pointing to custom Application wrapper.

#### [NEW] [template/src/myApp.hpp](template/src/myApp.hpp) / [myApp.cpp](template/src/myApp.cpp)
- Custom application logic displaying diagnostic boards, custom tabs, and registering serial commands.

---

## Resource Impact Assessment
*   **Flash (ROM)**: High-speed compilation hooks ensure custom web components are compiled directly inside Flash binary payload. Custom partition sizes prevent overlap errors on hardware boots.
*   **RAM/Heap**:
    *   No persistent memory is consumed under native test execution, and disk stubs write files to temporary files.
    *   `StaticJsonDocument` avoids dynamic allocation on configuration saves.
*   **Power**: Target partition definitions include standard sleep and Captive portal recovery timeouts to optimize device battery consumption.

---

## Verification Plan

### 3-Phase Testing Pipeline Compliance

#### Phase 1: Local Development & Native Tests
*   Compile and run host unit tests:
    ```bash
    pio test -e native
    ```
*   Verify that 100% of EventBus, ConfigManager, KeyManager, TimeManager, and DisplayCarousel test cases pass successfully.

#### Phase 2: Target Compilation & Integrated Build
*   Run PlatformIO compilation for the two targeted devices:
    ```bash
    pio run -e esp32dev
    pio run -e esp32s3nano
    ```
*   Assert that SCons pre-build hooks execute the `portalBuilder.py` asset compiler successfully and that the firmware compiles to completion under both hardware targets.

#### Phase 3: Skeleton Application Template Compilation
*   Navigate into `template/` and run the compilation:
    ```bash
    cd template && pio run
    ```
*   Verify that the skeleton boilerplate references framework core files correctly and compiles without link/include errors.
