---
id: clm-01M3YYSNYABBE4JM1JVFDKAC5R
type: decision
title: The import boundary test in internal/core/boundary_test.go uses standard library go/parser and go/token packages instead of external dependency golang.org/x/tools/go/packages.
relevant_when: Inspecting Go package imports or extending architecture boundary tests.
apply: Use go/parser and go/token for parsing Go file ASTs to keep tests lightweight and portable.
status: accepted
flags: []
anchors:
  - kind: file
    ref: internal/core/boundary_test.go
created: 2026-10-02T19:42:32.522413+01:00
updated: 2026-10-02T20:12:24.808158+01:00
---


