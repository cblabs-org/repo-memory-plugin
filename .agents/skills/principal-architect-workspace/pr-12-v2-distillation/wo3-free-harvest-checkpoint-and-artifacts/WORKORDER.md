---
name: pr-12-v2-distillation-wo3
description: Handoff instructions for Work Order 3 (Free Zero-Token CHECKPOINT & brain/ Artifact Harvest)
complexity: standard
---

# pr-12-v2-distillation / WO3 — Free Zero-Token `CHECKPOINT` & `brain/` Artifact Harvest

## Role Definition
> You are an expert senior software engineer and execution-focused worker agent. You are NOT the Principal Architect. Do not make high-level architectural decisions, do not create branches other than the active branch (`pr-12-v2-distillation/wo3-free-harvest-checkpoint-and-artifacts`), and do not plan. Your task is strictly limited to executing the checklist below.
>
> **Authorization & Governance:** Read `.agents/rules/00-operating-model.md` and `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/PROJECT.md` (which records human approval of this 6-WO decomposition). You are authorized to implement and commit this Work Order autonomously within the Strict Boundary Box below.

## Objective
Implement deterministic zero-token Stage 1b harvest (`SPEC.md` §6.3.2 / `docs/design-v2-distillation-retrieval.md` §5.3) in `internal/core/inbox/free_harvest.go` and wire it into `PipelineExecutor.Run` (`internal/core/tick/pipeline.go`): extract structured candidate inbox notes (`source: "checkpoint"` and `source: "artifact"`) from episode `Checkpoints` (`Model Knowledge`, `User Knowledge`, `Work Accomplished`) and `<data-dir>/<repo-id>/evidence/artifacts/<conv-id>/**/*.md` anchored to repository paths, deduplicated idempotently across ticks.

## Strict Boundary Box
- **Allowed to create/modify:**
  - `internal/core/inbox/free_harvest.go` and `internal/core/inbox/free_harvest_test.go`
  - `internal/core/tick/pipeline.go` (and `internal/core/tick/pipeline_test.go` if needed)
  - `.agents/skills/principal-architect-workspace/pr-12-v2-distillation/wo3-free-harvest-checkpoint-and-artifacts/WORKORDER.md` (Completion Report)
- **Forbidden:**
  - Do NOT import `internal/harness/antigravity` or any concrete adapter from `internal/core/...` (`boundary_test.go` enforces this).
  - Do NOT modify `internal/core/scrub/` or `internal/core/pipeline/prepare/` (belongs to WO4).
  - NEVER include any internal references (`google3`, `cl/`, `b/`, `go/`, `jetski`, `@google.com`, or absolute `/usr/local/google/...` paths) in any code, comment, or commit message.
  - All git commits must use `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.

## Tooling Constraints
- Use standard `read_file`, `write_file`, `edit_file`, and `run_command` tools.
- Run `go test ./...` and `go vet ./...` to verify.

## Line-Item Instructions

- [ ] **1. `internal/core/inbox/free_harvest.go` — Stage 1b Free Zero-Token Distillations (`CHECKPOINT` & `brain/` Artifacts)**
  - Create `internal/core/inbox/free_harvest.go` in package `inbox`.
  - Implement `HarvestCheckpointsAndArtifacts(ctx context.Context, dataDir, repoID, repoRoot string, epStore *episode.Store, inboxStore *Store) (int, error)`:
    - If `inboxStore == nil`, initialize `inboxStore = NewStore(dataDir, repoID)`.
    - If `epStore == nil` && `dataDir != ""` && `repoID != ""`, initialize `epStore = episode.NewStore(dataDir, repoID)`.
    - Build a repo-path validator helper `isValidRepoAnchor(repoRoot, cand string) bool`:
      - Strip optional `#symbol` or `:symbol` suffix.
      - Require `seed.IsValidAnchorPathSyntax(cand)`.
      - If `repoRoot != ""`, verify `os.Stat(filepath.Join(repoRoot, filepath.FromSlash(pathPart)))` succeeds.
    - **Part A: Harvest Episode `CHECKPOINT` Summaries (`source: "checkpoint"`):**
      - List episode headers via `epStore.ListHeaders()`.
      - For each header `h` with `len(h.Checkpoints) > 0`:
        - Inspect sections `"Model Knowledge"`, `"User Knowledge"`, and `"Work Accomplished"` (in deterministic order).
        - Compute a deterministic SHA-256 content hash over the non-empty text of those sections (`sha256Hex[:16]`) and construct `dedupKey := fmt.Sprintf("checkpoint:%s:%s", h.SessionID, sha256Hex[:16])`.
        - Check `exists, err := inboxStore.HasDedupKey(dedupKey)`. If `exists`, skip.
        - Extract markdown bullet points (`- `, `* `, `1. `) from `"Model Knowledge"` and `"User Knowledge"` (and any bullet in `"Work Accomplished"` that explicitly states a decision, invariant, gotcha, or ruled-out approach):
          - Filter out junk/boilerplate via `seed.IsJunkText(bullet)`.
          - Extract candidate file/directory/symbol anchors from inline backticks `` `...` `` or path tokens in the bullet (or fallback to valid repo anchors mentioned in `"Files and Code"` if the bullet discusses a module rule/decision), keeping only those passing `isValidRepoAnchor(repoRoot, a)`.
          - Require at least 1 valid repository anchor per extracted `NoteItem` so unanchored narrative bullets are never promoted.
          - Classify `NoteItem.Type` deterministically from bullet keywords:
            - `"ruled out"`, `"failed"`, `"did not work"`, `"abandoned"`, `"avoid"` -> `dossiers.EntryTypeRuledOut`
            - `"must "`, `"never "`, `"always "`, `"invariant"` -> `dossiers.EntryTypeInvariant`
            - `"error"`, `"panic"`, `"symptom"`, `"fix:"` -> `dossiers.EntryTypeFailureSignature`
            - `"gotcha"`, `"caveat"`, `"quirk"`, `"note:"` -> `dossiers.EntryTypeGotcha`
            - default -> `dossiers.EntryTypeDecision`
          - Populate `Title` (concise heading/prefix up to 80 chars), `Why` (full bullet explanation), `Do` (actionable part if required by the type), `Anchors`, `AnchorChecks: CheckAnchors(repoRoot, anchors)`, and `Evidence: fmt.Sprintf("checkpoint (%s)", sectionName)`.
        - If `len(items) > 0` (cap at 6 items per note):
          - Write `&Note{Source: SourceCheckpoint, ConversationID: h.SessionID, Model: h.Model, HarnessVersion: h.HarnessVersion, Branch: h.Branch, HeadCommit: nonEmpty(h.HeadEnd, h.HeadStart), Items: items, DedupKey: dedupKey}` via `inboxStore.Write(note)` and increment `notesCreated`.
    - **Part B: Harvest Conversation Artifacts (`source: "artifact"`):**
      - Scan `<dataDir>/<repoID>/evidence/artifacts/` across conversation subdirectories `<convID>/`. If the directory does not exist, return `notesCreated, nil`.
      - Walk each `<convID>` directory for regular `.md` files (skipping `.metadata.json`, `.resolved`, `.md.resolved`, and `.system_generated`).
      - For each artifact markdown file:
        - Read content and compute `sha256Hex[:16]` of the content. Construct `dedupKey := fmt.Sprintf("artifact:%s:%s:%s", convID, filepath.ToSlash(relPath), sha256Hex[:16])`.
        - Check `exists, err := inboxStore.HasDedupKey(dedupKey)`. If `exists`, skip.
        - Extract anchored items either via `seed.ParseDocToOps(relPath, string(content), "", time.Now().UTC(), nil, func(p string) bool { return isValidRepoAnchor(repoRoot, p) })` (converting each resulting `ops.Op` into a `NoteItem` with `Type`, `Title`, `Why`, `Do`, `Anchors`, `AnchorChecks: CheckAnchors(repoRoot, anchors)`, `Evidence: fmt.Sprintf("artifact:%s", filepath.ToSlash(relPath))`) AND/OR via the same anchored bullet extractor used for checkpoints if `ParseDocToOps` returns 0 ops on a free-form artifact.
        - Require every `NoteItem` to have at least 1 valid repository anchor (`len(item.Anchors) > 0`) and non-junk text (`!seed.IsJunkText(item.Title)`).
        - If `len(items) > 0` (cap at 6 items per artifact note):
          - Write `&Note{Source: SourceArtifact, ConversationID: convID, Items: items, DedupKey: dedupKey}` via `inboxStore.Write(note)` and increment `notesCreated`.
    - Return `notesCreated, nil`.

- [ ] **2. `internal/core/tick/pipeline.go` — Wire `HarvestFreeSources` into `PipelineExecutor.Run`**
  - Add method `(p *PipelineExecutor) HarvestFreeSources(ctx context.Context) (int, error)` calling `inbox.HarvestCheckpointsAndArtifacts(ctx, p.dataDir, p.repoID, p.repoRoot, p.epStore, inbox.NewStore(p.dataDir, p.repoID))`.
  - In `(p *PipelineExecutor) Run(ctx context.Context) error`, call `if _, err := p.HarvestFreeSources(ctx); err != nil { return err }` right after `p.convertCapturesToEvidence(ctx)`.

- [ ] **3. `internal/core/inbox/free_harvest_test.go` — Unit Tests**
  - Test `HarvestCheckpointsAndArtifacts` extracting a `source: "checkpoint"` note from an episode in `epStore` whose `Header.Checkpoints["Model Knowledge"]` and `Header.Checkpoints["User Knowledge"]` contain bullets referencing real files in `repoRoot`, verifying `AnchorChecks`, `DedupKey`, and `ListUnconsolidated()`.
  - Test `HarvestCheckpointsAndArtifacts` extracting a `source: "artifact"` note from a markdown artifact under `<dataDir>/<repoID>/evidence/artifacts/<convID>/analysis.md` referencing real files in `repoRoot` (while skipping `.metadata.json`, `.md.resolved`, and unanchored/junk bullets).
  - Test idempotency: running `HarvestCheckpointsAndArtifacts` (or `PipelineExecutor.HarvestFreeSources`) a second time with unchanged checkpoints and artifacts produces `0` new notes and does not duplicate inbox files.

## Verification Requirement
- Run `go test ./...` and `go vet ./...` and verify 100% pass (including `internal/core/boundary_test.go`).
- Ensure `git status` shows only expected files in the Strict Boundary Box.
- Commit your changes on `pr-12-v2-distillation/wo3-free-harvest-checkpoint-and-artifacts` using `--author="matthewmcneill <97467074+matthewmcneill@users.noreply.github.com>"`.
- Append your Completion Report to this `WORKORDER.md`, commit it, and send a message back to the Principal Architect confirming completion.

## Issue Surfacing
If you discover a defect, upstream bug, or concern *outside your boundary box*, surface it in your Completion Report below — do not file issues directly.

---

## Completion Report
<!-- Written by the worker at task completion -->
