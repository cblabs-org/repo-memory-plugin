# harness

## Ruled out, failure signatures and gotchas

### [seed-4c2dd2e9] When using `--input-format stream-json`, `agy` explicitly requires `--output-format stream-json`. The instruction requested `--output-format json`, but that was rejected by the CLI.
`ruled-out · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/core/tick/sweep.go, internal/harness/antigravity/attribution.go, internal/harness/fake/fake.go, internal/harness/fake/recorded.go`

### [seed-64752358] Sending raw string events like `{"prompt": "..."}` or `{"event": "prompt", ...}` over stdin for stream-json gets ignored by `agy` with "warning: ignoring unsupported stream input message event". Wrapping it dynamically as an event for test-compatibility but `agy` requires a specific event name to actually execute.
`ruled-out · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/core/tick/sweep.go, internal/harness/antigravity/attribution.go, internal/harness/fake/fake.go, internal/harness/fake/recorded.go`

## Open issues

### [seed-f5f24ef7] The `agy` CLI's `stream-json` input handler behavior isn't documented externally and ignores standard events like "prompt" or "request", which may block pipeline integration until the expected event name is documented.
`open-issue · anchors: internal/harness/interfaces.go, internal/harness/antigravity/executor.go, internal/core/tick/sweep.go, internal/harness/antigravity/attribution.go, internal/harness/fake/fake.go, internal/harness/fake/recorded.go`
