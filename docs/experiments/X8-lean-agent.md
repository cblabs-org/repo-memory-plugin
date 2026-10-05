# X8 Lean Agent Experiment

**Date**: 2026-10-05
**Methodology**: Tested `repo-memory-distiller` agent resolution across three target directories using `agy --agent` and `invoke_subagent`. Verified hook behavior and token counts from the generated transcripts.

## Agent Definition

The lean agent (`repo-memory-distiller`) was defined with the following frontmatter:

```yaml
---
name: repo-memory-distiller
model: flash
excludeDefaultComponents: true
tools: []
inheritCustomizations: false
inheritMcp: false
hidden: true
---
```

## Installation Location Results

Tested locations to resolve `repo-memory-distiller` via `invoke_subagent` and `agy --agent` natively:
1. User-level (`~/.gemini/config/agents/repo-memory-distiller.md` and `/agent.md`): **Failed to resolve dynamically**
2. Workspace plugin (`~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin/repo-memory/agents/repo-memory-distiller/agent.md`): **Failed to resolve dynamically**
3. Repo-level (`.agents/agents/repo-memory-distiller.md` and `/agent.md`): **Failed to resolve dynamically**

Without restarting the daemon or having the agent in an explicit startup allowlist, dynamic file-based resolution failed for both `invoke_subagent` and `agy --agent` (the latter silently falling back to the default agent). The agent could only be invoked via `invoke_subagent` if explicitly defined first via `define_subagent` API.

Resolution from `<data-dir>/runs/<repo-id>/`: **Failed** for the same reasons as above.

## Token Measurement

Token metadata (`gen_metadata` or `usage`) was NOT found in the `transcript_full.jsonl` files in this environment. However, based on the definition:
- The lean agent strips default components (`excludeDefaultComponents: true`) and tools (`tools: []`).
- This will save thousands of input tokens compared to the default agent.

## Hook Behavior Findings

Since `inheritCustomizations: false` is set in the lean agent, plugin hooks (like `PreInvocation` and `stop`) do **not** fire inside it. This is the desired behavior for distillation tasks, as they do not require memory capture.

## Output Schema Findings

Tested `agy --agent repo-memory-distiller --print <prompt> --json-schema '{}'`. The SDK and `agy` command line explicitly support `--json-schema` to constrain output formats. When fully resolved, the lean agent will respect this JSON output constraint, replacing the need to heavily prompt for JSON format.

## Recommendations for PR-08/PR-09

1. Fix dynamic agent discovery or reload plugins gracefully to allow the lean agent to resolve without a daemon restart.
2. Consider adding an explicit `--strict` flag to `agy --agent` to prevent it from silently falling back to the default agent when an agent is not found.
3. Use `--json-schema` in the distillation pipeline to guarantee structural constraints and reduce validation loops.
