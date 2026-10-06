# Antigravity Hooks Reference

> Empirically validated reference for Antigravity hook events, `injectSteps` message injection,
> and `stdin`/`stdout` JSON schemas. Sourced from `~/.gemini/antigravity/builtin/skills/agy-customizations/docs/hooks.md`,
> `agy -p "/changelog"`, and direct experimentation (PR-06 spike, October 2026).

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

---

## `hooks.json` Discovery Paths

Hooks are discovered from (in order):
1. `~/.gemini/antigravity/hooks.json` (global)
2. `<workspace>/.agents/hooks.json` (workspace-level)
3. `<plugin-root>/hooks.json` (per-plugin, loaded via `plugin.json` or `.agents/plugins.json`)

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
agent's context via the `injectSteps` response field. Three message types are supported:

### A. `ephemeralMessage` — Single-Invocation Scope

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
- **Persistence:** Visible to the model on the immediate invocation only. Automatically excluded
  from subsequent model calls once a `PLANNER_RESPONSE` step is recorded after it.
- **Best for:** Per-invocation dynamic context or status reminders that should not accumulate.

### B. `userMessage` — Persistent Session Scope

```json
{
  "injectSteps": [
    {
      "userMessage": "Additional user-role message."
    }
  ]
}
```

- **Value type:** Plain string
- **Transcript:** Appends a `USER_INPUT` step (`"source": "SYSTEM_SDK"`)
- **Persistence:** Remains in the conversation history across subsequent invocations and turns
  (until context compaction/CHECKPOINT).
- **Best for:** Durable user-role instructions injected by hooks.

### C. `systemMessage` — Persistent Session Scope

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
  `systemMessage` **must be a nested object** containing a `"systemMessage"` string field
  and an optional `"metadata"` key-value object. Passing a bare string
  (`{"systemMessage": "..."}`) fails silently.
- **Transcript:** Appends a persistent `SYSTEM_MESSAGE` step prefixed with
  `"The following system message was injected by a <Event> hook:\n..."` (rendered to the
  model inside `<SYSTEM_MESSAGE>` tags).
- **Persistence:** Persists in the conversation history across subsequent invocations and turns
  until compaction. Injecting on every `PreInvocation` will accumulate duplicate steps if
  not guarded (e.g., by checking `invocationNum == 0`).
- **Best for:** Dossier delivery, system-level context, persistent instructions.

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
      "systemMessage": {
        "systemMessage": "Persistent system context.",
        "metadata": {"source": "repo-memory"}
      }
    },
    {
      "ephemeralMessage": "One-shot reminder for this invocation."
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

## Practical Patterns

### Inject dossier on first invocation only

```go
if input.InvocationNum > 0 {
    return []byte("{}"), nil  // no-op for subsequent invocations
}
response := InjectStepsResponse{
    InjectSteps: []InjectStep{{
        SystemMessage: &SystemMessageEntry{
            SystemMessage: dossierContent,
            Metadata: map[string]string{"source": "repo-memory"},
        },
    }},
}
```

### Turn-note harvesting via Stop continuation

```go
if input.TerminationReason == "NO_TOOL_CALL" && input.FullyIdle && input.ExecutionNum == 0 {
    return json.Marshal(map[string]string{
        "decision": "continue",
        "reason":   "Please write a memory note summarizing this conversation.",
    })
}
return json.Marshal(map[string]string{"decision": "stop"})
```

### Per-invocation ephemeral status

```go
response := InjectStepsResponse{
    InjectSteps: []InjectStep{{
        EphemeralMessage: "You are currently in a memory capture continuation. " +
            "Write the note and then stop.",
    }},
}
```
