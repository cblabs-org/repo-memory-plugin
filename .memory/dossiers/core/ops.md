# core/ops

## Decisions

### [seed-7244721d] Used standard `github.com/oklog/ulid/v2` matching existing codebase patterns for ULID generation.
`decision · anchors: internal/core/ops/op.go, internal/core/ops/validate.go, internal/core/ops/writer.go, internal/core/ops/reader.go, internal/core/ops/iterator.go, internal/core/ops/migrate.go, internal/core/ops/doc.go, internal/core/ops/ops_test.go`

### [seed-b477db3f] Supported `WithGitDir` on `Reader` to enable testing and flexible repository location resolution for `IterateBefore`.
`decision · anchors: internal/core/ops/op.go, internal/core/ops/validate.go, internal/core/ops/writer.go, internal/core/ops/reader.go, internal/core/ops/iterator.go, internal/core/ops/migrate.go, internal/core/ops/doc.go, internal/core/ops/ops_test.go`

### [seed-bd287837] Made `split` validation accept both native Go slices/maps and JSON unmarshaled `[]any`/`map[string]any` to ensure round-trip validation seamlessly passes.
`decision · anchors: internal/core/ops/op.go, internal/core/ops/validate.go, internal/core/ops/writer.go, internal/core/ops/reader.go, internal/core/ops/iterator.go, internal/core/ops/migrate.go, internal/core/ops/doc.go, internal/core/ops/ops_test.go`

### [seed-c8ab4dea] Designed `Iterator` to support both Go 1.23+ range sequence iteration (`iter.Seq2[*OpBatch, error]`) and pull iteration (`Next() (*OpBatch, error)` via `iter.Pull2`).
`decision · anchors: internal/core/ops/op.go, internal/core/ops/validate.go, internal/core/ops/writer.go, internal/core/ops/reader.go, internal/core/ops/iterator.go, internal/core/ops/migrate.go, internal/core/ops/doc.go, internal/core/ops/ops_test.go`
