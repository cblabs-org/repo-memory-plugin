---
id: clm-01M3YZK3C60JYR42XHEF1Y4YPC
type: decision
title: Bounded command excerpt truncation during compaction preserves failing markers or the last 5-10 lines of output for non-zero exit code contexts.
relevant_when: When truncating command output during transcript compaction.
apply: Retain failing markers and the tail 5-10 lines of output when exit codes are non-zero.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/pipeline/compact/compact.go
created: 2026-10-02T19:56:25.478672+01:00
updated: 2026-10-02T19:56:25.478672+01:00
---


