# PR-01 / WO4 — Session Wrap-Up Report

> **Session span:** 2026-09-26T19:12 → 2026-10-04T16:06 (multiple bench sessions)  
> **Branch:** `fix/adc-gpio-conflict` (off `signal-logger/wo4-scope-gate`)  
> **Commits:** 18 ahead of `signal-logger/wo4-scope-gate`  
> **Tags:** `v0.1.0-dev.1` → `v0.1.0-dev.5`  
> **Role:** Interactive bench assistant (WO4 complexity: interactive)

---

## 1. Original Objective

The session was initiated with the WO4 bridge prompt:

> *"signal-logger / WO4 — Bench Setup + Scope Characterisation Gate.
> Walk me through the checklist step by step. I'll tell you what I see on
> the scope and DMM; you'll help interpret results and tell me what to capture."*

**WO4 checklist:**
- A. Flash and boot ✅
- B. Pin idle check ⬜ (not performed)
- C. Motor-drive scope captures ⬜ (not performed — redirected to encoder work)
- D. Motor current measurement ⬜ (not performed)
- E. VIN tolerance check ⬜ (not performed)
- F. Motor-sense taps ✅ (connected, verified reading on dashboard)
- G. Commit evidence ✅ (B01–B04 committed)

The session **diverged significantly** from the original WO4 scope. The user
directed work toward encoder characterisation, resistance calibration, and
MITM design analysis after discovering the encoder signal quality issues
during sensor bring-up.

---

## 2. What Was Accomplished

### 2.1 Sensor Bring-Up (B01)

All 10 ADC channels confirmed working on the Waveshare ESP32-S3-Nano:

| Signal Group | Channels | Status |
|---|---|---|
| Resistance pot (wiper, hi, lo) | CH0, CH1, CH2 | ✅ Working |
| Cadence (return, reed) | CH3, CH4 | ✅ Working (4.7kΩ + 100nF) |
| Motor sense (+, −) | CH5, CH6 | ✅ Working (÷4 divider) |
| Encoder (A, B, VCC) | CH7, CH8, CH9 | ✅ Working → released for PCNT |

**Issues resolved:**
- `gpio_config()` was breaking ADC analog path on GPIO 5/8 → fixed
- Internal pull-ups on tap pins violated passive premise → disabled
- Cadence signal needed 4.7kΩ series + 100nF filter (not 22kΩ)
- Encoder wire colour mapping corrected (Black=A, Red=B, Orange=VCC)

### 2.2 Encoder Signal Quality Analysis (B02)

60-second ADC baseline capture of the encoder quadrature signal:

| Metric | Value | Assessment |
|---|---|---|
| Bounce rate | 39% | Poor — mechanical encoder, no debounce |
| Quadrature skip rate | 10.7% | Direction errors in fast rotation |
| Console click miss rate | 28% | "Takes several clicks to change" |
| Single-click response | 58% | Nearly half of single clicks ignored |

### 2.3 Resistance Calibration (B03)

Full 32-level ascending/descending sweep with 4-second dwell per level:

| Finding | Value |
|---|---|
| Pot linearity | R² = 0.9989 (excellent) |
| Formula | `wiper_adc = 39.1 × level + 97.0` |
| Step size | 39 ADC counts / level (31.5 mV) |
| Hysteresis | < 15 ADC counts |
| Motor overshoot (mid/high) | 98% of moves |
| Motor overshoot (low) | 8% of moves |

### 2.4 Eccentric Cam Model (B03 §6)

Identified the resistance mechanism as an eccentric cam (e ≈ 8 mm):

```
displacement(θ) = e × (1 − cos(θ))
θ = (level − 1) × 4.87°,  sweep 0° → 151°
```

- Displacement step at level 19 is **22× larger** than at level 2
- Rider feels barely any change at levels 1–5, big jumps at 12–20
- Pot measures shaft **angle** (linear), not displacement (sinusoidal)

### 2.5 MITM Design Thinking (Design Notes)

Captured future architecture options in [`docs/mitm-encoder-design-notes.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/mitm-encoder-design-notes.md):

- 256 internal resistance levels (`uint8_t`)
- Architecture spectrum: Monitor → Encoder MITM → Full MITM → Console Bypass
- Console-as-signal-source model (ESP32 owns motor via PID, reads console hints)
- 10 console fault tolerance tests identified
- Echelon app must keep working (calories, power, resistance display)

### 2.6 EncoderModule Integration (WO4a + WO4b)

PCNT peripheral wired up on GPIO 8/9, validated against ADC baseline:

| Metric | ADC (B02) | PCNT (B04) |
|---|---|---|
| Bounce rate | 39% | **0%** |
| Skip rate | 10.7% | **0%** |
| Single-click detection | 58% | **100%** |
| Direction errors | Present | **None** |

`stepsPerDetent=4` confirmed correct. 5-minute soak test passed (stable heap, no crashes).

---

## 3. Evidence Committed

| File | Description |
|---|---|
| [`B01-20261003-sensor-bringup-report.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B01-20261003-sensor-bringup-report.md) | All 10 channels confirmed |
| [`B02-20261004-encoder-characterisation.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B02-20261004-encoder-characterisation.md) | Encoder signal quality analysis |
| [`B02-20261004-encoder-baseline-raw.csv`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B02-20261004-encoder-baseline-raw.csv) | 60s raw encoder data |
| [`B02-20261004-encoder-baseline-full.csv`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B02-20261004-encoder-baseline-full.csv) | 60s all 10 channels |
| [`B03-20261004-resistance-calibration.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B03-20261004-resistance-calibration.md) | 32-level calibration + cam model |
| [`B03-20261004-resistance-calibration.csv`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B03-20261004-resistance-calibration.csv) | 7,258-row calibration sweep |
| [`B04-20261004-pcnt-encoder-comparison.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/measurements/phase0/B04-20261004-pcnt-encoder-comparison.md) | PCNT vs ADC comparison |
| [`encoder-resistance-analysis.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/encoder-resistance-analysis.md) | Consolidated analysis document |
| [`mitm-encoder-design-notes.md`](file:///Users/mcneillm/Documents/Projects/Echenon/docs/mitm-encoder-design-notes.md) | MITM architecture + design thinking |

---

## 4. Outstanding Items

### From original WO4 scope (not completed):
- [ ] **B. Pin idle check** — tap pin bias verification with DMM
- [ ] **C. Motor-drive scope captures** — 6 captures per hardware spec §5.2
- [ ] **D. Motor current measurement** — inrush + steady-state
- [ ] **E. VIN tolerance check** — Waveshare 9V rail safety

### Branch housekeeping:
- [ ] **WO4c** — Merge `fix/adc-gpio-conflict` (18 commits) → `signal-logger/wo4-scope-gate`
- [ ] **Diagnostic commits** — `diag:` instrumentation stripped in WO4a, but the original `diag:` commits remain in history (commits `2a12820`, `fe60d76`)

---

## 5. Key Decisions Made

| Decision | Rationale |
|---|---|
| Cadence resistors changed from 22kΩ to 4.7kΩ + 100nF | Reed switch signal too weak through 22kΩ |
| Encoder wire colours corrected | Physical tracing confirmed Black=A, Red=B, Orange=VCC |
| GPIO 8/9 released from ADC for PCNT | PCNT requires exclusive GPIO ownership |
| ADC pattern reduced from 10→8 channels | CH7/CH8 freed; CH9 (VCC) retained for monitoring |
| `stepsPerDetent=4` confirmed | Single-click tests matched physical detent count |
| 256 internal resistance levels | `uint8_t`, maps cleanly to Echelon (÷8) and Peloton (×100/256) |

---

## 6. Recommendations for Chief Architect

1. **WO4 scope items (B, C, D, E)** should be a separate interactive WO —
   the user redirected to higher-priority encoder analysis work. These are
   still needed for the Phase 0 register rows #1, #2, #11.

2. **WO4c merge** can proceed immediately — it's a fast-forward merge.

3. **MITM design notes** contain architectural decisions that feed DR-1.
   The console fault tolerance tests (§12) should be planned before
   committing to an architecture.

4. **EncoderModule is validated** — PCNT completely eliminates the encoder
   signal quality problem. This is the confirmed solution for the MITM
   encoder ingest path.

5. **The cam model** reveals the rider experience is inherently non-linear.
   Linearisation should be considered for smart trainer integration
   (Peloton/Zwift/FTMS), but is a Phase 5+ concern.
