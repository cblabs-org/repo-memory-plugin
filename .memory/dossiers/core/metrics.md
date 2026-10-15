# core/metrics

## Ruled out, failure signatures and gotchas

### [seed-25510b9d] When using `io.Reader` sources with `Reader`, streams must implement `io.Seeker` or use separate readers if multiple scan passes (`ReadCaptureHealth`, `ReadHookExecutions`, etc.) are made against the same input stream. `reader.go` was updated to automatically seek to offset 0 when an `io.Seeker` is detected.
`ruled-out · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

## Decisions

### [seed-1204a99f] Compaction ratio: 5.4x (exceeding the PR-02a exit criterion target of >= 4x).
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-26d7775c] In Antigravity transcripts, tool traffic is accurately captured by including both tool call payloads (`tool_calls`) and subsequent tool response steps (type `GENERIC` / `TOOL_OUTPUT` / `COMMAND_OUTPUT` following a tool call), yielding 77%–83% tool traffic on real transcripts (matching §4.4.1's 86%–90%).
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-43acc4bb] Full transcripts were measured against surviving conversations on disk:
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-7b20551c] Median tokens: 56k (top-level: 50k tokens, subagent: 68k tokens)
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-a29b6500] Tool traffic: 78%
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-e3c9544a] Preserved strict boundary rules: `internal/core/metrics` does not import any concrete harness adapter (`antigravity`, `claudecode`, or `fake` in production code).
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-f9ba43c6] Mean tokens: 71k
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`

### [seed-fe162b79] Standardized token estimation at 4 characters per token (`chars / 4`) across raw transcript text, tool traffic, and compacted outputs.
`decision · kind: episodic · anchors: internal/core/metrics/types.go, internal/core/metrics/writer.go, internal/core/metrics/reader.go, internal/core/metrics/measure.go`
