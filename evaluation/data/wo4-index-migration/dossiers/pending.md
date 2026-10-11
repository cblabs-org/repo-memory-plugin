# pending

## Ruled out, failure signatures and gotchas

### [seed-6ebc82d6] `index.go` currently only exposes `InsertEpisode` but not individual claim indexing. We delegate that to the provided `Indexer` interface which caller logic must connect.
`ruled-out · anchors: internal/core/pipeline/publish/local.go, pending/, internal/core/pipeline/publish/local_test.go`

## Decisions

### [seed-20ddcc44] Used simple string matching for checking sensitive keywords.
`decision · anchors: internal/core/pipeline/publish/local.go, pending/, internal/core/pipeline/publish/local_test.go`

### [seed-bb928b56] Implemented `Indexer` interface to allow the Local Publisher to trigger indexing immediately without tightly coupling to `index.go`.
`decision · anchors: internal/core/pipeline/publish/local.go, pending/, internal/core/pipeline/publish/local_test.go`

## Open issues

### [seed-985afa4a] `index.go` needs a way to insert individual claims incrementally. Currently, it lacks an `InsertClaim` method. This needs to be built at the application layer or added to the `Index` type natively.
`open-issue · anchors: internal/core/pipeline/publish/local.go, pending/, internal/core/pipeline/publish/local_test.go`
