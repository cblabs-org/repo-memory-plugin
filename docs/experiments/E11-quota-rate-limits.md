# E11: Quota and Rate Limits (Preliminary)

> **Experiment ID:** E11
> **Date:** 2026-10-01
> **Status:** Complete (preliminary — full tracking in PR-05b)
> **Platform:** macOS (Apple Silicon), `agy` CLI v1.2.14
> **Subscription:** Personal OAuth (tier not explicitly exposed; availability of Claude Opus 4.6 and Gemini 3.1 Pro suggests Pro tier)

## Question

What are the observable rate limits and quota constraints for rapid housekeeping calls via `agy -p`?

## Method

1. Ran 10 `agy -p` calls in rapid succession using Gemini 3.8 Flash (Low)
2. Ran 5 calls with Gemini 3.6 Flash (Low) as the "lite" equivalent
3. Recorded status codes, timing, and any error responses
4. Checked for 429 errors and retry-after headers

## Raw Evidence

### Test 1: 10 rapid calls — Gemini 3.8 Flash (Low)
```
call  1: status=SUCCESS in=16,814 out=1 dur=6.4s wall=12.0s
call  2: status=SUCCESS in=16,809 out=1 dur=6.2s wall=12.6s
call  3: status=SUCCESS in=16,815 out=1 dur=4.7s wall=11.2s
call  4: status=SUCCESS in=16,811 out=1 dur=4.8s wall=11.0s
call  5: status=SUCCESS in=16,816 out=1 dur=3.9s wall=13.0s
call  6: status=SUCCESS in=16,800 out=1 dur=5.2s wall=10.8s
call  7: status=SUCCESS in=16,813 out=1 dur=4.4s wall=10.9s
call  8: status=SUCCESS in=16,816 out=1 dur=4.0s wall=11.5s
call  9: status=SUCCESS in=16,817 out=1 dur=4.7s wall=11.4s
call 10: status=SUCCESS in=16,809 out=2 dur=5.8s wall=11.7s

Summary: 10/10 successes, 0 errors, 0 rate limits
Total time: ~116s (sequential)
Total tokens consumed: ~168,120 input + ~11 output = ~168,131
```

### Test 2: 5 rapid calls — Gemini 3.6 Flash (Low)
```
call 1: status=SUCCESS in=18,073 out=1 dur=5.0s wall=11.1s
call 2: status=SUCCESS in=9,933  out=1 dur=5.1s wall=12.2s
call 3: status=SUCCESS in=9,925  out=1 dur=6.0s wall=10.7s
call 4: status=SUCCESS in=9,929  out=1 dur=4.6s wall=11.6s
call 5: status=SUCCESS in=9,926  out=1 dur=3.8s wall=10.4s

Summary: 5/5 successes, 0 errors, 0 rate limits
Total time: ~56s (sequential)
Total tokens consumed: ~57,786 input + ~5 output = ~57,791
```

### Additional context from the session
During this experiment session (WO3), approximately 25+ `agy -p` calls were made across various experiments (E7, E10, E11) within a ~15 minute window. **Zero rate limit errors were observed across all calls.**

### Timing Observations
- API inference time: 3.4–7.0s (variable, likely load-dependent)
- CLI overhead (startup + auth + setup): ~6-7s per call
- The sequential nature of `agy -p` calls (one at a time) means effective throughput is limited to ~1 call per 11s
- No parallel call testing was performed (each call blocks the shell)

### Subscription Tier
- Auth type: `oauth-personal`
- Available models include: Gemini 3.8/3.7/3.6 Flash, Gemini 3.1 Pro, Claude Sonnet 4.6 (Thinking), Claude Opus 4.6 (Thinking), GPT-OSS 120B
- The model roster suggests **Pro tier** or equivalent paid subscription
- No explicit tier identifier found in settings or state files

## Answer

1. **No rate limits observed for sequential calls.** 10 rapid calls (~1 every 11s) to Gemini 3.8 Flash showed zero 429 errors. The CLI's inherent latency (~11s per call) acts as a natural rate limiter, making it difficult to hit API-level rate limits.

2. **25+ calls in 15 minutes produced zero rate limit errors.** Across all experiments in this session, no throttling was observed on any model.

3. **The bottleneck is CLI overhead, not API limits.** At ~11s per call, the maximum throughput is ~5-6 calls/minute via sequential `agy -p`. This is well below any likely API rate limit.

4. **Model availability suggests Pro tier.** The availability of Claude Opus 4.6 and multiple Gemini Pro models is consistent with a paid subscription. Rate limits may be more restrictive on free-tier accounts.

5. **Parallel call behavior is untested.** All calls were sequential. Spawning multiple `agy -p` processes simultaneously could trigger different rate limit behavior.

### Rate Limit Summary

| Scenario | Calls | Duration | Rate Limits | Effective Rate |
|---|---|---|---|---|
| Gemini 3.8 Flash (Low) × 10 | 10 | ~116s | 0 | ~5.2 calls/min |
| Gemini 3.6 Flash (Low) × 5 | 5 | ~56s | 0 | ~5.4 calls/min |
| Combined session | 25+ | ~15min | 0 | ~1.7 calls/min |

## SPEC Impact

- **SPEC §Quota**: No rate limit concerns for the envisioned housekeeping cadence (1 call per 1-5 minutes). The CLI's inherent latency provides natural rate limiting.
- **Design consideration**: If housekeeping needs higher throughput (e.g., processing a backlog), parallel `agy -p` calls may be needed. This would require separate rate limit testing.
- **Design consideration**: The ~11s wall-clock time per call means housekeeping calls should be batched (multiple operations per call) rather than spawned frequently.
- **For PR-05b**: Monitor daily token consumption over 2 weeks. At 1 call/5min with 16.8k tokens: ~4.8M tokens/day, ~145M tokens/month. Check if this approaches any monthly quota cap.
- **For PR-05b**: Test parallel calls and higher-frequency patterns to find actual rate limit thresholds.
- **Tier dependency**: These results may not apply to free-tier users. Document minimum subscription requirements.
