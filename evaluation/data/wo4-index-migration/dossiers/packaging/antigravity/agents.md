# packaging/antigravity/agents

## Ruled out, failure signatures and gotchas

### [seed-1a1220d1] `invoke_subagent` cannot invoke dynamically created agent files without a daemon restart.
`ruled-out · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`

### [seed-620d211e] `agy --agent` silently falls back to the default agent when an agent is not found, which complicated testing.
`ruled-out · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`

## Decisions

### [seed-ca0efc16] Documented that dynamic file-based resolution fails without daemon restart or correct allowlisting.
`decision · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`

## Open issues

### [seed-8da94911] `gen_metadata` and `usage` metrics were not visible in the local transcript logs.
`open-issue · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`
