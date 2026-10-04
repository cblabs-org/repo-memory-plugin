---
id: clm-01M3YZ97RKS14NVCFTBENHQ4H3
type: lesson_learned
title: When generating chunked JSONL test fixtures where records straddle boundaries, avoid tools like standard cat that append trailing newlines at EOF; use printf or exact binary concatenation.
relevant_when: Creating scrubbed fixtures and synthetic test files representing split chunks.
apply: Ensure strict byte continuity across chunk boundary files without injecting extra newline characters.
status: proposed
flags: []
anchors:
  - kind: directory
    ref: testdata/fixtures/antigravity/
    fp: testdata/fixtures/antigravity/
created: 2026-10-02T19:51:02.291012+01:00
updated: 2026-10-02T19:51:02.291012+01:00
---


