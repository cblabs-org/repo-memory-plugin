# packaging/antigravity/agents

## Ruled out, failure signatures and gotchas

### [seed-1a1220d1] `invoke_subagent` cannot invoke dynamically created agent files without a daemon restart.
`ruled-out · kind: episodic · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`

### [seed-620d211e] `agy --agent` silently falls back to the default agent when an agent is not found, which complicated testing.
`ruled-out · kind: episodic · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`

## Decisions

### [seed-45ddb20f] Removed flat `title` from `op.Fields` after synthesizing `text` via `FormatCandidateEntryText` so `dossiers.Projector.applyAdd` does not overwrite the dossier's top-level `Title` with an individual entry's title.
`decision · kind: episodic · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/harness/antigravity/executor_test.go, packaging/antigravity/agents/repo-memory-distiller/agent.md, internal/core/consolidate/llm.go, internal/core/consolidate/llm_test.go`

### [seed-ca0efc16] Documented that dynamic file-based resolution fails without daemon restart or correct allowlisting.
`decision · kind: episodic · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`

## Open issues

### [seed-8da94911] `gen_metadata` and `usage` metrics were not visible in the local transcript logs.
`open-issue · kind: semantic · anchors: packaging/antigravity/agents/repo-memory-distiller/agent.md, docs/experiments/X8-lean-agent.md`
