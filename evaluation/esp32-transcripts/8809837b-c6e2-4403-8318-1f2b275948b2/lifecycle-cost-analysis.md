# Full Lifecycle Cost: Production + Recall

## The Missing Factor: Recall Cost

The 64× production cost comparison was misleading because it ignored **recall cost** — the tokens loaded every time knowledge is used.

The PA loads **~9,148 input tokens** of bulk knowledge every session (SKILL.md, MEMORY.md, PROJECT.md, recent WORKORDERs). Most of this is untargeted — the agent reads everything and picks what's relevant.

The pipeline delivers **~600 input tokens** of targeted claims via hooks — only what's relevant to the current tool call or task.

**The PA is 15× more expensive at recall.**

---

## Total Lifecycle Cost (Production + Recall × N sessions)

```
                            1 session    10 sessions   50 sessions   100 sessions
                            ---------    -----------   -----------   ------------
PA (in-session prod):        11,548        93,880       459,800       917,200
Pipeline (current):         154,600       160,000       184,000       214,000
Pipeline (batched):          33,600        39,000        63,000        93,000
Pipeline (in-session):        1,100         6,500        30,500        60,500
```

### Break-even analysis

| Architecture | Breaks even vs PA at... | Long-term (100 sessions) |
|---|---|---|
| Current pipeline (8.5 `agy -p` calls) | **Never** ❌ | 4.3× cheaper |
| Batched pipeline (1 `agy -p` call) | **~4 sessions** ✅ | 9.9× cheaper |
| In-session extraction (0 extra calls) | **Immediately** ✅ | **15.2× cheaper** |

> [!IMPORTANT]
> Even the current expensive pipeline becomes cheaper than the PA at ~18 sessions. The recall savings compound. But with in-session extraction, the pipeline is cheaper from day one.

---

## Dimension-by-Dimension Comparison

| Dimension | PA | Pipeline (in-session) | Winner |
|---|---|---|---|
| **Production cost** | ~2,400 tokens (in-session) | ~500 tokens (in-session) | **Pipeline** (5×) |
| **Recall cost / session** | ~9,148 tokens (bulk) | ~600 tokens (targeted) | **Pipeline** (15×) |
| **Recall quality** | Untargeted — loads everything | Targeted — relevant claims only | **Pipeline** |
| **Recall timing** | Session start only | Point-of-need (hook injection) | **Pipeline** |
| **Knowledge precision** | ~100% (human-curated) | 100% (LLM-extracted, verified) | Tie |
| **Knowledge recall** | ~100% for own sessions | ~80% (misses some cross-system) | **PA** |
| **Coverage** | PA sessions only | ALL sessions | **Pipeline** |
| **Context window usage** | ~9K tokens occupied | ~600 tokens occupied | **Pipeline** (15×) |
| **Staleness** | Gets stale (no auto-update) | Auto-reconciled on new evidence | **Pipeline** |
| **Searchable** | ❌ Free text, view_file only | ✅ FTS, MCP pull tool | **Pipeline** |
| **Machine-readable** | ❌ | ✅ Typed, anchored, scored | **Pipeline** |

### The PA's one advantage: recall depth for its own sessions
The PA has richer, more contextual knowledge for the specific projects it manages. 4KB of narrative context per WORKORDER vs 537 bytes of atomic claim. But it loads ALL of that context every time, whether relevant or not.

---

## The Context Window Argument

This may be the most important factor. Context windows are finite and expensive:

| Approach | Knowledge tokens in context | Working space impact |
|---|---|---|
| PA | ~9,148 tokens loaded at boot | **5-10% of context consumed** by bulk knowledge |
| Pipeline hooks | ~600 tokens injected at point-of-need | **<0.5% of context** |

At scale (20+ knowledge items, growing WORKORDERs), the PA approach scales linearly — more projects = more bulk loading. The pipeline approach scales by relevance — you only inject what matches.

---

## The Redeeming Architecture

With in-session extraction (conversation injection or Stop hook distillation):

```
Production:  Agent distils own knowledge at session end     → ~500 tokens, in-session
Storage:     Atomic claims in .memory/claims/               → 537 bytes avg, month-sharded
Dedup:       Embedding similarity, no LLM needed            → 0 tokens
Recall:      Hook injection at point-of-need                → ~600 tokens, targeted
Search:      FTS + pull tool on demand                      → ~200 tokens per query
```

**Total cost: ~1,100 tokens per episode-to-recall cycle vs PA's ~11,548.**

The pipeline wins on **every dimension that matters** — IF production is in-session. The multi-stage `agy -p` pipeline was solving the wrong problem. The right architecture is:

1. **In-session extraction** (the agent that did the work writes the claims)
2. **Mechanical dedup** (embedding match, no LLM)
3. **Targeted delivery** (hooks inject relevant claims at point-of-need)
4. **FTS search** (on-demand pull via MCP tool)
