---
id: clm-01M3Z00KVZXQ0MA88C4DQF4FK7
type: finding
title: Antigravity transcripts have zero lag at PreInvocation and PreToolUse hook execution points, with PreToolUse transcripts capturing the current turn's thinking/reasoning block.
relevant_when: Accessing transcript state inside PreInvocation or PreToolUse hooks, or implementing reasoning-aware tool gating.
apply: Read latest model reasoning directly from transcript_full.jsonl within PreToolUse hooks without delay.
status: established
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E03-transcript-currency.md
created: 2026-10-02T20:03:48.3519+01:00
updated: 2026-10-02T20:03:48.3519+01:00
---


