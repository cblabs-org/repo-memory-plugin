---
id: clm-01M3Z0PG5WNSH1RV3C0A7CQN4S
type: decision
title: The hook fast-path must check the kill-switch and enrollment cache before querying the enrollment database.
relevant_when: Designing or optimizing invocation hook latency for enrolled and non-enrolled repositories.
apply: Place the kill-switch check first, followed by an in-memory enrollment cache lookup, to avoid database query overhead on non-enrolled repos.
status: proposed
flags: []
anchors:
  - kind: file
    ref: /Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/wo3-runtime-and-cost/WORKORDER.md
    fp: pr-01b-wo3
created: 2026-10-02T20:15:45.468421+01:00
updated: 2026-10-02T20:15:45.468421+01:00
---


