---
id: clm-01M3YZVPP1CE4DG9F45ZVT0S5Z
type: convention
title: Episode IDs must follow the format `ep-<ulid>` generated using `github.com/oklog/ulid/v2`.
relevant_when: Creating new episode records or parsing episode identifiers.
apply: Use github.com/oklog/ulid/v2 to generate sortable ULIDs prefixed with 'ep-'.
status: proposed
flags: []
anchors:
  - kind: file
    ref: internal/core/episode/store.go
    fp: internal/core/episode/store.go
created: 2026-10-02T20:01:07.39387+01:00
updated: 2026-10-02T20:01:07.39387+01:00
---


