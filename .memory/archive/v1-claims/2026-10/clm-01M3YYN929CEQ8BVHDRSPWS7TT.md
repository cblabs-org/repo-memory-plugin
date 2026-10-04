---
id: clm-01M3YYN929CEQ8BVHDRSPWS7TT
type: constraint
title: The install command scaffolds the scheduled sidecar configuration in sidecars/tick/sidecar.json but must never auto-enable it, instructing the user to enable it manually via the Antigravity Automations dashboard.
relevant_when: Installing or configuring repo-memory under the Antigravity harness.
apply: Write sidecar.json without auto-enabling it and print clear user guidance to stdout on enabling it in the dashboard.
status: active
flags: []
anchors:
  - kind: file
    ref: packaging/antigravity/sidecar.json
created: 2026-10-02T19:40:08.265201+01:00
updated: 2026-10-02T20:07:09.080938+01:00
---


