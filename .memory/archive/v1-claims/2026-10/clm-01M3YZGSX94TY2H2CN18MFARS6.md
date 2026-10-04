---
id: clm-01M3YZGSX94TY2H2CN18MFARS6
type: pitfall
title: 'Git does not permit combining ''--reverse'' with reflog walking options like ''git log -g''; doing so results in ''fatal: options --reverse and --walk-reflogs cannot be used together''. Any chronological sorting of reflog commits must be performed in application code.'
relevant_when: Traversing or querying git reflog entries in chronological order.
apply: Omit the '--reverse' flag when running git reflog or 'git log -g', and reverse the parsed commit list in-memory instead.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/commit_linkage.go
    fp: internal/core/episode/commit_linkage.go
created: 2026-10-02T19:55:10.249541+01:00
updated: 2026-10-02T19:55:10.249541+01:00
---


