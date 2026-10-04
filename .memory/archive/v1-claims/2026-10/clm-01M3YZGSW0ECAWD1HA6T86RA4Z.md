---
id: clm-01M3YZGSW0ECAWD1HA6T86RA4Z
type: fact
title: Antigravity transcripts grow append-only, but step indices may have gaps due to unwritten internal dispatch steps, parallel tool calls can produce out-of-order step indices in JSONL, and session resets restart indexing at 0.
relevant_when: Ingesting, parsing, or reconciling Antigravity transcript files.
apply: Do not treat step indices as strictly contiguous or monotonic, and handle index reinitialization across session resets.
status: proposed
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E12.md
    fp: E12
created: 2026-10-02T19:55:10.208709+01:00
updated: 2026-10-02T19:57:51.819817+01:00
---


