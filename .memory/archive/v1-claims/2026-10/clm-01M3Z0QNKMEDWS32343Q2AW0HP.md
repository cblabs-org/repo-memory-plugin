---
id: clm-01M3Z0QNKMEDWS32343Q2AW0HP
type: decision
title: Housekeeping CLI calls should reuse a single persistent conversation per workspace using `--conversation <id>` to enable prefix caching and control token costs.
relevant_when: Invoking `agy -p` for automated housekeeping tasks.
apply: Pass `--conversation <id>` to persist and reuse a dedicated housekeeping conversation instead of creating fresh conversations per run.
status: proposed
flags: []
anchors:
  - kind: file
    ref: /Users/mcneillm/Documents/Projects/repo-memory-plugin/.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/wo3-runtime-and-cost/WORKORDER.md
    fp: pr-01b-wo3
created: 2026-10-02T20:16:23.796125+01:00
updated: 2026-10-02T20:16:23.796125+01:00
---


