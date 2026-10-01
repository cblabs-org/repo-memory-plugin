# WO1 → Architect: Injection Persistence & Token Cost Concern

## Context
E1 proved that `ephemeralMessage` is **per-invocation transient** — visible to the model only during the single invocation where it's injected, then dropped. This has a direct cost implication for the SPEC's §7.2 injection strategy.

## The Problem
The SPEC currently routes **all** context injection through `PreInvocation` → `ephemeralMessage`:
- Task brief (1–4k tokens) at `invocationNum == 0`
- Touch & error alerts (every invocation)
- Déjà-vu tier-1 warnings (every invocation)

Because `invocationNum` resets to 0 on every user prompt, **and** because `ephemeralMessage` doesn't persist across invocations within a prompt cycle, the task brief must be re-injected on every `PreInvocation` call — not just once per prompt.

A typical tool-calling prompt generates 2–10+ invocations. At 2k tokens per injection, that's **4–20k+ wasted tokens per user prompt** on content the model has already seen but can't retain.

## What We Know (from E1)

| Injection type | Persistence | Tested? |
|:---|:---|:---|
| `ephemeralMessage` | Per-invocation only (dropped after each invocation) | ✅ Proven (E1, conversations `e9b80453`, `e00ea91f`) |
| `userMessage` | **Unknown** — may persist across invocations | ❌ Not tested |
| `toolCall` | **Unknown** — may persist across invocations | ❌ Not tested |
| System prompt include (`@[...]()` in GEMINI.md) | Persistent, cached | ✅ Known working (MAP.md) |
| MCP tool call (model pulls via `memory.brief`) | N/A (pull model, no injection) | ✅ Known working |

## Proposed Injection Architecture (if `userMessage` persists)

| Content | Mechanism | When | Token cost |
|:---|:---|:---|:---|
| System map (~2k) | `@[memory map]` include in GEMINI.md | Always-on, cached | **0 marginal** |
| Task brief (1–4k) | `userMessage` via `injectSteps` | `invocationNum == 0` only | **Once per prompt** |
| Touch/error/déjà-vu alerts (<200 tokens) | `ephemeralMessage` via `injectSteps` | When something is due | **Tiny, acceptable** |
| Deep context (on demand) | MCP `memory.brief` / `memory.ask` | Model pulls when needed | **0 injection cost** |

## Proposed Injection Architecture (if `userMessage` also ephemeral)

| Content | Mechanism | When | Token cost |
|:---|:---|:---|:---|
| System map (~2k) | `@[memory map]` include in GEMINI.md | Always-on, cached | **0 marginal** |
| Task brief (1–4k) | **MCP pull only** (`memory.brief`) | Model decides | **0 injection cost** |
| Touch/error/déjà-vu alerts (<200 tokens) | `ephemeralMessage` via `injectSteps` | When something is due | **Tiny, acceptable** |

In this fallback design, `ephemeralMessage` is reserved for sub-200-token nudges — never for bulk context.

## Required Follow-Up: Experiment E1b

**Test `userMessage` persistence across invocations.** Same methodology as E1:
1. Inject `{"injectSteps": [{"userMessage": "MARKER-USRMSG-4P8W"}]}` at `invocationNum=0`
2. Return `{}` at `invocationNum=1+`
3. At invocation N+5, ask the model if it can see the marker
4. If visible → `userMessage` persists → use it for task brief
5. If not visible → pull-only architecture, no bulk injection

## Recommendation
- **Do not build** the §7.2 injection pipeline assuming `ephemeralMessage` can carry the task brief. The token cost is unacceptable.
- **Run E1b first** (one quick test, ~10 minutes) to determine whether `userMessage` is the right vehicle.
- **Regardless of E1b outcome**, cap `ephemeralMessage` payloads at ~200 tokens in the SPEC. Bulk context should go via persistent injection or pull.
- Consider asking the Antigravity team directly: *"Is `userMessage` in `injectSteps` persistent across invocations within the same prompt cycle, or is it ephemeral like `ephemeralMessage`?"*
