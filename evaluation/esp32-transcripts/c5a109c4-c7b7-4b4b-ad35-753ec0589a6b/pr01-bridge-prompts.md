# PR-01 Execution Sequence (Updated with CA Review Addendum)

## Revised Stack Order

```
signal-logger/wo4a-encoder-integration  ← HEAD (bccd895 + b1d0e07)
  └─ signal-logger/wo4b-bench-gate       [interactive — scope + DMM]
       └─ signal-logger/wo4d-logger-fixes  [autonomous — firmware fixes]
            └─ signal-logger/wo5-register-campaign  [interactive — ride + burst]
                 └─ signal-logger/wo6-native-tests  [autonomous — GoogleTest]
                      └─ signal-logger/wo7-doc-sync  [autonomous — doc cleanup]
```

## What Blocks What

| WO | Type | Blocked by | Can start when |
|---|---|---|---|
| **WO4b** | 🔧 Interactive (bench) | — | **Now** — you're at the bench |
| **WO4d** | 🤖 Autonomous | WO4b branch exists | After WO4b completes |
| **WO5** | 🔧 Interactive (bench) | WO4d (needs corrected firmware) | After WO4d completes + flash |
| **WO6** | 🤖 Autonomous | WO5 branch exists | After WO5 completes |
| **WO7** | 🤖 Autonomous | WO6 branch exists | After WO6 completes |

## Bridge Prompt: WO4b — Bench Gate

Copy and paste into a **fresh session** when at the bench:

````
You are a bench test assistant for the Echenon project. Load the following
context and work through the session interactively with me.

**Context:**
- Read `.agents/rules/` (files 00–04) first — mandatory.
- Read `.agents/skills/principal-architect-workspace/pr-01-signal-logger/wo4b-bench-gate/WORKORDER.md`
  — this is your work order.
- Read `docs/hardware_specification.md` §5.2 (motor-drive characterisation gate)
  and §5.4 (voltage dividers) for reference.
- Read `docs/measurements/phase0/README.md` for the register and naming convention.

**Branch:** `signal-logger/wo4b-bench-gate`
  - Create it from `signal-logger/wo4a-encoder-integration` before your first commit.

**Tooling rules (MANDATORY — enforced by hooks):**
  - Flashing: Tag → Build → Flash. Use `v0.1.0-dev.N` tags (autonomous, no
    approval needed). Use MCP tools (`build_project`, `upload_firmware`), never `pio` CLI.
  - The firmware is already flashed and running. You probably don't need to flash.

**Session goal:** Complete items B (pin idle), C (motor scope captures),
D (motor current) and E (VIN tolerance) from the WO4 checklist. Walk me
through each step. I'll tell you what I see on the scope and DMM; you help
interpret and tell me what to record.

**Evidence:** Commit files under `docs/measurements/phase0/` using the naming
convention `A<row>-<yyyymmdd>-<short-desc>.<ext>`. Update register rows #1,
#2, #11 in `README.md`.

**Boundary:** Do NOT make design decisions. If something unexpected comes up
(PWM where DC was expected, voltage excursions, etc.), record the observation
and note it in your completion report. The Principal Architect will handle
design implications.

**When done:** Fill in the Completion Report in the WORKORDER, commit it,
and give me a return prompt for the PA.
````

---

## Bridge Prompt: WO5 — Register Campaign

Copy and paste into a **fresh session** after WO4d is complete and flashed:

````
You are a bench test assistant for the Echenon project. Load the following
context and work through the session interactively with me.

**Context:**
- Read `.agents/rules/` (files 00–04) first — mandatory.
- Read `.agents/skills/principal-architect-workspace/pr-01-signal-logger/wo5-register-campaign/WORKORDER.md`
  — this is your work order. Pay special attention to the CA review addendum
  sections (G, H, I, J) which are new.
- Read `docs/measurements/phase0/README.md` for the register, naming convention,
  and which rows are still open.
- Read `docs/measurements/phase0/B01-20261003-sensor-bringup-report.md` for
  baseline sensor readings.
- Read `docs/mitm-encoder-design-notes.md` §12 for the console fault-tolerance
  test descriptions (the safe subset: #1, #2, #3, #7, #9).

**Branch:** `signal-logger/wo5-register-campaign`
  - Create it from `signal-logger/wo4d-logger-fixes` before your first commit.

**Tooling rules (MANDATORY — enforced by hooks):**
  - Flashing: Tag → Build → Flash. Use `v0.1.0-dev.N` tags (autonomous, no
    approval needed). Use MCP tools (`build_project`, `upload_firmware`), never `pio` CLI.

**Session goal:** Complete register rows #3–#10, #12–#13. Also:
  - E-02: Cross-check every ADC tap against a DMM, or rework 22 kΩ → 4.7 kΩ + C.
  - Row #6 source-impedance (load) test.
  - Console fault-tolerance tests #1, #2, #3, #7, #9 (safe subset from §12).
  - Minimum quadrature pulse width measurement.
  - Console mid-seek behaviour.
  Tests #4 and #6 are BLOCKED — do NOT run them without explicit owner approval.

Walk me through each item systematically. I'll exercise the bike controls
and tell you what happens.

**Rows that may need deferral:** #9 (motor disconnect tolerance), #10
(parallel supply tolerance), #12 (Echelon app BLE pairing). If we can't
do them with current equipment, mark them "Deferred to DR-1" in the register
with a brief reason.

**Evidence:** Commit files under `docs/measurements/phase0/` using the naming
convention `A<row>-<yyyymmdd>-<short-desc>.<ext>`. Update each register row
in `README.md` with Evidence and Result.

**Boundary:** Do NOT make design decisions. Record observations. If anything
contradicts the SPEC or suggests a design change, park it in the completion
report.

**When done:** Fill in the Completion Report in the WORKORDER, commit it,
and give me a return prompt for the PA.
````

---

## After Interactive Sessions — Autonomous Sequence

When you return after WO4b and WO5 are done, tell me. I will:

1. **WO4d** (autonomous) — Spawn a worker to fix the logger (L-01–L-08).
   Branch from `signal-logger/wo4b-bench-gate`. Tag, build, flash the fix.
2. *(You run WO5 with the corrected firmware)*
3. **WO6** (autonomous) — Spawn a worker for native tests.
4. **WO7** (autonomous) — Spawn a worker for doc-sync.
5. **Stack submission** — Torvalds-standard PR descriptions.

> **Important:** WO4d is autonomous but must be flashed before WO5 starts.
> After the WO4d worker finishes, I'll tag+build+flash, then present the WO5
> bridge prompt for your second bench session.
