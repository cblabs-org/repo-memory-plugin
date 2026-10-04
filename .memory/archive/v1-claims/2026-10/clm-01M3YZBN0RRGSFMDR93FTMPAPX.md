---
id: clm-01M3YZBN0RRGSFMDR93FTMPAPX
type: architecture
title: Antigravity sidecars run exclusively as IDE child processes under `language_server multicall schedule` with 60s ticking; the agy CLI does not spawn or execute sidecars.
relevant_when: Designing recurring background tasks, housekeeping, or scheduling for Antigravity workspaces.
apply: Require an external OS scheduler (cron or launchd) for CLI-only environments, and enforce tick.lock concurrency control in IDE environments.
status: proposed
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E06.md
    fp: E06
created: 2026-10-02T19:52:21.400068+01:00
updated: 2026-10-02T19:56:52.291715+01:00
---


