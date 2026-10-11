# Critical Evaluation: SPEC.md vs. Original Analysis

This evaluates the [SPEC.md](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/SPEC.md) alternative design against the [original analysis](file:///Users/mcneillm/.gemini/antigravity/brain/77465b32-05f6-4ea4-81d4-bfc2321d9f99/repo-memory-plugin-architecture-analysis.md) and its inline review comments, grounded in the actual [Antigravity customization system](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/SKILL.md) documentation.

---

## 1. Verdict Summary

The SPEC is a **substantially better design** than the original analysis. It fixes most of the genuine errors the inline comments identified, and it introduces several ideas that are architecturally superior. But it also introduces new risks and makes some choices that deserve scrutiny.

| Dimension | Original Analysis | SPEC.md | Winner |
|-----------|------------------|---------|--------|
| Accuracy of Antigravity hook usage | Multiple errors (see §2) | Correct against documented API | **SPEC** |
| Dream-RSI interpretation | Overextended mapping | Narrowly adopted where valid | **SPEC** |
| Token accounting honesty | Omitted major costs | Acknowledges unknowns, defers to measurement (B4) | **SPEC** |
| Decay/validity model | Usage-based decay (problematic) | Dependency-triggered re-check (sound) | **SPEC** |
| Extraction model | Working agent writes cards (conflicting incentives) | Sidecar extracts (clean separation) | **SPEC** |
| Data model rigour | Experience cards with ad hoc YAML | Typed claims with evidence chain and trust ladder | **SPEC** |
| Practical buildability | Moderate complexity, hand-waved | High complexity, 6 build phases | **Draw** — see §6 |
| Security model | Contradictory (auto-commit + quarantine) | Consistent (memory PR, scrubbing, never-on-working-branch) | **SPEC** |
| PA comparison fairness | Straw-manned the PA | Treats PA as complement, not rival | **SPEC** |

---

## 2. Where the Inline Comments Are Correct

The reviewer's criticisms of the original analysis are **almost entirely valid** when checked against the actual Antigravity documentation:

### 2.1 Hook API Errors in the Original

| Original Claim | Reality ([hooks.md](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md)) | SPEC Fix |
|----------------|---------|----------|
| "session_start hook" | No such event. The five events are `PreToolUse`, `PostToolUse`, `PreInvocation`, `PostInvocation`, `Stop`. | ✅ SPEC §12.1 uses the correct event names |
| "pre_tool_call hook can validate and inject memory" | `PreToolUse` can return `allow`/`deny`/`ask`/`force_ask` + a `reason`, plus `overwrite` args. It **cannot** inject ephemeral messages or memory content. | ✅ SPEC §12.1 correctly notes `PreToolUse` can only deny with reason, uses `PreInvocation` for injection |
| "session_end hook triggers extraction" | No `SessionEnd` event. `Stop` fires when the loop terminates. | ✅ SPEC uses `Stop` for capture |
| "PreInvocation fires once at session start" | `PreInvocation` fires **before every model call**, not once. It receives `invocationNum`. | ✅ SPEC §12.1 gates on `invocationNum == 0` for task brief, uses every call for touch/error/déjà-vu |
| "Hooks can inject unlimited context" | Hooks inject via `ephemeralMessage` / `userMessage` / `toolCall`, which consume the same context window. | ✅ SPEC §7.1 sets explicit per-channel and per-session token budgets |

**Verdict**: The original analysis had a flawed mental model of the Antigravity hooks API. The SPEC's §12.1 is well-grounded.

### 2.2 Dream-RSI Overreach

The reviewer correctly identifies that the original analysis's Dream-RSI mapping is largely aspirational:

- **Discovery trees ≠ transcripts**: Discovery trees have parallel branches scored by a fixed evaluator. Session transcripts are linear and have no objective score function. ✅ Valid criticism.
- **"Evolving worlds" mismatch**: In Dream-RSI, worlds *grow* but the task is fixed. An evolving codebase is the non-stationarity that replay *assumes away*. ✅ Valid.
- **Counterfactual replay unsupported**: Dream-RSI's replay reveals only recorded outcomes. Injecting a memory card is an unrecorded intervention — you can't know what would have happened. ✅ Valid.
- **Section 5.1 misread**: The paper compares directional guidance vs. no guidance on one kernel task. It doesn't study retrieval at all. ✅ Valid.

The SPEC adopts only what genuinely transfers: failure triage taxonomy, prefix-only observability for tuning, and in-sample no-regret policy selection (§10.2, Appendix B). This is disciplined.

### 2.3 Token Accounting

The reviewer correctly identifies omitted costs: MCP tool schemas, per-invocation hook processing, stop-hook distillation in the main session, and nightly transcript re-reading. The SPEC acknowledges this honestly (Q7, B4) and shifts the question to measurement rather than claiming savings.

### 2.4 Decay Model

The reviewer's strongest point: **decay by disuse forgets stable knowledge**. A subsystem nobody has touched in a year is exactly the one where you most need the rationale. Usage-based confidence is also self-reinforcing (injected → "used" → higher confidence → promoted to boot → injected more), which is the same shape as bootstrap poisoning.

The SPEC's dependency-triggered re-check (§8) is fundamentally sound: a claim is valid until something it depends on changes. Period.

---

## 3. Where the SPEC Genuinely Innovates

### 3.1 The Claim as Unit of Knowledge (P4)

The original's "experience card" is a blob of markdown with ad hoc YAML metadata. The SPEC's claim model is more rigorous:

- **One file per claim** — merge-safe, independently reviewable
- **Typed** (invariant, decision, ruled-out, failure-signature, procedure, fact, convention, open-issue) — each type has different delivery triggers
- **Evidence-backed** — every claim links to episode steps, commits, or human reviews
- **Anchored to code** — claims reference entities (symbols, files, configs, subsystems) with fingerprints
- **Trust ladder** — hypothesis → observed → verified → merged → confirmed, set only by evidence and humans, never by retrieval

This is a real improvement. It solves the "confidence: 0.92" false precision problem and the "last_used / use_count creates git diffs on every retrieval" problem.

### 3.2 Sidecar Architecture (G1, P1)

The principle "the working agent writes no memory" (G1) eliminates the conflicting incentive where a Stop hook forces the agent to write a card before it can exit (leading to low-value cards written just to satisfy the gate).

Separation of concerns:
- **Capture**: deterministic hooks (no LLM, ≤200ms) — record what happened
- **Distillation**: background sidecar (small models) — extract what it means
- **Delivery**: deterministic hooks + MCP tools — surface what's needed

This is architecturally cleaner than the original's hybrid (synchronous agent distillation + async dreaming).

### 3.3 Event-Sourced Knowledge (P2)

```
claims = project(pipeline_version, events)
```

This is a powerful idea the original lacks entirely. If the extraction pipeline improves, you re-run it over the full event history and get better claims everywhere, not just going forward. Human review decisions survive re-projection because they're keyed by claim ID + content hash.

The trade-off is that it requires keeping compacted episodes indefinitely (private, per-machine). At the stated sizes (~47k tokens median per session), this is feasible.

### 3.4 Branch Scoping (§6.7)

The reviewer's inline comment on the original correctly identified that dead-ends discovered on abandoned branches die with them. The SPEC's branch-scoping model handles this:

- Claims from unmerged branches are scoped to that branch
- On merge, scope moves to `main`
- On branch abandonment, `ruled-out` and `failure-signature` claims are promoted to `main` (preserving the knowledge) while other claims are retired

This is the right answer. It's a novel contribution specific to git-backed memory.

### 3.5 Failure Triage (§6.5, from Dream-RSI)

This is where Dream-RSI genuinely helps. Classifying each failed attempt as:
- `approach-unrecoverable` → becomes a `ruled-out` claim
- `implementation-bug` → at most a `failure-signature`
- `environment` → noted but not a fundamental insight
- `insufficient-evidence` → hypothesis only

This prevents the system from permanently banning a valid design approach because of a trivial implementation bug. It's the right taxonomy.

### 3.6 Push Delivery Model (§7)

The original's "inject top-k abstracts" treats all sessions equally. The SPEC's trigger-based delivery is more nuanced:

- **Task start**: system map + task brief (subsystem-aware, budget-capped)
- **File touch**: claims anchored to touched entities
- **Error**: failure-signature matching on tool output
- **Risky action**: procedures/invariants before dangerous tool calls
- **Déjà-vu**: live comparison against ruled-out claims

Each trigger has an explicit token budget and per-session deduplication. This is more sophisticated and more token-efficient than "retrieve top-k on the opening prompt."

---

## 4. Where the SPEC Has Problems

### 4.1 Complexity and Buildability

The SPEC defines a 6-stage pipeline (Compact → Triage → Extract → Reconcile → Verify → Publish), 5 build phases, 8 claim types, 7 status levels, 3 flag types, 6 relation types, 6 view types, 5 delivery triggers, and 5 pull tools. It's a full knowledge management system.

The original analysis was over-simplified. The SPEC may be over-engineered for v1.

**Key risk**: the value of the system depends on extraction quality (Q1). If small models over-extract or mis-classify, the entire downstream pipeline (reconcile, verify, publish, deliver) propagates errors. The SPEC acknowledges this (Q1) but doesn't have a fallback if extraction precision is low.

> [!WARNING]
> Phase P0 (Capture) produces value immediately — you can at least read episodes later. But Phases P1-P2 (Knowledge + Delivery) are an all-or-nothing bet on extraction quality. Consider a thinner P1 that starts with `failure-signature` claims only (deterministic matching, highest value) before attempting the full claim taxonomy.

### 4.2 Sidecar as a Daemon

The SPEC requires a **persistent background daemon** ("single binary or Python package per machine") with a Unix socket API. This is a significant operational burden compared to "hooks + scripts + scheduled agent tasks."

- It must be running for memory to work
- It must be started/stopped/monitored
- It's another process to debug when things go wrong
- It adds a hard dependency on a local service

The Antigravity ecosystem has no built-in sidecar/daemon management. You'd need to wrap it in launchd (macOS) or systemd (Linux), or run it as a long-lived shell process — none of which are first-class in the plugin model.

> [!NOTE]
> The SPEC's degraded mode (§12.1 "Native alternative: Glob and Model Decision rules can serve subsystem pages without the sidecar") is important. The system should still provide value when the sidecar is down. But the SPEC underspecifies this — how much functionality degrades, and what the user experience looks like.

### 4.3 Memory PR Review Fatigue (Q8)

The SPEC routes all memory changes through a dedicated `memory/pending` branch with batched PRs. This is correct for security/governance but creates a human bottleneck:

- Every session potentially generates claim create/support/refine ops
- Active development could produce multiple claim ops per day
- Each requires review before it's visible on main

The SPEC acknowledges this (Q8) and proposes auto-approval for low-risk ops. But the default is auto-approval **off**, which means the system produces zero value until someone reviews the first PR.

> [!TIP]
> For solo developers or small teams, consider defaulting auto-approval **on** for `support` and `promote` ops on non-security claims. The security risk is low for these operations, and the value of getting claims into the shared store quickly outweighs the governance benefit.

### 4.4 Entity Extraction Without LLM

The SPEC claims entity extraction (subsystem mapping, file/symbol/config/topic/tool/error extraction) uses "no LLM." This is achievable for files, symbols, and configs via tree-sitter and config parsing. But:

- **Subsystem mapping** requires hand-written `.memory/config.yaml` globs — this is manual setup per repo
- **Topic extraction** ("EventBus `data.source.health`") is ambiguous without LLM classification
- **Error string extraction** from code ("No manifest URL configured") requires string literal scanning, which is noisy

The no-LLM constraint for entity extraction is the right default (fast, deterministic, cheap), but the SPEC should be explicit about what's approximated vs. exact.

### 4.5 Transcript Retention Problem

Appendix A.3 reveals a critical fact: **only 41 of 188 top-level Antigravity conversations remain on disk.** The SPEC correctly identifies this and makes P0 (capture at session end) the first priority.

But the SPEC's `Stop` hook has a gap: if the agent crashes, times out, or the user kills the session, `Stop` may not fire. The SPEC mentions checkpoints every 200 steps (§6.2), but the `PostToolUse` hook (which fires on every tool completion) is a more reliable checkpoint mechanism — and the SPEC already uses it for the outcome ledger.

### 4.6 The `ephemeralMessage` Unknown (Q5)

Both the SPEC and the hooks documentation leave `ephemeralMessage` semantics unclear. The [hooks doc](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md#L269) describes it as a "transient system message" but doesn't specify whether it persists in conversation history.

This matters enormously for token accounting:
- If ephemeral = not persisted → injection is "free" after the current turn (ideal for memory)
- If ephemeral = persisted → it accumulates in the context window across turns (expensive)

The SPEC's approach (dedupe via delivery ledger regardless) is the right defensive strategy.

---

## 5. Evaluating Specific SPEC Decisions Against AGY Architecture

### 5.1 Plugin Location: Global vs. Per-Repo

The SPEC places the plugin globally (`~/.gemini/config/plugins/repo-memory/`) with per-repo data in `.memory/`. The original placed the plugin per-repo (`.agents/plugins/repo-memory/`).

**SPEC is correct.** Per the [plugins guide](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/plugins.md), plugins are discovered at both workspace and global levels. Code (hooks, scripts, MCP server) should be global (one installation). Data (claims, views, config) should be per-repo (committed to git). This avoids duplicating plugin code across every repository.

### 5.2 Always-On Rule for System Map

The SPEC proposes using an Always-On rule (`rules/memory-map.md`) that points at `.memory/views/MAP.md`. Per the [rules doc](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/rules.md):

- Always-on rules count against the 20k-token aggregate budget
- The system map is ≤2k tokens
- If the rule simply contains the map content, it uses ~10% of the rules budget

This is reasonable. The `@[label](path)` include syntax could pull MAP.md content directly into the rule, but the 2k budget is small enough to be inline. The alternative (injecting via `PreInvocation` on `invocationNum == 0`) works too but adds hook complexity.

**Verdict**: Always-on rule is simpler and correct. The SPEC's choice is sound.

### 5.3 MCP Server for Pull Tools

The SPEC bundles an MCP server providing `memory.ask`, `memory.why`, `memory.history`, `memory.brief`, `memory.note`. Per the [MCP guide](file:///Users/mcneillm/.gemini/antigravity/builtin/skills/agy-customizations/docs/mcp_servers.md), tool schemas are loaded into every session's context.

With 5 tools, each with name + description + parameter schemas, this is probably ~500-1,000 tokens of permanent context overhead. The SPEC's own token budget tables (§7.1) don't account for this, and the reviewer correctly flagged it.

> [!WARNING]
> Consider starting with 2-3 tools (`memory.ask`, `memory.why`, `memory.note`) and adding the rest only if measurement shows the agent needs them. Each tool adds permanent per-session context cost. Alternatively, use lazy-loaded tools to minimize upfront schema injection.

### 5.4 Delivery via PreInvocation

The SPEC uses `PreInvocation` (every model call) to read new transcript steps, query the sidecar, and inject relevant claims. This is the correct hook for injection (it's the only hook that can return `injectSteps` → `ephemeralMessage` outside of `PostInvocation`).

However, the SPEC requires this hook to:
1. Read the transcript file to find new steps since last cursor
2. Parse those steps for file touches, errors, and tool calls
3. Query the sidecar over a Unix socket
4. Format and return injection content

All within the 30-second default timeout (target ≤150ms per §7.8).

This is feasible if the sidecar is running and the index is warm. But:
- First invocation after sidecar restart could be slow (cold index)
- Transcript parsing at high step counts could lag
- If the sidecar is down, the hook must no-op gracefully (the SPEC says this: "if the sidecar is down, adapters no-op")

**Verdict**: Architecturally correct. The 150ms p95 target is ambitious but achievable with a warm SQLite index and Unix socket. The no-op fallback is essential.

---

## 6. What's Missing from Both

### 6.1 Multi-Developer Scenarios

Both documents are written from a single-developer perspective. When multiple developers work on the same repo:

- Episodes are private per machine — developer A's debugging insights don't help developer B unless they survive to claims
- The memory PR becomes a shared review surface, but claim ops from different developers could conflict
- Subsystem coverage depends on who's working on what

The SPEC acknowledges this (Q3) but defers it. For a v1, this is acceptable.

### 6.2 Cost Modelling

Neither document provides a concrete cost model. The SPEC's B4 benchmark ("sidecar tokens vs. PA distillation tokens") is the right question, but the answer could go either way:

- **Sidecar reads more volume** (full compacted episodes, ~47k tokens median)
- **PA writes more expensive tokens** (frontier model output for manual distillation)
- The sidecar uses small models for most stages, stronger models only for reconcile

Until B4 is run, the cost advantage is an assumption.

### 6.3 Incremental Value Path

The SPEC's P0 (capture only) is genuinely valuable: you preserve transcripts that would otherwise be deleted (Appendix A.3). But the gap between P0 and P2 (when delivery starts) is significant.

Consider adding a **P0.5**: captured episodes are searchable via a simple CLI tool (grep + SQLite FTS), without the full claim pipeline. This gives immediate value from P0's capture and validates whether the captured episodes contain useful information before investing in the extraction pipeline.

---

## 7. Responses to Review Comments (2026-09-30)

### 7.1 "Could you not end up with millions of files?" (on one-file-per-claim)

No. The numbers don't support that fear.

**esp32-app-foundation** is one of the most active repos I've seen — 1,201 commits, 188 top-level agent sessions, 100 PA project folders in ~4 months. Running the estimates:

| Timeframe | Estimated claims | After supersession/refutation |
|-----------|-----------------|------------------------------|
| Current backfill (4 months) | ~350-460 | ~350-460 (nothing to prune yet) |
| 1 year | ~1,400 | ~1,000-1,200 |
| 5 years (active repo) | ~6,200 | ~2,500-3,700 (40-60% active) |

For context, the PA workspace *already* has **1,364 markdown files** (4.8 MB). The claim store at 5 years would be roughly **3,000 active files at ~3 MB** — smaller than what the PA already generates, and git handles directories of this scale without difficulty.

The "millions" scenario would require ~500 claims per day sustained for years. The SPEC's triage (Stage 2) and reconciliation (Stage 4, which merges duplicates and discards noise) are specifically designed to prevent this. The 20-candidate-per-episode hard cap (§6.9) is the backstop.

That said, **subdirectory sharding** is a sensible precaution for repos that run for many years. The SPEC could partition claims by subsystem (`claims/ble/clm-xxx.md`, `claims/ota/clm-yyy.md`) or by year, though flat is fine well past the scales we're discussing.

### 7.2 "Feels like a proof-of-concept — needs to supersede the PA model" (on P1-slim)

Fair point. The phasing I suggested was overly cautious and doesn't answer the right question. The goal isn't to prove that failure-signatures work in isolation — it's to build a system that *replaces* the PA's manual memory management with something better.

The SPEC's own §13 (Principal-architect integration) makes this explicit:

| Today (PA) | With repo memory |
|------------|-----------------|
| Worker writes Completion Report by hand | Sidecar generates episode summary |
| PA writes session logs into PROJECT.md | Knowledge goes to claims automatically |
| PA hand-writes "Framework Facts Workers Need" | WORKORDERs embed generated task briefs |
| PA maintains MEMORY.md, PROJECT_INDEX.md, MODULE_INDEX.md | Knowledge indexes replaced by generated views |

To supersede the PA model, you need the **full claim taxonomy** working — not just failure-signatures. A worker needs `invariant`, `decision`, `ruled-out`, `procedure`, and `convention` claims to do its job without the PA hand-writing a "facts workers need" section.

**Revised recommendation**: keep the SPEC's phasing as-is (P0→P1→P2→P3→P4→P5), but add a **P1 exit gate**: run benchmark B1 (the pr-225 "Framework Facts Workers Need" reproduction) at the end of P1. If the generated task brief is at parity with the PA's hand-written version in correctness and completeness, that's the signal that the claim store is ready to underpin the PA migration (P4).

The risk mitigation isn't "start with fewer claim types" — it's "validate extraction quality early with a concrete benchmark before building delivery on top of it."

### 7.3 Sidecar Portability (Windows + operational burden)

This is a genuine problem. Unix sockets don't exist on Windows, `launchd`/`systemd` aren't cross-platform, and a persistent daemon is operational overhead regardless of OS.

Three alternatives worth evaluating:

**Option A: Process-per-session (no daemon)**
- Hooks spawn a short-lived process for each operation (capture, query, inject)
- The process opens the SQLite index, does its work, and exits
- No daemon to manage, fully portable
- Trade-off: cold-start latency on each invocation (~200-500ms to open SQLite + FTS). Likely acceptable for the `Stop` hook (capture), marginal for the `PreInvocation` hook (injection needs ≤150ms p95)

**Option B: Lazy-started daemon**
- The first hook invocation starts the sidecar if it's not running
- Sidecar listens on TCP localhost (cross-platform) instead of Unix socket
- Sidecar auto-exits after N minutes of inactivity
- Trade-off: first-invocation latency; auto-exit detection; port allocation

**Option C: In-process MCP server (most promising)**
- The MCP server config already spawns a process for the memory server. That process *is* the sidecar — it stays alive for the session duration and handles both pull tools and push queries from hooks
- Capture and pipeline stages run as post-session batch jobs triggered by the `Stop` hook (a simple `nohup` / `Start-Process` that runs after the hook returns)
- Trade-off: MCP server process is tied to Antigravity's lifecycle, not independent. But that's arguably a feature — no orphaned daemon

**Option C is the most promising** for portability: the MCP server mechanism is already cross-platform (stdio transport), the agent harness manages its lifecycle, and it solves the "must be running" problem because Antigravity starts it on session begin. Pipeline stages (heavy LLM work) run as fire-and-forget background processes after session end.

This should be an open question in the SPEC (add to §16).

### 7.4 "Add all tools but instrument their usage — observability layer across the whole plugin"

Agreed. Optimising for fewer tools upfront is premature. The right approach is:

1. **Ship all 5 tools** (`memory.ask`, `memory.why`, `memory.history`, `memory.brief`, `memory.note`)
2. **Instrument everything** — the SPEC already defines per-session metrics (§10.1), but these should extend to a full observability layer:

| Signal | Where recorded |
|--------|---------------|
| Tool invocations (which tool, args, response size, latency) | Delivery ledger |
| Push injections (trigger type, claim IDs, token count) | Delivery ledger |
| Hook execution time (per event type, p50/p95/p99) | Sidecar logs |
| Pipeline stage cost (tokens in/out, model, latency, per episode) | Pipeline log |
| Claim lifecycle (created, supported, superseded, refuted — by type) | Claim store metadata |
| Agent reference rate (did the agent act on a delivered claim?) | Post-session episode analysis |
| Schema overhead (total MCP tool description tokens per session) | Delivery ledger |

The delivery ledger (§7.1) is already the right place for most of this. The addition is a **plugin-wide metrics store** (`.memory/.cache/metrics.jsonl` or similar, gitignored) that aggregates across sessions and powers a simple dashboard or CLI report.

This is also how you answer the cost question (B4) and the "should we lazy-load some tools?" question — with data, not guesses.

> [!TIP]
> Consider adding a `memory.stats` tool that returns the observability summary (claim counts by type/status, delivery stats for recent sessions, pipeline cost). Useful for both the developer and the agent to self-assess whether memory is helping.

### 7.5 PA Cost Model Nuance

The PA's cost structure is more layered than "frontier model writes memory":

| PA Activity | Model Tier | Token Type | Cost Weight |
|-------------|-----------|------------|-------------|
| MEMORY.md / PROJECT.md reads on boot | Frontier (PA session) | Input | Low per read, but every session |
| Distillation into SESSION LOG | Frontier (PA session) | Output | **High** — PA reasons about what to record |
| Worker Completion Reports | Flash/Sonnet (worker) | Output | Medium — worker writes structured report |
| "Framework Facts Workers Need" | Frontier (PA session) | Output | **High** — PA hand-writes per project |
| PROJECT_INDEX / MODULE_INDEX graduation | Frontier (PA session) | Output | Low — infrequent |
| Chief Architect programme logs | Frontier | Output | **High** — cross-project synthesis |

The expensive operations are frontier-model *output tokens* for distillation and synthesis. The SPEC's sidecar replaces these with small-model *input tokens* (reading compacted episodes) plus small-model *output tokens* (claim extraction).

The cost equation is roughly:

```
PA cost ≈ frontier_output_price × distillation_tokens_per_project × projects
SPEC cost ≈ small_input_price × episode_tokens × sessions
           + strong_output_price × reconcile_tokens × claims
```

Given that frontier output is ~5-15× the cost of small-model input, and that distillation reasoning is verbose (the PA spends many tokens *deciding* what to write), the SPEC should win on cost — but by how much depends on episode volume and extraction hit rate. B4 is the right benchmark, and it should break down cost by the PA tiers above, not just total spend.

---

## 8. Final Assessment

### The SPEC Is Right About

1. **Separation of working and remembering** (G1) — eliminates conflicting incentives
2. **Evidence-backed trust ladder** — replaces fake confidence scores
3. **Dependency-triggered validity** — replaces decay-by-disuse
4. **Branch scoping** — solves the abandoned-branch knowledge loss
5. **Failure triage taxonomy** — the genuine Dream-RSI contribution
6. **Memory PR governance** — consistent security model
7. **Antigravity hook usage** — accurate against the documented API
8. **Event-sourced knowledge** — enables pipeline improvement over time

### The SPEC Needs Scrutiny On

1. **Complexity** — 6-stage pipeline may be over-engineered for v1; consider starting with failure-signatures only
2. **Sidecar daemon** — operational burden; needs graceful degradation story
3. **Extraction quality** (Q1) — the entire system's value depends on this; no fallback if it's poor
4. **Memory PR review fatigue** — default auto-approval off means zero shared value until first review
5. **MCP tool schema cost** — 5 tools × every session; start smaller
6. **`ephemeralMessage` semantics** — unknown persistence behaviour; mitigated by ledger deduplication
7. **Cost** — sidecar may not be cheaper than PA distillation; measure first (B4)

### Recommendation

**Adopt the SPEC's architecture and phasing (P0→P5)** with these additions:

1. **Sidecar portability**: evaluate Option C (MCP server as the sidecar process, pipeline stages as fire-and-forget background jobs) to eliminate the daemon requirement and gain cross-platform portability. Add to SPEC §16.
2. **Observability layer**: instrument all tools, hooks, pipeline stages, and claim lifecycle from day one. Add a `memory.stats` pull tool. Store metrics in `.memory/.cache/metrics.jsonl` (gitignored). This is how B4 gets answered.
3. **P1 exit gate**: run benchmark B1 (pr-225 task brief reproduction) at the end of P1 to validate extraction quality before building delivery on top of it. The system must produce a generated task brief at parity with the PA's hand-written version to proceed to P2.
4. **Subsystem sharding**: partition `claims/` by subsystem from the start (`claims/<subsystem>/clm-xxx.md`). The numbers say flat is fine, but sharding is free to add now and painful to retrofit.
5. **Claim count in SPEC §16**: add the scale analysis (§7.1 above) and the 20-candidate-per-episode cap as the explicit answer to "how many files?"
