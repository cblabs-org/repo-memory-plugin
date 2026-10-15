# harness/fake

## Ruled out, failure signatures and gotchas

### [seed-909c1e20] Running `go test ./...` with `suite_test.go` initially triggered a boundary failure in `TestCoreImportBoundaries` because `boundary_test.go` strictly forbade `internal/harness/fake` in all `.go` files under `internal/core`. Surfaced to PA, authorized adjustment, and resolved cleanly.
`ruled-out · anchors: internal/harness/types.go, internal/harness/interfaces.go, internal/harness/fake/fake.go, internal/harness/fake/fake_test.go, internal/core/suite_test.go, internal/core/boundary_test.go, internal/harness/fake`

## Decisions

### [seed-32c977ea] Allowed test files (`*_test.go`) under `internal/core` to import `internal/harness/fake` following approval from the Principal Architect to comply with SPEC §18.6 Rule 6 ("Core tests run against the fake adapter").
`decision · anchors: internal/harness/types.go, internal/harness/interfaces.go, internal/harness/fake/fake.go, internal/harness/fake/fake_test.go, internal/core/suite_test.go, internal/core/boundary_test.go, internal/harness/fake`
