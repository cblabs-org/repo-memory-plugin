---
id: clm-01M3Z062TF4W0AXYDAF15M25F0
type: decision
title: RenderTemplate in packaging/antigravity/templates.go must escape backslashes in Windows file paths before template substitution into JSON templates to ensure valid JSON across all platforms.
relevant_when: Rendering packaging templates or substituting filesystem paths into JSON configuration templates.
apply: Escape backslashes in paths prior to substituting them into JSON templates.
status: active
flags: []
anchors:
  - kind: file
    ref: packaging/antigravity/templates.go
created: 2026-10-02T20:06:47.503987+01:00
updated: 2026-10-02T20:06:47.503987+01:00
---


