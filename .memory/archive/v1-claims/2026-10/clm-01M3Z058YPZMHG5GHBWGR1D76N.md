---
id: clm-01M3Z058YPZMHG5GHBWGR1D76N
type: finding
title: Ephemeral messages injected in Antigravity persist only for the single invocation where injected; they are written to transcript_full.jsonl as EPHEMERAL_MESSAGE steps with source SYSTEM_SDK, but are not retained in model context on subsequent invocations.
relevant_when: Managing multi-turn memory or recurring context injections via ephemeral messages.
apply: Re-inject ephemeral messages on each user prompt (invocationNum == 0) if information must persist across turns.
status: established
flags: []
anchors:
  - kind: file
    ref: docs/experiments/E01-ephemeral-message.md
created: 2026-10-02T20:06:21.014119+01:00
updated: 2026-10-02T20:06:21.014119+01:00
---


