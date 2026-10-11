# testdata/fixtures/antigravity

## Ruled out, failure signatures and gotchas

### [seed-9d247137] `cat`'s EOF behavior on newlines broke the chunked test initially. Fixed using `printf` to ensure strict byte continuity over chunks without injection of newlines.
`ruled-out · anchors: internal/harness/antigravity/transcript.go, transcript_full.jsonl, transcript_full/*.jsonl, transcript.jsonl, testdata/fixtures/antigravity/v1/`

## Decisions

### [seed-324060b3] Used `bufio.Scanner` with a generous 50MB buffer on top of an `io.MultiReader` to ensure lexical chunks that straddle a single JSON line can seamlessly parse into unified lines before being evaluated.
`decision · anchors: internal/harness/antigravity/transcript.go, transcript_full.jsonl, transcript_full/*.jsonl, transcript.jsonl, testdata/fixtures/antigravity/v1/`
