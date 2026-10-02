---
id: clm-01M3Z0N66WZSWR696RVSF3YX6E
type: quirk
title: Antigravity transcript step indices contain gaps due to un-serialized internal dispatch steps, and parallel tool calls cause step indices to be written out of order.
relevant_when: Parsing or validating step indices in Antigravity transcript files.
apply: Do not assume transcript step indices are contiguous (line_count == max_step_index + 1) or monotonically increasing.
status: candidate
flags: []
anchors:
  - kind: file
    ref: /Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/wo1-transcript-and-hooks/WORKORDER.md
created: 2026-10-02T20:15:02.492477+01:00
updated: 2026-10-02T20:15:02.492478+01:00
---


