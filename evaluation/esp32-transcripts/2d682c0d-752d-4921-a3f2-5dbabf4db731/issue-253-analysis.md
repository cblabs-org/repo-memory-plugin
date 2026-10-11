# Issue #253 — Deep Analysis: How much of a problem is this, really?

## The Principal Architect's Review Was Based on Wrong Facts

The Oct 1 review made two factual errors that change the analysis significantly:

| Claim in review | Actual code |
|---|---|
| Container is `std::deque<DataJobEntry>` | Container is a **FreeRTOS `QueueHandle_t`** — already a fixed-size ring buffer of pointers |
| `JOB_QUEUE_DEPTH` is 16 | It's **8** |

There is no `std::deque` to replace. The FreeRTOS queue is already a statically-sized ring buffer holding 8 `DataJobEntry*` pointers. The "~30-line fix" the architect proposed doesn't apply.

## What Actually Happens Per Job

```
queueDataJob(lambda, options)
  │
  ├─ new (std::nothrow) DataJobEntry{std::move(job), options}   ← ~48-64 byte heap alloc
  ├─ xQueueSend(_jobQueue, &entryPtr, 0)                       ← pointer copied into ring buffer
  │
  ... worker task picks it up (typically within 100ms) ...
  │
  ├─ entry->job(ctx)                                            ← execute
  └─ delete entry                                               ← freed
```

Each job does **one** `new` and **one** `delete` of a uniform ~48-64 byte `DataJobEntry`. That's the entire heap impact.

## Does `std::function` Add a Second Hidden Allocation?

**No.** The issue body worried about SBO (Small Buffer Optimisation) being defeated by large captures. The research shows:

| Call site | Capture | Size (Xtensa 32-bit) |
|---|---|---|
| `WiserApiModule` (only production consumer) | `[this]` | **4 bytes** |
| All framework docs/examples | `[this]` | **4 bytes** |
| Unit tests (largest) | `std::shared_ptr` (2 ptrs) | **8 bytes** |
| SBO threshold (Xtensa GCC) | — | **16 bytes** |

Every real-world capture is well within SBO. The `std::function` payload **never** hits the heap. Zero hidden allocations.

## So the Total Heap Impact Is…

- **At most 8 × ~64 = 512 bytes** of transient, uniform-sized allocations
- Each one lives for < 100ms (queued → drained on next worker loop iteration)
- All the same size — the allocator reuses the same freed slots efficiently
- Small + uniform + short-lived = **the least problematic allocation pattern for fragmentation**

## Context: What's Actually Causing Fragmentation

The soak test data from Sep 25-26 (`tmp/soak/`) tells the real story:

| Condition | DRAM Free | Watermark | Largest Block | Fragmentation |
|---|---|---|---|---|
| **BLE active** | ~78 KB | ~40 KB | **31 KB** (capped) | **~60%** |
| **No BLE** | ~164 KB | ~123 KB | ~84-92 KB | ~46% |
| **BLE + Central connect** | — | **15.6 KB** (!!) | — | — |
| **Web portal + config save** | — | **22.2 KB** | — | — |

The big fragmentation drivers are:
1. **BLE** — consumes ~85 KB DRAM, dynamic buffer allocation during connection events
2. **TLS handshakes** — large, variable-sized allocations (FrameworkSecureClient)  
3. **JSON parsing** — variable-sized allocations during config saves
4. **Web portal serving** — already has a 20 KB heap guard for this reason

The DataManager job queue's 512 bytes of uniform transient allocations are **noise** in this picture.

## What Happens if Queue Depth 8 Is Too Small?

Today, when the queue is full:
- `xQueueSend` fails (non-blocking, timeout = 0)
- The `DataJobEntry` is immediately `delete`d
- `ESP_LOGW` logs: `"Job queue full — dropping '<tag>' (<N> pending)"`
- `queueDataJob()` returns `false` to the caller

**The job is silently dropped.** The caller gets `false` but most call sites don't check it. With only 1 production consumer (`WiserApiModule`) and jobs draining every ~100ms, queue-full is extremely unlikely in practice.

> [!NOTE]
> If a ring-buffer-by-value approach were used (to eliminate `new`/`delete`), the queue-full behaviour would be identical — a fixed-capacity buffer can still fill up and must drop or block.

## Verdict: Should This Be Fixed?

### The honest assessment

**This is a non-problem dressed up as a GRAVE-05 violation.** The classification was speculative — based on the general principle that `std::function` can cause heap churn, not on evidence that it actually does here. The data says:

- `std::function` never escapes SBO ✓
- Per-job allocations are small, uniform, and short-lived ✓  
- Queue depth is bounded at 8, so max exposure is ~512 bytes ✓
- Real fragmentation comes from BLE, TLS, and JSON — orders of magnitude larger ✓
- No observed failure in 8+ hour soak tests ✓

### Options

| Option | Effort | Risk | Benefit |
|---|---|---|---|
| **Close as won't-fix** | Zero | None | Removes speculative tech-debt noise |
| **Downgrade to P4 + backlog** | Trivial | None | Acknowledges it's theoretically impure but practically harmless |
| **Store entries by value** (custom ring buffer replacing FreeRTOS queue + pointer indirection) | Medium (~100 lines, new container, move semantics, thread safety) | Moderate — replacing a battle-tested FreeRTOS queue with custom code in a core service | Eliminates ~512 bytes of transient allocations |

> [!IMPORTANT]
> Option 3 is **not** the "~30-line fix" the architect described. FreeRTOS queues use `memcpy` internally, and `std::function` is not trivially copyable. You can't simply change the queue item size — you'd need to replace the FreeRTOS queue entirely with a mutex-protected `std::array` ring buffer that supports move semantics. That's a real refactor of a core service's threading model, for ~512 bytes of benefit.

### Recommendation

**Close or downgrade.** The GRAVE-05 rule exists to prevent *unbounded* heap churn from stored `std::function` callbacks — think event handlers that accumulate over time. A bounded queue of 8 entries that drain in <100ms is categorically different. The effort-to-benefit ratio of fixing this is very poor, and the risk of introducing a threading bug in the replacement container is real.
