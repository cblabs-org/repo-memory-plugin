---
id: clm-01M3YZCDPR09T5Q061A0AQ4Z2J
type: architecture
title: Global lifecycle hooks execute across every repository on the host and incur a ~57ms latency penalty on non-enrolled repositories unless an O(1) directory marker check is performed before registry parsing.
relevant_when: Developing global lifecycle hooks to ensure minimal latency overhead across all developer repositories.
apply: Check for repository marker files (such as `.memory/`) upfront before reading registry files or executing hook logic.
status: proposed
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E05.md
    fp: E05
created: 2026-10-02T19:52:46.680572+01:00
updated: 2026-10-02T19:58:12.362812+01:00
---


