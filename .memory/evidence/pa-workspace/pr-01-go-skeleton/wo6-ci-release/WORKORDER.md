---
name: pr-01-go-skeleton-wo6
description: Handoff instructions for Work Order 6 - CI & Release
complexity: standard
---

# WORKORDER: WO6 - CI & Release

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

## Objective
Set up GitHub Actions for CI (testing on macOS, Linux, and Windows) and CD (building the 5 cross-compiled targets and attaching them to a GitHub Release with a version stamp).

## Strict Boundary Box
- DO NOT change any application code beyond injecting a `Version` string in `main.go` and reading it if required.
- ONLY add GitHub Actions workflow files and update the `Makefile` to inject the version stamp into the build using `ldflags`.

## Tooling Constraints
- Standard Go tooling: `make build`, `go test`.
- YAML for GitHub Actions.

## Line-Item Instructions
- [x] Update `cmd/repo-memory/main.go` to include a `Version` variable (e.g. `var Version = "dev"`).
- [x] Update the `Makefile` to accept a `VERSION` variable and inject it into the binary using `-ldflags="-X 'main.Version=$(VERSION)'"` for all 5 compilation targets.
- [x] Create a CI workflow `.github/workflows/ci.yml`:
  - Trigger on push to `main` and pull requests.
  - Matrix strategy for OS: `ubuntu-latest`, `macos-latest`, `windows-latest`.
  - Steps: Checkout, Setup Go, run `go test ./...`.
- [x] Create a Release workflow `.github/workflows/release.yml`:
  - Trigger on tag pushes (e.g., `v*`).
  - Steps: Checkout, Setup Go.
  - Run `make build VERSION=${{ github.ref_name }}` (or equivalent platform-specific steps to build the 5 targets).
  - Use `softprops/action-gh-release` or `gh release create` to upload the 5 built binaries (`darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`, `windows-amd64.exe`) to the GitHub release.

## Verification Requirement
- Verify `make build` and `go test ./...` still work locally.
- Ensure the workflows are syntactically valid YAML.
- **CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly. The Principal Architect will triage and file them under the correct author identity.

---

## Completion Report

### Status
complete

### Changes Made
- `cmd/repo-memory/main.go` — Added package-level `Version` variable initialized to `"dev"` and wired `rootCmd.Version = Version` to support version querying.
- `Makefile` — Added `VERSION ?= dev` variable support and updated target recipes to pass `-ldflags="-X 'main.Version=$(VERSION)'"` into `go build` for all 5 cross-compiled targets.
- `.github/workflows/ci.yml` — Created GitHub Actions CI matrix workflow testing across `ubuntu-latest`, `macos-latest`, and `windows-latest` on pushes to `main` and all pull requests.
- `.github/workflows/release.yml` — Created GitHub Actions release workflow triggering on `v*` tags, running `make build VERSION=${{ github.ref_name }}`, and uploading the 5 target binaries to the GitHub release via `softprops/action-gh-release@v2`.

### Decisions Made
- Used `softprops/action-gh-release@v2` with `permissions: contents: write` and `env: GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` for robust Node 20 GitHub release asset management.
- Leveraged `go-version-file: 'go.mod'` in `actions/setup-go@v5` across both workflows to maintain version consistency with `go.mod`.

### Dead Ends & Warnings
(none)

### Out-of-Scope Findings
(none)

### Blockers
(none)
