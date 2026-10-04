# PR-10: Turn Notes, Fallback & Preparation (v2 Distillation)

> **Programme:** repo-memory
> **PR:** pr-10-v2-distillation
> **Base branch:** PR-09-v2-x6-lite
> **Status:** ☐ Planned
> **SPEC ref:** [SPEC.md](../../../SPEC.md) v0.10
> **Plan ref:** [PROGRAM-PLAN.md](../programme-repo-memory/PROGRAM-PLAN.md)

## Objective

Produce knowledge continuously:
- turn notes at milestones;
- marks;
- the subagent registry;
- the preparation layer;
- the lean-agent fallback;
- note quality validated on real work (X2).

## Design & SPEC Sections

- **v2 Design Sections:** §5.1–5.6
  - §5.1 Turn Notes at Milestones (`Stop` continue, gate and harvest)
  - §5.2 Marks & State Management
  - §5.3 Subagent Registry
  - §5.4 Lean-Agent Sidecar Fallback
  - §5.5 Op Types & Schema Integration
  - §5.6 Preparation Layer (Defences Against Misdirection and Prompt Injection)
- **Milestone Signals Defined:**
  - Commit (`git commit` succeeded)
  - PR opened (`gh pr create` succeeded)
  - Work order complete (PA completion report written)
  - Error resolved (the same command prefix failed, then passed, after ≥ 15 steps)
  - Multi-file edit followed by build/test (`≥ 2 files edited followed by a build or test`)
  - Cooldown: 30 minutes, waived for commits and PRs; daily cap: 20 per repository
  - Autonomous goal handling: condition applies only if X1 found a detectable signal; otherwise dropped
- **Turn Notes Configuration & Role Opt-Out:**
  - Settings: `turn_notes: on | fallback-only | off`
  - Opt-out by role: PA and worker sessions (per X1 findings)
  - Loop protection: as specified in v2 design
- **File & Package Paths:**
  - `internal/core/state/` (per-conversation marks, subagent registry)
  - `internal/core/pipeline/prepare/` (§5.6 defences, sanitized container records, validator)

## Dependencies & Gate Preconditions

- **Base branch:** `PR-09-v2-x6-lite`
- **Gate Precondition:**
  - **Early kill gate (X6-lite) must have passed:** time-cut dossiers demonstrated improvement over no-memory on the primary metric, and the user decided to continue.
- **Prerequisites:**
  - PR-05-v2-foundation cost telemetry and `/usage` quota-guard library.
  - PR-06-v2-spike findings: X1 (`Stop` continue and role opt-outs), X8 (lean agent install location and flags), and X9 (hook contracts).
  - PR-08-v2-delivery exposure list attached to mark state.

## Work Order Ledger

| # | WO | Scope | Complexity | Status |
|---|---|---|---|---|
| 1 | `wo1-stop-gate-harvest` | The production `Stop` gate and harvest, per v2 §5.1 and the X1/X9 findings. **Milestone signals defined:** commit (`git commit` succeeded); PR opened (`gh pr create` succeeded); work order complete (PA completion report written); error resolved (the same command prefix failed, then passed, after ≥ 15 steps); ≥ 2 files edited followed by a build or test. Cooldown 30 minutes, waived for commits and PRs; daily cap 20 per repository. **Settings:** `turn_notes: on \| fallback-only \| off`, and opt-out by role (PA and worker sessions, per X1). The "autonomous goal" condition applies only if X1 found a detectable signal; otherwise it is dropped. Loop protection as in the design. | architect | ☐ Planned |
| 2 | `wo2-marks` | `internal/core/state/`: per-conversation marks (step index, byte offset, timestamp), clamped on rewind, unmoved by checkpoints. The exposure list from PR-08 is attached to each note at harvest. Previous note titles are listed in the request after a checkpoint. | standard | ☐ Planned |
| 3 | `wo3-subagent-registry` | A SQLite table mapping child conversation → parent, populated from `PostToolUse` on `invoke_subagent` (`toolCall` is present). Used to exclude subagents from turn notes and to link them for the fallback. | standard | ☐ Planned |
| 4 | `wo4-preparation-layer` | `internal/core/pipeline/prepare/`: the §5.6 defences. Strip file reads, our own injections, `Stop` reasons, user-input wrappers and subagent prompts; reframe checkpoint headers; produce JSON records in delimited containers with framing; validate output (citations to agent, tool or human steps; rejection of copied instructions; random-token echo; detection of task-doing). **X10** test set of instruction-heavy sessions. | architect | ☐ Planned |
| 5 | `wo5-sidecar-fallback` | For conversations idle ≥ 2 hours with uncovered milestones: quota guard (PR-05 library); the lean agent from its X8-verified install location; input = latest checkpoint + tail since the later of checkpoint and mark + artifacts, prepared by wo4; batched prompts or `stream-json`. Notes tagged `source: sidecar`. Reuse the PR-07 seeding prompt where applicable. | architect | ☐ Planned |
| 6 | `wo6-note-quality` | **X2.** Run turn notes on this PR's own work orders and the next PR's, alongside the PA. Compare notes with the matching completion-report sections, blind. Tune the request wording; record model, length and types; cover Flash and Pro sessions. | interactive | ☐ Planned |

## Exit Criteria

- Harvest succeeds on ≥ 95% of ≥ 30 milestone requests; non-milestone turns stay silent; the settings work.
- Marks are correct through rewinds and checkpoints; subagents are excluded.
- **X10:** derailed outputs ≤ 2% on the test set, none reaching the inbox.
- The fallback produces notes within the quota budget.
- **X2:** notes judged at least as useful in ≥ 50% of pairs, at ≤ ⅓ of the length.

## Human Tasks

- **Blind-Judge Note Pairs (X2):** Blind-judge the X2 note pairs comparing turn notes against matching PA completion-report sections (wo6).
- **Role Opt-Out Confirmation:** Confirm the role opt-out settings for PA and worker sessions (wo1).

## Risks

- **Turn notes disrupt PA or worker sessions:** Continuation prompts could disrupt PA orchestration or worker completion reports. Mitigation: Use X1 prototype observations; configure role opt-out for PA/workers; non-milestone turns stay silent.
- **Users find end-of-turn notes noisy:** Visible continuations or unexpected messages. Mitigation: Strict milestone gate, 30-minute cooldown, daily cap of 20 per repository, and `turn_notes: on | fallback-only | off` config.
- **Processing models follow instructions in their input:** Sidecar or turn-note models misdirected by instructions in transcripts or artifacts. Mitigation: §5.6 defences (preparation layer in wo4), strip file reads and injected prompts, delimited JSON containers, output validation, and X10 test set verification (derailed outputs ≤ 2%).
- **Agents write wrong or self-serving notes:** Erroneous claims recorded into memory. Mitigation: Citations required; `verified` status only from tests, commands, merges, or humans; downstream anchor validation and memory-PR review.
- **Quota counts cached tokens at full rate:** High cost for turn-note continuations. Mitigation: Milestone-only gate; fallback-only option; quota-guard enforcement.

## Session Log

*(empty, ready for use)*
