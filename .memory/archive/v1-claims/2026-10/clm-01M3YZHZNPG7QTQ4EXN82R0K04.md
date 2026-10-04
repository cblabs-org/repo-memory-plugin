---
id: clm-01M3YZHZNPG7QTQ4EXN82R0K04
type: architecture
title: Commit linkage uses a candidate discovery window with a time margin around the session boundary coupled with confirmation against transcript commit commands (matching commit hashes, messages from -m/-am/--message, or command sequences) to avoid false positives from clock skew.
relevant_when: Linking repository commits to episode transcript sessions or reconciling git history with agent actions.
apply: Query commits on the session branch within [Started - timeMargin, Ended + timeMargin] and confirm them against successful commit commands in the transcript before recording them in the episode header.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/commit_linkage.go
    fp: internal/core/episode/commit_linkage.go
created: 2026-10-02T19:55:48.918326+01:00
updated: 2026-10-02T19:55:48.918326+01:00
---


