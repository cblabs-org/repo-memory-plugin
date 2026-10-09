# <data-dir>/<repo-id>/episodes

## Decisions

### [seed-396eaa3e] Implemented atomic rename on write within the target directory to prevent partial files or reader corruption.
`decision · anchors: internal/core/episode/store.go, <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst, internal/core/episode/store_test.go, github.com/klauspost/compress/zstd, github.com/oklog/ulid/v2, go.mod`

### [seed-586677a0] Implemented streaming chunk-based line reader `readLineBytes` with `bufio.Reader` to ensure arbitrarily long JSON lines do not fail with buffer overflow.
`decision · anchors: internal/core/episode/store.go, <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst, internal/core/episode/store_test.go, github.com/klauspost/compress/zstd, github.com/oklog/ulid/v2, go.mod`

### [seed-7fc83a86] Automated gap detection on write if gaps have not been explicitly populated.
`decision · anchors: internal/core/episode/store.go, <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst, internal/core/episode/store_test.go, github.com/klauspost/compress/zstd, github.com/oklog/ulid/v2, go.mod`

### [seed-e07cd549] Selected pure-Go `github.com/klauspost/compress/zstd` per SPEC §18.4 (Episode compression: Pure-Go zstd, zero cgo).
`decision · anchors: internal/core/episode/store.go, <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst, internal/core/episode/store_test.go, github.com/klauspost/compress/zstd, github.com/oklog/ulid/v2, go.mod`

### [seed-f57f5c43] Selected `github.com/oklog/ulid/v2` for generating standardized `ep-<ulid>` episode IDs.
`decision · anchors: internal/core/episode/store.go, <data-dir>/<repo-id>/episodes/<yyyy>/<session-id>.jsonl.zst, internal/core/episode/store_test.go, github.com/klauspost/compress/zstd, github.com/oklog/ulid/v2, go.mod`
