---
id: clm-01M3Z02SHDP9NV52HZS5WDPK9M
type: finding
title: The transcriptPath payload parameter in Antigravity hook events always references transcript_full.jsonl rather than transcript.jsonl.
relevant_when: Handling Antigravity hook payloads that supply transcript paths.
apply: Expect transcriptPath to point to transcript_full.jsonl and use it for hook-triggered transcript inspection.
status: established
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E03-transcript-currency.md
created: 2026-10-02T20:04:59.693735+01:00
updated: 2026-10-02T20:04:59.693735+01:00
---


