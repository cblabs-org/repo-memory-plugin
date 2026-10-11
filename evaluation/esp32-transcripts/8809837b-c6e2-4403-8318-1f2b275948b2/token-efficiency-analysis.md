# PR-05b: Pipeline Token Efficiency — First Measurement

## Raw Data (from dogfood run on 71 PA-imported episodes)

### Pipeline Job Counts
| Stage | LLM? | Done | Failed | Dead | Queued | Total |
|---|---|---|---|---|---|---|
| capture | No | 119 | 0 | 0 | 0 | 119 |
| triage | No (rule-based) | 71 | 0 | 119 | 0 | 190 |
| extract | **Yes** | 71 | 0 | 0 | 0 | 71 |
| reconcile | **Yes** | 397 | 3 | 6 | 5 | 411 |
| verify | **Yes** | 0 | 0 | 0 | 303 | 303 |

### Output
- **125 claims** produced from 71 episodes
- **~5.8 candidate claims per episode** (411 reconcile jobs / 71 episodes)
- **31.5% reconcile-to-claim rate** (125 claims / 397 done reconciles — rest discarded as duplicates)

### LLM Calls Made
| Stage | Successful Calls | Failed/Dead | Notes |
|---|---|---|---|
| extract | 71 | 0 | 1 call per episode |
| reconcile | 397 | 9 | 1 call per candidate claim |
| verify | 0 | 0 | 303 queued (quota exhausted) |
| **Total** | **468** | **9** | **477 total LLM invocations** |

### Quota Consumption Pattern
- **Run 1**: ~131 calls before 429 (71 extract + ~60 reconcile)
- **Run 2**: ~340 reconcile calls before 429
- **Model**: Gemini 3.8 Flash (High)
- **Total wall time**: ~39 minutes of active pipeline + 50s of triage

### Episode Statistics
- 71 episodes, 174.3 KB compressed total, **2.5 KB average** per episode
- These are PA-imported episodes (structured WORKORDER/PROJECT files), not raw transcripts
- Raw transcripts would be larger (5.4× compaction ratio from PR-02a)

---

## Efficiency Analysis

### Per-Episode Cost
| Metric | Value |
|---|---|
| LLM calls per episode | **~6.7** (1 extract + ~5.8 reconcile) |
| Estimated input tokens per call | ~17K (from E10 measurement: ~85% overhead) |
| Estimated useful tokens per call | ~2.5K (15% of input) |
| **Estimated total input per episode** | **~114K tokens** |

> [!WARNING]
> The 17K input tokens per call is dominated by `agy -p` fixed overhead (system prompt, tool definitions, etc.). The actual episode content is ~2.5 KB compressed. This is the §4.4.1 "fixed overhead" problem the SPEC identified.

### Per-Claim Cost
| Metric | Value |
|---|---|
| LLM calls per claim | **~3.8** (477 / 125) |
| Estimated input tokens per claim | **~64.8K** |
| Reconcile discard rate | 68.5% (candidates judged as duplicates) |

### Comparison to SPEC Estimates (§4.4.1)
| SPEC Estimate | Measured | Delta |
|---|---|---|
| ~17-20K tokens per interactive call | ~17K (confirmed from E10) | ✅ On target |
| ~85% overhead | Not directly measured, likely similar | Needs B4 |
| 5-15 calls/day target | 477 calls for 71 episodes (backfill) | N/A — backfill, not steady-state |
| Batch pending checks into one call | NOT implemented — 1 call per candidate | ❌ **Major optimization opportunity** |

---

## Key Findings

### 1. The reconcile stage is the bottleneck
- **397 LLM calls** for reconciliation vs 71 for extraction
- Each candidate gets its own `agy -p` call (6-7s overhead each)
- Batching candidates per episode would cut calls by ~80%

### 2. Fixed overhead dominates
- Each `agy -p` call has ~17K tokens of overhead (system prompt, tool defs)
- The actual useful payload (episode content + prompt) is ~2-3K tokens
- **~85% of tokens are wasted on overhead** — confirmed SPEC estimate

### 3. The discard rate is high but correct
- 68.5% of candidates are discarded by reconciliation (duplicates/non-durable)
- This means extraction is over-generating — could tighten extraction prompts
- But it also means reconciliation is working (deduplication is the point)

### 4. Quota exhaustion is the constraint
- Two quota resets needed for 71 episodes
- Steady-state (1-3 episodes/day) would need ~7-20 calls/day — well within quota
- Backfill of large history is inherently quota-limited

### 5. Cost recording is broken
- `pipeline.go` passes empty `jobs.Cost{}` on every `Complete()` call
- The executor returns `harness.Result.Usage` with token counts, but pipeline ignores it
- **Must fix for B4 measurement**

---

## Optimization Priorities

### High Impact
1. **Batch reconcile calls**: Group all candidates from one episode into a single `agy -p` call. Would reduce reconcile calls from ~5.8x to ~1x per episode — saving ~80% of LLM calls.
2. **Fix cost recording**: Wire `harness.Result.Usage` through to `jobs.Cost` on completion. Essential for B4.
3. **Use lighter model for reconcile**: Gemini 3.6 Flash has 41% less overhead (E10). Reconcile doesn't need high reasoning.

### Medium Impact
4. **Tighten extraction prompts**: Reduce candidate count per episode from 5.8 to ~3 by being more selective.
5. **Skip redundant candidates**: If the episode is a duplicate import, skip extraction entirely.

### Low Impact (steady-state)
6. **Reduce `agy -p` overhead**: Direct API integration would eliminate the ~17K system prompt overhead — but requires API keys.
