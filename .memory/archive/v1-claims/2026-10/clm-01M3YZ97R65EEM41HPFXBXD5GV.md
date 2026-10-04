---
id: clm-01M3YZ97R65EEM41HPFXBXD5GV
type: pattern
title: Use bufio.Scanner configured with an expanded buffer (e.g., 50MB) on top of io.MultiReader when parsing chunked transcript JSON lines.
relevant_when: Scanning concatenated JSONL streams that may contain very large step records.
apply: Initialize bufio.NewScanner with a buffer sized for large payloads to prevent bufio.ErrTooLong errors.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/harness/antigravity/transcript.go
    fp: internal/harness/antigravity/transcript.go
created: 2026-10-02T19:51:02.27831+01:00
updated: 2026-10-02T19:51:02.27831+01:00
---


