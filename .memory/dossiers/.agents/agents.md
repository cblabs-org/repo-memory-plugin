# .agents/agents

## Invariants & contracts

### [docseed-23493cf3] Markdown custom agents (`repo-memory-distiller`) shipped in `.agents/agents/` work with `agy -p --agent repo-memory-distiller` and cut fixed input overhead by ~38% (~6.6k tokens saved per invocation) by setting `model: flash`, `enable_write_tools: false`, `enable_mcp_tools: false`, and `enable_subagent_tools: false`. Multi-turn `--input-format stream-json --output-format stream-json` reuses the...
`invariant · kind: semantic · anchors: .agents/agents · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`
