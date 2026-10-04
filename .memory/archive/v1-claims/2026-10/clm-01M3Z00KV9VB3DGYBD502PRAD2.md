---
id: clm-01M3Z00KV9VB3DGYBD502PRAD2
type: finding
title: Antigravity transcript step indices contain gaps due to non-serialized internal dispatch steps, and parallel tool calls cause step entries to be written out of monotonic step-index order.
relevant_when: Parsing transcript step indices or validating transcript step sequence.
apply: Do not assume contiguous step indices or monotonic ordering when parsing transcript.jsonl or transcript_full.jsonl.
status: established
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E12-transcript-file-set.md
created: 2026-10-02T20:03:48.329385+01:00
updated: 2026-10-02T20:03:48.329385+01:00
---


