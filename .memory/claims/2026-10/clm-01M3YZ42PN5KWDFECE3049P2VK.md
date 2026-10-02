---
id: clm-01M3YZ42PN5KWDFECE3049P2VK
type: decision
title: 'Transcript reading must follow priority order: transcript_full.jsonl first; then lexical concatenation of chunks/transcript_full/*.jsonl as an io.MultiReader byte stream before line splitting; and finally transcript.jsonl with truncated_fields flagged.'
relevant_when: Parsing Antigravity agent execution transcripts from disk.
apply: Concatenate chunk files into a single byte stream prior to line splitting to preserve records straddling file boundaries.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/harness/antigravity/transcript.go
    fp: internal/harness/antigravity/transcript.go
created: 2026-10-02T19:48:13.269906+01:00
updated: 2026-10-02T19:48:13.269906+01:00
---


