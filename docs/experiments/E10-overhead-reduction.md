# E10: Housekeeping Overhead Reduction

> **Experiment ID:** E10
> **Date:** 2026-10-01
> **Status:** Complete
> **Platform:** macOS (Apple Silicon), `agy` CLI v1.2.14

## Question

What is the fixed token overhead of a housekeeping `agy -p` call, and can it be reduced below the ~17k token baseline?

## Method

1. Ran `agy -p --output-format json "Return the word HELLO"` with different configurations
2. Compared `input_tokens` across: default config, minimal agent definition, disabled slash commands, sandbox mode, and non-workspace directory
3. Tested prefix caching by running 5 identical calls in quick succession
4. Compared overhead between Gemini 3.8 Flash and Gemini 3.6 Flash model families

## Raw Evidence

### Overhead by Configuration

| Configuration | Input Tokens | Delta from Baseline | Notes |
|---|---|---|---|
| Default config, from repo dir | 16,810 | — | Baseline |
| Default config, from /tmp | 16,660 | -150 | No GEMINI.md or workspace-specific rules |
| `--agent '{"identity":{"name":"test"}}'` | 16,809 | -1 | Minimal agent definition has no effect |
| `--disable-slash-commands` | 16,815 | +5 | Flag itself adds a token or two, no reduction |
| `--sandbox` | 17,593 | +783 | Sandbox restrictions added to system prompt |
| Gemini 3.6 Flash (Low), from repo | ~9,930 | -6,880 | **Different model family, dramatically lower** |

### Prefix Caching Test (5 runs, Gemini 3.8 Flash Low)
```
run 1: in=16,816 cache_read=0 out=1 dur=6.6s wall=11.8s
run 2: in=16,809 cache_read=0 out=1 dur=7.0s wall=13.9s
run 3: in=16,816 cache_read=0 out=1 dur=3.4s wall=10.1s
run 4: in=16,808 cache_read=0 out=1 dur=4.5s wall=12.1s
run 5: in=16,811 cache_read=0 out=1 dur=4.5s wall=11.0s
```
**Result: No prefix caching observed.** `cache_read_tokens = 0` for all 5 runs despite identical prompts. Each `agy -p` call creates a new conversation, preventing session-level prefix caching.

### Model Family Comparison (Gemini 3.6 Flash Low)
```
call 1: in=18,073 out=1 dur=5.0s wall=11.1s
call 2: in=9,933  out=1 dur=5.1s wall=12.2s
call 3: in=9,925  out=1 dur=6.0s wall=10.7s
call 4: in=9,929  out=1 dur=4.6s wall=11.6s
call 5: in=9,926  out=1 dur=3.8s wall=10.4s
```
First call anomalously high (18,073), subsequent calls ~9,930. This suggests either:
- Different tokenizer efficiency between model families
- First-call system prompt compilation cost amortized across session
- Different skill/tool rendering for older model versions

### Timing Breakdown
```
Total wall time per call: 10-14s
  - API duration:  3-7s (model inference)
  - CLI overhead:  6-7s (process startup, auth, conversation setup)
```

### Token Budget Breakdown (estimated)

The ~16.8k tokens for Gemini 3.8 Flash comprise:
- **System prompt** (identity, communication style, guidelines): ~2-3k tokens
- **Tool descriptions** (30+ tools with full JSON schemas): ~6-8k tokens
- **Skill descriptions** (25+ skills with trigger descriptions): ~3-4k tokens
- **MCP server listings**: ~1-2k tokens
- **User rules** (GEMINI.md includes): ~150 tokens
- **Workspace/conversation metadata**: ~500 tokens
- **User prompt**: ~10 tokens

## Answer

1. **The fixed overhead is ~16.8k tokens for Gemini 3.8 Flash, ~9.9k for Gemini 3.6 Flash.** The overhead cannot be reduced below these floors with any available CLI flags.

2. **No available mechanism to disable plugins per-call.** The `--agent` flag does not suppress plugin/skill loading. The `--disable-slash-commands` flag only affects slash command expansion, not system prompt size. No `--no-plugins` or `--no-skills` flag exists.

3. **Prefix caching does NOT work for `agy -p` calls.** Each call creates a new conversation with a new conversation ID, which prevents the API from recognizing the system prompt as a cache-hit. This means every housekeeping call pays the full ~17k input token cost.

4. **Model family choice has a major impact.** Gemini 3.6 Flash uses ~9.9k tokens vs ~16.8k for Gemini 3.8 Flash — a 41% reduction. This is likely due to different tokenizer efficiency and/or a simpler system prompt for older models.

5. **CLI overhead adds ~6-7 seconds per call.** The wall-clock time includes process startup, auth token acquisition, and conversation setup — significant for high-frequency housekeeping.

### Cost Projection

At 1 housekeeping call per minute:
| Model | Tokens/call | Tokens/day | Tokens/month |
|---|---|---|---|
| Gemini 3.8 Flash (Low) | 16,810 | 24.2M | 726M |
| Gemini 3.6 Flash (Low) | 9,930 | 14.3M | 429M |

At current Gemini Flash pricing (~$0.075/1M input tokens), this is:
- Gemini 3.8: ~$1.82/month
- Gemini 3.6: ~$1.07/month

The token cost is manageable, but the 10-14s wall-clock time per call makes 1-call-per-minute infeasible via `agy -p`.

## SPEC Impact

- **SPEC §Housekeeping**: The ~17k token floor is an irreducible cost of the `agy -p` approach. Cannot be reduced with current CLI flags.
- **Design decision needed**: Prefix caching requires same-conversation continuation. The housekeeping engine should use `--conversation` to resume a persistent housekeeping conversation rather than creating new conversations each time. This could enable prefix caching and reduce ongoing token costs.
- **Design consideration**: Consider Gemini 3.6 Flash for housekeeping tasks — 41% fewer tokens with equivalent capability for simple memory pipeline operations.
- **Design consideration**: The 10-14s wall-clock time per `agy -p` call means the housekeeping engine should batch operations within a single conversation turn rather than spawning many short calls.
- **Alternative path**: Instead of `agy -p`, consider using the direct API (via MCP or Go SDK) to avoid CLI overhead entirely. A direct API call with a minimal system prompt could reduce to ~500 tokens.
