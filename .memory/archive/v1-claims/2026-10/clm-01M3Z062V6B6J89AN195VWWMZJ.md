---
id: clm-01M3Z062V6B6J89AN195VWWMZJ
type: decision
title: Plugin directory resolution checks REPO_MEMORY_PLUGIN_DIR and ANTIGRAVITY_CONFIG_DIR environment variables before falling back to default user directories to support hermetic unit testing.
relevant_when: Resolving Antigravity plugin installation directory during install/uninstall operations or testing directory resolution.
apply: Check REPO_MEMORY_PLUGIN_DIR and ANTIGRAVITY_CONFIG_DIR environment variables before falling back to ~/.gemini/config/plugins.
status: active
flags: []
anchors:
  - kind: file
    ref: cmd/repo-memory/cmd_install.go
  - kind: file
    ref: cmd/repo-memory/cmd_uninstall.go
created: 2026-10-02T20:06:47.526592+01:00
updated: 2026-10-02T20:06:47.526592+01:00
---


