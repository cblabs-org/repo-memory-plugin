---
id: clm-01M3YZW81DEME58FG636ZB9QNG
type: architecture
title: Reading episode records uses a chunk-based line reader (`readLineBytes`) with `bufio.Reader` to support arbitrarily long JSON lines without buffer overflow.
relevant_when: Streaming or reading lines from compacted episode files.
apply: Use streaming chunk-based line reading rather than fixed-size bufio.Scanner when decoding episode JSONL entries.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/store.go
    fp: internal/core/episode/store.go
created: 2026-10-02T20:01:25.165937+01:00
updated: 2026-10-02T20:01:25.165937+01:00
---


