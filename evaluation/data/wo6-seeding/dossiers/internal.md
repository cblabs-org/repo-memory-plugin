# internal

## Decisions

### [seed-01f06942] `ScopeResolver.ResolveScope` prioritizes aliases first, then module scopes (path prefix match), then declared scopes (keywords, tool names, commands), resolving through aliases at every step.
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/, cmd/, ops.Reader, internal/core/config/scopes_test.go, ops.OpBatch`

### [seed-1e638d55] Anchors and path segments are normalized with forward slashes and underscores converted to hyphens to ensure consistent kebab-case scope names matching the v2 design.
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/, cmd/, ops.Reader, internal/core/config/scopes_test.go, ops.OpBatch`

### [seed-97de0061] `ScopeKind` connects resolved scope names to `dossiers.DossierKind` (`module`, `topic`, `external`, `procedure`).
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/, cmd/, ops.Reader, internal/core/config/scopes_test.go, ops.OpBatch`

### [seed-bd8c65c3] `AliasMap.Resolve` protects against recursive alias loops using both a visited-set cycle detector and a maximum recursion depth of 10.
`decision · anchors: internal/core/config/scopes.go, config.yaml, internal/, cmd/, ops.Reader, internal/core/config/scopes_test.go, ops.OpBatch`
