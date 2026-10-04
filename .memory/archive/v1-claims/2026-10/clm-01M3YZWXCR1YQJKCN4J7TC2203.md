---
id: clm-01M3YZWXCR1YQJKCN4J7TC2203
type: architecture
title: Step gaps (`StepGap [2]int`) are automatically detected and recorded into the Header during write operations if not explicitly populated.
relevant_when: Serializing episode headers and auditing transcript completeness.
apply: Rely on automatic DetectGaps on store write to record missing step intervals in Header.StepGap.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/store.go
    fp: internal/core/episode/store.go
created: 2026-10-02T20:01:47.032162+01:00
updated: 2026-10-02T20:01:47.032162+01:00
---


