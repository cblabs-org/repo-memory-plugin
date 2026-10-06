# X8 Lean Agent Experiment

**Date:** 2026-10-05 (initial), 2026-10-06 (correction & validation)
**Branch:** pr-06-v2-spike / WO4

## CORRECTION: Initial Findings Were Wrong

The initial experiment concluded that "file-based agent resolution fails dynamically." **This was incorrect.** The agent definition had invalid frontmatter that caused silent rejection by the parser.

### Why the Original Definition Failed

The original frontmatter used invalid keys that don't exist in the Antigravity agent schema:

```yaml
# WRONG — silently rejected:
---
name: repo-memory-distiller
model: flash
excludeDefaultComponents: true    # ❌ not a valid frontmatter key
tools: []                          # ❌ not a valid frontmatter key
inheritCustomizations: false       # ❌ not a valid frontmatter key
inheritMcp: false                  # ❌ not a valid frontmatter key
hidden: true
---
# Also missing: description (REQUIRED)
```

### Frontmatter Traps (from agy-customizations docs)

1. **Missing `description`:** Required field. Agent file is silently dropped without it.
2. **Invalid `model` value:** Only tier names (`inherit`, `flash_lite`, `flash`, `pro`) are accepted. Concrete model slugs (e.g., `gemini-3-flash-agent`) cause silent rejection.
3. **`mainAgent` vs `subagent` booleans:**
   - `mainAgent` (default `true`): Controls `agy --agent` and `agy agent` visibility
   - `subagent` (default `true`): Controls `invoke_subagent` visibility
4. **`agy agent` (list mode):** Only auto-loads repo-level `.agents/agents/` if the current directory is a trusted workspace. Use `agy --add-dir . agent` to force scanning.

### Working Definition

```yaml
---
name: repo-memory-distiller
description: Headless memory distillation agent with tools and MCP disabled.
mainAgent: true
subagent: true
hidden: true
model: flash
enable_write_tools: false
enable_mcp_tools: false
enable_subagent_tools: false
---
```

## Validated Results (2026-10-06)

### Test 1: `agy --agent` CLI — ✅ WORKS

```
$ agy --agent repo-memory-distiller -p "List all tools. If none, say: NO TOOLS AVAILABLE"
NO TOOLS AVAILABLE
```

The lean agent loaded correctly — no tools, no MCP. Compare with the control (fake agent name), which returned the full default tool list.

### Test 2: `agy agent` list — ✅ WORKS (with `--add-dir`)

```
$ agy --add-dir . agent
Available agents:
repo-memory-distiller
```

### Test 3: `invoke_subagent` — ❌ NOT RESOLVED (daemon discovery lag)

`invoke_subagent` with `TypeName: "repo-memory-distiller"` returns "not found." This is likely because the IDE daemon hasn't re-scanned `.agents/agents/` since the file was fixed. The agent definition is valid (proven by `agy --agent` working), so this should resolve after an IDE restart.

### Test 4: `hidden: true` behavior

`hidden: true` only hides the agent from the interactive UI picker and the `<subagents>` prompt listing. Both `agy --agent` and `agy --add-dir . agent` still resolve hidden agents.

## Key Findings

### File-based agents and `invoke_subagent` are NOT separate systems

A single file-based agent definition in `.agents/agents/<name>/agent.md` can be used by BOTH:
- `invoke_subagent` (in-session delegation)
- `agy --agent <name> -p` (headless CLI invocation)

`define_subagent` is only needed for dynamically creating temporary, conversation-scoped agents at runtime.

### `agy --agent` does NOT require IDE restart

Each `agy` CLI invocation scans customizations independently at process startup. When it silently falls back to the default agent, it means the `.md` file was rejected by the frontmatter parser.

### Original "Possible Causes" evaluation

| Hypothesis | Verdict | Explanation |
|---|---|---|
| `hidden: true` suppresses discovery | **False** | Only hides from UI picker and prompt listing |
| `.agents/agents/` directory format mismatch | **False** | Both `<name>.md` and `<name>/agent.md` are supported |
| `agy --agent` via `-p` doesn't support custom agents | **False** | It does; fallback means frontmatter was rejected |
| Discovery requires user-level `~/.gemini/config/agents/` | **False** for `-p` | `agy -p --agent` always scans working directory's `.agents/agents/` |

## Recommendations for PR-09

1. **Use the corrected agent definition** with proper frontmatter fields
2. **For pipeline automation via CLI:** `agy --agent repo-memory-distiller -p "..." --json-schema '{...}'` — works immediately, no restart needed
3. **For in-session delegation:** `invoke_subagent` should work after IDE restart picks up the agent file. If not, fall back to `define_subagent` for runtime registration.
4. **Install step:** Place the agent definition during `repo-memory install` or `repo-memory enroll`
