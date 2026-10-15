# Makefile

## Ruled out, failure signatures and gotchas

### [docseed-a5f09188] **`procedural`** *(Executable Workflows & Error Playbooks)* — `recipe` (alias `procedure`), `failure-signature` — Verified step-by-step commands/workflows and symptom → cause → fix patterns — **Execution-Outcome Coupled:** `PostToolUse` tracks command exit codes against active recipes (`success_count` / `failure_count`) and flags `stale_suspect` when build manifests (`Makefile`, `CMak...
`failure-signature · kind: procedural · anchors: Makefile · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

## Procedures / Recipes

### [docseed-b52782ec] **Build-manifest sensitivity:** If a commit or working-tree diff modifies a repository build or environment manifest (`Makefile`, `CMakeLists.txt`, `platformio.ini`, `sdkconfig*`, `go.mod`, `package.json`, `Taskfile.yml`, `.github/workflows/*.yml`) anchored to a `recipe`, `tick` flags that recipe `stale_suspect: true` until its command next succeeds (`exit 0`).
`recipe · kind: procedural · anchors: Makefile, go.mod · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`
