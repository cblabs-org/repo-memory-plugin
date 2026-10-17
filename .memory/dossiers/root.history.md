# root (History)

## Invariants & contracts

### [docseed-dfdc29da] **Architectural implication for `repo-memory`:** Because the child's first `PreInvocation` fires at `initialNumSteps == 0` before its prompt is written to `transcript.jsonl`, `PreInvocation` starts the child with the baseline startup dossier (`.memory/ATLAS.md` + `global`) via `userMessage` and automatically injects task-specific scopes on subsequent `PreInvocation` calls once the child reads/t...
`invariant · kind: semantic · anchors: .memory/ATLAS.md · source_doc: docs/antigravity-hooks-reference.md · evidence: doc:docs/antigravity-hooks-reference.md`
