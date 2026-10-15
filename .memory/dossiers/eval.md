# eval

## Ruled out, failure signatures and gotchas

### [seed-6f48704f] When adding packages to index and runner, `index.EpisodeIndex` was incorrect; the correct type was `index.Index` and `InsertEpisode`.
`ruled-out · kind: episodic · anchors: internal/eval/dataset.go, internal/eval/store.go, internal/eval/runner.go, cmd/repo-memory/cmd_eval.go, cmd/repo-memory/main.go, internal/eval/eval_test.go`

### [seed-c868830a] `git add` behavior: `.gitignore` has an entry for `repo-memory` which causes git to ignore the `cmd/repo-memory` directory entirely. Use `git add -f` to stage files inside this directory.
`ruled-out · kind: episodic · anchors: internal/eval/dataset.go, internal/eval/store.go, internal/eval/runner.go, cmd/repo-memory/cmd_eval.go, cmd/repo-memory/main.go, internal/eval/eval_test.go`

## Decisions

### [seed-54e8e651] Used JSONL for result storage because it easily supports appending individual runs and meshes well with `encoding/json`.
`decision · kind: episodic · anchors: internal/eval/dataset.go, internal/eval/store.go, internal/eval/runner.go, cmd/repo-memory/cmd_eval.go, cmd/repo-memory/main.go, internal/eval/eval_test.go`

### [seed-8f2ba13e] Used `-f` to `git add` the CLI command `cmd/repo-memory/cmd_eval.go` because `.gitignore` globally ignores anything matching `repo-memory` unless specified otherwise.
`decision · kind: episodic · anchors: internal/eval/dataset.go, internal/eval/store.go, internal/eval/runner.go, cmd/repo-memory/cmd_eval.go, cmd/repo-memory/main.go, internal/eval/eval_test.go`

## Procedures / Recipes

### [docseed-c7299f01] **Tooling.** `repo-memory eval compare <a.json> <b.json>` in `internal/eval` does the pairing, tests and correction.
`recipe · kind: procedural · anchors: internal/eval · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`
