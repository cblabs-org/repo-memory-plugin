# V2 Spike Summary (PR-06)

## Experiment Results

| Experiment | Question | Result | Impact |
|---|---|---|---|
| X1 | Stop-continue for turn notes | ✅ Works | Gate: terminationReason=NO_TOOL_CALL, fullyIdle, executionNum=0 |
| X3 | Quota visibility | ✅ API available | `/usage` JSON exposes exact `remaining_fraction` and `reset_time` |
| X5 | systemMessage/userMessage channels | ✅ Works (corrected) | Use `injectSteps` with nested `systemMessage` object |
| X8 | Lean agent for distillation | ⚠ Partial | File resolution fails dynamically; must use `define_subagent` |
| X9 | Hook contracts | ✅ Documented | PreToolUse is fail-closed; others fail-open |
| Phase 0 | Plugin split & architecture | ✅ Validated | Nested dirs isolated; sidecars tolerate configs; absolute paths required |

## V2 Design Sections: Confirmed vs Changed

| Section | Status | Notes |
|---|---|---|
| §1 Summary | Untested | General architecture vision |
| §2 Background (Platform Facts) | ⚠ Changed | Some facts confirmed (Stop hook), others invalidated (systemMessage) |
| §2.3 Plugin loading (from Phase 0) | ✅ Confirmed | Split architecture works, but requires absolute paths in plugins.json |
| §3 Goals and non-goals | Untested | Architectural goals |
| §4 Overview | Untested | High-level data flow |
| §4.4 Quota guard | ✅ Confirmed | Can use `agy -p /usage --output-format json` |
| §5.1 Distillation agent / Turn notes | ✅ Confirmed | Stop hook continuation is a viable mechanism (X1) |
| §5.4 Sidecar fallback / Lean agent | ⚠ Changed | Must use `define_subagent` instead of file-based discovery (X8) |
| §6 Consolidation | Untested | Data processing pipeline |
| §7 Storage | Untested | Data models and index schema |
| §8 Retrieval and delivery | ✅ Confirmed | `systemMessage` via `PreInvocation` `injectSteps` works (initial test used wrong JSON shape) |
| §9 Cost model | ⚠ Changed | Token tracking via transcripts is impossible (missing metadata) |
| §10 Portability | Untested | Claude Code specific |

## Platform Assumptions Validated & Invalidated

**Validated:**
- **Stop hook continuation:** Works perfectly for turn notes, using the gate `terminationReason=NO_TOOL_CALL`, `fullyIdle=true`, and `executionNum=0`.
- **Quota API:** `agy -p /usage --output-format json` successfully provides high-resolution quota limits and precise reset timestamps without spending tokens.
- **Hook contracts:** PreToolUse correctly defaults to fail-closed, other hooks fail-open.
- **Nested Plugin Isolation:** Nested bundles inside global plugins are isolated and ignored globally until explicitly enrolled.
- **Sidecar configs:** Sidecars tolerate extraneous config files.
- **PreInvocation Message Injection (corrected):** `systemMessage`, `userMessage`, and `ephemeralMessage` all work via the `injectSteps` response format. Initial test used wrong JSON shape (bare string instead of nested object for `systemMessage`, missing `injectSteps` wrapper).

**Invalidated:**
- **File-based Agent Resolution:** Natively resolving a lean agent dynamically via files fails; it requires a daemon restart or API registration (`define_subagent`). `agy --agent` silently falls back to default agent on resolution failure.
- **Token Metadata Visibility:** `transcript_full.jsonl` does not expose token usage metrics (`gen_metadata` is absent).
- **Path Expansion:** Antigravity does not expand `~` or `$HOME` in `.agents/plugins.json`; absolute paths are strictly required.

## Open Questions for PR-07+

1. **Lean Agent Registration:** How exactly will we orchestrate `define_subagent` dynamically in the distillation pipeline? Or does `agy --agent` work after a daemon restart?
2. **Cost Tracking:** Without token metadata in transcripts, how will the sidecar track and bound its LLM token costs? Do we rely entirely on `/usage` quota diffs?
3. **CHECKPOINT Survival:** Do injected `systemMessage` steps survive context compaction? Needs deliberate testing with a long session.

