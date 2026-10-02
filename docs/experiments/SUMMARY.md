# Experiment Spike (E01–E12) Summary & Synthesis

> **Status:** Complete  
> **Date:** 2026-10-01  
> **Work Order:** WO4 — Spec Reconciliation  
> **Source Evidence:** `docs/experiments/E01-ephemeral-message.md` through `E12-transcript-file-set.md`, plus `E01-architect-brief.md`

---

## 1. Executive Summary Table

| Experiment | Status | Answer (1-line) | SPEC Sections Confirmed | SPEC Sections Changed |
|:---|:---:|:---|:---|:---|
| **E01** (`ephemeralMessage`) | `changed` | `ephemeralMessage` delivers via `{"injectSteps":[{"ephemeralMessage":"..."}]}` but is per-invocation transient only; `invocationNum` resets to 0 per prompt; malformed responses silently fail. | §7.1, §12.1.1 | §4.2, §7.1, §7.2, §12.1, §12.1.1, §16 (Q5) |
| **E02** (Include resolution) | `confirmed` | `@[label](path)` inlines content relative to `GEMINI.md` (repo root) into `user_rules`, refreshes per session, missing targets silently ignore, 24 KB / 20k token caps hold. | §4.2, §4.6, §7.2, §12.1, §12.1.1 | §12.1.1 |
| **E03** (Transcript currency) | `confirmed` | Zero lag at `PreInvocation` and `PreToolUse`; reasoning (`thinking` + tool call) is flushed to disk before `PreToolUse` fires; payload points to `transcript_full.jsonl`. | §4.1, §6.2, §7.6, §12.1, §12.1.1 | §12.1.1 |
| **E04** (Rule activation) | `changed` | Frontmatter key must be `trigger: model_decision` (`activation:` is silently ignored); activation is pull-based via conditional rule pointers costing 1 tool call (`view_file`). | §7.9, §12.1, §12.1.1 | §7.9, §12.1, §12.1.1 |
| **E05** (Hook latency) | `changed` | Enrolled repos pass easily (p50 ~3ms, p95 ~6.1ms ≤ 60ms) and kill switch is fast (~2.8ms), but non-enrolled repos take ~57ms (failing ≤ 5ms target); requires cached registry or fast marker. | §7.8, §18.5 | §4.6, §7.8, §12.1.1, §16 (Q11, Q24), §18.5 |
| **E06** (Sidecar behavior) | `changed` | Sidecars are IDE-only (managed by IDE language server; CLI does not tick); 60s ticking is rock-solid in IDE with ~592ms startup jitter; tick command overlap possible; log grows ~14 MB/mo. | §4.3, §12.1 | §4.1, §7.9, §12.1, §12.1.1, §16 (Q12) |
| **E07** (Housekeeping isolation) | `changed` | `--project` flag is silently ignored by `agy -p` (all CLI calls share `default-cli-project`); isolation relies on dedicated run dir and `REPO_MEMORY_HOUSEKEEPING=1`; IDE and CLI DBs are fully separate. | §4.4, §4.4.1 | §4.4, §6.2, §12.1.1 |
| **E08** (Subagent hooks) | `changed` | Subagents fire all 4 lifecycle hooks (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`) with unique `conversationId` and `transcriptPath`; returning `{}` for `PreToolUse` is an implicit deny. | §4.1, §4.2 | §4.2, §7.2, §12.1, §12.1.1, §13, §16 (Q15) |
| **E09** (Deny behavior) | `changed` | Deny+reason delivers verbatim; parallel tool call denials are isolated; advisory wording prompts retries vs policy wording prompts pivot; `overwrite` enables non-blocking rewrites; hook exit != 0 is implicit deny. | §7.5, §7.6 | §7.5, §12.1, §12.1.1, §18.5 |
| **E10** (Overhead reduction) | `changed` | Fixed overhead is irreducible via CLI flags (~16.8k tokens for Gemini 3.8 Flash), but Gemini 3.6 Flash uses ~9.9k tokens (41% lower); no prefix caching across fresh CLI calls; 10–14s wall-clock latency per call. | §4.4, §4.4.1 | §4.4.1, §6.9, §12.1.1, §16 (Q14) |
| **E11** (Quota & rate limits) | `confirmed` | Zero rate limits observed across 25+ calls in 15m; CLI wall-clock latency (~11s) acts as natural rate limiter; subscription is Pro-tier personal OAuth; long-term tracking continues in PR-05b. | §4.4, §6.9 | §12.1.1 |
| **E12** (Transcript file set) | `changed` | Append-only growth confirmed via MD5 prefix matching; step-index gaps are unwritten internal dispatch steps; parallel tool calls produce out-of-order step indices in JSONL; session resets restart index at 0. | §6.2 | §5.1, §6.2, §12.1.1 |

*Categorization Legend:*  
- `confirmed`: SPEC assumptions and mechanisms are correct as-is.  
- `changed`: SPEC requires a functional, architectural, or parameter modification based on experimental findings.  
- `new-info`: Findings that reveal previously undocumented runtime behaviors which the SPEC must incorporate.

---

## 2. In-Depth Experiment Analysis

### E01: `ephemeralMessage` Persistence
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E01-ephemeral-message.md`, `docs/experiments/E01-architect-brief.md`
- **Core Findings:**
  1. `ephemeralMessage` is delivered to the model only when returned inside `{"injectSteps": [{"ephemeralMessage": "..."}]}`. A top-level key like `{"ephemeral_message": "..."}` is **silently ignored** without errors.
  2. Persistence is **strictly per-invocation**. In conversation `e00ea91f`, a marker injected at `invocationNum=1` was recorded in the transcript (`type=EPHEMERAL_MESSAGE`, `source=SYSTEM_SDK`) but was completely invisible to the model on subsequent invocations within the exact same prompt cycle.
  3. `invocationNum` resets to 0 on **every user prompt**, not once per session.
  4. Injecting bulk context (such as a 1–4k token task brief) via `ephemeralMessage` at `invocationNum=0` means the model loses the brief immediately on the second invocation (e.g. after a tool call returns). If re-injected on every invocation, token cost explodes (4–20k+ wasted tokens per prompt).
- **SPEC Impact:**
  - Cap `ephemeralMessage` payloads at ≤ 200 tokens (for sub-turn nudges, error alerts, and déjà-vu warnings only).
  - Bulk context (task brief) must use persistent injection (`userMessage`, subject to follow-up E1b) or pull delivery (MCP `memory.brief`).
  - Document the exact schema requirement `{"injectSteps": [{"ephemeralMessage": "..."}]}` in §12.1.

### E02: Include Resolution and Refresh
- **Categorization:** `confirmed`
- **Report Reference:** `docs/experiments/E02-include-resolution.md`
- **Core Findings:**
  1. `@[label](path)` includes in `GEMINI.md` inline target file contents into `<RULE[...]>` blocks in the system prompt.
  2. Paths resolve **relative to `GEMINI.md`'s directory** (the repository root).
  3. Content refreshes cleanly on every **new session** without needing an IDE or daemon restart. Mid-session hot reload does not occur.
  4. Missing include targets are **silently ignored** (the directive tag remains in the text, but no content is injected and no error is raised).
  5. Documented limits confirmed: 24 KB per file post-expansion (truncated on line boundaries), 20,000 tokens aggregate budget for always-on rules (rules exceeding budget demote to file path pointers).
- **SPEC Impact:**
  - Validates §7.2 always-on system map delivery via `@[memory map](.memory/views/MAP.md)`.
  - Validate file existence before adding include references to avoid silent failures.

### E03: Transcript Currency at Hooks
- **Categorization:** `confirmed`
- **Report Reference:** `docs/experiments/E03-transcript-currency.md`
- **Core Findings:**
  1. **Zero lag at `PreInvocation`:** `initialNumSteps` in the hook payload matches the transcript file's line count exactly at the moment the hook executes.
  2. **Zero lag at `PreToolUse`:** The `PLANNER_RESPONSE` step that generated the proposed tool call—including model `thinking` and proposed tool arguments—is flushed to disk before `PreToolUse` fires.
  3. The `transcriptPath` in payloads explicitly points to `transcript_full.jsonl` (untruncated), not the compact version.
  4. The Stop hook payload provides rich lifecycle status: `terminationReason`, `fullyIdle`, `executionNum`, and `error`.
- **SPEC Impact:**
  - Confirms §7.6 tier-1 déjà-vu inspection of current-turn reasoning in `PreToolUse`.
  - Confirms §6.2 capture safety relying on `transcript_full.jsonl`.

### E04: `model_decision` Rule Activation
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E04-rule-activation.md`
- **Core Findings:**
  1. Rule frontmatter MUST use `trigger: model_decision`. Using `activation: model_decision` causes the rule to be **silently ignored** without error or warning.
  2. Mechanism is **PULL-based**, not push. The rule description appears in the system prompt as a conditional rule pointer (`- file:///path: description`). When the prompt matches, the model emits a `view_file` tool call to read the rule file.
  3. Unrelated prompts do not trigger the rule read (clean negative control).
  4. Pulling conditional rules consumes 1 tool call turn before real work begins.
- **SPEC Impact:**
  - Update all references in §7.9, §12.1 to specify `trigger: model_decision`.
  - Account for the 1-tool-call latency cost when using the native-floor fallback.

### E05: Hook Latency (Probe Estimate)
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E05-hook-latency.md`
- **Core Findings:**
  1. Enrolled repos are very fast: p50 ~3.0ms, p95 ~6.1ms, easily satisfying the p95 ≤ 60ms fast-path budget.
  2. The globally disabled kill switch (`REPO_MEMORY_DISABLED=1`) returns in ~2.8ms (p95 ~4.1ms), meeting the ≤ 5ms fast-exit target.
  3. Returning injection payloads adds virtually zero overhead (<0.1ms; total p95 ≤ 3.0ms vs ≤ 150ms budget).
  4. **Critical failure:** Non-enrolled repositories consistently take **~55–57ms** (p95 ~59ms), failing the ≤ 5ms target by 10x. This is due to disk/registry lookup before checking enrollment status.
- **SPEC Impact:**
  - The hook dispatch architecture must be optimized for non-enrolled workspaces: check a lightweight in-memory cache, an environment variable, or a local marker file (`.memory/`) before reading the machine registry.
  - Update §4.6 and §7.8 performance expectations.

### E06: Sidecar Ticking Behavior
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E06-sidecar-behavior.md`
- **Core Findings:**
  1. Sidecars are **IDE-only infrastructure**. The scheduler is spawned as a child process of the IDE's language server (`/Applications/Antigravity.app/.../language_server multicall schedule`). The CLI (`agy`) does not run or manage sidecars.
  2. Ticking is perfectly regular while the IDE is running: 119 ticks in 2 hours with 60.0s intervals, zero missed ticks.
  3. Scheduler uses a fixed startup jitter (~592ms, resulting in a consistent +26s execution second).
  4. The scheduler schedules the next execution *before* the current tick command finishes, proving that overlapping executions can occur if a tick takes >60s. This validates the absolute necessity of `tick.lock`.
  5. Log growth is ~320 bytes/tick (~14 MB/month), requiring automated log pruning or rotation.
- **SPEC Impact:**
  - Clarify in §4.1 and §12.1 that scheduled sidecars run only when the Antigravity IDE is open. CLI-only environments require the OS scheduler fallback.
  - Emphasize `tick.lock` and budget constraints to prevent sidecar overlap.
  - Add log rotation for `sidecar_data/<id>/logs/`.

### E07: Housekeeping Run Isolation
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E07-housekeeping-isolation.md`
- **Core Findings:**
  1. The `--project <name>` flag is **silently ignored** by `agy -p`. All CLI runs are assigned `project_id = "default-cli-project"`.
  2. CLI and IDE maintain completely separated databases (`~/.gemini/antigravity-cli/` vs `~/.gemini/antigravity/`). Housekeeping conversations NEVER appear in the IDE history UI or IDE database.
  3. `--continue` in the repo directory resumes the most recent conversation in `default-cli-project`. If a housekeeping call was the most recent CLI invocation, `--continue` could accidentally resume it.
  4. Fixed token overhead per housekeeping call is 16,814 input tokens.
- **SPEC Impact:**
  - Remove reliance on `--project` for isolation in §4.4 and §6.2.
  - Rely on running housekeeping calls from `<data-dir>/runs/<repo-id>/` (isolated working directory) with `REPO_MEMORY_HOUSEKEEPING=1`.
  - Disallow `--continue` on housekeeping sessions; use explicit conversation IDs or fresh one-off invocations.

### E08: Subagent Hook Coverage
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E08-subagent-hooks.md`
- **Core Findings:**
  1. Subagents spawned via `invoke_subagent` **fire ALL lifecycle hooks** independently: `PreInvocation`, `PreToolUse`, `PostToolUse`, and `Stop`.
  2. Subagents possess their **own unique identity**: distinct `conversationId` and dedicated `transcriptPath` (`.../brain/<subagentId>/.../transcript_full.jsonl`).
  3. Lifecycle interaction: Parent `Stop` fires when parent goes idle waiting for subagent; parent resumes with `PreInvocation` upon receiving `send_message`.
  4. **Critical contract detail:** Returning `{}` from `PreToolUse` is treated by Antigravity as an **IMPLICIT DENY**. The hook MUST explicitly return `{"decision": "allow"}`.
- **SPEC Impact:**
  - Major architectural simplification: Subagents on Antigravity are first-class citizens for memory injection and tool gating, exactly like Claude Code (§13, §16 Q15).
  - Workers do NOT depend solely on WORKORDER static briefs; they can receive dynamic touch, error, and déjà-vu delivery via hooks.
  - Document the strict `{"decision": "allow"}` contract in §12.1.

### E09: `deny` + `reason` Behavior
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E09-deny-behavior.md`
- **Core Findings:**
  1. Deny reasons are delivered verbatim to the model in the tool call error response (`"tool call denied by pre-tool hook: <reason>"`).
  2. **Batch isolation confirmed:** In parallel tool calls, denying one call does NOT disrupt the others. Each tool call is evaluated independently.
  3. **Wording sensitivity:** Advisory wording ("if your approach differs, re-issue") produced ~3 retries with command variations before giving up; policy-sounding wording ("not permitted... safety policy") resulted in faster abandonment and pivoting to other tools (`view_file`).
  4. **`overwrite` capability:** Returning `{"decision": "allow", "overwrite": {...}}` successfully rewrites tool arguments before execution. Antigravity prefixes the output with an argument rewrite notice.
  5. Hook command failure (exit code != 0) is treated as an implicit deny.
  6. Hook command paths in `hooks.json` resolve relative to the directory containing `hooks.json`, not the repository root.
- **SPEC Impact:**
  - Incorporate wording guidance in §7.5 (advisory for `gate-once` vs authoritative for hard safety blocks).
  - Document `overwrite` in §7.5 and §18.3 as an alternative non-blocking advisory/steering capability.
  - Mandate absolute paths in `hooks.json` templates (§18.5).

### E10: Housekeeping Overhead Reduction
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E10-overhead-reduction.md`
- **Core Findings:**
  1. For Gemini 3.8 Flash, the ~16.8k fixed input token overhead cannot be reduced with CLI flags (`--agent`, `--disable-slash-commands`, or directory changes have zero effect; `--sandbox` increases tokens by +783).
  2. **Prefix caching is ineffective across distinct calls:** Each `agy -p` call generates a new conversation ID, resulting in `cache_read_tokens = 0`.
  3. **Model family impact:** Gemini 3.6 Flash (Low) incurs only **~9.9k tokens**—a 41% reduction in fixed overhead.
  4. Wall-clock duration per call is 10–14s (API 3–7s + CLI startup/auth 6–7s).
- **SPEC Impact:**
  - Update §4.4.1 cost model and §6.9 model policy: Use Gemini 3.6 Flash (Low) for lightweight stages (triage, verification) where a 41% token reduction provides substantial savings.
  - Mandate batching: Due to the 10–14s execution floor, interactive small calls must be batched per tick rather than executed on demand.

### E11: Quota and Rate Limits
- **Categorization:** `confirmed`
- **Report Reference:** `docs/experiments/E11-quota-rate-limits.md`
- **Core Findings:**
  1. Sequential calls to Gemini 3.8 Flash (10 calls) and Gemini 3.6 Flash (5 calls) experienced 0 errors, 0 rate limits, and 0 throttling.
  2. 25+ calls across 15 minutes produced zero issues.
  3. The CLI's inherent 10–14s latency acts as an organic rate limiter, preventing typical API rate limit exhaustion.
  4. Account appears to be a Pro-tier personal OAuth subscription.
- **SPEC Impact:**
  - Confirms viability of running the pipeline on the user's subscription without dedicated API keys (§4.4).
  - Full two-week usage tracking will proceed in PR-05b as planned.

### E12: Transcript File Set (Remaining Parts)
- **Categorization:** `changed`
- **Report Reference:** `docs/experiments/E12-transcript-file-set.md`
- **Core Findings:**
  1. **Append-only verified:** MD5 checksum of an initial 37-line prefix was byte-identical after the file grew past 63 lines.
  2. **Step-index gaps explained:** Gaps in `transcript_full.jsonl` are caused by internal runtime dispatch steps (tool execution scheduling, checkpoint preparation) that Antigravity allocates step indices for but does not serialize to disk. Gaps are identical between `transcript.jsonl` and `transcript_full.jsonl`.
  3. **Out-of-order writes in parallel calls:** When parallel tools run, child tool result steps are appended *before* the parent `PLANNER_RESPONSE` is written, causing temporary step index non-monotonicity in the JSONL.
  4. **Session reset restarts index:** Multiple prompts or sessions within a single conversation file append new steps with `step_index` restarting at 0.
- **SPEC Impact:**
  - Update §5.1 and §6.2 to explicitly document that step indices are non-contiguous and may appear non-monotonically in parallel tool scenarios.
  - Compaction parsers must sort by step index or rely on JSONL file order rather than assuming contiguous incrementation.

---

## 3. Cross-Experiment Validation & Consistency Check

All 12 experiments have been cross-checked to ensure complete consistency:
1. **Silent Failures on Invalid Configurations:** E01 (malformed hook response silently ignored), E02 (missing include silently ignored), E04 (wrong frontmatter key silently ignored), and E07 (`--project` silently ignored) exhibit identical platform behavior: Antigravity fails closed or silent rather than throwing user-visible syntax errors.
2. **Implicit Deny on Contract Violations:** E08 (empty `{}` response in `PreToolUse`) and E09 (hook script exit code != 0) both result in implicit tool denials, demonstrating that `PreToolUse` strictly enforces `{"decision": "allow"}`.
3. **Transcript Accuracy and Completeness:** E03 (hook payloads point to `transcript_full.jsonl` with zero lag) and E12 (`transcript_full.jsonl` is append-only with internal dispatch step gaps) form a coherent picture of transcript serialization.
4. **Execution Overhead:** E07, E10, and E11 independently measured the ~16.8k token overhead and ~11–13s wall-clock latency of `agy -p` calls with Gemini 3.8 Flash, validating the cost model.

No contradictions exist across the 12 experiment reports.

---

## 4. Ambiguities & Recommended Follow-Up Investigations

| Issue | Description | Recommended Action |
|:---|:---|:---|
| **E1b: `userMessage` Persistence** | E01 showed `ephemeralMessage` is dropped after 1 invocation. Can `userMessage` in `injectSteps` persist across multiple invocations within a prompt cycle? | Run E1b: Inject `{"injectSteps": [{"userMessage": "MARKER"}]}` at `inv=0` and probe at `inv=2+`. If it persists, use it for task brief; if not, task brief must be pull-only (MCP). |
| **E05: Non-enrolled Hook Caching** | Non-enrolled workspaces take ~57ms because the hook inspects disk/registry on every call. | Prototype an O(1) in-process check (e.g. check for local `.memory/` directory before parsing registry). |
| **E06: Sleep/Wake & Command Overlap** | Sidecar behavior across system sleep/wake and behavior when tick execution takes >60s could not be simulated without user OS interaction. | Re-evaluate during long-term soak testing in PR-05b. `tick.lock` provides immediate protection against tick overlap. |
| **E07: CLI `--continue` Crossover** | Because all CLI runs share `default-cli-project`, `--continue` in the repo directory resumes whatever ran last. | Ensure all housekeeping calls execute with a unique working directory (`runs/<repo-id>`) to keep CLI workspace recency clean. |
| **E11: Long-Term Quota Ceiling** | Daily quota limits for heavy usage could not be saturated in 25 calls. | Carry out continuous token spend tracking during PR-05b. |
| **E12: Interactive UI Branch/Edit** | Direct UI editing of prior turns was inferred from JSONL resets but not triggered interactively in the web/electron UI. | Verify prefix hash validation in Stage 0 capture handles full transcript rewrites safely. |
