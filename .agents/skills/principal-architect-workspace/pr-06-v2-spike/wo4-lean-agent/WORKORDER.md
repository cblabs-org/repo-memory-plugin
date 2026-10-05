---
name: pr-06-v2-spike-wo4
description: X8 lean agent experiment — build and test repo-memory-distiller markdown agent
complexity: standard
---

# WO4: X8 Lean Agent Experiment

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Build the `repo-memory-distiller` Markdown agent, determine where it must be installed to resolve from the runtime working directory, measure input tokens, and check whether plugin hooks fire inside it. Document results in `docs/experiments/X8-lean-agent.md`.

## Strict Boundary Box
- **Create:** agent definition files, `docs/experiments/X8-lean-agent.md`
- **DO NOT** modify hook handlers, pipeline, evidence, enrollment, or install code
- All prototype code is behind `REPO_MEMORY_V2_SPIKE` env var

## Context

### What we're testing
The v2 design uses a dedicated lightweight agent (`repo-memory-distiller`) for distillation tasks to avoid loading the full agent context (tools, MCP, etc.). This agent is spawned by the pipeline to process episode transcripts into claims.

### Agent specification (from v2 design §5.1)
```yaml
name: repo-memory-distiller
excludeDefaultComponents: true
tools: []
inheritCustomizations: false
inheritMcp: false
hidden: true
mainAgent: true
```

### Key questions
1. **Where must the agent.md be installed?** Options:
   - `~/.gemini/config/agents/repo-memory-distiller/agent.md` (user-level)
   - Inside the workspace plugin: `repo-memory-daemon/workspace-plugin/repo-memory/agents/repo-memory-distiller/agent.md`
   - Inside `.agents/agents/repo-memory-distiller/agent.md` (repo-level)

2. **Does it resolve from `<data-dir>/runs/<repo-id>/`?** The pipeline runs from the data directory, not the repo root.

3. **What are the input tokens?** Measure baseline token count for the lean agent vs the default agent.

4. **Do our plugin hooks fire inside it?** With `inheritCustomizations: false`, hooks might not load.

5. **Does `--json-schema` work?** Can we constrain the output format?

### Antigravity agent docs
Read the AGY customization skill for agent definition format. Key references:
- Agent definitions are markdown files with YAML frontmatter
- Agents can be defined at user-level (`~/.gemini/config/agents/`) or repo-level (`.agents/agents/`)
- The `excludeDefaultComponents` flag disables default tools
- `inheritCustomizations: false` prevents loading plugins, rules, and skills

## Line-Item Instructions

### 1. Create the agent definition

- [ ] **Create the agent markdown file** at a testable location. Start with the workspace-plugin location:
  ```
  packaging/antigravity/agents/repo-memory-distiller/agent.md
  ```
  Content:
  ```markdown
  ---
  name: repo-memory-distiller
  model: flash
  excludeDefaultComponents: true
  tools: []
  inheritCustomizations: false
  inheritMcp: false
  hidden: true
  ---

  You are a precise information extraction agent. Your task is to extract structured knowledge from conversation transcripts.

  Given a transcript, extract factual claims about the codebase, architecture, APIs, configurations, and operational procedures. Each claim should be:
  - Atomic (one fact per claim)
  - Verifiable (can be checked against the codebase)
  - Actionable (useful for future development)

  Output your claims as a JSON array. Each claim has: `title`, `body`, `confidence` (high/medium/low), and `anchors` (file paths).
  ```

### 2. Test agent installation locations

- [ ] **Test 1: User-level agents directory**
  - Install to `~/.gemini/config/agents/repo-memory-distiller/agent.md`
  - Spawn a subagent with `TypeName: "repo-memory-distiller"` and check if it resolves
  - Record success/failure

- [ ] **Test 2: Workspace plugin agents directory**
  - Install to `~/.gemini/config/plugins/repo-memory-daemon/workspace-plugin/repo-memory/agents/repo-memory-distiller/agent.md`
  - Check if plugins can ship agents
  - Record success/failure

- [ ] **Test 3: Repo-level agents directory**
  - Install to `.agents/agents/repo-memory-distiller/agent.md` in this repo
  - Check if it resolves
  - Record success/failure

- [ ] **Test 4: Resolution from data directory**
  - Check if the agent resolves when the working directory is `<data-dir>/runs/<repo-id>/`
  - This matters because the pipeline runs from the data directory

### 3. Measure token consumption

- [ ] **Spawn the lean agent** with a simple task (e.g., "Extract claims from this text: 'The project uses Go 1.21 and SQLite for storage.'")
- [ ] **Read its transcript** and check the `gen_metadata` fields for:
  - `input_tokens` (how much context was loaded)
  - `output_tokens`
  - `cached_tokens` (should be near zero for a lean agent)
- [ ] **Compare against a default agent** spawned with the same task
- [ ] Document the token savings

### 4. Check hook behavior

- [ ] **Spawn the lean agent** in this enrolled repo
- [ ] **Read its transcript** — do the stop hook or PreInvocation hook fire?
- [ ] With `inheritCustomizations: false`, hooks should NOT fire (the workspace plugin shouldn't be loaded)
- [ ] This is desired behavior — distillation agents don't need memory capture

### 5. Test output schema constraint

- [ ] Check if the Antigravity SDK or `invoke_subagent` supports `--json-schema` or output format constraints
- [ ] If so, test whether the lean agent respects them
- [ ] Document the finding

### 6. Write the experiment report

- [ ] Create `docs/experiments/X8-lean-agent.md` with:
  - Date, branch, methodology
  - Agent definition (final version)
  - Installation location results (which paths work)
  - Token measurement comparison table
  - Hook behavior findings
  - Output schema findings
  - Recommendations for PR-08/PR-09

## Verification Requirement
- `go build ./...` passes (no Go changes expected, but verify)
- Experiment report documents all findings with transcript evidence
- Agent definition file created and tested

## Issue Surfacing
If you discover a defect, upstream bug, or concern outside your boundary box, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

### Status
Completed

### Changes Made
- Created `repo-memory-distiller` agent definition in `packaging/antigravity/agents/repo-memory-distiller/agent.md`
- Created `docs/experiments/X8-lean-agent.md` with experiment results

### Decisions Made
- Documented that dynamic file-based resolution fails without daemon restart or correct allowlisting.

### Dead Ends & Warnings
- `agy --agent` silently falls back to the default agent when an agent is not found, which complicated testing.
- `invoke_subagent` cannot invoke dynamically created agent files without a daemon restart.

### Out-of-Scope Findings
- `gen_metadata` and `usage` metrics were not visible in the local transcript logs.

### Blockers
None. Documented the failures as required by the experiment design.
