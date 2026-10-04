---
id: clm-01M3YYRWH7GTSRZQV4XBV6G9AF
type: gotcha
title: Hook execution latency on non-enrolled repositories has a p50 of ~57ms compared to ~3ms on enrolled repositories, requiring fast-path directory detection.
relevant_when: Executing hooks across mixed environments with enrolled and non-enrolled repositories.
apply: Implement a fast-path kill-switch or an O(1) directory marker check at the start of hook execution to minimize overhead.
status: active
flags: []
anchors:
  - kind: file
    ref: /Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/PROJECT.md
created: 2026-10-02T19:42:06.503612+01:00
updated: 2026-10-02T19:42:06.503612+01:00
---


