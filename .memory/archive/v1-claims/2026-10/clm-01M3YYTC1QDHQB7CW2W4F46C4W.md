---
id: clm-01M3YYTC1QDHQB7CW2W4F46C4W
type: constraint
title: Antigravity transcripts (`transcript_full.jsonl`) contain step-index gaps from internal dispatch steps and can record parallel tool call results non-monotonically before parent `PLANNER_RESPONSE` steps.
relevant_when: Parsing or validating Antigravity transcript files (`transcript.jsonl` or `transcript_full.jsonl`).
apply: Do not assume transcript step indices are contiguous (`line_count == max_step_index + 1`) or strictly monotonically increasing.
status: proposed
flags: []
created: 2026-10-02T19:42:55.159713+01:00
updated: 2026-10-02T19:42:55.159713+01:00
---


