# Proposed Specification Changes from Experiment Spike (E01–E12)

> **Document:** Proposed Changes to `SPEC.md`  
> **Status:** Pending Review / Awaiting User Approval  
> **Date:** 2026-10-01  
> **Work Order:** WO4 — Spec Reconciliation  
> **Evidence Base:** `docs/experiments/E01-ephemeral-message.md` through `E12-transcript-file-set.md`, plus `E01-architect-brief.md`

---

## Overview

During the PR-01b experiment spike, all 12 planned P0 experiments (E01–E12) were executed and analyzed against the Antigravity harness runtime. This document details the exact changes required in `SPEC.md` to reconcile the specification with empirical runtime behavior.

### Summary of Major Impacts
1. **Hook Payload Schemas & Contracts (E01, E08, E09):** `PreInvocation` requires camelCase `{"injectSteps": [{"ephemeralMessage": "..."}]}`. `PreToolUse` strictly enforces `{"decision": "allow"}`—an empty `{}` response acts as an implicit deny. Hook script failure (exit code != 0) also acts as an implicit deny.
2. **Ephemeral vs Persistent Delivery (E01, E02):** `ephemeralMessage` is transient across turns within a prompt cycle and `invocationNum` resets to 0 on every prompt. Large task briefs cannot be injected repeatedly as ephemeral messages without token bloat. Task briefs rely on the always-on map include and MCP pull tools (`memory.brief`), while ephemeral messages are capped at ≤ 200 tokens for reactive nudges and alerts.
3. **Execution Topology & Sidecars (E05, E06, E07):** Scheduled sidecars run exclusively under the Antigravity IDE language server and do not tick under the headless CLI. Non-enrolled repos take ~57ms on hooks without an in-memory/marker check. CLI `--project` is silently ignored by `agy -p`, requiring isolation via dedicated working directories (`runs/<repo-id>`) and environment guards.
4. **Subagent Parity (E08):** Subagents fire all 4 lifecycle hooks with their own `conversationId` and `transcriptPath`, unlocking first-class hook delivery for worker subagents.
5. **Transcript Structure (E03, E12):** Transcripts are append-only; step gaps are caused by internal unwritten dispatch steps; parallel tool calls produce out-of-order step indices in JSONL; session resets restart `step_index` at 0.
6. **Cost Optimization (E10, E11):** Gemini 3.6 Flash (Low) reduces fixed overhead by 41% (~9.9k vs ~16.8k tokens), making it the preferred default for triage and verify.

---

## Detailed Section-by-Section Proposed Diffs

---

### 1. §4.1 Invocation Paths — Sidecar IDE Scope
- **Evidence:** `docs/experiments/E06-sidecar-behavior.md`
- **Location:** `SPEC.md` lines 130–137
- **Rationale:** E06 proved that the Antigravity scheduled sidecar runs as a child process of `Antigravity.app` (`language_server multicall schedule`). The Antigravity CLI (`agy`) has no sidecar runner. Environments operating purely via the CLI cannot rely on sidecars for background ticks.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -131,3 +131,3 @@
 | Fired by | Agent, model, tool and session events in the harness | Cron, every minute (Antigravity plugin sidecar, `builtin: schedule`). **[R2]** On Claude Code, harness-run `async: true` hooks also start it (§12.2). Elsewhere, the OS scheduler running `repo-memory tick`. |
+| Fired by | Agent, model, tool and session events in the harness | Cron, every minute (Antigravity plugin sidecar, `builtin: schedule`, active while the IDE is running; see E6). In CLI-only environments, the OS scheduler running `repo-memory tick`. **[R2]** On Claude Code, harness-run `async: true` hooks also start it (§12.2). |
```

---

### 2. §4.2 Invocation Map — Subagent Coverage & Task Brief Timing
- **Evidence:** `docs/experiments/E01-ephemeral-message.md`, `docs/experiments/E08-subagent-hooks.md`
- **Location:** `SPEC.md` lines 150–166
- **Rationale:** 
  - E08 proved that subagents fire their own `Stop` and `PreInvocation` hooks directly, so capture does not rely solely on the sweep.
  - E01 proved that `invocationNum` resets to 0 on *every* user prompt (not once per session) and that `ephemeralMessage` drops after a single turn. Multi-kilobyte briefs cannot be repeatedly pushed as ephemeral messages; bulk briefs rely on MAP include + MCP pull, with `PreInvocation` providing lightweight pointers/nudges.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -153,1 +153,1 @@
-| Capture a finished session | `Stop`: write a spool entry and copy the transcript | Sweep catches sessions with no `Stop` (crash, kill, sessions from before install, subagents per E8); promotes idle sessions to processing |
+| Capture a finished session | `Stop`: write a spool entry and copy the transcript (fires for top-level and subagent sessions; E8) | Sweep catches sessions with no `Stop` (crash, kill, sessions from before install); promotes idle sessions to processing |
@@ -156,1 +156,1 @@
-| Task brief | `PreInvocation` (`invocationNum == 0`) inline from the index. If lexical linking fails, spool a brief request. | Next tick runs it as a housekeeping run; the next `PreInvocation` delivers it. The agent can also call `memory.brief` synchronously. |
+| Task brief | Always-on map include; on-demand refresh via pull tool `memory.brief`. `PreInvocation` (`invocationNum == 0`) provides a ≤ 200-token pointer/summary nudge (E1). If lexical linking fails, spool a brief request. | Next tick runs it as a housekeeping run; the next `PreInvocation` delivers it. The agent can also call `memory.brief` synchronously. |
```

---

### 3. §4.4 & §4.4.1 Cost Model & Housekeeping Isolation
- **Evidence:** `docs/experiments/E07-housekeeping-isolation.md`, `docs/experiments/E10-overhead-reduction.md`, `docs/experiments/E11-quota-rate-limits.md`
- **Location:** `SPEC.md` lines 247–267
- **Rationale:**
  - E07 proved that `--project` is silently ignored by `agy -p`. Isolation is achieved via a dedicated execution directory (`<data-dir>/runs/<repo-id>/`), `REPO_MEMORY_HOUSEKEEPING=1`, and unique workspace paths.
  - E10 discovered that Gemini 3.6 Flash incurs ~9.9k tokens overhead vs ~16.8k for Gemini 3.8 Flash (41% lower), making it a superior choice for triage and verify.
  - E10 proved prefix caching is inactive across separate `agy -p` calls due to distinct conversation IDs.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -250,4 +250,5 @@
 | Steady-state extraction, triage, reconcile and verify | ~100–200k tokens a day | Payload and overhead roughly equal | Pack a parent session with its subagents into one call; reconcile per subsystem batch |
-| Interactive small calls (LLM brief fallback, déjà-vu tier 2, pull-tool synthesis) | ~17–20k tokens **each**, ~85% of it overhead; 5–15 a day ≈ 100–300k tokens | **Fixed overhead** | Deterministic paths first; batch all pending tier-2 checks into one call per tick; cache briefs by entity set; per-day caps; E10 |
+| Interactive small calls (LLM brief fallback, déjà-vu tier 2, pull-tool synthesis) | ~17–20k tokens each on Gemini 3.8 Flash (~10k on Gemini 3.6 Flash; E10), ~85% of it overhead; 5–15 a day ≈ 75–250k tokens | **Fixed overhead** | Deterministic paths first; batch all pending tier-2 checks into one call per tick; use Gemini 3.6 Flash for simple stages (41% overhead reduction); per-day caps |
 
-The fixed overhead matters most for the small, interactive calls, not for extraction. E10 (reducing the overhead) and E11 (how usage counts against the quota) therefore gate how freely those features are enabled. The P1 exit reports cost per stage from the `agy` usage output.
+The fixed overhead matters most for the small, interactive calls, not for extraction. E10 confirmed that fixed overhead cannot be reduced with CLI flags and prefix caching is inactive across distinct calls (`cache_read_tokens = 0`), but model selection (Gemini 3.6 Flash) cuts overhead by 41%. E11 showed sequential calls hit zero rate limits due to the natural 10–14s wall-clock latency of `agy -p`.
@@ -262,3 +263,3 @@
-They run with the working directory `<data-dir>/runs/<repo-id>/`, not the repository, so workspace rules and hooks don't load. They use a dedicated `--project` (E7).
+They run with the working directory `<data-dir>/runs/<repo-id>/`, not the repository, so workspace rules and hooks don't load. (Note: `--project` is silently ignored by `agy -p`, so isolation relies strictly on the isolated working directory and environment variables; E7).
-They set `REPO_MEMORY_HOUSEKEEPING=1`. Hooks inherit `agy`'s environment, so this plugin's own hooks exit immediately inside housekeeping runs.
-Capture excludes them by project and workspace URI.
+Capture excludes them by working directory URI and `REPO_MEMORY_HOUSEKEEPING=1`.
```

---

### 4. §4.6 Repository Enrolment — Fast-Path Registry Caching
- **Evidence:** `docs/experiments/E05-hook-latency.md`
- **Location:** `SPEC.md` lines 304–305
- **Rationale:** E05 demonstrated that non-enrolled repository checks currently take ~55–57ms due to registry file reads on every invocation, failing the ≤ 5ms target. An in-memory cache or local directory marker (e.g. presence of `.memory/`) must be checked before parsing registry files. The global kill switch (`REPO_MEMORY_DISABLED=1`) satisfies the target at ~2.8ms.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -304,1 +304,1 @@
-| Hooks | The first thing every hook does is map the workspace path to the registry (a small file, cached by modification time). If the workspace isn't enrolled, the hook returns the harness's no-op output at once: within 5 ms of process start. |
+| Hooks | The first thing every hook does is check the fast kill switch (`REPO_MEMORY_DISABLED=1`, ~2.8ms; E5), then map the workspace path against the registry. To meet the ≤ 5 ms non-enrolled target (unoptimized disk lookups cost ~55ms; E5), hooks check for a local `.memory/` directory marker or cached in-process registry before parsing disk state. Non-enrolled sessions return no-op output immediately. |
```

---

### 5. §5.1 Episode Record — Step Index Gaps & Parallel Tool Reordering
- **Evidence:** `docs/experiments/E12-transcript-file-set.md`
- **Location:** `SPEC.md` lines 337, 363–374
- **Rationale:** E12 settled the remaining transcript questions: transcripts are append-only; step gaps are normal and caused by unwritten internal dispatch steps; parallel tool calls produce out-of-order step indices in JSONL; session resets cause step indices to restart at 0.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -337,1 +337,1 @@
-  "step_gaps": [[812, 820]],          // missing step indices, if any
+  "step_gaps": [[812, 820]],          // unwritten internal dispatch steps (standard; E12)
@@ -374,2 +374,6 @@
 | Who wrote each `user` step (`human` or `automation`) | **Memory content the session received** (hook injections, pull-tool results, reads of `.memory/**`): replaced by `exposure` steps listing claim IDs. The content itself is never kept. |
 
+**Transcript ordering and gaps (E12):**
+- Step indices in raw JSONL transcripts contain gaps (typically delta=2 after tool calls) representing internal dispatch steps that the harness never serializes. Parsers must not treat gaps as dropped data.
+- Parallel tool executions result in child tool results being appended before the parent `PLANNER_RESPONSE` is written to disk, producing non-monotonic step indices in raw JSONL. Compaction sorts steps by `step_index`.
+- Conversations reused across multiple prompts or reset sessions restart `step_index` at 0 while appending to the same file. Compaction splits episodes on prompt boundaries.
```

---

### 6. §6.2 Stage 0: Capture — Append-Only Verification & Recursion Guard
- **Evidence:** `docs/experiments/E07-housekeeping-isolation.md`, `docs/experiments/E08-subagent-hooks.md`, `docs/experiments/E12-transcript-file-set.md`
- **Location:** `SPEC.md` lines 654, 679–692
- **Rationale:** Incorporate E12 append-only proof and step-gap explanation, E08 subagent `Stop` hook coverage, and E07 recursion guard update.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -654,3 +654,3 @@
-If the prefix hash no longer matches (the harness rewrote the file, reportedly when the user branches or edits an earlier step), the file is copied in full once. Without this, a long session would re-copy multi-megabyte transcripts on every turn;
+If the prefix hash no longer matches (verified append-only during normal flow via MD5; rewrites occur on branching or session resets; E12), the file is copied in full once. Without this, a long session would re-copy multi-megabyte transcripts on every turn;
@@ -679,2 +679,2 @@
-Step-index gaps (§6.2, transcript completeness) are present identically in both monolithic files, so they are recorded as gaps; their cause remains part of E12.
+Step-index gaps are present identically in both monolithic files and reflect unwritten internal dispatch steps rather than missing transcript content (E12).
@@ -684,1 +684,1 @@
-- subagent sessions, if they don't fire their own `Stop` hooks (experiment E8);
+- subagent sessions, if their process is killed before `Stop` fires (subagents fire their own `Stop` hooks under normal completion; E8);
@@ -690,1 +690,1 @@
-Housekeeping runs (§4.4) are excluded from capture entirely, identified by their dedicated project and working directory.
+Housekeeping runs (§4.4) are excluded from capture entirely, identified by their isolated working directory (`runs/<repo-id>/`) and `REPO_MEMORY_HOUSEKEEPING=1` (since `--project` is ignored by the CLI; E7).
```

---

### 7. §6.9 Model and Cost Policy — Gemini 3.6 Flash Recommendation
- **Evidence:** `docs/experiments/E10-overhead-reduction.md`
- **Location:** `SPEC.md` lines 829–832
- **Rationale:** Recommend `gemini-3.6-flash-low` for triage and verify stages to capitalize on the 41% token overhead savings documented in E10.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -830,3 +830,3 @@
-  - `gemini-3.8-flash-low` for triage and verify;
+  - `gemini-3.6-flash-low` (or `gemini-3.8-flash-low`) for triage and verify (Gemini 3.6 Flash incurs 41% less fixed overhead: ~9.9k vs ~16.8k tokens; E10);
   - `gemini-3.8-flash-medium` for extraction;
   - `gemini-3.1-pro-low` for reconcile and view synthesis.
```

---

### 8. §7.1 Delivery Budgets — `ephemeralMessage` Token Ceiling
- **Evidence:** `docs/experiments/E01-ephemeral-message.md`, `docs/experiments/E01-architect-brief.md`
- **Location:** `SPEC.md` lines 845–855
- **Rationale:** E01 and the architect brief showed that `ephemeralMessage` drops out of context on the subsequent turn. Pushing bulk context (task briefs of 1–8k tokens) via ephemeral messages wastes thousands of tokens. Ephemeral messages are strictly capped at ≤ 200 tokens for alerts and nudges.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -848,1 +848,1 @@
-| Task brief | ≤ 8k tokens | once per session |
+| Task brief | ≤ 8k tokens (pull via `memory.brief` / MAP include; ≤ 200 tokens if nudged via `ephemeralMessage`; E1) | once per session |
```

---

### 9. §7.2 Task Start — Ephemeral Lifecycle Reconciliation
- **Evidence:** `docs/experiments/E01-ephemeral-message.md`
- **Location:** `SPEC.md` lines 871–884
- **Rationale:** Clarify that `invocationNum` resets to 0 on every prompt, not once per session. Task briefs cannot be persistently held in context via `ephemeralMessage`.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -871,2 +871,2 @@
 1. Always-on: `MAP.md`, loaded by the harness's own instruction file. The repo's root `AGENTS.md` (Antigravity) and `CLAUDE.md` (Claude Code) each include it: `@[memory map](.memory/views/MAP.md)` and `@.memory/views/MAP.md` respectively. This costs ~2k of the 20k rules budget and is cached with the system prompt. Fallback, if include semantics don't work as expected (P0 experiment E2): inject at `PreInvocation` with `invocationNum == 0`.
-2. At the first model invocation, the delivery hook reads the task text (first user message) and branch from the transcript.
+2. At the first model invocation of a task (`invocationNum == 0`, which fires on every prompt; E1), the delivery hook reads the task text and branch from the transcript.
@@ -876,2 +876,2 @@
-if lexical matching yields nothing, the hook injects the map-level summary with a one-line pointer to the claim indexes (§5.5), so the agent can choose the relevant subsystems itself at once. It also spools a brief request. The next tick prepares the brief in a housekeeping run, and the next `PreInvocation` picks it up from `alerts/` (typically within ~75 s). If it needs the brief sooner, the agent can call `memory.brief` synchronously (~13 s). An LLM is never called inside a hook.
-4. It renders the **task brief**, in priority order:
+if lexical matching yields nothing, the hook injects a concise pointer (≤ 200 tokens via `ephemeralMessage`) to the relevant claim indexes (§5.5). Because `ephemeralMessage` context drops after one turn (E1), full task briefs are delivered via `memory.brief` pull or MAP views rather than massive ephemeral payloads.
+4. When generated, the **task brief** prioritizes:
```

---

### 10. §7.5 Risky Action — Batch Isolation, Wording, & `overwrite`
- **Evidence:** `docs/experiments/E08-subagent-hooks.md`, `docs/experiments/E09-deny-behavior.md`
- **Location:** `SPEC.md` lines 916–923
- **Rationale:**
  - E09 confirmed parallel tool call denials are isolated (denying one does not break others).
  - E09 demonstrated wording effects: advisory wording ("re-issue if different") encourages retries with command variations; policy wording causes tool pivoting/abandonment.
  - E09 demonstrated that `overwrite` is a viable non-blocking steering mechanism.
  - E08 and E09 proved that returning `{}` or an exit code != 0 results in an implicit deny.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -916,8 +916,9 @@
-`gate-once` costs a model round-trip and has two failure modes, both tested in E9:
-- a denial in the middle of a batch of parallel tool calls can disrupt the batch;
-- a model may read a hook denial as a policy block and stop to ask the user instead of re-issuing.
+`gate-once` costs a model round-trip. E9 validated its mechanics and resolved its failure modes:
+- **Parallel tool call isolation:** Denying one tool call in a parallel batch does *not* disrupt the other calls; each call is evaluated and returned independently (E9).
+- **Reason wording sensitivity:** Advisory wording ("This action failed previously... If your approach differs, re-issue the call") encourages 2–3 retries with command variations. Policy wording ("Action not permitted by repository policy") causes immediate pivoting to other tools or abandonment. `gate-once` uses advisory wording; hard blocks use policy wording.
+- **`overwrite` capability:** PreToolUse can return `{"decision": "allow", "overwrite": {...}}` to non-destructively rewrite command arguments with an explicit harness warning notice (E9).
+- **Strict contract:** `PreToolUse` hooks must explicitly return `{"decision": "allow"}`. An empty `{}` response or hook process crash (exit != 0) is treated by the harness as an implicit deny (E8, E9).
```

---

### 11. §7.8 Performance & §7.9 Degraded Modes
- **Evidence:** `docs/experiments/E04-rule-activation.md`, `docs/experiments/E05-hook-latency.md`, `docs/experiments/E06-sidecar-behavior.md`
- **Location:** `SPEC.md` lines 969–985
- **Rationale:**
  - E05 showed non-enrolled paths take ~57ms without an in-memory/marker check; enrolled repos take ~3ms (passing ≤ 60ms budget).
  - E06 showed sidecars do not run under CLI-only environments.
  - E04 showed `model_decision` rules require `trigger: model_decision` and operate via pull costing 1 tool call.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -969,3 +969,3 @@
 - **Delivery hook fast path.** Check the conversation's `alerts/` directory and read the transcript from the last cursor (tail only). If nothing new matches, exit. Target: p95 ≤ 60 ms, p50 ≤ 30 ms.
 - **Delivery hook with injection.** Local SQLite read-only queries plus formatting. Target: p95 ≤ 150 ms.
-- **Implementation.** Hooks run the compiled Go binary (§18), so start-up costs milliseconds. E5 confirms the fast-path budget on macOS and Windows.
+- **Implementation.** Hooks run the compiled Go binary (§18), starting in ~2.5ms. Enrolled repos easily meet targets (p50 ~3ms, p95 ~6.1ms; E5). Non-enrolled repos require an in-memory or directory marker cache to avoid the ~57ms disk scan penalty. E5 confirmed macOS Apple Silicon; Windows to be verified.
@@ -980,1 +980,1 @@
-| **No ticks** | Scheduled sidecar not running (Antigravity closed, sidecars disabled, machine without Antigravity and no OS schedule) | Capture (`Stop` copies transcripts), delivery from the last index, déjà-vu tier 1, pull tools (`ask`/`brief` synthesis still runs synchronously). Spooled work waits for the next tick. |
+| **No ticks** | Scheduled sidecar not running (CLI-only session, Antigravity closed, sidecars disabled, or machine without Antigravity and no OS schedule; E6) | Capture (`Stop` copies transcripts), delivery from the last index, déjà-vu tier 1, pull tools (`ask`/`brief` synthesis still runs synchronously). Spooled work waits for the next tick. |
@@ -983,1 +983,1 @@
-| **Native floor** | No hooks and no tick (e.g. a harness where the plugin's hooks are disabled) | The always-on map include; generated path-scoped subsystem rules (`.claude/rules/` with `paths:` on Claude Code; `model_decision` rules on Antigravity, per E4); MCP pull tools over the last index |
+| **Native floor** | No hooks and no tick (e.g. a harness where the plugin's hooks are disabled) | The always-on map include; generated path-scoped subsystem rules (`.claude/rules/` with `paths:` on Claude Code; `trigger: model_decision` rules on Antigravity pulling on match via `view_file`, per E4); MCP pull tools over the last index |
```

---

### 12. §12.1 Antigravity Harness Integration & Hook Table
- **Evidence:** `docs/experiments/E01-ephemeral-message.md`, `docs/experiments/E04-rule-activation.md`, `docs/experiments/E06-sidecar-behavior.md`, `docs/experiments/E08-subagent-hooks.md`, `docs/experiments/E09-deny-behavior.md`
- **Location:** `SPEC.md` lines 1161–1200
- **Rationale:** Incorporate exact hook schemas, the strict `{"decision": "allow"}` PreToolUse requirement, `overwrite` support, absolute path resolution, and IDE-only sidecar details.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -1164,3 +1164,3 @@
-| `PreInvocation` (every model call) | `invocationNum == 0`: task brief (inline, or a spooled request). Every call: read `transcriptPath` from the last cursor and `alerts/`; touch, error and déjà-vu tier-1 delivery; spool tier-2 requests. Exit immediately if `REPO_MEMORY_HOUSEKEEPING` is set. Must meet §7.8. | `injectSteps` → `ephemeralMessage` when something is due; otherwise `{}` |
-| `PreToolUse` (matcher: `run_command\|write_to_file\|replace_file_content\|call_mcp_tool`) | Risky-action gates and déjà-vu tier 1 on the concrete proposed call (§7.5, §7.6) | `allow`, or `deny`/`ask` with a `reason` for `gate-once`/`ask` modes. Never `overwrite`. |
+| `PreInvocation` (every model call) | `invocationNum == 0`: task brief nudge (≤ 200 tokens; E1). Every call: read `transcriptPath` from the last cursor and `alerts/`; touch, error and déjà-vu tier-1 delivery; spool tier-2 requests. Exit immediately if `REPO_MEMORY_HOUSEKEEPING` is set. Must meet §7.8. | `{"injectSteps": [{"ephemeralMessage": "..."}]}` when due; otherwise `{}`. Malformed payloads silently ignored (E1). |
+| `PreToolUse` (matcher: `run_command\|write_to_file\|replace_file_content\|call_mcp_tool`) | Risky-action gates and déjà-vu tier 1 on the concrete proposed call (§7.5, §7.6) | MUST return `{"decision": "allow"}` to proceed (`{}` acts as implicit deny; E8). Returns `deny`/`ask` with `reason`, or `overwrite` for argument steering (E9). |
@@ -1174,3 +1174,3 @@
-| `repo-memory/tick` | `builtin: schedule`, `* * * * *`, `restart_policy: "always"`, with `display_name`, `description` and `agent_permissions` (`access_grants` and `workspace_uris` for the report conversations it starts) | `repo-memory tick` (§4.3, §12.3) |
+| `repo-memory/tick` | `builtin: schedule`, `* * * * *`, `restart_policy: "always"`, with `display_name`, `description` and `agent_permissions` | `repo-memory tick` (§4.3, §12.3). Runs only while Antigravity IDE is open (child of `language_server multicall schedule`; E6). Fixed jitter ~592ms (+26s). Logs grow ~14MB/mo; requires rotation. |
@@ -1184,1 +1184,1 @@
-| Native alternative | `model_decision` rules (description-led, loaded on demand) | The built-in docs document `always_on` and `model_decision` only; `glob` activation is undocumented there and reportedly deprecated, so it isn't used (E4). |
+| Native alternative | `model_decision` rules (description-led, loaded on demand) | Requires frontmatter `trigger: model_decision` (`activation:` is silently ignored; E4). Mechanism is pull: description is exposed in system prompt; model calls `view_file` on match. |
@@ -1191,2 +1191,2 @@
-├── hooks.json                   # generated by `repo-memory install`: Stop, PreInvocation,
-│                                #   PreToolUse, PostToolUse → `<bin> hook antigravity <event>`
+├── hooks.json                   # generated by `repo-memory install` with ABSOLUTE paths (paths resolve
+│                                #   relative to hooks.json dir; E9) → `<bin> hook antigravity <event>`
```

---

### 13. §12.1.1 P0 Experiments Table Update
- **Evidence:** All Experiment Reports E01–E12
- **Location:** `SPEC.md` lines 1202–1220
- **Rationale:** Update all 12 experiment rows to document their complete experimental resolution and outcomes.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -1206,14 +1206,14 @@
 | # | Question | Method |
 |---|---|---|
-| E1 | Does an `ephemeralMessage` persist in the model's context on later turns, and does it appear in `transcript.jsonl`? | Inject a unique marker at invocation N; ask the model about it at N+5; inspect the transcript. Dedupe via the ledger regardless of the answer. |
-| E2 | Do `@[…](path)` includes in `GEMINI.md`/`AGENTS.md` resolve relative to the including file, and are they re-read when the target changes? (Known: the 24 KB per-file cap applies after expansion, within the 20k-token always-on budget.) | Include `MAP.md`, change it, start a new session. |
-| E3 | Is `transcript.jsonl` current at the moment `PreInvocation` fires? | Compare the last step index in the file with `initialNumSteps` and the hook's view. |
-| E4 | ~~Are Glob and Model Decision activations supported?~~ **Answered from the built-in docs:** `always_on` and `model_decision` are documented; `glob` isn't, and is reportedly deprecated. The native floor uses `model_decision` rules only. Remaining check: a `model_decision` rule loads when its description matches the task. | Minimal `model_decision` rule. |
-| E5 | Delivery hook latency of the Go binary on macOS and Windows (fast path and injection path) | Benchmark the fast path and the injection path. |
-| E6 | Do scheduled sidecars tick while only the Antigravity CLI is open? What happens on sleep and wake? What happens when a tick is still running when the next is due? (Known from the built-in automation guidance: `sidecar.json` fields, `restart_policy`, `agent_permissions`, user enablement via the dashboard.) | Observe `sidecar_data/.../logs`. The tick takes `tick.lock`, so overlapping ticks exit immediately. |
-| E7 | Are housekeeping runs isolated? Does `--project` place them in a separate project? Can `agy --continue` or the CLI's history ever resume or surface them? Do they ever appear in the app/IDE store (e.g. through the shared history)? | Run tagged housekeeping calls, then inspect both `conversation_summaries.db` stores and try `agy -c` from the repo directory. |
-| E8 | Do subagent conversations (e.g. PA workers started with `invoke_subagent`) fire their own `PreInvocation`, `PreToolUse` and `Stop` hooks, with their own `conversationId` and `transcriptPath`? | Spawn a subagent with logging hooks enabled. If not, worker sessions are captured by the sweep only, and workers get their brief through the WORKORDER (§13) rather than through hooks. |
-| E9 | How does `PreToolUse` `deny` + `reason` read to the agent in practice? Does it reliably re-issue the call after a `gate-once` denial? How does a denial behave inside a batch of parallel tool calls? Which reason wording avoids being read as a policy block? Would `overwrite` (whose result is prefixed with a rewrite notice) be a usable non-blocking alternative for non-destructive commands? | Scripted sessions against a test gate; tune the reason wording. |
-| E10 | Can the ~17k-token fixed overhead of a housekeeping run be reduced (a minimal `--agent` definition, fewer global plugins, no skills)? Does repeated use of the same prefix produce cache reads? | Measure `usage` from `agy -p --output-format json` across variants. |
-| E11 | How do housekeeping runs count against the Antigravity subscription quota, and what are the rate limits per model? | Track usage and failures over the first two weeks of P1; set the daily budgets from the results. |
-| E12 | ~~Antigravity transcript file set~~ **Answered (2026-10-01, issue #1 follow-up, verified on all 537 conversations):** `chunks/` are fixed 100 KiB byte slices of the monolithic files, and `transcript_full.jsonl` matches `transcript.jsonl` line for line with nothing truncated (§6.2). Remaining: confirm append-only growth during normal turns and when `transcript_full.jsonl` is rewritten (reported: only when the user branches or edits an earlier step); explain the step-index gaps, which are present identically in both files. | Watch one live conversation grow; branch or edit an earlier step and diff the file. |
+| E1 | **Resolved:** `ephemeralMessage` delivers via `{"injectSteps":[{"ephemeralMessage":"..."}]}` but is per-invocation transient only; `invocationNum` resets to 0 per prompt; malformed format silently ignored. Capped at ≤ 200 tokens. | Verified via marker injection. Follow-up E1b to test `userMessage`. |
+| E2 | **Resolved:** `@[label](path)` inlines content relative to `GEMINI.md` (repo root) into `user_rules`; refreshes per new session; missing targets silently ignored; 24 KB / 20k token caps confirmed. | Verified in live session. |
+| E3 | **Resolved:** Zero lag at `PreInvocation` (`initialNumSteps` matches line count) and `PreToolUse` (reasoning flushed before hook fires); payload points to `transcript_full.jsonl`. | Verified with logging hooks. |
+| E4 | **Resolved:** Frontmatter MUST use `trigger: model_decision` (`activation:` is silently ignored). Mechanism is pull: description placed as conditional pointer; model reads rule via `view_file` on match. | Verified with positive and negative prompt tests. |
+| E5 | **Resolved:** Enrolled repos pass (p50 ~3ms, p95 ~6.1ms ≤ 60ms) and kill switch is fast (~2.8ms). Non-enrolled repos take ~57ms (failing ≤ 5ms target); requires cached registry or directory marker. | Benchmarked across 20 iterations. |
+| E6 | **Resolved:** Sidecars are IDE-only (child of IDE language server; CLI does not tick). 60s ticking is rock-solid in IDE; fixed jitter ~592ms (+26s); log grows ~14MB/mo; overlap requires `tick.lock`. | Observed over 119 ticks / 2 hours. |
+| E7 | **Resolved:** `--project` is silently ignored by `agy -p` (all CLI calls share `default-cli-project`). CLI and IDE DBs are separate. Isolation relies on dedicated run dir and `REPO_MEMORY_HOUSEKEEPING=1`. | Tested with tagged calls and DB inspection. |
+| E8 | **Resolved:** Subagents fire ALL 4 lifecycle hooks with unique `conversationId` and `transcriptPath`. Returning `{}` from `PreToolUse` is an implicit deny; must return `{"decision": "allow"}`. | Verified in multi-agent test session. |
+| E9 | **Resolved:** Deny+reason delivers verbatim; parallel tool call denials are isolated; advisory wording prompts retries vs policy wording prompts pivot; `overwrite` allows argument steering; exit != 0 is implicit deny. | Scripted test sessions with parallel and overwrite hooks. |
+| E10 | **Resolved:** Overhead irreducible via CLI flags (~16.8k tokens for Gemini 3.8 Flash), but Gemini 3.6 Flash uses ~9.9k tokens (41% lower). No prefix caching across fresh CLI calls. 10–14s wall time. | Measured across 5 configurations. |
+| E11 | **Resolved (preliminary):** 0 rate limits across 25+ rapid calls in 15m; CLI wall-clock latency acts as natural rate limiter; Pro-tier OAuth subscription confirmed. Full tracking continues in PR-05b. | Rapid sequential calls. |
+| E12 | **Resolved:** Monolithic `transcript_full.jsonl` is append-only during normal flow (verified by MD5 prefix hash). Step-index gaps are unwritten internal dispatch steps. Parallel tools produce out-of-order JSONL records. | MD5 prefix tracking and conversation analysis. |
```

---

### 14. §13 Principal-Architect Integration — Worker Subagent Parity
- **Evidence:** `docs/experiments/E08-subagent-hooks.md`
- **Location:** `SPEC.md` lines 1309–1310
- **Rationale:** E08 proved subagents fire all hooks with dedicated `conversationId` and `transcriptPath`, giving Antigravity parity with Claude Code subagent capabilities.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -1309,2 +1309,2 @@
 | Abandoned work orders lose dead-ends | Branch-abandonment promotion (§6.7) keeps `ruled-out` claims. |
-| Workers learn only what the WORKORDER tells them | Workers get the generated brief in the WORKORDER in every case. On Claude Code, `SubagentStart` also injects it, and hooks inside subagents give workers touch, error, risky-action and déjà-vu delivery. On Antigravity the same depends on E8. |
+| Workers learn only what the WORKORDER tells them | Workers get the generated brief in the WORKORDER in every case. On both Claude Code and Antigravity, subagents fire lifecycle hooks with dedicated conversation IDs and transcript paths, enabling dynamic touch, error, risky-action and déjà-vu delivery inside workers (E8). |
```

---

### 15. §16 Open Questions and Risks — Resolution Updates
- **Evidence:** All Experiment Reports E01–E12
- **Location:** `SPEC.md` lines 1371, 1377–1381, 1390
- **Rationale:** Update status of Q5, Q11, Q12, Q14, Q15, and Q24 from open to resolved/clarified.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -1371,1 +1371,1 @@
-| Q5 | `ephemeralMessage` / `additionalContext` persistence semantics | E1 (Antigravity), C2 (Claude Code). Dedupe via the ledger regardless. |
+| Q5 | `ephemeralMessage` persistence semantics | **Resolved (Antigravity):** Per-invocation only; drops out of context on next turn; `invocationNum` resets per prompt (E1). Capped at ≤ 200 tokens. Follow-up E1b for `userMessage`. |
@@ -1377,5 +1377,5 @@
-| Q11 | Cumulative hook latency, especially on Windows | Budgets in §7.8, measured per session (E5). The compiled Go binary (D1) removes interpreter start-up. |
-| Q12 | Scheduled sidecars only tick while Antigravity is running | Claude Code async-hook ticks and the optional OS scheduler cover other machines. Without any tick, only the backstop and calendar jobs are lost (§7.9). |
+| Q11 | Cumulative hook latency | **Resolved (macOS):** Enrolled repos pass easily (p50 ~3ms, p95 ~6.1ms); kill switch takes ~2.8ms. Non-enrolled repos require in-memory/marker caching to avoid ~57ms disk scan (E5). Windows to be measured in PR-07. |
+| Q12 | Scheduled sidecars only tick while Antigravity is running | **Resolved:** Sidecars are IDE-only child processes of `language_server` (E6). CLI-only environments use the OS scheduler (`repo-memory schedule install`). |
-| Q14 | Fixed overhead of ~17k input tokens and ~13 s per housekeeping run | Dominates the small interactive calls rather than extraction (§4.4.1). Mitigated by packing, batching tier-2 checks per tick, caching briefs, per-day caps, E10 and C1; quota impact from E11. |
-| Q15 | Hook coverage of subagents | **Claude Code: resolved** (`SubagentStart`, hooks carry `agent_id`). Antigravity: E8. The WORKORDER brief is the fallback. |
+| Q14 | Fixed overhead of ~17k input tokens and ~13 s per housekeeping run | **Resolved:** Irreducible via CLI flags, but Gemini 3.6 Flash reduces overhead by 41% to ~9.9k tokens (E10). Natural 10–14s CLI latency prevents rate limits (E11). Batching per tick is mandatory. |
+| Q15 | Hook coverage of subagents | **Resolved on both harnesses:** Claude Code (`SubagentStart`, `agent_id`) and Antigravity (all 4 lifecycle hooks fire with dedicated `conversationId` and `transcriptPath`; E8). |
@@ -1390,1 +1390,1 @@
-| Q24 | ~~Which repositories does a global plugin act on?~~ | **Resolved.** Only explicitly enrolled repositories (§4.6); hooks no-op within 5 ms elsewhere; kill switch for the whole machine or one repository. |
+| Q24 | ~~Which repositories does a global plugin act on?~~ | **Resolved.** Only explicitly enrolled repositories (§4.6); kill switch exits in ~2.8ms; fast marker/cache check needed to hit ≤ 5ms on non-enrolled paths (E5). |
```

---

### 16. §18.5 Hooks in Go — Execution Constraints
- **Evidence:** `docs/experiments/E05-hook-latency.md`, `docs/experiments/E08-subagent-hooks.md`, `docs/experiments/E09-deny-behavior.md`
- **Location:** `SPEC.md` lines 1510–1514
- **Rationale:** Document absolute path resolution requirement, explicit `allow` return requirement, and non-enrolled caching.

#### Unified Diff:
```diff
--- a/SPEC.md
+++ b/SPEC.md
@@ -1510,4 +1510,5 @@
 - **Invocation.** `hooks.json` entries call `<absolute path>/repo-memory hook antigravity <Event>`, written by `install`, so they work under `sh -c` and `cmd /c` without `PATH`.
+- **Path resolution:** Paths in `hooks.json` resolve relative to the directory containing `hooks.json` (i.e. `.agents/`), not the repo root. All generated hook entries must use absolute paths (E9).
 - **Fast path.** The hook reads a per-conversation cursor (a byte offset into the transcript), reads only new bytes, checks `alerts/`, and opens SQLite read-only only when there is something to look up.
-- **Budget.** p95 ≤ 60 ms on the fast path (§7.8), confirmed by E5 on macOS and Windows.
-- **Failure behaviour.** On any error, the hook returns the harness's no-op output and logs the error. It never blocks the agent.
+- **Budget.** p95 ≤ 60 ms on the fast path (§7.8), confirmed for enrolled repos (p95 ~6.1ms; E5). To satisfy the ≤ 5ms target on non-enrolled repos, hooks check for local repository markers before parsing disk state.
+- **Failure behaviour.** On any error, the hook logs the error and returns the harness's permissive output (`{"decision": "allow"}` for `PreToolUse`, `{}` for others). Note that returning `{}` or exiting non-zero from `PreToolUse` is treated by the harness as an implicit deny (E8, E9).
```
