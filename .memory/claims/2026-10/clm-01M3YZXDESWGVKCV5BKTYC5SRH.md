---
id: clm-01M3YZXDESWGVKCV5BKTYC5SRH
type: convention
title: Spool entry timestamps in filenames must use format 20060102T150405.000000000Z_<kind>_<id>.json to maintain lexicographical chronological ordering without colons, which are forbidden on Windows.
relevant_when: Generating timestamped filenames for spool files or persisted storage on Windows-compatible filesystems.
apply: Format timestamps without colons (e.g., 20060102T150405.000000000Z) when constructing sortable file paths.
status: candidate
flags: []
anchors:
  - kind: file
    ref: internal/core/spool/entry.go
created: 2026-10-02T20:02:03.481987+01:00
updated: 2026-10-02T20:02:03.481987+01:00
---


