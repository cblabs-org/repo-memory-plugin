# core/delivery

## Decisions

### [seed-5592e8e5] Addressed unacknowledged `gate: edit` lookups by adding `ledger *Ledger` explicitly as a parameter to `GateEntries()`.
`decision · kind: episodic · anchors: internal/core/delivery/doc.go, internal/core/delivery/types.go, internal/core/delivery/ledger.go, internal/core/delivery/anchor.go, internal/core/delivery/ranker.go, internal/core/delivery/formatter.go, internal/core/delivery/engine.go, internal/core/delivery/delivery_test.go`

### [seed-dea19596] Handled diff path directory-vs-exact matching heuristically via path containment checks.
`decision · kind: episodic · anchors: internal/core/delivery/doc.go, internal/core/delivery/types.go, internal/core/delivery/ledger.go, internal/core/delivery/anchor.go, internal/core/delivery/ranker.go, internal/core/delivery/formatter.go, internal/core/delivery/engine.go, internal/core/delivery/delivery_test.go`

## Open issues

### [seed-c91eb6fc] `typeOrder` in the `dossiers/projector.go` is unexported, so it had to be re-declared in the `ranker.go` file.
`open-issue · kind: semantic · anchors: internal/core/delivery/doc.go, internal/core/delivery/types.go, internal/core/delivery/ledger.go, internal/core/delivery/anchor.go, internal/core/delivery/ranker.go, internal/core/delivery/formatter.go, internal/core/delivery/engine.go, internal/core/delivery/delivery_test.go`
