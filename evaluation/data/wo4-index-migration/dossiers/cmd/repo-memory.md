# cmd/repo-memory

## Decisions

### [seed-6c7b0946] Leveraged `go-version-file: 'go.mod'` in `actions/setup-go@v5` across both workflows to maintain version consistency with `go.mod`.
`decision · anchors: cmd/repo-memory/main.go, .github/workflows/ci.yml, .github/workflows/release.yml, softprops/action-gh-release@v2`

### [seed-e8c7bff7] Used `softprops/action-gh-release@v2` with `permissions: contents: write` and `env: GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` for robust Node 20 GitHub release asset management.
`decision · anchors: cmd/repo-memory/main.go, .github/workflows/ci.yml, .github/workflows/release.yml, softprops/action-gh-release@v2`

## Open issues

### [seed-48936155] Work order originally mentioned creating `EpisodeRecord` entries directly, but `harness: "import"` refers to the JSONL store `episode.Episode` header schema. I implemented the latter (saving to JSONL store via `episode.NewStore`), which behaves seamlessly with downstream components like FTS index population.
`open-issue · anchors: cmd/repo-memory/cmd_import_pa.go, PROJECT.md, WORKORDER.md, context_bridge.md, cmd/repo-memory/cmd_import_pa_test.go, cmd/repo-memory/main.go`
