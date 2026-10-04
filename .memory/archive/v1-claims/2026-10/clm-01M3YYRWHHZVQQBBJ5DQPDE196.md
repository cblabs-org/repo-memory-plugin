---
id: clm-01M3YYRWHHZVQQBBJ5DQPDE196
type: decision
title: Episode persistence uses pure-Go zstandard compression (klauspost/compress/zstd) with atomic temporary-file-and-rename writes and StepGap handling for unwritten dispatch steps.
relevant_when: Writing and persisting compacted episode records to local disk.
apply: Compress serialized episode records via klauspost/compress/zstd, write to a temp file, atomically rename, and record StepGap markers for unwritten internal dispatch steps.
status: accepted
flags: []
created: 2026-10-02T19:42:06.5139+01:00
updated: 2026-10-02T20:00:48.832821+01:00
---


