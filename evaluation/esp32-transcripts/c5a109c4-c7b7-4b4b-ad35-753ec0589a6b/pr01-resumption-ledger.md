# PR-01 Resumption — Pre-Flight Report & Proposed Ledger

## 1. Pre-Flight Check (Rule 01)

| Check | Result |
|---|---|
| **Working tree** | ✅ Clean — no modified or untracked files |
| **Current branch** | `fix/adc-gpio-conflict` at `da5ffee` |
| **Stash** | ✅ Empty |
| **Submodules** | ✅ Both clean — `framework` at `61f7f754` (`v0.0.12-dev.82`), `framework-setup` at `4e8a10a` |
| **Latest tag** | `v0.1.0-dev.5` (on `d7ab6c9`, 2 commits behind HEAD) |
| **Remote** | `origin` → `github.com/cblabs-org/echenon.git` |
| **Unpushed on `main`** | 1 commit (`5c04505`) ahead of `origin/main` (`2992155`) |

### Stack Topology (verified)

```
main (5c04505)
 └─ signal-logger/wo0-adc-spike (5e72a23)     [4 commits, ✅ WO0]
     └─ signal-logger/wo1-scaffold (dfbeaa9)    [3 commits, ✅ WO1]
         └─ signal-logger/wo2-capture-engine (fe46b38) [3 commits, ✅ WO2]
             └─ signal-logger/wo3-dashboard (4af2e74)  [3 commits, ✅ WO3]
                 └─ signal-logger/wo4-scope-gate (6ab48be, v0.1.0-dev.2) [3 commits, ⏳ WO4]
                     └─ fix/adc-gpio-conflict (da5ffee) [19 commits, ✅ WO4a]
```

All 6 branches exist locally. Linear stack, no divergence. The `fix/adc-gpio-conflict` branch is 19 commits ahead of `wo4-scope-gate` — confirmed against your stated count.

### `diag:` Commits (mid-stack)

| SHA | Message | Files | Tag |
|---|---|---|---|
| `2a12820` | `diag: add per-channel ADC sample count and CH4 range diagnostic` | `SignalLogger.cpp` (+26/−2) | — |
| `fe60d76` | `diag: add raw DMA sample dump per channel to debug CH4` | `SignalLogger.cpp` (+18) | `v0.1.0-dev.4` |

Both add diagnostic instrumentation that was later stripped by commit `a8c7dd4` (WO4a). Net code contribution: zero. The commits tell the debugging story but are pure noise for a reviewer.

> [!IMPORTANT]
> **Recommendation: Keep in history; do NOT rewrite now.** The commits are harmless, mid-stack history rewriting is risky, and `v0.1.0-dev.4` is tagged on one of them. At submission time (Step 6), I recommend a note in the PR description that these are diagnostic commits superseded by WO4a — a reviewer can skip them. If you prefer squashing at submission, that's also safe at that point (no open PRs, no pushed branches). **Your call.**

---

## 2. Owner Decisions to Record

These are APPROVED by the owner and will be recorded in PROJECT.md's session log upon your approval of this ledger:

1. **EncoderModule (PCNT) for the knob.** GPIO 8/9 come off the ADC, leaving 8 ADC channels. Encoder VCC stays on GPIO 10 as ADC monitor.
2. **Cadence tap: 4.7 kΩ + 100 nF** (not 22 kΩ). Bench-verified in B01.
3. **Encoder wire colours:** Black = Ch A, Red = Ch B, Orange = VCC (measured in B01).
4. **256 internal resistance levels and "console-as-signal-source" MITM direction are design inputs for DR-1.** The Chief Architect folds them into the DR-1 record. NOT implemented in PR-01.

---

## 3. Proposed Updated Ledger

### Completed Work Orders (no changes)

| WO | Description | Branch | Status |
|---|---|---|---|
| WO0 | ADC-continuous spike | `signal-logger/wo0-adc-spike` | ✅ Complete |
| WO1 | Project scaffold + framework bump | `signal-logger/wo1-scaffold` | ✅ Complete |
| WO2 | SignalLogger capture engine | `signal-logger/wo2-capture-engine` | ✅ Complete |
| WO3 | Dashboard, CSV export, mDNS | `signal-logger/wo3-dashboard` | ✅ Complete |
| WO4 | Bench setup + scope gate (partial) | `signal-logger/wo4-scope-gate` | ⏳ Partial |
| WO4a | EncoderModule via PCNT | `fix/adc-gpio-conflict` | ✅ Complete |

### New / Remaining Work Orders

| | WO | Branch | Base | Complexity | Register rows | Deps |
|---|---|---|---|---|---|---|
| **WO4c** | Branch housekeeping | _(rename in place)_ | — | `mechanical` | — | WO4a |
| **WO4b** | Bench gate: pin idle, scope captures, motor current, VIN | `signal-logger/wo4b-bench-gate` | `signal-logger/wo4a-encoder-integration` | `interactive` | #1, #2, #11 | WO4c |
| **WO5** | Register campaign: ride log, burst, remaining rows | `signal-logger/wo5-register-campaign` | `signal-logger/wo4b-bench-gate` | `interactive` | #3–#10, #12–#13 | WO4b |
| **WO6** | Native unit tests (decimation, event log, CSV writer, calibration) | `signal-logger/wo6-native-tests` | `signal-logger/wo5-register-campaign` | `standard` | — | WO5 (base), WO2 (code) |
| **WO7** | Doc-sync: SPEC, hardware spec, ex3 spec, absolute paths | `signal-logger/wo7-doc-sync` | `signal-logger/wo6-native-tests` | `standard` | — | WO6 |

### Detail: WO4c — Branch Housekeeping

- **What:** Rename `fix/adc-gpio-conflict` → `signal-logger/wo4a-encoder-integration` so the stack follows STACKING_REFERENCE §1.1 naming. Update the ledger in PROJECT.md.
- **`diag:` commits:** See recommendation above — keep or squash at submission. No history rewrite without your approval.
- **Risk:** None. No open PRs, no pushed branches. A `git branch -m` is safe.

### Detail: WO4b — Bench Gate (Outstanding Scope Items)

Interactive session with the bench assistant. Strictly observation — no design decisions. The four items:

| Item | What | Evidence | Register row |
|---|---|---|---|
| **B** | Pin idle check — DMM/scope on all 10 GPIOs with bike power OFF | `A02-*` idle readings | #2 (baseline) |
| **C** | Motor-drive scope captures — both wires, both directions, at rest, 10 ms/div + µs/div (hardware spec §5.2, 6 captures minimum) | `A01-*` scope captures | #1 |
| **D** | Motor current — inrush + steady-state under load (shunt or clamp) | `A02-*` current readings | #2 |
| **E** | Waveshare VIN 9V tolerance — datasheet review or bench test | `A11-*` finding | #11 |

**Boundary:** If the session starts making design decisions (e.g., "we should change the divider values"), park them in the completion report as proposals for the PA.

### Detail: WO5 — Register Campaign

Interactive session covering all remaining register rows:

| Row | Subject | Evidence needed |
|---|---|---|
| #3 | Motor stops within one level | Ride-log CSV: pot mV vs time for a full cam revolution |
| #4 | Wiper 1160–1700 mV, linear | Wiper, Yellow, Green at every console level, both directions |
| #5 | Pot shape over one revolution | Drive a full revolution, mark click-offs and extremes |
| #6 | Green = GND, Yellow = 3.3 V | Measure GPIO 2/3 taps + source impedance |
| #7 | Cadence: Orange pulled up, reed to GND | Orange/Grey/Brown voltages open and closed |
| #8 | Encoder: counts/detent, pull direction, Ch B shape | PCNT data (from B04) + VCC stability |
| #9 | Console tolerates motor disconnect | Unplug motor, turn knob, watch console |
| #10 | Console 3.3 V rail tolerates parallel supplies | Console rail voltage at idle/active/sleep |
| #12 | Echelon app pairs any `0x180D` peripheral | nRF Connect emulation spike |
| #13 | Power-on behaviour | Scope single-shot on motor + wiper, apply 9 V |

> [!NOTE]
> Rows #9, #10 and #12 may not be achievable with the current hardware setup. If they require equipment or BLE work not available during the session, they'll be explicitly marked deferred to DR-1 with your approval.

### Detail: WO6 — Native Tests

GoogleTest on `env:native` via MCP `run_tests`:

| Test suite | Covers |
|---|---|
| `test_decimation` | `DecimationBuffer` min/mean/max, edge cases |
| `test_event_log` | `EventLog` ring buffer, wrap-around, timestamps |
| `test_csv_writer` | CSV formatting, headers, escaping |
| `test_calibration` | ADC→voltage conversion, ÷4 divider maths |

### Detail: WO7 — Doc-Sync

| Task | Files |
|---|---|
| Wire colours, cadence tap values, ADC/PCNT split, EncoderModule | SPEC §4.4.1, §5.1, §5.4; hardware spec §5; `docs/ex3_specification.md` |
| Update test list if tests changed | SPEC §13.2 |
| Remove all `/Users/…` and `file:///` absolute paths | `docs/encoder-resistance-analysis.md`, `docs/mitm-encoder-design-notes.md`, others |
| **Do NOT edit** | `PROGRAM.md`, `PROGRAM-PLAN.md`, `reviews/*`, `reviews/DR-1-design-freeze.md` |

> [!WARNING]
> Hardware spec has internal inconsistencies already present: §5.3 schematic still shows old encoder wire colours (Brown/Orange/Red) while §5.6 has the corrected colours (Black/Red/Orange). §5.7 build checklist still says "RS1–RS8 (22 kΩ)" despite the 4.7 kΩ change. These are all WO7 scope.

### File Overlaps

All WOs are linearly stacked. No parallel conflicts.

| Path | Touched by | Conflict? |
|---|---|---|
| `docs/measurements/phase0/` (new evidence files) | WO4b, WO5 | No — new files only, different rows |
| `docs/measurements/phase0/README.md` (register) | WO4b, WO5 | Serial — WO4b updates rows #1,#2,#11; WO5 updates #3–#10,#12–#13 |
| `SPEC.md` | WO7 only | No |
| `docs/hardware_specification.md` | WO7 only | No |
| `test/native/` | WO6 only | No |
| `src/echenon/` | WO6 reads only | No |

### Tooling Constraints (all WOs)

> [!CAUTION]
> **Rule 03:** Flashing follows Tag → Build → Flash using `v0.1.0-dev.N` tags. Dev tags are autonomous (no user approval needed).
>
> **Rule 04:** Use PlatformIO MCP tools (`build_project`, `upload_firmware`, `run_tests`), never the `pio` CLI directly. Both rules enforced by hooks in `.agents/hooks.json`.

---

## 4. Decisions Needed From You

1. **Approve this ledger** (WO4c → WO4b → WO5 → WO6 → WO7) so I can record approval in PROJECT.md and begin execution.
2. **`diag:` commits:** Keep in history (my recommendation) or squash at submission?
3. **Rows #9, #10, #12:** If these prove infeasible during the interactive sessions, are you OK with deferring them to DR-1?
