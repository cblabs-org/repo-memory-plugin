# bin

## Ruled out, failure signatures and gotchas

### [seed-54b99462] Go was not installed initially on the host machine; installed Go 1.27.1 via Homebrew (`brew install go`).
`ruled-out · anchors: go.mod, go.sum, github.com/cblabs-org/repo-memory-plugin, github.com/spf13/cobra, cmd/repo-memory/main.go, darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64, .gitignore, bin/, internal/core/.../doc.go, pipeline/compact, pipeline/triage, pipeline/extract, pipeline/reconcile, pipeline/verify, pipeline/publish, internal/harness/.../doc.go, harness/antigravity, harness/claudecode, harness/fake, internal/mcpserver/doc.go, internal/core/boundary_test.go, internal/core/...`

## Decisions

### [seed-2c90e2ae] Used standard library `go/parser` and `go/token` in `internal/core/boundary_test.go` instead of external dependencies for maximum speed and portability.
`decision · anchors: go.mod, go.sum, github.com/cblabs-org/repo-memory-plugin, github.com/spf13/cobra, cmd/repo-memory/main.go, darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64, .gitignore, bin/, internal/core/.../doc.go, pipeline/compact, pipeline/triage, pipeline/extract, pipeline/reconcile, pipeline/verify, pipeline/publish, internal/harness/.../doc.go, harness/antigravity, harness/claudecode, harness/fake, internal/mcpserver/doc.go, internal/core/boundary_test.go, internal/core/...`

### [seed-fec5210b] Configured `cobra` command stubs with `cobra.ArbitraryArgs` to accept CLI parameters without failing flag/arg validation during early integration.
`decision · anchors: go.mod, go.sum, github.com/cblabs-org/repo-memory-plugin, github.com/spf13/cobra, cmd/repo-memory/main.go, darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64, .gitignore, bin/, internal/core/.../doc.go, pipeline/compact, pipeline/triage, pipeline/extract, pipeline/reconcile, pipeline/verify, pipeline/publish, internal/harness/.../doc.go, harness/antigravity, harness/claudecode, harness/fake, internal/mcpserver/doc.go, internal/core/boundary_test.go, internal/core/...`
