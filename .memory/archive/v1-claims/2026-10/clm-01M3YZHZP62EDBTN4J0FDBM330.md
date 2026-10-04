---
id: clm-01M3YZHZP62EDBTN4J0FDBM330
type: decision
title: Deterministic transcript compaction in internal/core/pipeline/compact/ strips exposure marker text while retaining IDs, collapses duplicate repeating log lines, and scrubs secrets using regex and entropy heuristics into «secret:type» placeholders to achieve >=4x footprint reduction.
relevant_when: When compacting execution transcripts or modifying the pipeline compaction logic.
apply: Follow Compactor.Compact() and scrubSecrets() rules in internal/core/pipeline/compact/compact.go.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/pipeline/compact/compact.go
  - kind: file
    ref: internal/core/pipeline/compact/compact_test.go
created: 2026-10-02T19:55:48.934059+01:00
updated: 2026-10-02T19:55:48.934059+01:00
---


