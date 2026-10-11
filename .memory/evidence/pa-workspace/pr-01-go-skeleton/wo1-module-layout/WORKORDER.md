---
name: pr-01-go-skeleton-wo1
description: Handoff instructions for Work Order 1 - Module Layout
complexity: standard
---

# WORKORDER: WO1 - Module Layout

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Initialise the Go module, wire up the main entry point with empty subcommand stubs, create the cross-compile Makefile, generate package stubs with docs, and implement the import-graph boundary test.

## Strict Boundary Box
- DO NOT implement the logic of any subcommands. Just wire them up so they print their name or help text.
- DO NOT implement any interface types or core structs yet. Just create the directories and `doc.go` files with package definitions and a comment.
- DO NOT edit `SPEC.md` or any other documentation outside your boundary box.

## Tooling Constraints
- Use standard Go toolchain (`go mod init`, `go build`, `go test`).
- Use `Makefile` for the build targets.

## Line-Item Instructions
- [x] Run `go mod init github.com/cblabs-org/repo-memory-plugin` (this is the module name).
- [x] Create `cmd/repo-memory/main.go` and set up subcommand wiring (using `cobra` is recommended, so run `go get github.com/spf13/cobra@latest`) for the following subcommands: `hook`, `tick`, `mcp`, `install`, `uninstall`, `search`, `report`, `status`, `reindex`, `doctor`. They should just be empty stubs.
- [x] Create a `Makefile` with a `build` target that cross-compiles to the following 5 targets (with `CGO_ENABLED=0`):
  - darwin/arm64
  - darwin/amd64
  - linux/amd64
  - linux/arm64
  - windows/amd64
- [x] Create stub packages (create a `doc.go` file with a package docstring in each) for:
  - `internal/core/episode`
  - `internal/core/claims`
  - `internal/core/entities`
  - `internal/core/views`
  - `internal/core/index`
  - `internal/core/jobs`
  - `internal/core/tick`
  - `internal/core/pipeline/compact`
  - `internal/core/pipeline/triage`
  - `internal/core/pipeline/extract`
  - `internal/core/pipeline/reconcile`
  - `internal/core/pipeline/verify`
  - `internal/core/pipeline/publish`
  - `internal/core/delivery`
  - `internal/core/gitops`
  - `internal/core/scrub`
  - `internal/core/metrics`
  - `internal/core/config`
  - `internal/harness`
  - `internal/harness/antigravity`
  - `internal/harness/claudecode`
  - `internal/harness/fake`
  - `internal/mcpserver`
- [x] Create `internal/core/boundary_test.go` that parses the AST/packages (using `golang.org/x/tools/go/packages` or a simple string scan of imports) to ensure no package under `internal/core/...` imports any concrete adapter (`internal/harness/antigravity`, `internal/harness/claudecode`, `internal/harness/fake`). The test must pass.

## Verification Requirement
- Verify `make build` compiles all 5 targets successfully.
- Verify `go test ./...` passes the boundary test.
- Verify `go mod tidy` is run.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
complete

### Changes Made
- `go.mod`, `go.sum` — Initialized module `github.com/cblabs-org/repo-memory-plugin` and added `github.com/spf13/cobra` dependency.
- `cmd/repo-memory/main.go` — Root CLI entry point wiring empty subcommands (`hook`, `tick`, `mcp`, `install`, `uninstall`, `search`, `report`, `status`, `reindex`, `doctor`).
- `Makefile` — Multi-platform cross-compilation `build` target for `darwin/arm64`, `darwin/amd64`, `linux/amd64`, `linux/arm64`, `windows/amd64` using `CGO_ENABLED=0`.
- `.gitignore` — Added `bin/` to ignore local build binaries.
- `internal/core/.../doc.go` — Package doc stubs for `episode`, `claims`, `entities`, `views`, `index`, `jobs`, `tick`, `pipeline/compact`, `pipeline/triage`, `pipeline/extract`, `pipeline/reconcile`, `pipeline/verify`, `pipeline/publish`, `delivery`, `gitops`, `scrub`, `metrics`, `config`.
- `internal/harness/.../doc.go` — Package doc stubs for `harness`, `harness/antigravity`, `harness/claudecode`, `harness/fake`.
- `internal/mcpserver/doc.go` — Package doc stub for `mcpserver`.
- `internal/core/boundary_test.go` — AST-based import graph boundary unit test enforcing that `internal/core/...` never imports concrete adapters.

### Decisions Made
- Used standard library `go/parser` and `go/token` in `internal/core/boundary_test.go` instead of external dependencies for maximum speed and portability.
- Configured `cobra` command stubs with `cobra.ArbitraryArgs` to accept CLI parameters without failing flag/arg validation during early integration.

### Dead Ends & Warnings
- Go was not installed initially on the host machine; installed Go 1.27.1 via Homebrew (`brew install go`).

### Out-of-Scope Findings
(none)

### Blockers
(none)
