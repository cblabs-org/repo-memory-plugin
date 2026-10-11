# Evaluation Template Manifest (eval-v2)

## Metadata

- **Creation Date:** 2026-10-11
- **Source Repository:** `~/Documents/Projects/esp32-app-foundation`
- **Source HEAD Commit:** `45d4b5d17e6f6da4ce96d1cd52bd4fa4d92568aa`
- **Source Branch:** `chore/migrate-bug-fixer-to-daemon-plugin`

## Exclusions Applied

The template was cloned from the working tree of `esp32-app-foundation` excluding:

- `private_key.pem` (deploy-only secret, not required for builds or native tests)
- `.pio/build/` (build directory containing machine/path-specific CMakeCache entries)
- `.agents/plugins.json` (disables global BugFixer daemon plugin)
- `.memory/.enrolled`
- `.memory/`
- `.vscode/`
- `.agents/tmp/` (scratch / python venvs)
- `release/`
- `releases/`
- `.DS_Store`

*Note:* `.pio/libdeps` and packages were preserved to ensure reproducible, isolated builds without remote dependency fetch during evaluation runs.

## Critical Key Hashes

- `platformio.ini` SHA-256: `d4e548eb511ebcb2a1769d96028da61acc3b7431f6038a884fbcf0455c1e8855`

## Condition Templates

| Template | Condition | PA Workspace | repo-memory Hook / Enrolment | Commits on `main` |
|---|---|---|---|---|
| `template/` | Master Template | Present | None | 1 (`baseline`) |
| `tpl-a/` | Condition (a): Nothing | Removed (`disabled-skills.txt`) | Disabled | 2 (`baseline`, `condition-a: no memory`) |
| `tpl-b/` | Condition (b): PA only | Present (as-is) | Disabled | 2 (`baseline`, `condition-b: PA only`) |
| `tpl-c/` | Condition (c): repo-memory push | Removed (`disabled-skills.txt`) | Enabled (run-time enrolled) | 2 (`baseline`, `condition-c: repo-memory push`) |
| `tpl-d/` | Condition (d): Both | Present (as-is) | Enabled (run-time enrolled) | 2 (`baseline`, `condition-d: both`) |

## Native Test Results (`pio test -e native`)

Ran across all 36 test suites in `evaluation/eval-v2/template`:

```
=================================== SUMMARY ===================================
Environment    Test                                 Status    Duration
-------------  -----------------------------------  --------  ------------
native         test_unit/test_hardware_board        PASSED    00:00:01.893
native         test_unit/test_wifiManager           PASSED    00:00:02.305
native         test_unit/test_webServer             PASSED    00:00:02.144
native         test_unit/test_otaUpdateManager      PASSED    00:00:02.858
native         test_unit/test_deviceCrypto          PASSED    00:00:01.581
native         test_unit/test_bleManager            PASSED    00:00:02.781
native         test_unit/test_ota_fail_closed       PASSED    00:00:01.509
native         test_unit/test_ota_download_engine   PASSED    00:00:01.530
native         test_unit/test_structural_scrubber   PASSED    00:00:01.510
native         test_unit/test_displayServiceHelper  PASSED    00:00:02.409
native         test_unit/test_audit_module          PASSED    00:00:02.070
native         test_unit/test_authManager           PASSED    00:00:01.989
native         test_unit/test_ota_channel           PASSED    00:00:01.543
native         test_unit/test_buttonModule          PASSED    00:00:02.585
native         test_unit/test_ota_manifest          PASSED    00:00:01.560
native         test_unit/test_lvglDisplay           PASSED    00:00:03.020
native         test_unit/test_dataManager           PASSED    00:00:02.057
native         test_unit/test_mcpServer             PASSED    00:00:03.551
native         test_unit/test_application           PASSED    00:00:02.592
native         test_unit/test_periodic              PASSED    00:00:01.530
native         test_unit/test_encoderModule         PASSED    00:00:02.230
native         test_unit/test_ota_verifier          PASSED    00:00:01.501
native         test_unit/test_eventbus              PASSED    00:00:01.765
native         test_unit/test_configManager         PASSED    00:00:02.297
native         test_unit/test_logpipeline           PASSED    00:00:01.579
native         test_unit/test_redactor              PASSED    00:00:01.660
native         test_unit/test_timeManager           PASSED    00:00:02.023
native         test_unit/test_lvgl_stubs            PASSED    00:00:02.362
native         test_unit/test_pngWriter             PASSED    00:00:01.652
native         test_unit/test_u8g2Display           PASSED    00:00:03.264
native         test_unit/test_precondition_engine   PASSED    00:00:01.723
native         test_unit/test_semver                PASSED    00:00:01.573
native         test_unit/test_keyManager            PASSED    00:00:02.150
native         test_unit/test_md5Utils              PASSED    00:00:01.681
native         test_unit/test_deepStackExecutor     PASSED    00:00:01.524
native         test_unit/test_config_manifest       PASSED    00:00:01.495
================ 989 test cases: 989 succeeded in 00:01:19.067 ================
```

**Result:** PASSED (989/989 test cases passed, 0 failures).
