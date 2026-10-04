---
id: clm-01M3YYTC1KYWMHQMNCBJ6AEKPF
type: fact
title: Injected ephemeral messages in Antigravity are scoped strictly per-invocation and not remembered by the model on subsequent turns, though they are recorded in `transcript_full.jsonl` under step type `EPHEMERAL_MESSAGE` with source `SYSTEM_SDK`.
relevant_when: Designing context injection strategies or implementing transcript parsers for Antigravity.
apply: Re-inject ephemeral context on each new prompt/invocation when needed, and support `EPHEMERAL_MESSAGE` and `SYSTEM_SDK` when parsing transcript steps.
status: proposed
flags: []
created: 2026-10-02T19:42:55.155896+01:00
updated: 2026-10-02T20:14:23.178045+01:00
---


