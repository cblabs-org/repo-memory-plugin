---
id: clm-01M3YZ42PEFPJYAXDG2NYFX6BR
type: constraint
title: Packages in internal/core must never import packages in internal/harness.
relevant_when: Defining package imports or module dependencies between core logic and harness implementations.
apply: Keep core abstractions self-contained and ensure dependency arrows strictly point inward toward core.
status: proposed
flags: []
anchors:
  - kind: path
    ref: internal/core
    fp: internal/core
  - kind: path
    ref: internal/harness
    fp: internal/harness
created: 2026-10-02T19:48:13.262726+01:00
updated: 2026-10-02T19:48:13.262727+01:00
---


