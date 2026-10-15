# core/pipeline/extract

## Ruled out, failure signatures and gotchas

### [seed-3a98c0c9] When writing job pipelines, `Housekeeper` must be securely injected via `TickerOption`, as it requires context. Ensure mock harnesses correctly simulate `Usage` return data for the pipeline token budget accounting.
`ruled-out · anchors: internal/core/index/index.go, internal/core/pipeline/extract/extractor.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-d378e842] Note that extraction requires `sync.Mutex` during payload combination; race conditions can appear if `ExtractionResult` arrays are joined outside the lock.
`ruled-out · anchors: internal/core/index/index.go, internal/core/pipeline/extract/extractor.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

## Decisions

### [seed-1efa4e49] Chose to maintain `pipeline.go` as the core orchestrator, directly interacting with `jobs.Store` and passing payloads sequentially between the pipeline components, ensuring that duplicate idempotency keys result in no-op processing per the workorder specifications.
`decision · anchors: internal/core/index/index.go, internal/core/pipeline/extract/extractor.go, internal/core/tick/tick.go, internal/core/tick/pipeline.go, internal/core/tick/pipeline_test.go`

### [seed-7fa7cddc] P13 checks were hardened directly into `extractor.go` rather than purely relying on the LLM, rejecting any cited evidence that isn't `tool_outcome` or `human`.
`decision · anchors: internal/core/pipeline/extract/packer.go, internal/core/pipeline/extract/extractor.go, internal/core/pipeline/extract/extractor_test.go, prompts/extract/prompt.md`

### [seed-e96d91d7] Used heuristic token counting (`bytes / 4`) as standard across the project.
`decision · anchors: internal/core/pipeline/extract/packer.go, internal/core/pipeline/extract/extractor.go, internal/core/pipeline/extract/extractor_test.go, prompts/extract/prompt.md`
