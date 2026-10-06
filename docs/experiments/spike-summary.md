# V2 Spike Summary (PR-06)

## Experiment Results

| Experiment | Question | Result | Impact |
|---|---|---|---|
| X1 | Stop-continue for turn notes | ✅ Works | Gate: terminationReason=NO_TOOL_CALL, fullyIdle, executionNum=0 |
| X3 | Quota visibility | ✅ API available | `/usage` JSON exposes exact `remaining_fraction` and `reset_time` |
| X5 | systemMessage/userMessage channels | ✅ Works (corrected ×3) | `injectSteps` works from plugin hooks for top-level sessions; `invoke_subagent` inconclusive but should work per docs |
| X8 | Lean agent for distillation | ✅ Works (corrected) | Invalid frontmatter caused silent rejection; valid definition works via `agy --agent` and `agy agent` |
| X9 | Hook contracts | ✅ Documented | PreToolUse is fail-closed; others fail-open |
| Phase 0 | Plugin split & architecture | ✅ Validated | Nested dirs isolated; sidecars tolerate configs; absolute paths required |

## V2 Design Sections: Confirmed vs Changed

| Section | Status | Notes |
|---|---|---|
| §1 Summary | Untested | General architecture vision |
| §2 Background (Platform Facts) | ⚠ Changed | Some facts confirmed (Stop hook), others updated (systemMessage JSON shape, agent frontmatter) |
| §2.3 Plugin loading (from Phase 0) | ✅ Confirmed | Split architecture works, but requires absolute paths in plugins.json |
| §3 Goals and non-goals | Untested | Architectural goals |
| §4 Overview | Untested | High-level data flow |
| §4.4 Quota guard | ✅ Confirmed | Can use `agy -p /usage --output-format json` |
| §5.1 Distillation agent / Turn notes | ✅ Confirmed | Stop hook continuation is a viable mechanism (X1) |
| §5.4 Sidecar fallback / Lean agent | ✅ Confirmed | File-based `agy --agent` works with valid frontmatter; file-based agents and `invoke_subagent` are NOT separate systems |
| §6 Consolidation | Untested | Data processing pipeline |
| §7 Storage | Untested | Data models and index schema |
| §8 Retrieval and delivery | ✅ Confirmed | `systemMessage` via `PreInvocation` `injectSteps` works from plugin hooks. No split hook placement needed — all hooks stay in `<plugin>/hooks.json`. |
| §9 Cost model | ⚠ Changed | Token tracking via transcripts is impossible (missing metadata) |
| §10 Portability | Untested | Claude Code specific |

## Platform Assumptions Validated & Invalidated

**Validated:**
- **Stop hook continuation:** Works perfectly for turn notes, using the gate `terminationReason=NO_TOOL_CALL`, `fullyIdle=true`, and `executionNum=0`.
- **Quota API:** `agy -p /usage --output-format json` successfully provides high-resolution quota limits and precise reset timestamps without spending tokens.
- **Hook contracts:** PreToolUse correctly defaults to fail-closed, other hooks fail-open.
- **Nested Plugin Isolation:** Nested bundles inside global plugins are isolated and ignored globally until explicitly enrolled.
- **Sidecar configs:** Sidecars tolerate extraneous config files.
- **PreInvocation Message Injection (corrected ×3):** `systemMessage`, `userMessage`, and `ephemeralMessage` all work via the `injectSteps` response format from plugin-level hooks in top-level sessions. Initial tests failed due to: (1) wrong JSON shape, (2) missing `--add-dir .` in CLI, (3) transcript analysis is unreliable (SYSTEM_MESSAGE steps don't appear in transcripts even when injection works). `invoke_subagent` injection is inconclusive but should work per authoritative docs.
- **File-based Agent Resolution (corrected):** `agy --agent <name>` works without restart — each CLI invocation scans independently. Initial failure was due to invalid frontmatter (missing `description`, invalid keys like `excludeDefaultComponents`). Silent rejection makes debugging hard.
- **Hooks Discovery:** Antigravity only loads `<plugin-root>/hooks.json`. The `hooks/hooks.json` path is Claude Code's convention — the foreign-plugin importer stages it when importing.

**Invalidated:**
- **Token Metadata Visibility:** `transcript_full.jsonl` does not expose token usage metrics (`gen_metadata` is absent).
- **Path Expansion:** Antigravity does not expand `~` or `$HOME` in `.agents/plugins.json`; absolute paths are strictly required.
- **Transcript-based injection verification:** `SYSTEM_MESSAGE` steps from `injectSteps` don't appear in `transcript_full.jsonl` even for confirmed-working injections. Cannot use transcripts to verify injection.

## Key Design Knowledge for PR-07+

1. **No split hook placement needed.** All hooks (PreInvocation, PreToolUse, Stop, PostToolUse) can remain in `<plugin>/hooks.json`.
2. **PreInvocation fires before subagent prompt.** For `invoke_subagent` sessions, `initialNumSteps=0` and `userPrompt=""` at hook time. The parent's prompt arrives after hooks run. Hook must not require `userPrompt != ""`.
3. **Agent frontmatter traps.** `name` and `description` are REQUIRED (silent drop). `model:` only accepts tier names (`inherit`, `flash_lite`, `flash`, `pro`). `mainAgent`/`subagent` booleans control discovery. `hidden: true` only hides from UI picker.
4. **Stop continuation cap:** 10 (v1.1.9).

## Resolved Questions

1. **Cost Tracking:** Without token metadata in transcripts, the sidecar relies entirely on `/usage` quota diffs. **Accepted** — no alternative available.
2. **CHECKPOINT Survival:** Assume injected `systemMessage` steps do NOT survive context compaction. The mitigation is compaction re-injection (PR-08 WO5, X11): scan backwards to detect compaction events, re-inject a consolidated `systemMessage` of still-relevant entries ≤ 1k tokens, re-arm first-entry and first-touch deduplication.
3. **`invoke_subagent` injection:** Inconclusive in testing but should work per docs. Will be confirmed in PR-08 delivery implementation. Either way, acceptable — subagents inherit parent context.
