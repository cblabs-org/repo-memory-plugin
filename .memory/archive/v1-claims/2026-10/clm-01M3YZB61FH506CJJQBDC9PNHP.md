---
id: clm-01M3YZB61FH506CJJQBDC9PNHP
type: architecture
title: 'PreToolUse lifecycle hooks must explicitly return {"decision": "allow"} to permit tool execution; returning an empty object {} or a non-zero exit code is treated as an implicit denial.'
relevant_when: Implementing or debugging Antigravity PreToolUse lifecycle hooks for agents or subagents.
apply: 'Always output {"decision": "allow"} on success from PreToolUse hooks rather than an empty JSON object.'
status: proposed
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E08.md
    fp: E08
  - kind: file
    ref: docs/experiments/E09.md
    fp: E09
created: 2026-10-02T19:52:06.063937+01:00
updated: 2026-10-02T19:57:25.098583+01:00
---


