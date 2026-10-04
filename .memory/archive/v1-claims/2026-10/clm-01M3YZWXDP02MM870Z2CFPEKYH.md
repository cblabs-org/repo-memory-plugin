---
id: clm-01M3YZWXDP02MM870Z2CFPEKYH
type: pitfall
title: On Windows, open file handles must be synced and closed before calling os.Rename to avoid ERROR_ACCESS_DENIED, and renames should use retry backoff against transient indexer locks.
relevant_when: Writing or renaming files atomically across platforms including Windows.
apply: Sync and close file handles before invoking os.Rename, and wrap rename calls in a retry loop with backoff.
status: candidate
flags: []
anchors:
  - kind: file
    ref: internal/core/spool/writer.go
created: 2026-10-02T20:01:47.062035+01:00
updated: 2026-10-02T20:01:47.062035+01:00
---


