---
id: clm-01M3YYMM60NR53CGC5SGB4E4SP
type: architecture
title: Core packages must not import concrete adapters; this architectural boundary is enforced via an AST-based boundary test in internal/core/boundary_test.go, with exceptions granted only to test files importing the fake adapter.
relevant_when: Adding imports or creating new packages under internal/core or internal/harness.
status: active
flags: []
anchors:
  - kind: file
    ref: internal/core/boundary_test.go
  - kind: file
    ref: internal/harness/fake
created: 2026-10-02T19:39:46.880417+01:00
updated: 2026-10-02T19:42:48.271555+01:00
---


