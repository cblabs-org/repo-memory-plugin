# core

## Decisions

### [seed-01e79d6d] Symlinks & Subdirectory Enrolment: Registered paths are indexed both as raw paths and evaluated symlink paths (e.g. `/var` vs `/private/var` on macOS). `FindRegistryEntry` also supports subdirectories within enrolled repositories so hooks invoked from subfolders match correctly.
`decision · kind: episodic · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`

### [seed-4367d3cc] Origin URL Canonicalization: Normalized SSH (`git@host:owner/repo.git`), schemes (`ssh://`, `git://`), trailing slashes, and `.git` suffixes to `https://host/owner/repo` so all clone protocols produce matching Repo IDs.
`decision · kind: episodic · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`

### [seed-71a4a73a] Combined `seed.ParseDocToOps` with keyword classification overrides (`classifyBulletType`) and a fallback anchored bullet extractor in `extractItemsFromArtifact` so both structured multi-section docs and free-form markdown artifacts in `evidence/artifacts/<convID>/` produce properly typed, repo-anchored `NoteItem` records capped at 6 items per note.
`decision · kind: episodic · anchors: internal/core/inbox/free_harvest.go, internal/core/tick/pipeline.go, internal/core/inbox/free_harvest_test.go, internal/core/tick/pipeline_test.go`

### [seed-81f5d9db] Computed deterministic SHA-256 content prefixes (`sha256Hex[:16]`) over ordered non-empty checkpoint sections (`checkpoint:<sessionID>:<hash16>`) and artifact file contents (`artifact:<convID>:<relPath>:<hash16>`) so unchanged checkpoints and artifacts are skipped in $O(1)$ via `inboxStore.HasDedupKey`.
`decision · kind: episodic · anchors: internal/core/inbox/free_harvest.go, internal/core/tick/pipeline.go, internal/core/inbox/free_harvest_test.go, internal/core/tick/pipeline_test.go`

## Open issues

### [seed-29440817] Legacy v1 oracle test suites under `evaluation/oracle/wo1-op-log`, `wo2-dossier-schema-renderer`, `wo4-index-migration`, `wo1-ledger-engine`, and `wo4-gates` reference removed `internal/core/claims` or unexported package symbols when running unscoped `go test ./...` across `evaluation/oracle/...`; all production and test packages under `./cmd/...`, `./internal/...`, and `./packaging/...` pass 100%.
`open-issue · kind: semantic · anchors: internal/core/inbox/free_harvest.go, internal/core/tick/pipeline.go, internal/core/inbox/free_harvest_test.go, internal/core/tick/pipeline_test.go`
