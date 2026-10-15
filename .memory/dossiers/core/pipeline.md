# core/pipeline

## Ruled out, failure signatures and gotchas

### [seed-6ebc82d6] `index.go` currently only exposes `InsertEpisode` but not individual claim indexing. We delegate that to the provided `Indexer` interface which caller logic must connect.
`ruled-out · kind: episodic · anchors: internal/core/pipeline/publish/local.go, internal/core/pipeline/publish/local_test.go`

## Decisions

### [seed-20ddcc44] Used simple string matching for checking sensitive keywords.
`decision · kind: episodic · anchors: internal/core/pipeline/publish/local.go, internal/core/pipeline/publish/local_test.go`

### [seed-364ac945] Chose simple heuristic checks for `classifyAuthor` based on keyword analysis of transcript text.
`decision · kind: episodic · anchors: internal/core/pipeline/compact/compact.go`

### [seed-7cc85282] Bounded command excerpt truncation retains failing markers or the last 5-10 lines of output for contexts yielding a non-zero exit code.
`decision · kind: episodic · anchors: internal/core/pipeline/compact/compact.go`

### [seed-8206d969] Collapsing duplicate identical log lines for device/test outputs ensures major compaction without losing structure.
`decision · kind: episodic · anchors: internal/core/pipeline/compact/compact.go`

### [seed-bb928b56] Implemented `Indexer` interface to allow the Local Publisher to trigger indexing immediately without tightly coupling to `index.go`.
`decision · kind: episodic · anchors: internal/core/pipeline/publish/local.go, internal/core/pipeline/publish/local_test.go`

## Open issues

### [seed-985afa4a] `index.go` needs a way to insert individual claims incrementally. Currently, it lacks an `InsertClaim` method. This needs to be built at the application layer or added to the `Index` type natively.
`open-issue · kind: semantic · anchors: internal/core/pipeline/publish/local.go, internal/core/pipeline/publish/local_test.go`
