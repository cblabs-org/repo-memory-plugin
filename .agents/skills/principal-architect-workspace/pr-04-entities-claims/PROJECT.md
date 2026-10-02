# PR-04: Entities, Claims, Index & Events Store

> **Programme:** repo-memory
> **PR:** pr-04-entities-claims
> **Base branch:** `main`
> **Status:** In Progress
> **SPEC:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Build the data model layer: entity extraction from the repository (no LLM), the claim file schema and store, the event store for human decisions, and the rebuildable SQLite index. This is the foundation the pipeline writes into and delivery reads from.

## Work Order Ledger

| # | WO | Branch | Status | Complexity | Worker |
|---|---|---|---|---|---|
| 1 | `wo1-entity-types` | `pr-04-entities-claims/wo1-entity-types` | ☑ Complete | standard | — |
| 2 | `wo2-entity-extractor` | `pr-04-entities-claims/wo2-entity-extractor` | ☑ Complete | architect | — |
| 3 | `wo3-claim-schema` | `pr-04-entities-claims/wo3-claim-schema` | ☑ Complete | standard | — |
| 4 | `wo4-claim-store` | `pr-04-entities-claims/wo4-claim-store` | ☑ Complete | standard | — |
| 5 | `wo5-sqlite-index` | `pr-04-entities-claims/wo5-sqlite-index` | ☑ Complete | standard | — |
| 6 | `wo6-subsystem-config` | `pr-04-entities-claims/wo6-subsystem-config` | ☑ Complete | standard | — |
| 7 | `wo7-events-store` | `pr-04-entities-claims/wo7-events-store` | ☐ Pending | standard | — |

## Exit Criteria
- Entity extractor produces correct entities from test fixtures
- Claim files round-trip (write → read → write produces identical output)
- Index is rebuildable from claim files, entities and events
- `repo-memory reindex` works
- All entity kinds from §5.2 are extractable
- A review event recorded against a claim still applies after re-projection

## Session Log
- **2026-10-02**: Human approved the Work Order decomposition for PR-04.

- **WO1**: Entity Types complete. Entities, Relations, and Registry interface defined.
- **WO2**: No-LLM entity extractor implemented successfully (file tree walk, config parsing, C++ symbol scanner, rule-based extraction).
- **WO3**: Claim schema, ULID generation, and YAML+Markdown parser/writer implemented.
- **WO4**: Claim store, month-sharded CRUD operations, and inverse relation index implemented.
- **WO5**: SQLite index schema, WAL concurrency, write transactions, and the rebuild mechanism added. Added `reindex` CLI command.
- **WO6**: Subsystem configuration glob matching and unassigned file detection implemented.
