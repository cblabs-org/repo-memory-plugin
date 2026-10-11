# Task List — ESP32 App Foundation Framework Implementation

This task list tracks the implementation progress of the new ESP32 App Foundation framework, as approved in the implementation plan.

- `[x]` **Phase 1: Foundation Kernel Interfaces & Base Constants**
  - `[x]` Define abstract base interfaces in `include/interfaces/`
  - `[x]` Create `include/AppState.hpp`, `include/FrameworkConfig.hpp`, and single-include façade `include/AppFoundation.h`
- `[x]` **Phase 2: Core Utility Libraries**
  - `[x]` Implement ESP-IDF/MbedTLS cryptography wrapper in `lib/deviceCrypto/`
  - `[x]` Implement ESP-IDF logging wrapper in `lib/logger/` with `LOG_SECURE()`
  - `[x]` Implement streaming MD5 validator in `lib/md5Utils/`
  - `[x]` Implement HTTPS release search and download client in `lib/githubClient/`
- `[x]` **Phase 3: Kernel Core**
  - `[x]` Implement type-safe synchronous publisher `modules/eventBus/`
  - `[x]` Implement composition root, boot state machine, and vector compaction in `modules/application/`
- `[x]` **Phase 4: Core Services**
  - `[x]` Implement configuration management `/etc/config.json` in `modules/configManager/`
  - `[x]` Implement encrypted API key store `/etc/apikeys.json` in `modules/keyManager/`
  - `[x]` Implement non-blocking WiFi manager with AP fallback in `modules/wifiManager/`
  - `[x]` Implement NTP synchronization and POSIX timezone configuration in `modules/timeManager/`
  - `[x]` Implement priority-tiered background data task in `modules/dataManager/`
  - `[x]` Implement secure OTA updates with app verification rollback in `modules/otaUpdateManager/`
  - `[x]` Implement ESPAsyncWebServer and contributors route mapping in `modules/webServer/`
  - `[x]` Implement serial and HTTP JSON-RPC tools server in `modules/mcpServer/`
- `[x]` **Phase 5: Display Module & Optional Modules**
  - `[x]` Implement monochrome graphic engine, fonts, and system boards in `modules/u8g2Display/`
  - `[x]` Build 3-tier widget toolkit (Primitives, Generics, Module-Aware) in `modules/u8g2Display/widgets/`
  - `[x]` Implement time-based callback scheduler in `modules/scheduler/`
  - `[x]` Implement board LED and physical button helper in `lib/boardLED/` / `lib/buttonHandler/`
- `[x]` **Phase 6: Web Portal Frontend & Build Scripts**
  - `[x]` Write HTML/CSS/JS base Single Page Application shell with `ApiQueue` in `web/portal/`
  - `[x]` Build GZIP asset compiler script `scripts/portalBuilder.py`
  - `[x]` Build PIO hooks and standalone compilers in `scripts/`
- `[x]` **Phase 7: Project Scaffolding & CI Configuration**
  - `[x]` Write `platformio.ini` defining `esp32dev`, `esp32s3nano`, and `native` environments
  - `[x]` Create custom CSV partition layout files
  - `[x]` Add GoogleTest host tests in `test/test_unit/` and stub mocks in `test/mocks/`
  - `[x]` Create skeleton application skeleton under `template/`
