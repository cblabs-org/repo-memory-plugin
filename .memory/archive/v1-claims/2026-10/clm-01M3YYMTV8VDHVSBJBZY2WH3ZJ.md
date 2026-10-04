---
id: clm-01M3YYMTV8VDHVSBJBZY2WH3ZJ
type: architecture
title: Packages under internal/core/... must never import concrete harness adapters (internal/harness/antigravity, internal/harness/claudecode, internal/harness/fake), as enforced by an AST-based boundary test.
relevant_when: Adding imports or creating new packages under internal/core/.
apply: Ensure internal/core packages depend only on domain interfaces and never directly import concrete harness adapters.
status: active
flags: []
anchors:
  - kind: file
    ref: internal/core/boundary_test.go
created: 2026-10-02T19:39:53.704706+01:00
updated: 2026-10-02T20:12:08.004041+01:00
---


