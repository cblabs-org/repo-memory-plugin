---
id: clm-01M3Z077KMXHDSNZ6586XJ28G4
type: architecture
title: Cobra CLI subcommands (hook, tick, mcp, install, uninstall, search, report, status, reindex, doctor) follow a modular structure with dedicated cmd_<name>.go files wired to rootCmd in main.go.
relevant_when: Adding, extending, or maintaining CLI subcommands in cmd/repo-memory.
apply: Define command entry points, flags, and argument parsing in cmd/repo-memory/cmd_<name>.go and wire them to rootCmd in main.go.
status: active
flags: []
anchors:
  - kind: file
    ref: cmd/repo-memory/main.go
created: 2026-10-02T20:07:25.173043+01:00
updated: 2026-10-02T20:07:25.173043+01:00
---


