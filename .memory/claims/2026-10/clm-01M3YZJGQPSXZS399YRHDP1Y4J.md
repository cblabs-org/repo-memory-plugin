---
id: clm-01M3YZJGQPSXZS399YRHDP1Y4J
type: decision
title: Author classification in classifyAuthor() differentiates 'human' from 'automation' (scheduled ticks, system messages, agentapi) using keyword analysis of transcript text.
relevant_when: When classifying transcript event authorship during compaction or transcript analysis.
apply: Maintain heuristic keyword analysis in classifyAuthor() within internal/core/pipeline/compact/compact.go.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/pipeline/compact/compact.go
created: 2026-10-02T19:56:06.390175+01:00
updated: 2026-10-02T19:56:06.390175+01:00
---


