# Pipeline vs PA: Token Efficiency Deep Review

## The Business Case Problem

The PA distils knowledge **inside sessions that are already running** — zero additional LLM calls. The pipeline makes **new LLM calls** after each session, and 85% of each call is `agy -p` overhead. This fundamentally undermines the cost proposition.

---

## Head-to-Head: PA vs Pipeline

### Knowledge Output

| Metric | PA (7 PRs) | Pipeline (71 episodes) |
|---|---|---|
| Knowledge corpus | 279.6 KB across 60 files | 65.6 KB across 125 files |
| Knowledge units | ~17 explicit statements + rich narrative context | 125 atomic claims |
| Avg unit size | ~4 KB (rich, contextual) | 537 bytes (atomic, structured) |
| File types | PROJECT.md, WORKORDER.md, MEMORY.md | Claim cards (YAML + body) |
| Machine-searchable | ❌ Free text | ✅ Typed, anchored, scored |

### Token Cost

| Metric | PA | Pipeline | Ratio |
|---|---|---|---|
| **Additional LLM calls** | **0** | **~8.5 per episode** | ∞ |
| Marginal tokens per session | ~2,400 (within session) | ~154,000 (new calls) | **64×** |
| Of which useful payload | ~2,400 (100%) | ~23,000 (15%) | — |
| Of which `agy -p` overhead | 0 | ~131,000 (85%) | — |

> [!CAUTION]
> **The pipeline spends ~64× more tokens per knowledge unit than the PA approach, and 85% of that spend is pure overhead.** The useful work (prompt + episode content) is only 15% of each call.

### What Each Approach Captures

| Capability | PA | Pipeline |
|---|---|---|
| Knowledge from PA-managed sessions | ✅ Explicit | ✅ Extracted |
| Knowledge from non-PA sessions | ❌ Not captured | ✅ Captured |
| Knowledge from Claude Code | ❌ | ✅ |
| Knowledge from debugging sessions | ❌ | ✅ Failure signatures |
| Proactive delivery (hooks) | ❌ | ✅ Déjà-vu, task briefs |
| Cross-session deduplication | Manual (MEMORY.md) | ✅ Automatic reconcile |
| Searchable by agents | ❌ (view_file only) | ✅ FTS, MCP pull tool |

---

## Where the Tokens Go

```
Per episode (current, unbatched):

Extract:    1 call  ×  17K overhead + 2.5K payload  = ~19.5K tokens
Reconcile:  5.8 calls × 17K overhead + 2K payload   = ~110K tokens  ← 71% of total
Verify:     1.7 calls × 17K overhead + 1.5K payload = ~31.5K tokens
                                                       ─────────
                                                       ~161K total
                                                       ~137K overhead (85%)
                                                        ~24K useful  (15%)
```

### The `agy -p` overhead breakdown (from E10)
| Component | Tokens |
|---|---|
| System prompt | ~8K |
| Tool definitions (50+ tools) | ~7K |
| Permission/sandbox setup | ~1K |
| Session init | ~1K |
| **Total fixed overhead per call** | **~17K** |

This overhead is paid **per `agy -p` invocation**. With 8.5 calls per episode, that's **~145K tokens of overhead per episode** for nothing.

---

## Three Architectural Fixes

### Fix 1: Batch all LLM work into ONE call per episode

Instead of 8.5 separate `agy -p` calls, send one prompt containing:
- The episode content
- Extract + reconcile + verify instructions
- All existing claims for context

| Metric | Current | Batched | Savings |
|---|---|---|---|
| `agy -p` calls per episode | 8.5 | **1** | **88%** |
| Overhead tokens per episode | ~145K | **~17K** | **88%** |
| Total tokens per episode | ~161K | **~33K** | **80%** |
| Useful payload | ~24K | ~16K | — |
| Overhead % | 85% | 52% | — |

**Effort:** Medium. Combine the three prompts into one structured prompt. Parse a combined response. The reconciler already has the claim context.

### Fix 2: In-session extraction (zero marginal cost)

Do extraction **inside the Stop hook or the session itself** — not as a separate `agy -p` call.

The agent is already running. The session already has the context. Ask it to produce candidate claims as part of its completion:

```
# In the Stop hook or session wrap-up:
"Before ending, extract any durable knowledge from this session as claims..."
```

| Metric | Current | In-session | Savings |
|---|---|---|---|
| Additional LLM calls for extract | 1 | **0** | **100%** |
| Quality | Lossy (compressed episode) | **Full context** | Better |
| Latency | Async (next tick) | **Inline** | Faster |

The remaining reconcile + verify could still run async, but with the extraction already done, reconcile shrinks (candidates are pre-formed).

**Effort:** Low-medium. Add a claim extraction instruction to the Stop hook response. The agent produces structured claims as part of its natural completion flow.

> [!IMPORTANT]
> This is the insight: **the best time to extract knowledge is while the session is live and the model already has full context.** Running a separate LLM call on a compressed episode is strictly worse — less context AND more expensive.

### Fix 3: Drop `agy -p` for direct Gemini API

Replace `agy -p` subprocess with direct `google.golang.org/genai` API calls:

| Metric | `agy -p` | Direct API |
|---|---|---|
| Fixed overhead per call | ~17K tokens | **~200 tokens** (system prompt only) |
| Startup latency | 6-7s | **<1s** |
| Requires | Antigravity installed | API key |
| Quota | User's Antigravity quota | Separate billing |

This eliminates the 85% overhead problem entirely but introduces an API key dependency the SPEC explicitly avoided.

**Effort:** Medium. Replace executor with `genai.Client`. Need to handle auth, retries, billing.

---

## Recommended Path

### Short-term (PR-05b/PR-06): Fix 1 — Batch
- Combine extract + reconcile + verify into one `agy -p` call per episode
- Drops from 8.5 calls to 1 call per episode
- Drops overhead from 85% to ~52%
- **Makes the cost comparable to the PA approach** (~33K per episode vs ~2.4K in-session, but covering ALL sessions not just PA-managed ones)

### Medium-term (PR-07+): Fix 2 — In-session extraction
- The Stop hook already fires at session end
- Add claim extraction to the session completion flow
- Zero marginal LLM cost for extraction
- Reconcile/verify can still run async with 1 batched call

### Long-term (P2+): Fix 3 — Direct API
- Only if quota becomes a constraint at scale
- Evaluate after P1 gate

---

## The Real Value Proposition (Reframed)

The pipeline's value is NOT "cheaper knowledge distillation" — the PA does that for free. The value is:

1. **Coverage**: Captures knowledge from ALL sessions — debugging, bug-fixing, exploration — not just PA-managed work
2. **Automation**: No human architect needed to write completion reports
3. **Proactive delivery**: Hooks serve relevant knowledge to agents at the right moment
4. **Searchability**: Machine-queryable claim index vs. free-text markdown

The cost model must reflect this: the pipeline should be **cheap enough that running on every session is viable**, not just PA sessions. Fix 1 (batching) gets us there. Fix 2 (in-session extraction) makes it essentially free.
