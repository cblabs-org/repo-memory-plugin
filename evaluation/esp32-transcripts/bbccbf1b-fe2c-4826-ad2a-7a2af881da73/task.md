# Tasks: Phase 7 — Build Infrastructure, Partitions, Host Test Suite, and Boilerplate Starter Template

- [x] Propose plan to user and get approval
- [x] Configure root `platformio.ini` with target and native environments
- [x] Create custom partition tables `partitions.esp32dev.csv` and `partitions.esp32s3nano.csv`
- [x] Create mock headers in `test/mocks/freertos/` (FreeRTOS.h, task.h, queue.h, semphr.h)
- [x] Create mock filesystem stubs (`test/mocks/FS.h`, `test/mocks/LittleFS.h`, `test/mocks/MockLittleFS.hpp`, `test/mocks/MockLittleFS.cpp`)
- [x] Create mock hardware stubs (`test/mocks/WiFi.h`, `test/mocks/WiFiClient.h`, `test/mocks/WiFiClientSecure.h`, `test/mocks/HTTPClient.h`, `test/mocks/Preferences.h`, `test/mocks/U8g2lib.h`, `test/mocks/Stubs.cpp`)
- [x] Create native unit tests in `test/test_native/` (`test_main.cpp`, `eventBusTests.cpp`, `configManagerTests.cpp`, `keyManagerTests.cpp`, `timeManagerTests.cpp`, `displayCarouselTests.cpp`)
- [x] Create skeleton application template in `template/` (`platformio.ini`, `src/main.cpp`, `src/myApp.hpp`, `src/myApp.cpp`)
- [x] Verify Phase 1: Compile and run native host unit tests successfully (`pio test -e native`)
- [ ] Verify Phase 2: Compile firmware targets successfully (`pio run -e esp32dev`, `pio run -e esp32s3nano`) [BLOCKED: python littlefs package missing in PlatformIO environment]
- [ ] Verify Phase 3: Compile boilerplate starter template successfully [BLOCKED: python littlefs package missing in PlatformIO environment]
- [x] Finalize code style review and checklist completion
