---
name: pr-07-v2-storage-wo3
description: Handoff instructions for Work Order 3 — Scopes & Aliases
complexity: standard
---

# Role Definition

> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not branch, and do not plan. Your task is strictly limited to executing the checklist below.

# Objective

Build the scope and alias system that routes entries to the correct dossier. Scopes are either hand-declared in `config.yaml` or derived from the entity extractor's output. Aliases come from `split` ops.

# Context

## v2 Design References
- **§7.2 (Dossier kinds):** module (path globs + symbols), architectural topic, external system, procedure area
- **§6 (Consolidation):** routing by anchors and scopes; derived scopes proposed through the memory PR

## Scope Types
| Kind | Scope Source | Example |
|---|---|---|
| **Module** | Derived from anchors (path globs rolled up to parent directory) | `core/transcript-parser` for `internal/harness/antigravity/transcript*.go` |
| **Architectural topic** | Declared in `config.yaml` | `persistence-and-sqlite` |
| **External system** | Declared in `config.yaml` | `antigravity-hooks`, `esp-idf` |
| **Procedure area** | Declared in `config.yaml` | `release`, `flashing` |

## Existing Code
- **Entity extractor:** `internal/core/entities/extractor/` — extracts files, symbols, configs, topics, tools, errors, externals from episodes
- **Config:** `internal/core/config/` — existing config package
- **Existing config.yaml:** `.memory/config.yaml` — may already have some configuration
- **Dossiers (WO2):** `internal/core/dossiers/` — Entry has `DossierScope string`, Dossier has `Scope string` and `Kind DossierKind`
- **Ops (WO1):** `internal/core/ops/` — `split` op has `Fields["new_dossiers"]` and `Fields["entry_moves"]`

# Strict Boundary Box

**DO NOT touch:**
- `internal/core/ops/` — op log (WO1)
- `internal/core/dossiers/` — dossier schema (WO2) — but you WILL import from it
- `internal/core/pipeline/` — v1 pipeline (WO5)
- `internal/core/claims/` — v1 claims (WO5)
- `internal/core/index/` — index (WO4)
- `internal/core/delivery/` — delivery (PR-08)
- `cmd/repo-memory/` — CLI
- `internal/harness/`
- `.agents/`

**DO create/modify:**
- `internal/core/config/scopes.go` — new file (the main deliverable)
- `internal/core/config/scopes_test.go` — tests

# Tooling Constraints

- **Build:** `go build ./...` must pass
- **Test:** `go test ./internal/core/config/...` must pass
- **Vet:** `go vet ./internal/core/config/...` must pass

# Line-Item Instructions

## 1. Declared Scopes Config
- [ ] Create `internal/core/config/scopes.go`:
  - Define `ScopeConfig` struct:
    - `Topics []DeclaredScope` — architectural topics
    - `Externals []DeclaredScope` — external systems
    - `Procedures []DeclaredScope` — procedure areas
  - Define `DeclaredScope` struct:
    - `Name string` — scope name (e.g. `persistence-and-sqlite`)
    - `Keywords []string` — matching keywords
    - `ToolNames []string` — matching tool names (for externals)
    - `Commands []string` — matching commands (for procedures)
    - `Description string` — human-readable description
  - `func LoadScopeConfig(configPath string) (*ScopeConfig, error)` — reads from `config.yaml` under a `scopes:` key. Use `gopkg.in/yaml.v3` (already in the project). If the `scopes:` key is missing, return an empty config (not an error).

## 2. Derived Module Scopes
- [ ] Add to `internal/core/config/scopes.go`:
  - `type ModuleScope struct`:
    - `Name string` — derived scope name (e.g. `core/transcript-parser`)
    - `PathGlobs []string` — file paths that belong to this module
    - `Symbols []string` — symbol names that belong to this module
  - `func DeriveModuleScopes(anchors []string) []ModuleScope`:
    - Given a list of file path anchors, derive module scopes by rolling up to the nearest meaningful parent directory:
      - Strip the filename, take the directory
      - If a directory has ≤ 2 files, roll up to the parent
      - Use the path relative to `internal/` or `cmd/` or the repo root as the scope name
      - Convert path separators to `/` and prefix with the top-level dir (e.g. `core/transcript-parser` for `internal/core/episode/transcript_parser.go`)
    - Return deduplicated, sorted module scopes

## 3. Alias Resolution
- [ ] Add to `internal/core/config/scopes.go`:
  - `type AliasMap struct` — maps old scope names to new ones (from `split` ops)
  - `func BuildAliasMap(opsReader *ops.Reader, ctx context.Context) (*AliasMap, error)`:
    - Iterate all ops, collect `split` ops
    - For each split, map the old DossierScope → each of the new_dossiers
    - Return the alias map
  - `func (a *AliasMap) Resolve(scope string) string`:
    - Given a scope, follow alias chain to the current scope name
    - Prevent infinite loops (max depth 10)

## 4. Scope Resolver
- [ ] Add to `internal/core/config/scopes.go`:
  - `type ScopeResolver struct` — combines declared scopes, derived module scopes, and aliases
  - `func NewScopeResolver(config *ScopeConfig, modules []ModuleScope, aliases *AliasMap) *ScopeResolver`
  - `func (r *ScopeResolver) ResolveScope(anchors []string, keywords []string) string`:
    - First check aliases
    - Then try to match anchors against module scopes (path prefix match)
    - Then try to match keywords against declared topics, externals, procedures
    - Return the best matching scope name, or empty string if no match
  - `func (r *ScopeResolver) AllScopes() []string` — returns all known scope names (declared + derived + from aliases)

## 5. Tests
- [ ] Create `internal/core/config/scopes_test.go`:
  - Test `LoadScopeConfig` with a sample YAML that has topics, externals, and procedures
  - Test `LoadScopeConfig` with missing `scopes:` key returns empty config
  - Test `DeriveModuleScopes` with sample file paths, verify rollup and naming
  - Test `BuildAliasMap` with split ops, verify alias chain
  - Test `ScopeResolver.ResolveScope` with various anchor/keyword combinations
  - Test alias resolution follows the chain correctly

# Verification Requirement

1. Run `go build ./...` — must pass
2. Run `go test ./internal/core/config/...` — all tests must pass
3. Run `go vet ./internal/core/config/...` — must pass
4. Commit all changes on the current branch

**CRITICAL:** Before committing, you MUST ask the user if they are happy with the changes.

# Issue Surfacing

If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report (written by the worker at task completion)

<!-- Template — append this section when done -->
