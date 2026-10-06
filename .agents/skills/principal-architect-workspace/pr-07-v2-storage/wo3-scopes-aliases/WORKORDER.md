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
- [x] Create `internal/core/config/scopes.go`:
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
- [x] Add to `internal/core/config/scopes.go`:
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
- [x] Add to `internal/core/config/scopes.go`:
  - `type AliasMap struct` — maps old scope names to new ones (from `split` ops)
  - `func BuildAliasMap(opsReader *ops.Reader, ctx context.Context) (*AliasMap, error)`:
    - Iterate all ops, collect `split` ops
    - For each split, map the old DossierScope → each of the new_dossiers
    - Return the alias map
  - `func (a *AliasMap) Resolve(scope string) string`:
    - Given a scope, follow alias chain to the current scope name
    - Prevent infinite loops (max depth 10)

## 4. Scope Resolver
- [x] Add to `internal/core/config/scopes.go`:
  - `type ScopeResolver struct` — combines declared scopes, derived module scopes, and aliases
  - `func NewScopeResolver(config *ScopeConfig, modules []ModuleScope, aliases *AliasMap) *ScopeResolver`
  - `func (r *ScopeResolver) ResolveScope(anchors []string, keywords []string) string`:
    - First check aliases
    - Then try to match anchors against module scopes (path prefix match)
    - Then try to match keywords against declared topics, externals, procedures
    - Return the best matching scope name, or empty string if no match
  - `func (r *ScopeResolver) AllScopes() []string` — returns all known scope names (declared + derived + from aliases)

## 5. Tests
- [x] Create `internal/core/config/scopes_test.go`:
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

### Status
Completed

### Changes Made
- Created `internal/core/config/scopes.go`:
  - `ScopeConfig` & `DeclaredScope`: Implemented schema for hand-declared architectural topics, external systems, and procedure areas with YAML decoding (supporting `tool_names` and `toolNames`).
  - `LoadScopeConfig(configPath string)`: Reads scopes from `config.yaml` under `scopes:`, gracefully returning an empty configuration when the key is omitted or the file does not exist.
  - `ModuleScope` & `DeriveModuleScopes(anchors []string)`: Derives module scopes from file path anchors, rolling up directories with ≤ 2 files to the nearest meaningful parent directory under `internal/`, `cmd/`, or repo root. Slugs names with hyphenated directory segments (`_` -> `-`), extracts anchor symbols, deduplicates, and sorts scopes.
  - `AliasMap` & `BuildAliasMap(opsReader, ctx)`: Extracts `split` ops from `ops.Reader`, recording old dossier scopes to replacement scopes. `Resolve(scope string)` resolves alias chains with cycle prevention and depth capped at 10.
  - `ScopeResolver`: Combines declared scopes, derived module scopes, and aliases. `ResolveScope(anchors, keywords)` resolves by aliases first, then module scopes via path prefix match, then declared scopes via keyword/tool/command matching. Implemented `AllScopes()` and `ScopeKind()`.
- Created `internal/core/config/scopes_test.go`:
  - `TestLoadScopeConfig_SampleYAML`: Validates parsing of topics, externals, and procedures.
  - `TestLoadScopeConfig_MissingScopesKey`: Validates fallback to empty config when key or file is missing.
  - `TestDeriveModuleScopes_RollupAndNaming`: Validates rollup threshold (≤ 2 files), naming/slugification, symbol extraction, and sorting.
  - `TestBuildAliasMap_SplitOps`: Validates alias map construction from real `ops.OpBatch` split ops and chain resolution.
  - `TestAliasMap_ResolveChainAndCycle`: Validates multi-step chains, cycle termination, unknown scopes, and nil map safety.
  - `TestScopeResolver_ResolveScope`: Validates alias precedence, module prefix matching, symbol matching, and keyword matching.
  - `TestScopeResolver_AllScopesAndScopeKind`: Validates retrieval of all known scope names and kind mappings.

### Decisions Made
- Anchors and path segments are normalized with forward slashes and underscores converted to hyphens to ensure consistent kebab-case scope names matching the v2 design.
- `ScopeResolver.ResolveScope` prioritizes aliases first, then module scopes (path prefix match), then declared scopes (keywords, tool names, commands), resolving through aliases at every step.
- `AliasMap.Resolve` protects against recursive alias loops using both a visited-set cycle detector and a maximum recursion depth of 10.
- `ScopeKind` connects resolved scope names to `dossiers.DossierKind` (`module`, `topic`, `external`, `procedure`).

### Verification Results
- `go build ./...`: passed cleanly.
- `go test -v ./internal/core/config/...`: all tests passed (7 suites, all green).
- `go vet ./internal/core/config/...`: passed with 0 warnings.
- `go test ./internal/core/...`: all tests passed.
- Git commit created on branch `pr-07-v2-storage/wo3-scopes-aliases`: `fdb82ed feat(config): add scope resolver with declared, derived & alias support`.

### Blockers
None.

### Out-of-Scope Findings
None.
