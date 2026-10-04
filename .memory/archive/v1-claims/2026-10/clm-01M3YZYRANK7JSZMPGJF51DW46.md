---
id: clm-01M3YZYRANK7JSZMPGJF51DW46
type: decision
title: Application versioning is defined in cmd/repo-memory/main.go via a package-level Version variable wired to rootCmd.Version, injected at build time using Makefile ldflags (-ldflags="-X 'main.Version=$(VERSION)'") defaulting to 'dev'.
relevant_when: Building binaries, creating releases, or modifying version querying logic.
apply: Maintain the package-level Version variable in cmd/repo-memory/main.go and pass VERSION through -ldflags in Makefile target recipes.
status: active
flags: []
anchors:
  - kind: file
    ref: Makefile
  - kind: file
    ref: cmd/repo-memory/main.go
created: 2026-10-02T20:02:47.381175+01:00
updated: 2026-10-02T20:07:50.779296+01:00
---


