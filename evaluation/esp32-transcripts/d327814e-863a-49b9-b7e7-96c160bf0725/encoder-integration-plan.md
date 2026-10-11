# PR-01 / Encoder Integration — PA Decomposition Plan

> **Project:** PR-01 Signal Logger (Phase 0)
> **Current branch:** `fix/adc-gpio-conflict` (off `signal-logger/wo4-scope-gate`)
> **Current tag:** `v0.1.0-dev.4` (13 commits ahead of wo4-scope-gate base)
> **Decomposition date:** 2026-10-04

---

## Situation

We have completed the encoder and resistance analysis (B02, B03 reports).
The ADC-based encoder reading has a 39% bounce rate, 10.7% skip rate, and
28% console miss rate. The next step is to wire up the framework's
`EncoderModule` (PCNT peripheral) on GPIO 8/9 and compare it against the
ADC baseline.

### Pre-flight State
- **Branch:** `fix/adc-gpio-conflict` — 13 commits ahead of `signal-logger/wo4-scope-gate`
- **Working tree:** Clean
- **Diagnostic instrumentation:** Two `diag:` commits with per-channel sample counters
  and CH4 range logging in `SignalLogger.cpp` — must be stripped before merge
- **GPIO 8/9:** Currently in ADC continuous scan (channels 7 and 8). Must be released
  for PCNT.

---

## Work Order Decomposition

### WO4a — EncoderModule Code Integration

| Field | Value |
|---|---|
| **Complexity** | `standard` (autonomous worker) |
| **Branch** | `fix/adc-gpio-conflict` (continue on current branch) |
| **Estimated size** | 4 files, ~30 lines changed |

**Changes:**

- [ ] **`platformio.ini`** — Add `-DWITH_ENCODER` to `build_flags` for `ex3-waveshare-nano`
- [ ] **`board.h`** — Add `BOARD_ENCODERS[]` array gated by `WITH_ENCODER`:
  ```cpp
  #ifdef WITH_ENCODER
  #include "EncoderConfig.h"
  constexpr BoardEncoder BOARD_ENCODERS[] = {
      {.id = "knob", .pinA = 8, .pinB = 9,
       .stepsPerDetent = 4, .reversed = false,
       .accel = {.enabled = false}}
  };
  constexpr size_t BOARD_ENCODER_COUNT = sizeof(BOARD_ENCODERS) / sizeof(BOARD_ENCODERS[0]);
  #endif
  ```
- [ ] **`SignalLogger.cpp`** — Reduce ADC pattern from 10 → 8 channels (remove CH7/CH8,
  keep CH9 for encoder VCC monitoring). Update `pattern_num`, buffer sizing, and
  `_currentRow` initialisation loop. Strip the two `diag:` instrumentation additions
  (per-channel sample counter and CH4 range logging).
- [ ] **`main.cpp`** — Register `EncoderModule`:
  ```cpp
  #ifdef WITH_ENCODER
  #include "EncoderModule.hpp"
  EncoderModule encoderModule;
  #endif
  // In setup():
  #ifdef WITH_ENCODER
  for (size_t i = 0; i < BOARD_ENCODER_COUNT; i++) {
      encoderModule.addEncoder(BOARD_ENCODERS[i]);
  }
  encoderModule.begin(app);
  app.registerModule(&encoderModule);
  #endif
  ```
- [ ] **Build verification** — Must compile with 0 errors via `build_project` MCP tool

> [!IMPORTANT]
> **`stepsPerDetent`** — set to `4` initially (standard full-cycle quadrature).
> The bench test (WO4b) will verify whether this needs to be `2` (half-cycle)
> based on how PCNT counts compare to physical detent clicks.

---

### WO4b — Bench Test: PCNT vs ADC Baseline Comparison

| Field | Value |
|---|---|
| **Complexity** | `interactive` (human at bench) |
| **Branch** | `fix/adc-gpio-conflict` (continue) |
| **Evidence** | `docs/measurements/phase0/B04-*` |

**Protocol:**

1. Tag → Build → Flash the WO4a firmware
2. Verify `encoder_status` MCP tool responds (or `/api/mcp` endpoint)
3. Run the same three-phase encoder test as B02:
   - Phase 1: Small adjustments (1–2 clicks)
   - Phase 2: Medium turns
   - Phase 3: Fast multi-click spins
4. Compare PCNT position vs ADC-derived position
5. Determine correct `stepsPerDetent` (2 or 4)
6. Commit B04 comparison report

**Key metrics to compare:**
- Bounce rate (should be 0% with PCNT glitch filter)
- Skip rate (should be 0%)
- Click-to-position accuracy
- Position tracking during fast rotation

---

### WO4c — Branch Cleanup and Merge

| Field | Value |
|---|---|
| **Complexity** | `mechanical` (autonomous) |
| **Branch** | Merge `fix/adc-gpio-conflict` → `signal-logger/wo4-scope-gate` |

**Steps:**

1. Verify all diagnostics stripped (done in WO4a)
2. Merge `fix/adc-gpio-conflict` into `signal-logger/wo4-scope-gate`
3. Delete `fix/adc-gpio-conflict` branch
4. Verify build on merged branch

---

## Execution Order

```
WO4a (autonomous) → WO4b (interactive/bench) → WO4c (autonomous)
     code changes        bench validation         merge cleanup
```

All three operate on the same branch — no parallel conflicts.

## Risk

| Risk | Mitigation |
|---|---|
| PCNT and ADC sharing GPIO 8/9 | GPIOs released from ADC pattern before PCNT init |
| `stepsPerDetent` wrong | WO4b explicitly tests and corrects |
| Encoder VCC (CH9) lost from ADC | Kept — only CH7/CH8 removed |
| Merge conflicts with wo4-scope-gate | `fix/adc-gpio-conflict` is a child of wo4 — fast-forward merge |

---

> [!NOTE]
> **Approval requested.** Shall I proceed with this decomposition?
> WO4a will be spawned as an autonomous worker (`standard` / `flash` model).
> WO4b will be a bridge prompt for your bench session.
> WO4c will be spawned after WO4b completes.
