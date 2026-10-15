# Antigravity Hooks Reference

> Empirically validated reference for Antigravity hook events, `injectSteps` message injection,
> two-plugin packaging, subagent lifecycle ordering, and `stdin`/`stdout` JSON schemas. Sourced from
> `~/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md`, `agy -p "/changelog"`,
> and direct experimentation (`E01`–`E12`, `X1`–`X9`, `phase0-plugin-split`, and `PR-08`/`PR-09`, October 2026).

## Documentation Sources

1. **Bundled Customization Guide:** `~/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md`
   - Covers `hooks.json` discovery paths, execution order, `stdin`/`stdout` JSON schemas, and timeout configuration.

2. **CLI Slash Commands:** `agy -p "/hooks"` (inspect active hooks) and `agy -p "/changelog"` (release notes).
   Use `--output-format json` (added v1.1.12) for structured output.

3. **Transcript Inspection:** Every hook receives `transcriptPath` on `stdin` pointing to
   `<appDataDir>/brain/<conversation-id>/.system_generated/logs/transcript.jsonl`
   (and `transcript_full.jsonl`).

### Notable Changelog Entries

| Version | Change |
|---|---|
| v1.1.9 | Fixed `PostToolUse` firing on non-tool steps; added Stop hook continuation cap (10) |
| v1.1.10 | Hook ordering: `hooks.json` hooks run before built-in termination checks |
| v1.2.1 | Added `excludeDefaultComponents: true` for custom agents, preserving `PostInvocation` hooks |
| v1.2.3–4 | Fixed `/hooks` for plugin-bundled hooks; fixed `hooks.json` dropped during token budget truncation |
| v1.3.3 | Confirmed `userMessage` plain-string persistence (`USER_INPUT` / `SYSTEM_SDK`) vs 1-step `ephemeralMessage` (`EPHEMERAL_MESSAGE`) and nested-object `systemMessage` |

---

## `hooks.json` Discovery Paths & Two-Plugin Packaging Split

Antigravity discovers `hooks.json` from:

1. **Workspace:** `<workspace>/.agents/hooks.json` (or `.agent/hooks.json`)
2. **Global:** `~/.gemini/config/hooks.json`
3. **Plugin:** `<plugin-root>/hooks.json` (e.g., `~/.gemini/config/plugins/<name>/hooks.json`
   or per-workspace plugins listed in `<workspace>/.agents/plugins.json`)

### Two-Plugin Packaging Split (`sidecar.json` vs Workspace Hooks)

Empirically (`docs/experiments/phase0-plugin-split.md`), Antigravity only spawns `sidecar.json` scheduled background daemons (`builtin: schedule`) from **global** plugins under `~/.gemini/config/plugins/<name>/`, whereas workspace-scoped hooks, MCP servers, and skills can be activated per repository via `<workspace>/.agents/plugins.json`:

1. **Global Daemon Plugin (`~/.gemini/config/plugins/repo-memory-daemon/`):** Declares `plugin.json` and `sidecar.json` (`builtin: schedule`) so the scheduled tick runs while the IDE is open, and bundles the workspace plugin inside `workspace-plugin/repo-memory/`. It declares **no global hooks**, preventing hooks from firing in non-enrolled workspaces.
2. **Per-Workspace Plugin (`<workspace>/.agents/plugins.json`):** Enrolled repositories reference the nested `workspace-plugin/repo-memory` directory via a resolved **absolute path** (because `.agents/plugins.json` does not expand `~` or `$HOME`). This activates `hooks.json`, `mcp.json`, agents, and skills exclusively for that workspace.

### `<plugin>/hooks.json` vs `<plugin>/hooks/hooks.json`

**At runtime, Antigravity ONLY loads `<plugin-root>/hooks.json`.** It never reads
`<plugin-root>/hooks/hooks.json` directly. The `hooks/hooks.json` path is Claude Code's
convention, not Antigravity's.

**Why `hooks/hooks.json` can appear to work:** When you run `agy plugin install <path>` or
`agy plugin import [claude|gemini]` on a pure Claude Code plugin (containing
`.claude-plugin/plugin.json` and no top-level `plugin.json`) or a Gemini CLI extension
(`gemini-extension.json`), the CLI's foreign-plugin importer stages the extension by reading
`<source>/hooks/hooks.json` (interpolating `${extensionPath}` and `${workspacePath}`) and
writing the result to `~/.gemini/config/plugins/<name>/hooks.json` at the plugin root.
However, if the directory already contains a native Antigravity manifest (`plugin.json`,
`.antigravity-plugin/plugin.json`, or `.gemini-plugin/plugin.json`), `agy plugin install`
treats it as a native plugin and copies the directory as-is without staging.

**For dual-host (Antigravity + Claude Code) plugins:** The hook schema itself differs between
hosts — Antigravity's `hooks.json` has a top-level `<hook-group-name>` map key wrapping the
event arrays and sets `cwd` to `<plugin-root>`, whereas Claude Code's `hooks/hooks.json` puts
`"hooks": { "<Event>": [...] }` at the top level. A dual-host repository can safely keep both
at their respective paths without collision.

---

## Common `stdin` Fields (All 6 Events)

Every hook command receives a JSON object on `stdin` containing:

```json
{
  "conversationId": "022ef0e8-...",
  "workspacePaths": ["/path/to/workspace"],
  "transcriptPath": "/path/to/brain/<conversationId>/.system_generated/logs/transcript.jsonl",
  "artifactDirectoryPath": "/path/to/brain/<conversationId>",
  "modelName": "..."
}
```

---

## Message Injection via `injectSteps`

`SessionStart`, `PreInvocation`, and `PostInvocation` all support injecting messages into the
agent's context via the `injectSteps` response field. Three message types are supported, with
critically different lifetimes:

### A. `userMessage` — Persistent Session Scope (Recommended for Dossiers & Context)

```json
{
  "injectSteps": [
    {
      "userMessage": "<repo-memory context=\"startup\" scopes=\"global,auth\">...</repo-memory>"
    }
  ]
}
```

- **Value type:** Plain string
- **Transcript:** Appends a `USER_INPUT` step (`"source": "SYSTEM_SDK"`)
- **Persistence:** **Remains in the conversation history across all subsequent invocations and turns**
  until context compaction (`CHECKPOINT`) or transcript rewind (`step_index` regression).
- **Best for:** Durable session-scoped memory dossiers, startup `.memory/ATLAS.md` context, and
  newly entered scope entries that are tracked in a per-session `InjectedSet`.
- **Compaction & Rewind interaction:** Because `userMessage` persists until `CHECKPOINT` or
  transcript rewind, pair `InjectedSet` deduplication with `DetectCompaction` (checking for new
  `CHECKPOINT` steps in `transcript.jsonl`) and `DetectRewind` (checking whether the highest
  `step_index` regressed below `LastSeenStepIndex`), clearing `InjectedSet` when either occurs.

### B. `ephemeralMessage` — Single-Invocation Scope (1 Model Call Only)

```json
{
  "injectSteps": [
    {
      "ephemeralMessage": "Transient context for this model call only."
    }
  ]
}
```

- **Value type:** Plain string
- **Transcript:** Appends an `EPHEMERAL_MESSAGE` step (`"source": "SYSTEM_SDK"`)
- **Persistence:** **Visible to the model on the immediate invocation (`invocationNum = k`) ONLY.**
  Automatically excluded from all subsequent model calls (`invocationNum = k+1, k+2, ...`) once a
  `PLANNER_RESPONSE` step is recorded after it.
- **Anti-Pattern Warning (`ephemeralMessage` + `InjectedSet`):** Never combine `ephemeralMessage`
  with session-level `InjectedSet` deduplication! If an entry is injected via `ephemeralMessage` at
  `invocationNum == 0` and marked as injected in `InjectedSet`, the harness drops the message on
  `invocationNum == 1` (right after the agent's first tool call) and the deduplicator suppresses
  it for the rest of the session — causing "vanishing memory" before the agent writes any code.
- **Best for:** Strictly 1-step transient nudges, post-`PreToolUse` gate reminders, or status
  notices that should evaporate immediately after one model step.

### C. `systemMessage` — Nested-Object Persistent Scope (Avoid Plain Strings)

```json
{
  "injectSteps": [
    {
      "systemMessage": {
        "systemMessage": "System-level instruction injected by hook.",
        "metadata": {
          "source": "my-hook"
        }
      }
    }
  ]
}
```

- **Value type:** Nested object (NOT a plain string)
- **Schema caveat:** Unlike `ephemeralMessage` and `userMessage` (which take a plain string),
  passing a plain string `{"systemMessage": "..."}` in `agy 1.3.3` fails proto deserialization
  (`type mismatch: cannot convert string to message`). Because `userMessage` accepts a plain string
  directly and persists identically across the session (`USER_INPUT` with `"source": "SYSTEM_SDK"`),
  `repo-memory` uses `userMessage` as its canonical persistent delivery channel.

### D. `toolCall` — No-Op (Documented but Non-Functional)

```json
{
  "injectSteps": [
    {
      "toolCall": {
        "name": "tool_name",
        "args": {}
      }
    }
  ]
}
```

- Documented in `hooks.md` under `PreInvocation` but empirically a no-op in current builds.
  No step is written to `transcript.jsonl`.

### Multiple Steps

Multiple inject steps can be combined in a single response:

```json
{
  "injectSteps": [
    {
      "userMessage": "Persistent session-scoped dossier context."
    },
    {
      "ephemeralMessage": "One-shot reminder for this immediate invocation only."
    }
  ]
}
```

---

## Per-Event Reference

### 1. `SessionStart`

| | |
|---|---|
| **Layout** | Flat array of hook entries |
| **When** | Once per conversation, before the very first model call (`invocationNum == 0` on turn 1) |
| **Extra `stdin` fields** | *(Common fields only)* |
| **`stdout` fields** | `{"injectSteps": [...]}` (`ephemeralMessage`, `userMessage`, `systemMessage`) |
| **Notes** | Ideal for one-time session bootstrap context that should persist across the conversation. |

### 2. `PreInvocation`

| | |
|---|---|
| **Layout** | Flat array of hook entries |
| **When** | Before **each** model call in a turn (initial prompt + before each follow-up model call after tool execution) |
| **Extra `stdin` fields** | `invocationNum` (`int`, 0-based within the turn), `initialNumSteps` (`int`) |
| **`stdout` fields** | `{"injectSteps": [...]}` (`ephemeralMessage`, `userMessage`, `systemMessage`) |
| **Notes** | If a turn makes N tool-call rounds, `PreInvocation` fires N+1 times (`invocationNum`: `0`, `1`, …). Prefer `ephemeralMessage` if injecting on every invocation to avoid transcript bloat. |

### 3. `PostInvocation`

| | |
|---|---|
| **Layout** | Flat array of hook entries |
| **When** | After each model call and its associated tool calls complete |
| **Extra `stdin` fields** | `invocationNum` (`int`), `initialNumSteps` (`int`) |
| **`stdout` fields** | `{"injectSteps": [...]}`, `"terminationBehavior": "force_continue" \| "terminate" \| ""` |
| **Notes** | `stdin` does **not** include the model's response text directly; read the last line(s) of `transcriptPath` to inspect. `"force_continue"` forces another model invocation even if the model made no tool calls; `"terminate"` ends the turn immediately. |

**Example — force continuation with injected context:**
```json
{
  "injectSteps": [
    { "ephemeralMessage": "Please verify the test output before finishing." }
  ],
  "terminationBehavior": "force_continue"
}
```

### 4. `PreToolUse`

| | |
|---|---|
| **Layout** | Grouped array with `"matcher"` regex + `"hooks"` array |
| **When** | Before each tool call whose tool name matches `"matcher"` (e.g., `"run_command"`, `"write_to_file\|replace_file_content"`, or `".*"`) |
| **Extra `stdin` fields** | `stepIdx` (`int`), `toolCall` (`{"name": "...", "args": {...}}`) |
| **`stdout` fields** | `"decision"` (`"allow"`, `"deny"`, `"ask"`, `"force_ask"`), `"reason"` (`string`), `"permissionOverrides"` (`string[]`), `"overwrite"` (`object`) |
| **Notes** | `"overwrite"` shallow-merges top-level keys into `toolCall.args`. Across multiple matching hooks, the most restrictive decision wins (`deny` > `force_ask` > `ask` > `allow`). Non-zero exit code also aborts the tool call. **Fail-CLOSED:** empty stdout, `{}`, malformed JSON, or non-zero exit all result in implicit deny. |

**Example — allow with argument rewrite:**
```json
{
  "decision": "allow",
  "reason": "Normalized command flags",
  "overwrite": {
    "CommandLine": "pytest -q"
  }
}
```

**Example — deny:**
```json
{
  "decision": "deny",
  "reason": "Blocked by policy"
}
```

### 5. `PostToolUse`

| | |
|---|---|
| **Layout** | Grouped array with `"matcher"` regex + `"hooks"` array |
| **When** | After each matching tool call finishes |
| **Extra `stdin` fields** | `stepIdx` (`int`), `toolCall` (`{"name": "...", "args": {...}}`), `error` (`string`, present if the tool errored) |
| **`stdout` fields** | `{}` *(observational only)* |
| **Notes** | `hooks.md` omits `toolCall` from its `PostToolUse` example, but empirically `toolCall` (`name` and `args`) **is** passed on `stdin`. The tool's output/result is not on `stdin` (read `transcript_full.jsonl` at `stepIdx` if needed). **Fail-open.** |

### 6. `Stop`

| | |
|---|---|
| **Layout** | Flat array of hook entries |
| **When** | When the agent loop is about to end the turn |
| **Extra `stdin` fields** | `executionNum` (`int`), `terminationReason` (`string`), `error` (`string`), `fullyIdle` (`bool`) |
| **`stdout` fields** | `"decision": "continue" \| "stop"`, `"reason"` (`string`) |
| **Notes** | While `hooks.md` shows `"model_stop"` as an example `terminationReason`, empirically normal completion passes `"NO_TOOL_CALL"`. Returning `{"decision": "continue", "reason": "..."}` blocks termination, injects `reason` as a `SYSTEM_MESSAGE` step, and starts a new invocation (up to the built-in continuation cap of 10, introduced v1.1.9). **Fail-open.** |

**Example — continue:**
```json
{
  "decision": "continue",
  "reason": "Uncommitted changes detected; please run the linter before stopping."
}
```

---

## Fail Behavior Summary

| Hook | Fail Behavior | Effect of Empty/Malformed/Non-Zero Exit |
|---|---|---|
| `SessionStart` | Fail-open | No injection, session proceeds |
| `PreInvocation` | Fail-open | No injection, invocation proceeds |
| `PostInvocation` | Fail-open | No injection, turn proceeds |
| `PreToolUse` | **Fail-CLOSED** | Implicit deny — tool call blocked |
| `PostToolUse` | Fail-open | Observational only |
| `Stop` | Fail-open | Turn ends normally |

---

## `hooks.json` Format Examples

### Flat hooks (SessionStart, PreInvocation, PostInvocation, Stop)

```json
{
  "my-plugin": {
    "enabled": true,
    "PreInvocation": [
      {
        "type": "command",
        "command": "/path/to/binary hook antigravity PreInvocation"
      }
    ],
    "Stop": [
      {
        "type": "command",
        "command": "/path/to/binary hook antigravity Stop"
      }
    ]
  }
}
```

### Grouped hooks (PreToolUse, PostToolUse)

```json
{
  "my-plugin": {
    "enabled": true,
    "PreToolUse": [
      {
        "matcher": "run_command|write_to_file",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/binary hook antigravity PreToolUse"
          }
        ]
      }
    ]
  }
}
```

### Key format notes

- Hook keys **must be PascalCase** (`Stop`, `PreToolUse`, not `stop`, `preToolUse`). Lowercase keys are silently ignored.
- The top-level key (e.g., `"my-plugin"`) is the plugin name. The `"enabled"` field controls whether hooks fire.

---

## Subagent Lifecycle & Hook Ordering (`invoke_subagent`)

Empirically validated in `E08` and `X8`:

1. Subagents spawned via `invoke_subagent` receive their own `conversationId` and `transcriptPath` and fire all 4 lifecycle hooks (`PreInvocation`, `PreToolUse`, `PostToolUse`, `Stop`).
2. **Exact hook firing order during `invoke_subagent` (`X8`):**
   - **Step 1:** `PreToolUse` (`toolCall.name == "invoke_subagent"`) fires in the **parent** session with `toolCall.args.Prompt`, `Role`, and `TypeName`.
   - **Step 2:** `PreInvocation` (`invocationNum == 0`, `initialNumSteps == 0`) fires in the **child** subagent session *before* the parent's initial prompt step is written to the child's `transcript.jsonl`.
   - **Step 3:** The child subagent executes its tool calls (`PreToolUse` / `PostToolUse` / `PreInvocation`) and eventually triggers `Stop` (`terminationReason == "NO_TOOL_CALL"`).
   - **Step 4:** `PostToolUse` (`toolCall.name == "invoke_subagent"`) fires in the **parent** session.
3. **Architectural implication for `repo-memory`:** Because the child's first `PreInvocation` fires at `initialNumSteps == 0` before its prompt is written to `transcript.jsonl`, `PreInvocation` starts the child with the baseline startup dossier (`.memory/ATLAS.md` + `global`) via `userMessage` and automatically injects task-specific scopes on subsequent `PreInvocation` calls once the child reads/touches files or receives its prompt step.

---

## Practical Patterns

### 1. Persistent dossier injection with `InjectedSet`, `DetectCompaction`, and `DetectRewind`

```go
// Check for transcript rewind (step count regression) or context compaction (CHECKPOINT)
if DetectRewind(steps, ledger.LastKnownStepCount) {
    ledger.RearmDedup()
}
ledger.LastKnownStepCount = len(steps)

if len(ledger.Injections) > 0 && len(steps) > 0 {
    res := DetectCompaction(steps, ledger)
    if res.Detected && res.CheckpointStepIdx > ledger.LastHandledCheckpointStep {
        reinjStep, err := BuildReinjection(ctx, idx, res.LostInjections, checkpointText, 1000)
        if err == nil && reinjStep != nil {
            injectSteps = append(injectSteps, *reinjStep)
            ledger.LastHandledCheckpointStep = res.CheckpointStepIdx
            ledger.RearmDedup()
        }
    }
}

// Engine filters out entries already present in ledger.InjectedSet
taskPayload, err := engine.EvaluateMoment(ctx, delivery.MomentTaskStart, taskCtx)
if err == nil && len(taskPayload.Entries) > 0 {
    // Inject as persistent UserMessage so entries survive across tool calls and turns
    injectSteps = append(injectSteps, InjectStep{
        UserMessage: taskPayload.FormattedText,
    })
}
```

### 2. Automatic Stage 1 milestone-gated turn note via `Stop` continuation

```go
if strings.EqualFold(input.TerminationReason, "NO_TOOL_CALL") && input.FullyIdle && input.ExecutionNum == 0 {
    if PassesMilestoneGate(stepsSinceMark) {
        return json.Marshal(map[string]string{
            "decision": "continue",
            "reason":   BuildTurnNotePrompt(priorNoteTitles),
        })
    }
}
return json.Marshal(map[string]string{"decision": "stop"})
```

### 3. Single-step `ephemeralMessage` for transient hints or error-match fixes

```go
// Used strictly for 1-step hints (about-to-edit pointers, error-match fixes, FTS warnings)
injectSteps = append(injectSteps, InjectStep{
    EphemeralMessage: errPayload.FormattedText,
})
```
