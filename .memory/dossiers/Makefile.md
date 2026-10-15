# Makefile

## Procedures / Recipes

### [docseed-b52782ec] **Build-manifest sensitivity:** If a commit or working-tree diff modifies a repository build or environment manifest (`Makefile`, `CMakeLists.txt`, `platformio.ini`, `sdkconfig*`, `go.mod`, `package.json`, `Taskfile.yml`, `.github/workflows/*.yml`) anchored to a `recipe`, `tick` flags that recipe `stale_suspect: true` until its command next succeeds (`exit 0`).
`recipe · kind: procedural · anchors: Makefile, go.mod · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`
