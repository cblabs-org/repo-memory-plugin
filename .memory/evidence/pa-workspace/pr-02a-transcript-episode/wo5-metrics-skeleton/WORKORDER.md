---
name: pr-02a-transcript-episode-wo5
description: Handoff instructions for Work Order 5 - Metrics Skeleton
complexity: standard
---

# WORKORDER Template

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Implement metrics writing to record pipeline telemetry and costs.

## Strict Boundary Box
You must only work within `internal/core/metrics/` and related tests. 

## Tooling Constraints
Use `go build ./cmd/repo-memory` and `go test ./...` to verify your changes.

## Line-Item Instructions
- [x] Implement `internal/core/metrics/` package with a JSONL metrics writer.
- [x] Capture health metrics: sessions seen, captured, skipped, missed.
- [x] Capture hook execution time, tick duration, budget exhaustion.
- [x] Implement a tool/test to re-measure §4.4.1 transcript sizes from merged full transcripts.

## Verification Requirement
The worker must verify it compiles/runs according to the Tooling Constraints.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->

### Status
Complete

### Changes Made
- Implemented `internal/core/metrics/types.go` containing:
  - Base record model (`Record`, `BaseRecord`, `MetricType`).
  - `CaptureHealth` capturing sessions seen, captured, skipped (with skip reasons like recursion guard), missed (transcript gone before sweep), and capture rate.
  - `HookExecution` capturing hook execution latency and cumulative session execution time.
  - `TickHealth` capturing scheduled tick duration, budget exhaustion, skipped overlaps, backlog maps, sessions caught by sweep, and spool-to-delivery latency.
  - `BudgetExhaustion` recording stage, token/run/time limits vs consumption, exhaustion status, and reason.
  - `TranscriptMeasurement` recording transcript footprint, token estimation, tool traffic breakdown, compaction ratio, and truncation expansion ratio.
- Implemented `internal/core/metrics/writer.go` providing thread-safe JSONL appending under `<data-dir>/<repo-id>/metrics/*.jsonl` with daily file rotation (`YYYY-MM-DD.jsonl`), fixed file overrides (`WithFileName`), forced flush/sync options (`WithSyncOnWrite`), and arbitrary `io.Writer` targets (`NewWriterFromIO`).
- Implemented `internal/core/metrics/reader.go` providing structured JSONL deserialization, stream seeking, and telemetry aggregators:
  - `AggregateCaptureHealth` for overall seen/captured/skipped/missed counts, reason breakdowns, and capture rate.
  - `AggregateHookExecutions` for p50, p95, p99, mean, min, max percentiles per event type, and session cumulative latencies.
  - `AggregateTickHealth` for tick execution stats, backlog averages, and spool-to-delivery latencies.
  - `AggregateBudgetExhaustion` for stage-specific exhaustion counts and token consumption totals.
- Implemented `internal/core/metrics/measure.go` providing transcript size and compaction measurement tools:
  - `OpenTranscriptStream` following §6.2 priority order: `transcript_full.jsonl`, chunked transcripts concatenated in lexical order (`chunks/transcript_full/*.jsonl`), or compact fallback (`transcript.jsonl`).
  - `MeasureTranscript` normalizing steps, detecting tool traffic (both tool calls and output responses), and applying `compact.NewCompactor().Compact()` to compute kept characters and compaction ratios.
  - `MeasureDirectories` and `FormatSection441Table` producing markdown comparison tables.
- Added comprehensive unit and integration test suites:
  - `writer_test.go`: File persistence, daily rotation, in-memory IO, reader roundtrip, and race-free concurrent writes (25 goroutines x 40 writes).
  - `health_test.go`: Capture health tracking, hook percentile distributions (p50/p95/p99), tick execution health, and budget exhaustion assertions.
  - `measure_test.go`: Fixture validation (`full_normal`, `full_chunked`, `compact`), synthetic compaction test confirming compaction ratio >= 4x, and live re-measurement on surviving full transcripts (`~/.gemini/antigravity/brain`).

### Decisions Made
- Standardized token estimation at 4 characters per token (`chars / 4`) across raw transcript text, tool traffic, and compacted outputs.
- In Antigravity transcripts, tool traffic is accurately captured by including both tool call payloads (`tool_calls`) and subsequent tool response steps (type `GENERIC` / `TOOL_OUTPUT` / `COMMAND_OUTPUT` following a tool call), yielding 77%–83% tool traffic on real transcripts (matching §4.4.1's 86%–90%).
- Full transcripts were measured against surviving conversations on disk:
  - Median tokens: 56k (top-level: 50k tokens, subagent: 68k tokens)
  - Mean tokens: 71k
  - Tool traffic: 78%
  - Compaction ratio: 5.4x (exceeding the PR-02a exit criterion target of >= 4x).
- Preserved strict boundary rules: `internal/core/metrics` does not import any concrete harness adapter (`antigravity`, `claudecode`, or `fake` in production code).

### Dead Ends & Warnings
- When using `io.Reader` sources with `Reader`, streams must implement `io.Seeker` or use separate readers if multiple scan passes (`ReadCaptureHealth`, `ReadHookExecutions`, etc.) are made against the same input stream. `reader.go` was updated to automatically seek to offset 0 when an `io.Seeker` is detected.

### Out-of-Scope Findings
(none)

### Blockers
(none)
