# core/spool

## Ruled out, failure signatures and gotchas

### [seed-24267d8b] When writing files on Windows, file handles must be closed before calling `os.Rename`. Attempting to rename an open handle results in `ERROR_ACCESS_DENIED`. The writer ensures handles are synced and closed before renaming, with a retry backoff loop for transient file indexer locks.
`ruled-out · anchors: internal/core/spool/doc.go, internal/core/spool/entry.go, internal/core/spool/entry_test.go, internal/core/spool/writer.go, internal/core/spool/writer_test.go, internal/core/spool/reader.go, internal/core/spool/reader_test.go, internal/core/spool/copy.go, internal/core/spool/copy_test.go`

## Decisions

### [seed-59260e24] Crash-safe state writes: State cursors in `state/` are written using temporary files and atomic rename to prevent corrupted state during unexpected process termination.
`decision · anchors: internal/core/spool/doc.go, internal/core/spool/entry.go, internal/core/spool/entry_test.go, internal/core/spool/writer.go, internal/core/spool/writer_test.go, internal/core/spool/reader.go, internal/core/spool/reader_test.go, internal/core/spool/copy.go, internal/core/spool/copy_test.go`

### [seed-8a74da28] Filename timestamps format: Formatted filenames as `20060102T150405.000000000Z_<kind>_<id>.json` to ensure chronological lexicographical ordering while avoiding colons (`:`) which are illegal in Windows filenames.
`decision · anchors: internal/core/spool/doc.go, internal/core/spool/entry.go, internal/core/spool/entry_test.go, internal/core/spool/writer.go, internal/core/spool/writer_test.go, internal/core/spool/reader.go, internal/core/spool/reader_test.go, internal/core/spool/copy.go, internal/core/spool/copy_test.go`

### [seed-fbcb7efb] Single-pass hashing: When performing prefix verification during incremental copying, bytes `0` to `state.ByteOffset` are hashed; if the prefix matches, subsequent appended bytes are piped through the same hasher via `io.TeeReader`, computing the complete new prefix hash in a single pass without re-reading the entire file.
`decision · anchors: internal/core/spool/doc.go, internal/core/spool/entry.go, internal/core/spool/entry_test.go, internal/core/spool/writer.go, internal/core/spool/writer_test.go, internal/core/spool/reader.go, internal/core/spool/reader_test.go, internal/core/spool/copy.go, internal/core/spool/copy_test.go`
