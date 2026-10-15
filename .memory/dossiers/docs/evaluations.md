# docs/evaluations

## Invariants & contracts

### [docseed-c506a563] X6 — **B1 replay (`X6-lite`) & Forward-Task Eval v2.** Re-run past work orders (`X6-lite`) and evaluate 2×2 forward-task design (`repo-memory` + `esp32`) where baseline is <50%. — `X6-lite` identified 84% baseline ceiling, 31.6% step reduction on `T2`, and `ephemeralMessage` bug (`docs/evaluations/x6-lite-verdict.md`). Eval v2 (`PR-11`) tests fixed v2.2 stack on non-ceiling tasks. — **X6...
`invariant · kind: semantic · anchors: docs/evaluations/x6-lite-verdict.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-dfbfa1dd] **231-Transcript PA Memory Retrieval Study (`docs/evaluations/pa-memory-retrieval-effectiveness.md`):** Auditing 231 real multi-tier sessions (`19,586` steps) across two repositories revealed that **passive "pull" memory (`MEMORY.md`, `WORKORDER.md` archives, and MCP pull tools) goes 94–99% unread by implementation workers** (`1.3%` read rate for `MEMORY.md`, `0` MCP pull calls across 231 tra...
`invariant · kind: semantic · anchors: docs/evaluations/pa-memory-retrieval-effectiveness.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

## Ruled out, failure signatures and gotchas

### [docseed-b21de2cf] **X6-lite B1 Replay (`docs/evaluations/x6-lite-verdict.md`):** Across 5 historical work orders replayed from their parent commits, the **No-Memory baseline already achieved 84.0% pass rate** (4.2/5 tasks passing `go test` on the first run) because work order prompts, nearby code comments, and compiler errors already specified the local task. In the single discriminating task (`T2: pr-01/wo4`),...
`failure-signature · kind: procedural · anchors: docs/evaluations/x6-lite-verdict.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`

### [docseed-1e145149] **77-Session `esp32` Static Doc Audit (`docs/evaluations/memory-value-evaluation-proposal.md`):** Even when rich human-authored documentation (`docs/developer/build-and-flash.md`, `.agents/skills/hil-Validation/SKILL.md`) exists in the repository, agents read it in only **1.3%–7.8% of sessions** unless prompted, repeatedly rediscovering build flags, flash partitions, and hardware traps from s...
`gotcha · kind: semantic · anchors: docs/evaluations/memory-value-evaluation-proposal.md · source_doc: docs/design-v2-distillation-retrieval.md · evidence: doc:docs/design-v2-distillation-retrieval.md`
