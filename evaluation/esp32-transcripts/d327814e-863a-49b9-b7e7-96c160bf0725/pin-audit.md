# WO4 — Pin Configuration Audit

## Phase 1: Root Cause Investigation

### Symptom
Cadence sensor connected to GPIO 5 (Orange wire, via 22kΩ resistor). Pedaling
produces no visible change on the dashboard, no cadence events (type=0) in the
event log, and the event buffer is saturated with spurious motor-onset events.

### Evidence Gathered

#### 1. ESP32-S3 ADC1 Channel ↔ GPIO Mapping (confirmed via Espressif docs)

| ADC1 Channel | GPIO | Board Label | board.h Signal         | Wire   | Connected? |
|:---:|:---:|:---:|:---|:---|:---:|
| CH0 | GPIO 1 | **A0** | `PIN_POT_WIPER`       | Blue   | ❌ No (not yet) |
| CH1 | GPIO 2 | **A1** | `PIN_POT_HI`          | Yellow | ❌ No (not yet) |
| CH2 | GPIO 3 | **A2** | `PIN_POT_LO`          | Green  | ❌ No (not yet) |
| CH3 | GPIO 4 | **A3** | `PIN_CADENCE_RETURN`   | Grey   | ✅ Yes |
| CH4 | GPIO 5 | **D2** | `PIN_CADENCE`          | Orange | ✅ Yes |
| CH5 | GPIO 6 | **D3** | `PIN_MOTOR_SENSE_P`    | Red    | ❌ No (not yet) |
| CH6 | GPIO 7 | **D4** | `PIN_MOTOR_SENSE_N`    | Black  | ❌ No (not yet) |
| CH7 | GPIO 8 | **D5** | `PIN_KNOB_ENC_A`       | Brown  | ❌ No (not yet) |
| CH8 | GPIO 9 | **D6** | `PIN_KNOB_ENC_B`       | Orange | ❌ No (not yet) |
| CH9 | GPIO 10| **D7** | `PIN_KNOB_ENC_VCC`     | Red    | ❌ No (not yet) |

**Key fact:** On the ESP32-S3, ADC1 channel number = GPIO number − 1.  
GPIO 5 → ADC1_CH4. GPIO 6 → ADC1_CH5. etc.

#### 2. SignalLogger ADC configuration (SignalLogger.cpp line 93-98)

```cpp
adc_digi_pattern_config_t adc_pattern[10];
for (int i = 0; i < 10; i++) {
    adc_pattern[i].channel = i; // assumes channels 0-9
    ...
}
```

This is **correct** — it configures ADC1 channels 0-9, which map to GPIO 1-10.
The `adc_continuous` driver returns data tagged with the **ADC channel number**
(0-9), not the GPIO number.

#### 3. Live ADC readings (`/api/echenon/live`)

| ADC CH | GPIO | Signal | min | mean | max | Assessment |
|:---:|:---:|:---|:---:|:---:|:---:|:---|
| 0 | 1 | Pot wiper | 0 | 833 | 3706 | Floating (not connected) ✓ |
| 1 | 2 | Pot HI | 0 | 732 | 3230 | Floating (not connected) ✓ |
| 2 | 3 | Pot LO | 0 | 1061 | 4095 | Floating (not connected) ✓ |
| 3 | 4 | Cadence return | 0 | 0 | 0 | ✅ Connected — reads 0V (return path grounded) |
| **4** | **5** | **Cadence** | **4095** | **4095** | **4095** | **📌 Connected but pegged HIGH by internal pull-up** |
| 5 | 6 | Motor+ | 95 | 1402 | 3182 | Floating (not connected) ✓ |
| 6 | 7 | Motor− | 0 | 553 | 1895 | Floating (not connected) ✓ |
| 7 | 8 | Enc A | 4095 | 4095 | 4095 | Floating + internal pull-up = pegged HIGH |
| 8 | 9 | Enc B | 677 | 1196 | 1864 | Floating (not connected) ✓ |
| 9 | 10 | Enc VCC | 0 | 390 | 1295 | Floating (not connected) ✓ |

#### 4. GPIO ISR configuration (SignalLogger.cpp line 133-146)

```cpp
io_conf.pin_bit_mask = (1ULL << 5) | (1ULL << 8);
io_conf.pull_up_en = GPIO_PULLUP_ENABLE;   // ← !!
...
gpio_isr_handler_add((gpio_num_t)5, gpioIsrHandler, (void*)5);
gpio_isr_handler_add((gpio_num_t)8, gpioIsrHandler, (void*)8);
```

**Bug #1: Internal pull-up enabled on GPIO 5 and GPIO 8.**  
The hardware spec §5.5 explicitly states: *"Every tap pin is configured with
**no internal pull-up or pull-down**."* The pull-up on GPIO 5 explains why
ADC CH4 reads 4095 (pegged at 3.3V) — the internal pull-up is fighting
the cadence signal through the 22kΩ series resistor.

#### 5. Software threshold logic (SignalLogger.cpp line 256)

```cpp
} else if (channel == 5 || channel == 6) { // Motor
```

**Bug #2: Channel number confusion.** This checks ADC channel 5 and 6, which
are GPIO 6 and GPIO 7 (the motor sense pins). The comment says "Motor" which
is correct for those ADC channels. However, the event log shows these channels
oscillating wildly because those GPIOs are **floating** (motor dividers not
connected yet). This is filling the 4000-event ring buffer with garbage,
which is why `event_count` = 4000 (saturated).

#### 6. GPIO ISR vs ADC channel numbering

The ISR passes the **GPIO number** as the event channel:
```cpp
_eventLogBuffer[idx].channel = pin;  // pin = 5 or 8 (GPIO numbers)
```

But `handleSoftwareThresholds` receives the **ADC channel number**:
```cpp
handleSoftwareThresholds(chan, mv, ts_us);  // chan = 0-9 (ADC channels)
```

So the event log mixes two numbering schemes — GPIO ISR events use GPIO
numbers (5, 8) while software threshold events use ADC channel numbers (0-9).
Not a functional bug since they go to different event types, but confusing.

---

## Phase 2: Findings Summary

### Root Causes (Confirmed)

| # | Bug | Impact | Status |
|---|---|---|---|
| **1** | `GPIO_PULLUP_ENABLE` on GPIO 5 and 8 | Violated hardware spec §5.5 no-pulls rule | ✅ Fixed (v0.1.0-dev.2) |
| **2** | `gpio_config(GPIO_MODE_INPUT)` after `adc_continuous_start()` | Disconnected GPIO 5/8 from ADC analog peripheral | ✅ Fixed (v0.1.0-dev.3) |
| **3** | **22kΩ series resistor exceeds ESP32-S3 ADC source impedance limit** | S/H capacitor can't settle during brief reed switch closure. Espressif recommends <10kΩ source impedance for ADC continuous mode. | 🔧 Confirmed root cause |
| **4** | Floating unconnected pins flooding event ring buffer | 4000 garbage events from motor-sense threshold crossings | Known, deferred |

### Evidence for Root Cause #3

| Test | CH3 min | CH3 mean | Result |
|---|---|---|---|
| 22kΩ + no cap, reed closed briefly (pedaling) | 4095 | 4095 | ❌ No detection |
| 22kΩ + no cap, reed held closed for seconds | 215 | 240 | ✅ Eventually settles |
| 22kΩ + 100nF cap, reed closed briefly | ~3250 | 4095 | ⚠️ Partial improvement |
| **Direct (no resistor, no cap), reed closed briefly** | **~2680** | **~217** | **✅ Cadence detected!** |

Cadence at 120 RPM confirmed with ~480ms pulse spacing, matching actual pedal speed.

> [!NOTE]
> The min values (~2680) even on direct connection are from ADC multiplexer
> cross-talk — a known ESP32-S3 artefact when scanning 10 channels at 83 kHz.
> The **mean** value is the reliable indicator.

### Recommended Fix

Replace the 22kΩ series resistors with **4.7kΩ** on all ADC-sampled lines:
- Still provides current limiting for GPIO protection (0.7mA at 3.3V)
- Well within Espressif's <10kΩ source impedance recommendation
- The ÷4 divider on motor-sense lines (30kΩ/10kΩ) may also need review
