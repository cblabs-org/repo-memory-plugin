---
id: clm-01M3Z03CQB046FE335XGFXAZJS
type: finding
title: In Antigravity hook events, invocationNum resets to 0 on each new user prompt rather than incrementing across the entire session.
relevant_when: Designing deduplication, caching, or injection gating logic based on invocationNum.
apply: Track prompt-level transitions rather than assuming session-wide monotonic increments when checking invocationNum == 0.
status: established
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E03-transcript-currency.md
created: 2026-10-02T20:05:19.339143+01:00
updated: 2026-10-02T20:05:19.339143+01:00
---


