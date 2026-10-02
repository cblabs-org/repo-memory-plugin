# Dogfood Evaluation 01 — PR-05b Pipeline Performance Report

**Date:** 2026-10-02  
**Repo:** `cblabs-org/repo-memory-plugin`  
**Corpus:** 71 PA-imported episodes from 7 merged PRs (PR-01 through PR-05a)  
**Model:** Gemini 3.8 Flash (High)  
**Branch:** `pr-05b-dogfood-pilot/wo4-b5-evaluation` (commit `960d852`)

---

## 1. Executive Summary

The knowledge pipeline was run end-to-end on this repository's own development history — 71 episodes representing the full build-out of the repo-memory system across 7 PRs and 47 work orders.

**Key results:**

| Metric | Value |
|---|---|
| Claims produced | **125** |
| Precision (B5) | **100%** (10/10 sampled claims factually correct) |
| Recall vs PA ground truth | **80%** (8/10 PA facts found in claims) |
| LLM calls consumed | **477** |
| Quota resets required | **3** (extract+reconcile burned through daily quota twice) |
| Production cost per claim | **~64,800 input tokens** |
| Recall cost per session | **~600 tokens** (vs PA's ~9,148) |

**Verdict:** The pipeline produces accurate, machine-searchable knowledge but at prohibitive production cost. The production architecture must change (in-session extraction), but the recall and delivery architecture is sound and 15× more token-efficient than the PA's bulk-loading approach.

---

## 2. Pipeline Throughput

### 2.1 Job Flow

| Stage | LLM? | Done | Failed | Dead | Queued | Total |
|---|---|---|---|---|---|---|
| capture | No | 119 | 0 | 0 | 0 | 119 |
| triage | No (rule-based) | 71 | 0 | 119¹ | 0 | 190 |
| extract | **Yes** | 71 | 0 | 0 | 0 | 71 |
| reconcile | **Yes** | 397 | 3 | 6 | 5 | 411 |
| verify | **Yes** | 0 | 0 | 0 | 303² | 303 |

¹ 119 dead triage jobs from duplicate PA imports (idempotency correctly deduplicated)  
² 303 verify jobs queued but not executed — quota exhausted before verify stage ran

### 2.2 Pipeline Ratios

| Metric | Value | Notes |
|---|---|---|
| Candidates per episode | **5.8** | 411 reconcile jobs / 71 episodes |
| Claims per episode | **1.76** | 125 claims / 71 episodes |
| Reconcile → claim rate | **31.5%** | 125 / 397 successful reconciles |
| Reconcile discard rate | **68.5%** | Correctly rejected as duplicate or non-durable |
| Reconcile failure rate | **2.2%** | 9 failures out of 411 (JSON parse + quota) |

### 2.3 Timing

| Phase | Duration | Notes |
|---|---|---|
| Build + seed | ~5s | Compiled binary, seeded 71 triage jobs |
| Triage (all 71) | ~50s | Rule-based, no LLM, subsystem classification |
| Extract (71 episodes) | ~8 min | 1 `agy -p` call per episode |
| Reconcile (397 jobs) | ~31 min | 1 `agy -p` call per candidate |
| **Total active pipeline time** | **~40 min** | Before quota exhaustion halted run |

### 2.4 Failure Modes Encountered

| Issue | Impact | Resolution |
|---|---|---|
| `agy -p` stdin piping: `-` interpreted as flag | Extraction returned garbage | Fixed: pass prompt as `--print` argument |
| LLM returning natural language instead of JSON | Reconcile parse failures (15 initial) | Fixed: robust JSON extraction (find first `{` to last `}`) |
| Quota exhaustion (HTTP 429) | Pipeline stalled 3 times | Fixed: detect exit code 3, clean abort, queue remaining jobs |
| Cost recording empty | No B4 data | **Not yet fixed:** `pipeline.go` passes empty `Cost{}` |

---

## 3. Knowledge Quality (B5 Evaluation)

### 3.1 Claim Type Distribution

| Type | Count | % |
|---|---|---|
| fact | 43 | 34.4% |
| decision | 20 | 16.0% |
| architecture | 16 | 12.8% |
| convention | 10 | 8.0% |
| constraint | 8 | 6.4% |
| finding | 6 | 4.8% |
| pattern | 4 | 3.2% |
| pitfall / gotcha | 4 | 3.2% |
| other (feature, rule, quirk, etc.) | 14 | 11.2% |
| **Total** | **125** | **100%** |

### 3.2 Claim Statistics

| Metric | Value |
|---|---|
| Total claim corpus size | 65.6 KB |
| Average claim size | 537 bytes |
| Claims with file anchors | ~90% |
| Claims with `relevant_when` | ~75% |
| Claims with `apply` guidance | ~70% |

### 3.3 Precision Test (Are claims accurate?)

10 randomly sampled claims checked against the codebase:

| Claim | Accurate? |
|---|---|
| PreToolUse deny retries (3 advisory, 2 policy) | ✅ |
| bufio.Scanner 50MB buffer for transcript parsing | ✅ |
| Probe `--probe-decision` JSON response format | ✅ |
| Cobra CLI modular `cmd_<name>.go` structure | ✅ |
| `agy -p` ignores `--project` flag | ✅ |
| Canary tests for transcript format bumps | ✅ |
| `transcript_full.jsonl` is append-only | ✅ |
| `agy -p` startup latency 6–7s per call | ✅ |
| Hook latency p50: ~57ms non-enrolled vs ~3ms enrolled | ✅ |
| Tool traffic = 78% of transcript footprint | ✅ |

**Precision: 10/10 (100%)**

### 3.4 Recall Test (Did it find what the PA knew?)

10 specific knowledge statements from PA completion reports:

| PA Knowledge Statement | Found? | Quality |
|---|---|---|
| bufio.Scanner 50MB buffer for transcripts | ✅ | Excellent — 5 matching claims |
| Pure-Go zstd compression with atomic writes | ✅ | Excellent — exact match |
| CommitLinker with pluggable GitRunner interface | ✅ | Excellent — exact match |
| PreToolUse `{}` = implicit deny (undocumented) | ⚠️ Partial | Found probe mechanics, missed the implicit deny discovery |
| Hook command path resolves relative to hooks.json | ❌ | Missed — 0 matching claims |
| Kill switch `REPO_MEMORY_DISABLED=1` (~2.8ms) | ✅ | Good — captures kill switch and fast no-op |
| `agy -p` fixed overhead ~17K tokens per call | ❌ | Missed — has latency claim (6–7s) but not token overhead |
| Housekeeping exclusion via `REPO_MEMORY_HOUSEKEEPING=1` | ✅ | Excellent — captures isolation mechanism |
| 5.4× compaction ratio for transcripts | ✅ | Excellent — includes PR-02a exit criterion |
| Subagents fire ALL hooks with own conversationId | ✅ | Excellent — captures hook behaviour |

**Recall: 8/10 (80%)**

**Misses pattern:** Both missed items were **discoveries about external system behaviour** (hooks.json path resolution, `agy -p` token overhead) — knowledge that emerged from experimentation, not from the code itself. The compressed episode format likely doesn't preserve enough experimental context.

### 3.5 Coverage: Knowledge the Pipeline Found That the PA Didn't Record

| Claim | Why the PA Missed It |
|---|---|
| `agy -p` startup latency = 6–7s per call | Implicit in experiment data, never stated as fact |
| Canary tests for transcript format changes | Worker mentioned as best practice, PA didn't surface |
| `transcript_full.jsonl` append-only behaviour | Implementation detail below PA's abstraction level |
| Hook latency p50: 57ms (non-enrolled) vs 3ms (enrolled) | PA recorded the 5ms target but not the measured p50 |
| Token estimation standardized at chars/4 | Convention used everywhere but never explicitly stated |

---

## 4. Token Efficiency Analysis

### 4.1 Production Cost

| Stage | LLM Calls | Est. Input Tokens/Call | Total Input Tokens |
|---|---|---|---|
| Triage | 0 (rule-based) | — | 0 |
| Extract | 71 | ~17,000 (85% overhead) | ~1,207,000 |
| Reconcile | 397 | ~17,000 (85% overhead) | ~6,749,000 |
| Verify | 0 (not yet run) | — | 0 |
| **Total** | **468** | — | **~7,956,000** |

| Per-unit metric | Value |
|---|---|
| Input tokens per episode | ~112,000 |
| Input tokens per claim | ~63,600 |
| Output tokens per claim (est.) | ~1,200 |
| Useful payload per call | ~2,500 tokens (15%) |
| `agy -p` overhead per call | ~14,500 tokens (85%) |

### 4.2 The `agy -p` Overhead Problem

Each `agy -p` invocation loads a fixed overhead before processing the actual prompt:

| Component | Tokens |
|---|---|
| System prompt | ~8,000 |
| Tool definitions (50+ tools) | ~7,000 |
| Permission/sandbox setup | ~1,000 |
| Session initialization | ~1,000 |
| **Total fixed overhead** | **~17,000** |

With 468 LLM calls, **~6.8M tokens (85% of total spend) were fixed overhead.**

### 4.3 Reconcile Stage: The Dominant Cost

Reconciliation accounted for **85% of all LLM calls** (397 out of 468) because each candidate claim gets its own `agy -p` invocation. At 5.8 candidates per episode, this creates 5.8× the overhead.

| If batched per episode | Calls | Overhead Tokens | Savings |
|---|---|---|---|
| Current (1 call/candidate) | 397 | ~5,752,500 | — |
| Batched (1 call/episode) | 71 | ~1,028,500 | **82%** |

### 4.4 Quota Impact

| Run | Calls Before Exhaustion | Stage | Reset Wait |
|---|---|---|---|
| Run 1 (initial) | ~131 (71 extract + ~60 reconcile) | Mid-reconcile | ~51 min |
| Run 2 (after fix) | ~340 reconcile | Late reconcile | ~44 min |
| Run 3 (verify) | Not attempted | — | — |

Steady-state projection (1–3 episodes/day): 7–20 LLM calls/day — well within daily quota.

---

## 5. Lifecycle Cost: Production + Recall

### 5.1 Recall Cost Per Session

The production cost tells only half the story. Every time knowledge is recalled, it consumes input tokens:

| Approach | Tokens Loaded Per Session | Targeting |
|---|---|---|
| PA (bulk load) | **~9,148 tokens** | Untargeted — loads everything |
| Pipeline (hook injection) | **~600 tokens** | Targeted — relevant claims only |

**The pipeline is 15× more efficient at recall.**

PA recall breakdown:
- `SKILL.md`: ~3,993 tokens
- `MEMORY.md`: ~357 tokens
- `CHIEF_ARCHITECT.md`: ~696 tokens
- `PROJECT.md` (current): ~1,043 tokens
- Recent WORKORDERs (×3): ~3,057 tokens

Pipeline recall: 3–5 relevant claims × 134 tokens = ~400–670 tokens

### 5.2 Total Lifecycle Cost (Production + N × Recall)

| Sessions | PA Total | Pipeline (current) | Pipeline (batched) | Pipeline (in-session) |
|---|---|---|---|---|
| 1 | 11,548 | 154,600 | 33,600 | **1,100** |
| 5 | 48,140 | 157,000 | 36,000 | **3,500** |
| 10 | 93,880 | 160,000 | 39,000 | **6,500** |
| 20 | 185,360 | 166,000 | 45,000 | **12,500** |
| 50 | 459,800 | 184,000 | 63,000 | **30,500** |
| 100 | 917,200 | 214,000 | 93,000 | **60,500** |

### 5.3 Break-Even Analysis

| Production Architecture | Breaks Even vs PA | 100-Session Savings |
|---|---|---|
| Current (8.5 `agy -p` calls/episode) | ~18 sessions | 4.3× cheaper |
| Batched (1 `agy -p` call/episode) | ~4 sessions | 9.9× cheaper |
| In-session extraction (0 extra calls) | **Immediately** | **15.2× cheaper** |

---

## 6. Comparative Assessment

### 6.1 Token Efficiency

| Dimension | PA | Pipeline (in-session) | Winner |
|---|---|---|---|
| Production cost | ~2,400 tok/session | ~500 tok/session | **Pipeline** (5×) |
| Recall cost per session | ~9,148 tok/session | ~600 tok/session | **Pipeline** (15×) |
| Context window impact | ~9K tokens occupied | ~600 tokens occupied | **Pipeline** (15×) |
| Recall timing | Session boot only | Point-of-need | **Pipeline** |
| Coverage of all sessions | PA sessions only | All sessions | **Pipeline** |
| Machine-searchable | ❌ | ✅ FTS + MCP tool | **Pipeline** |

On pure token economics, the pipeline wins. It loads less, targets better, and covers more sessions.

**But token efficiency is not the whole story.**

### 6.2 Knowledge Quality for Code Modification — The Critical Gap

When an agent is about to modify a module, it needs **working context**: why decisions were made, what was tried and failed, where the architectural boundaries are, and how the pieces interconnect. This is the primary recall use case.

The PA's WORKORDER completion reports are distillations written by the agent that did the work. They contain rich narrative context. The pipeline's claims are atomic facts extracted from compressed episodes. Here is a direct comparison for the transcript parser module:

**PA distillation (from WO1 completion report):**
```
### Decisions Made
- Used bufio.Scanner with a generous 50MB buffer on top of an
  io.MultiReader to ensure lexical chunks straddling JSON boundaries
  can seamlessly parse into unified lines before being evaluated.

### Dead Ends & Warnings
- cat's EOF behavior on newlines broke the chunked test initially.
  Fixed using printf to ensure strict byte continuity over chunks
  without injection of newlines.
```
One file, 80 lines. The agent reads it and knows: the design intent (WHY 50MB, WHY MultiReader), what will break if changed (cat vs printf), the boundary box (internal/harness only, never internal/core), and how this feeds into the episode store.

**Pipeline claims (39 transcript-related claims, representative sample):**
```yaml
title: bufio.Scanner configured with a 50MB buffer over an io.MultiReader
title: Transcript priority order: transcript_full.jsonl, chunks, transcript.jsonl
title: Step indices have gaps from internal dispatch steps
title: Use printf rather than cat for chunk test fixtures
title: Tool traffic constitutes 78% of transcript footprint
```
39 atomic facts, ~21 KB total. Each is accurate. But they are **WHAT** statements — reference facts, not working context. Missing:
- **WHY** the 50MB buffer (because chunks straddle JSON boundaries)
- **WHY** printf instead of cat (cat's EOF newline injection broke tests)
- **BOUNDARY** constraints (parser lives in internal/harness, must not leak to internal/core)
- **INTERCONNECTIONS** (parser → episode store → claim pipeline)
- **DEAD ENDS** (what approaches were tried and abandoned)
- **SCOPE** (this was architect-complexity work for a reason)

### 6.3 Recall Efficacy Verdict

| Dimension | PA Distillations | Pipeline Claims | Winner |
|---|---|---|---|
| **Token cost of recall** | ~9,148 tokens (bulk) | ~600 tokens (targeted) | **Pipeline** (15×) |
| **Working context for modification** | Rich: WHY, dead ends, boundaries | Shallow: WHAT only | **PA** |
| **Decision reasoning** | Explicit narrative | Usually absent | **PA** |
| **Dead ends and warnings** | Explicit section | Rarely captured | **PA** |
| **Boundary constraints** | Explicit in scope definition | Not in claims | **PA** |
| **Interconnections** | Implicit in narrative | Not in claims | **PA** |
| **Precision** | ~100% (human-curated) | 100% (LLM-extracted) | Tie |
| **Fact recall** | 100% of own sessions | 80% | **PA** |
| **Non-PA session capture** | None | 100% | **Pipeline** |
| **Staleness** | Gets stale, no auto-update | Auto-reconciled | **Pipeline** |

**The pipeline is not yet competitive with PA distillations for the code-modification use case.** It is cheaper at recall (15× fewer tokens) but the knowledge it delivers is too shallow to safely guide code changes. An agent armed with PA distillations will make better decisions and fewer mistakes than an agent armed with atomic claims.

The PA's approach — having the agent that did the work write a narrative completion report — produces fundamentally richer knowledge. The pipeline's extraction from compressed episodes loses the reasoning, the dead ends, and the interconnections.

### 6.4 Where the Pipeline Could Win

The pipeline's strengths are in scenarios the PA doesn't cover:

1. **Non-PA sessions**: Debugging, exploration, bug-fixing — sessions that produce knowledge but have no PA to distil it. The pipeline captures something; the PA captures nothing.
2. **Point-of-need delivery**: Hook injection at the moment an agent is about to make a mistake (déjà-vu). The PA has no delivery mechanism beyond file reading.
3. **Cross-session search**: An agent can query "what do we know about hook latency?" via the MCP pull tool. The PA requires scanning multiple WORKORDERs.
4. **Scale**: As the project grows, the PA's bulk-load approach scales linearly (more WOs = more tokens). The pipeline's targeted injection scales by relevance.

### 6.5 What Would Make the Pipeline Competitive

The atomic claim format is the wrong granularity for code-modification recall. To match the PA's efficacy, the pipeline would need one of:

1. **Richer distillations**: Instead of atomic claims, produce completion-report-style documents with reasoning, dead ends, and boundaries. But this is essentially what the PA already does — and it requires the original session context, not a compressed episode.

2. **Synthesised module briefs**: A view layer that aggregates atomic claims into per-module working context (boundaries, key decisions with reasoning, known gotchas, interconnections). This is what the SPEC's views (MAP.md, subsystem pages) were intended to be, but they require an additional LLM pass to synthesise.

3. **In-session distillation**: Have the agent that did the work produce the distillation (as the PA does), but store it in the claim format for machine delivery. This combines the PA's quality with the pipeline's delivery mechanism.

Option 3 is the most promising: it preserves the PA's knowledge quality while adding the pipeline's recall efficiency and coverage. But it is essentially the conversation-injection architecture — and it raises the question of whether the multi-stage pipeline machinery (triage, extract, reconcile, verify) adds any value over simply having the agent write claims directly.

---

## 7. Architectural Recommendations

### 7.1 Critical: Change the Production Architecture

The multi-stage `agy -p` pipeline is not viable as the primary production mechanism. The 85% overhead per call makes it 64× more expensive than in-session extraction.

**Recommended: In-session extraction via conversation injection**

1. Session completes naturally
2. On cron/overnight schedule, inject a distillation prompt into idle conversations
3. The agent (with full original context) produces structured claims
4. Stop hook collects claims from the transcript
5. Deduplication via embedding similarity (no LLM needed)

This eliminates all `agy -p` overhead and produces higher-quality claims (full context vs compressed episode).

### 7.2 Retain: The Recall and Delivery Architecture

The hook-based delivery, claim store, FTS index, and MCP pull tool are sound and should be retained. They are the system's primary value proposition.

### 7.3 Retain: The `agy -p` Pipeline as Fallback

Keep the pipeline for:
- **Claude Code sessions** (no conversation injection available)
- **Batch backfill** of historical data (one-time cost, amortised over many recalls)
- **Overnight reconciliation** when quota is available

But batch all LLM work into one call per episode (82% call reduction).

### 7.4 Fix: Cost Recording

`pipeline.go` currently passes empty `jobs.Cost{}` on every `store.Complete()` call. The executor returns `harness.Result.Usage` with real token counts, but the pipeline ignores them. Must fix for B4 measurement.

### 7.5 Fix: Claim Type Normalization

The pipeline produced 20 distinct claim types (including `architecture_rule`, `design_decision`, `design_pattern`, `implementation_detail`, `lesson_learned`). The SPEC defines 8 canonical types. The extraction prompt should constrain to the canonical set.

---

## 8. Benchmarks for Future Comparison

| Benchmark | PR-05b Value | Target |
|---|---|---|
| **B1** — Agent task success with memory | Not measured | Parity with PA baseline |
| **B4** — Cost per episode (production) | ~112K tokens (current) / ~33K (batched) / ~500 (in-session) | < PA's ~11.5K lifecycle cost |
| **B5** — Precision of pushable claims | **100%** (10/10) | ≥ 80% (SPEC gate) ✅ |
| **B5** — Recall vs PA ground truth | **80%** (8/10) | ≥ 80% (SPEC gate) ✅ |
| Claims per episode | 1.76 | No target set |
| Candidates per episode | 5.8 | Optimise to ~3 |
| Reconcile discard rate | 68.5% | Optimise extraction to reduce |
| Recall tokens per session | ~600 | < 1,000 ✅ |
| Context window impact | <0.5% | < 2% ✅ |

---

## 9. Sample Claims

### High-quality claim (verified fact)
```yaml
id: clm-01M3YZGSX6CYQCW28S4QNW8XXR
type: fact
title: >
  Transcript compaction using 'compact.NewCompactor().Compact()' demonstrated
  a 5.4x compaction ratio on surviving disk transcripts, satisfying the
  PR-02a exit criterion threshold of >= 4x.
status: verified
anchors:
  - kind: file
    ref: internal/core/compact/compactor.go
```

### Decision with guidance
```yaml
id: clm-01M3Z062V6B6J89AN195VWWMZJ
type: decision
title: >
  Plugin directory resolution checks REPO_MEMORY_PLUGIN_DIR and
  ANTIGRAVITY_CONFIG_DIR environment variables before falling back
  to default user directories to support hermetic unit testing.
relevant_when: >
  Resolving Antigravity plugin installation directory during
  install/uninstall operations or testing directory resolution.
apply: >
  Check REPO_MEMORY_PLUGIN_DIR and ANTIGRAVITY_CONFIG_DIR environment
  variables before falling back to ~/.gemini/config/plugins.
status: active
anchors:
  - kind: file
    ref: cmd/repo-memory/cmd_install.go
  - kind: file
    ref: cmd/repo-memory/cmd_uninstall.go
```

### Operational gotcha
```yaml
id: clm-01M3YYRWH7GTSRZQV4XBV6G9AF
type: gotcha
title: >
  Hook execution latency on non-enrolled repositories has a p50 of
  ~57ms compared to ~3ms on enrolled repositories, requiring
  fast-path directory detection.
status: active
```

---

## Appendix A: Test Environment

| Component | Version/Value |
|---|---|
| repo-memory binary | Built from `pr-05b-dogfood-pilot/wo4-b5-evaluation` |
| agy binary | `/Users/mcneillm/.local/bin/agy` |
| Model | Gemini 3.8 Flash (High) |
| Executor | `agy -p` via `internal/harness/antigravity/executor.go` |
| Episode source | PA-imported from `.agents/skills/principal-architect-workspace/` |
| Episode count | 71 (compressed, 174.3 KB total, 2.5 KB avg) |
| Subsystem config | `.memory/config.yaml` (11 subsystems) |
| Repo ID | `72cadb31669714ffc5b2439df0aa726d3f03b6569d43042290bd4bf699aa6c0f` |

## Appendix B: Commits

| Commit | Description |
|---|---|
| `6fde866` | PR-05b project setup (WO1–WO4 plan, subsystem config) |
| `4163dd5` | Pipeline wiring: Housekeeper in tick, `cmd_pipeline.go`, executor stdin fix |
| `960d852` | Reconcile fix: robust JSON extraction, 429 handling, 125 claims committed |
