---
id: clm-01M3YZZRTJ1SAHMMZBYR8KSQ8G
type: finding
title: In Antigravity, transcript_full.jsonl is append-only during normal conversation flow, enabling safe cursor-based incremental reading without re-reading prefix bytes.
relevant_when: Reading Antigravity transcripts or designing transcript parser caching strategies.
apply: Use prefix byte tracking and cursor-based reads against transcript_full.jsonl during normal flow.
status: established
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E12-transcript-file-set.md
created: 2026-10-02T20:03:20.658262+01:00
updated: 2026-10-02T20:03:20.658262+01:00
---


