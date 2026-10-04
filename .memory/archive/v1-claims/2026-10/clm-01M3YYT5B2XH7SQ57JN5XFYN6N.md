---
id: clm-01M3YYT5B2XH7SQ57JN5XFYN6N
type: architecture
title: In Antigravity hooks, `invocationNum` resets to 0 on every user prompt rather than incrementing monotonically across the entire session.
relevant_when: Implementing session deduplication, state tracking, or context injection gating in Antigravity hooks.
apply: Track deduplication across prompts rather than relying on session-level monotonicity of `invocationNum` (gating solely on `invocationNum == 0` will re-inject on every user prompt).
status: proposed
flags: []
created: 2026-10-02T19:42:48.290904+01:00
updated: 2026-10-02T19:42:48.290904+01:00
---


