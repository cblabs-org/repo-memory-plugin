---
id: clm-01M3YZT3CBZBFGF0AJZ72NNZCS
type: architecture
title: Compacted episodes must be stored at `<data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst` using pure-Go zstd compression (`github.com/klauspost/compress/zstd`) without cgo.
relevant_when: Persisting, locating, or reading compacted episode files.
apply: Use Store in internal/core/episode/store.go with pure-Go zstd compression following the multi-year directory layout.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/store.go
    fp: internal/core/episode/store.go
created: 2026-10-02T20:00:14.859674+01:00
updated: 2026-10-02T20:00:14.859674+01:00
---


