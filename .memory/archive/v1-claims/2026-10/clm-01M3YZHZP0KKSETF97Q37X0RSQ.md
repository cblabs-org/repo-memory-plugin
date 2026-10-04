---
id: clm-01M3YZHZP0KKSETF97Q37X0RSQ
type: convention
title: Header.UnmarshalJSON in internal/core/episode/store.go supports polymorphic representation of commits (string slice, comma/space-delimited string, or array of map objects with sha/commit/hash keys).
relevant_when: Parsing or unmarshaling episode header JSON across different formats or versions.
apply: Preserve lenient unmarshaling for the 'commits' field in Header to tolerate diverse commit serialization formats without failure.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/store.go
    fp: internal/core/episode/store.go
created: 2026-10-02T19:55:48.928792+01:00
updated: 2026-10-02T19:55:48.928792+01:00
---


