# Work Order: wo5-dogfood-doc-seed-and-verification

> **PR:** pr-11-v2-doc-seeding-and-lifecycles | **WO:** 5 of 5
> **Branch:** `pr-11-v2-doc-seeding-and-lifecycles/wo5-dogfood-doc-seed-and-verification`
> **Base:** `pr-11-v2-doc-seeding-and-lifecycles/wo4-async-doc-freshness-dreaming`
> **Complexity:** `standard`

## Objective

Dogfood `repo-memory seed-docs` and `{section?}` / `kind:` dossier rendering on `repo-memory-plugin`'s own `.memory/` directory, rebuild the installed `~/.local/bin/repo-memory` binary and `<data-dir>/<repo-id>/index.db`, and run full cross-platform and race-detector verification across the repository.

## Context & Architectural Constraints

- **Strictly zero internal references:** Never include `google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or local `/usr/local/google/...` paths in any files, comments, or commit messages.
- **Curated `doc_seeds` in `.memory/config.yaml`:**
  Configure `doc_seeds` at the top of `.memory/config.yaml` to seed from the repository's canonical architecture and reference docs (excluding literature surveys and evaluation reports):
  ```yaml
  doc_seeds:
    - "docs/antigravity-hooks-reference.md"
    - "docs/design-v2-distillation-retrieval.md"
    - "docs/addendum-a-provenance-and-hardening.md"
  ```
- **Idempotency & Cleanliness Verification:**
  - Running `repo-memory seed-docs` a second time immediately after the first run must report `0 added, 0 amended, 0 retired` and `3 unchanged skipped`.
  - All projected `.memory/dossiers/*.md` files must render `kind: <semantic|procedural|episodic>` in every entry metadata line, include `source_doc: <path>` on `doc-seed` entries, and omit empty optional sections (`{section?}`).
  - No entry in `.memory/dossiers/*.md` may contain junk fragments or non-existent file anchors.

## Checklist of Changes

- [ ] **1. Configure `doc_seeds` in `.memory/config.yaml`**:
  - Add `doc_seeds:` to `.memory/config.yaml` with:
    - `"docs/antigravity-hooks-reference.md"`
    - `"docs/design-v2-distillation-retrieval.md"`
    - `"docs/addendum-a-provenance-and-hardening.md"`
  - Note: check how `config.LoadRepoConfig` loads `.memory/config.yaml` vs `config.LoadScopeConfig`:
    - Verify `internal/core/config/config.go` (`LoadRepoConfig`) reads `doc_seeds` from `.memory/config.yaml` even when `config.yaml` is YAML (or ensure `LoadRepoConfig` unmarshals YAML/JSON `.memory/config.yaml` properly!).
    - **IMPORTANT CHECK:** Inspect `internal/core/config/config.go` (`LoadRepoConfig`). Does `LoadRepoConfig` only use `json.Unmarshal`, or does it also support YAML (e.g. `gopkg.in/yaml.v3`) when `.memory/config.yaml` is YAML? If `LoadRepoConfig` currently uses `json.Unmarshal` while `.memory/config.yaml` uses YAML syntax (`subsystems: ...`), update `LoadRepoConfig` in `internal/core/config/config.go` to try `yaml.Unmarshal` as well (or support both YAML and JSON seamlessly) so `doc_seeds:` in `.memory/config.yaml` is parsed cleanly without error!
- [ ] **2. Rebuild `~/.local/bin/repo-memory` and Run `repo-memory seed-docs`**:
  - Build and install `~/.local/bin/repo-memory`:
    ```bash
    go build -o ~/.local/bin/repo-memory ./cmd/repo-memory
    ```
  - Run `repo-memory seed-docs` from the repository root:
    ```bash
    ~/.local/bin/repo-memory seed-docs
    ```
  - Run `repo-memory seed-docs` a second time to verify idempotency (`0 added, 0 amended, 0 retired`, `3 unchanged skipped`).
  - Inspect the resulting `.memory/dossiers/*.md` and `.memory/ATLAS.md` to verify:
    - Every entry metadata line includes `kind: semantic` or `kind: procedural` or `kind: episodic`.
    - `doc-seed` entries include `source_doc: docs/...`.
    - Empty optional sections are omitted (`{section?}`).
- [ ] **3. Rebuild Local SQLite Index (`index.db`)**:
  - Run a small command or test/tick (or `go run` helper / `repo-memory status` / `tick`) so the local SQLite index (`index.db`) for `repo-memory-plugin` is rebuilt from `.memory/ops` with the new `v2.2` columns and `doc-seed` entries.
- [ ] **4. Full Verification Suite**:
  - Run:
    - `go build ./...`
    - `go vet ./...`
    - `GOOS=windows go build ./...`
    - `go test -race ./cmd/... ./internal/...`
  - Verify zero hygiene leaks (`google3`, `jetski`, `/usr/local/google`, `@google.com`) in any tracked files (`git diff pr-10b-delivery-and-storage-fixes/wo5-dry-run-verification..HEAD`).

## Verification Criteria

Run:
```bash
go build ./... && go vet ./... && GOOS=windows go build ./... && go test -race ./cmd/... ./internal/...
```
All commands must exit 0.

---

## Completion Report

*(To be filled in by the Execution Worker upon approval)*

- **Status:** Pending
- **Commits:**
- **Summary of Changes:**
- **Verification Output:**
