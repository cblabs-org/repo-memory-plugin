---
id: clm-01M3YYRWHECRW5F472WZR66SGJ
type: pattern
title: Transcript compaction achieves high reduction ratios by collapsing duplicate output lines, truncating successful command excerpts while retaining failure markers, and sanitizing secrets into typed placeholder tokens.
relevant_when: Compacting raw conversation logs into episode store formats.
apply: Collapse duplicate identical log lines, keep only reference IDs in exposure markers, retain the last 5-10 lines or failing markers on non-zero exit codes, and replace AWS keys, UUIDs, and tokens with «secret:type».
status: accepted
flags: []
created: 2026-10-02T19:42:06.510492+01:00
updated: 2026-10-02T19:42:06.510492+01:00
---


