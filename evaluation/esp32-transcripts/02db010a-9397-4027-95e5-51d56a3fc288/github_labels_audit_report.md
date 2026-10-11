# Comprehensive Audit: GitHub Labels in Bug Fixer Loop Plugin

> Complete audit of all GitHub label references across the codebase at `/Users/mcneillm/Documents/Projects/bug-fixer-loop-plugin`.

## Executive Summary

- **Total Unique Labels Tracked**: 52 labels/patterns
- **Core Bug-Fixer Workflow Labels to Rename**: 15 distinct labels + 2 dynamic prefixes (`complexity:*`, `batch:*`)
- **Total Affected Files (Core Bug-Fixer Workflow)**: **34 files** (12 active code/config files, 9 rules & skills, 2 docs, 11 historical workorders)
- **Total Distinct Line References to Update**: **386 lines** (126 in active code/config, 138 in rules & skills, 17 in docs, 105 in historical workorders)
- **Labels NOT Recommended for Renaming**: Priority labels (`p0-p3`, `urgent`, etc.), target repo exclusion/effort labels (`hardware-required`, `P1-blocker`, `S/L/XL`), security quarantine (`security:prompt-injection`), and plugin repo internal tracking labels (`component:*`, `found-in:*`, `backfill`).

## 1. Label Categorization & Namespacing Status

| Current Label | Category | Current Namespacing | Proposed Renamed Label | Rename? |
|---|---|---|---|---|
| `auto-fixable` | Triage / Classification (T1) | Bare | `bug-fixer:auto-fixable` | ✅ YES |
| `needs-approach-review` | Triage / Classification (T2) | Bare | `bug-fixer:needs-approach-review` | ✅ YES |
| `input-needed` | Triage / Classification (T3) | Bare | `bug-fixer:input-needed` | ✅ YES |
| `project-candidate` | Triage / Classification (T4) | Bare | `bug-fixer:project-candidate` | ✅ YES |
| `human-required` | Triage / Classification (T5) | Bare | `bug-fixer:human-required` | ✅ YES |
| `status:in-progress` | Issue Status / Lease | Namespaced (`status:`) | `bug-fixer:in-progress` *(or `bug-fixer:status:in-progress`)* | ✅ YES |
| `status:awaiting-review` | Issue Status | Namespaced (`status:`) | `bug-fixer:awaiting-review` *(or `bug-fixer:status:awaiting-review`)* | ✅ YES |
| `bug-fixer` | PR Identification / Meta | Bare (unprefixed) | `bug-fixer` *(already has prefix)* or `bug-fixer:pr` | ✅ Keep or adjust |
| `bug-fixer-health` | Diagnostic / System Report | Bare (kebab-case) | `bug-fixer:health` | ✅ YES |
| `spec-change-request` | Escalation / Spec Alignment | Bare | `bug-fixer:spec-change-request` | ✅ YES |
| `architect-approved` | PR Review Loop | Bare | `bug-fixer:architect-approved` | ✅ YES |
| `needs-iteration` | PR Review Loop | Bare | `bug-fixer:needs-iteration` | ✅ YES |
| `needs-human-intervention` | PR Review Loop Escalation | Bare | `bug-fixer:needs-human-intervention` | ✅ YES |
| `discovered-in-review` | Tech Debt Discovery Chain | Bare | `bug-fixer:discovered-in-review` | ✅ YES |
| `complexity:mechanical` | Model Routing Tier | Namespaced (`complexity:`) | `bug-fixer:complexity:mechanical` | ✅ YES |
| `complexity:standard` | Model Routing Tier | Namespaced (`complexity:`) | `bug-fixer:complexity:standard` | ✅ YES |
| `complexity:architect` | Model Routing Tier | Namespaced (`complexity:`) | `bug-fixer:complexity:architect` | ✅ YES |
| `batch:<slug>` | PR Bundle Affinity | Namespaced (`batch:`) | `bug-fixer:batch:<slug>` | ✅ YES |
| `security:prompt-injection` | Security Quarantine | Namespaced (`security:`) | `security:prompt-injection` | ❌ NO (Standard security taxonomy) |
| `hardware-required` | Target Repo Exclusion | Bare | `hardware-required` | ❌ NO (Target repo native label) |
| `P1-blocker` | Target Repo Exclusion / Severity | Bare | `P1-blocker` | ❌ NO (Target repo native label) |
| `urgent`, `high`, `medium`, `low` | Target Repo Priority | Bare | `urgent`, `high`, `medium`, `low` | ❌ NO (Target repo native priority) |
| `p0`, `p1`, `p2`, `p3` | Target Repo Priority | Bare | `p0`, `p1`, `p2`, `p3` | ❌ NO (Target repo native priority) |
| `priority:urgent/high/med/low` | Target Repo Priority | Namespaced (`priority:`) | `priority:*` | ❌ NO (Target repo native priority) |
| `S`, `S-effort`, `L`, `XL`, etc. | Target Repo Effort Heuristic | Bare | `S`, `L`, `XL` | ❌ NO (Target repo native effort) |
| `P2-correctness`, `P3-docs-dx`, `P4-enhancement` | Plugin Repo Issue Severity | Bare | Unchanged | ❌ NO (Plugin repo internal) |
| `component:*` (`triage`, `engineering`, etc.) | Plugin Repo Components | Namespaced (`component:`) | Unchanged | ❌ NO (Plugin repo internal) |
| `found-in:<repo>`, `backfill` | Plugin Repo Attribution | Namespaced / Bare | Unchanged | ❌ NO (Plugin repo internal) |

---

## 2. Config Schema Analysis (`config.json`)

### Current Schema Representation
1. **`required_labels`** (`array[string]`):
   - Maintained in `config.json` (lines 9-19) and `.agents/plugins/bug-fixer-loop-workspace/config.json` (lines 24-34).
   - Default array currently defines 9 labels:
     - `auto-fixable`
     - `human-required`
     - `needs-approach-review`
     - `project-candidate`
     - `input-needed`
     - `status:in-progress`
     - `bug-fixer`
     - `bug-fixer-health`
     - `spec-change-request`
2. **`exclusion_labels`** (`array[string]`):
   - In workspace config (lines 16-20): `["human-required", "P1-blocker", "hardware-required"]`.
   - If `human-required` is renamed, this default must be updated.
3. **`auto_fixable_labels`** (`array[string]`):
   - In workspace config (lines 21-23): `["auto-fixable"]`.
   - Must be updated to `["bug-fixer:auto-fixable"]`.
4. **Missing from `required_labels` Schema**:
   - The following labels are created ad-hoc or used by dispatchers/skills without being declared in `required_labels`:
     - `discovered-in-review` (created in `configure.sh:390`)
     - `status:awaiting-review` (used when PR is opened to free concurrency slot)
     - `architect-approved` (used by architect review loop)
     - `needs-iteration` (used when architect requests changes)
     - `needs-human-intervention` (used when PR exceeds max review rounds)
     - `complexity:mechanical`, `complexity:standard`, `complexity:architect` (created dynamically in `bug-fixer-triage/SKILL.md`)
   - **Recommendation**: Expand `required_labels` to include all workflow labels so `configure.sh` provisions them in one step.

---

## 3. Setup Logic Analysis (`install.sh`, `configure.sh`)

### `scripts/install.sh`
- Lines 170-176: Hardcodes Python defaults when generating workspace config for a new repo:
  ```python
  "exclusion_labels": ["human-required", "P1-blocker", "hardware-required"],
  "auto_fixable_labels": ["auto-fixable"],
  "required_labels": [
      "auto-fixable", "human-required", "needs-approach-review",
      "project-candidate", "input-needed", "status:in-progress",
      "bug-fixer", "bug-fixer-health", "spec-change-request"
  ]
  ```
- Note: `install.sh` does not run `gh label create`; it writes the workspace configuration.

### `scripts/configure.sh`
- Lines 353-376: Defines `LABELS_META` dictionary with colors and descriptions and `REQUIRED_LABELS` array.
- Lines 377-387: Iterates over `REQUIRED_LABELS` and runs `gh label create "${label}" --repo "${TARGET_REPO}" --force ...`.
- Lines 390-394: Creates `discovered-in-review` label separately.
- Lines 475-491: Merges `req_labels` into `config.json`.
- Lines 539-545 & 687-691: Dry-run / interactive triage preview logic using `exclusion_labels`, `auto_labels`, `small_effort_labels`, `large_effort_labels`.
- Lines 739-743: Applies classified labels via `gh issue edit ... --add-label <label>`.

---

## 4. Dispatchers Analysis

1. **`engineering_dispatcher.py`**:
   - Line 159: Checks `name.startswith("complexity:")` to map model tier (`mechanical` → `flash_lite`, `standard` → `flash`, `architect` → `pro`).
   - Line 244, 302, 549: Removes lease label: `--remove-label status:in-progress`.
   - Line 285, 532: Prompt context: mentions `status:in-progress` is applied.
   - Line 289, 536: Prompt instruction: transition to `status:awaiting-review` upon PR creation.
   - Line 326: Concurrency check: `gh issue list --label status:in-progress`.
   - Line 346: Queries PRs: `gh pr list --label bug-fixer,needs-iteration`.
   - Line 397, 429: Prompt instruction: remove label: `gh pr edit ... --remove-label needs-iteration`.
   - Line 416: Evaluates PR body when `needs-iteration` is present.
   - Line 456: Tracking metadata: `labels_to_restore=["needs-iteration"]`.
   - Line 470: Queries candidate issues: `safe_ingest.py issue-list --label auto-fixable`.
   - Line 482: Filters out issues having `status:in-progress` or `status:awaiting-review`.
   - Line 510: Claims issue: `gh issue edit <num> --add-label status:in-progress`.

2. **`triage_dispatcher.py`**:
   - Lines 22-28: `CLASSIFICATION_LABELS = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`.
   - Lines 120-122: `SKIP_LABELS = CLASSIFICATION_LABELS | {"status:in-progress"}`.
   - Line 128: Filters out issues possessing any of `SKIP_LABELS`.
   - Line 133: Checks `status:awaiting-review` (only skips if classification label is also present).

3. **`architect_dispatcher.py`**:
   - Line 132: Queries open draft PRs: `gh pr list --label bug-fixer --state open --draft`.
   - Line 165: Skips PR if `needs-iteration` or `architect-approved` is in `labels`.
   - Lines 194-196: On reaching `MAX_REVIEW_ROUNDS` (5), escalates via: `gh pr edit <num> --add-label needs-human-intervention`.

4. **`trajectory_dispatcher.py`**:
   - Line 120: Detects bug-fixer transcripts by scanning for keywords including `auto-fixable`.
   - Line 150: Checks count of active issues: `gh issue list --label status:in-progress`.
   - Line 156: Checks count of PRs: `gh pr list --label bug-fixer`.

---

## 5. Security & Ingestion Analysis (`safe_ingest.py`)

- Line 40: `QUARANTINE_LABELS = frozenset({"security:prompt-injection"})`.
- Lines 123-140: `PRIORITY_RANK_BY_NAME` defines canonical mapping for priority names and labels:
  - `urgent`, `p0`, `priority:urgent`, `priority:p0` → (`Urgent`, 0)
  - `high`, `p1`, `priority:high`, `priority:p1` → (`High`, 1)
  - `medium`, `p2`, `priority:medium`, `priority:p2` → (`Medium`, 2)
  - `low`, `p3`, `priority:low`, `priority:p3` → (`Low`, 3)
- Lines 307-315: `has_quarantine_label(item)` checks if any item label matches `QUARANTINE_LABELS`.
- Lines 651, 691-692: Strips or denies unapproved content if `has_quarantine_label` is true.
- Lines 828, 920: Blocks PR inline review comments if quarantine label is detected.
- Lines 942-947: Resolves issue priority rank by matching labels against `PRIORITY_RANK_BY_NAME`.
- Lines 1345, 1350, 1364: In `pr-comments`, fetches `labels` and validates quarantine status.

---

## 6. Liveness Analysis (`liveness.py`)

- Line 353: Success detection for `fresh_fix`/`reattach`: `jq '[.labels[].name] | any(. == "status:awaiting-review")'`.
- Line 363: Success detection for `pr_iteration`: `jq '[.labels[].name] | any(. == "needs-iteration")'` (checks that label is removed).
- Line 453: Stalled recovery cleanup: `_gh("issue", "edit", str(issue_num), "--repo", target_repo, "--remove-label", "status:in-progress")`.
- Lines 465, 468: Logs and preserves `needs-iteration` for stalled PR iterations.

---

## 7. Complete Label Inventory (Current Name → Every File:Line Reference)

### `P1-blocker` (10 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 18: `"P1-blocker",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 68: `"P1-blocker",`
- **`README.md`**:
  - Line 97: `- Checks for exclusion labels (`hardware-required`, `P1-blocker`, etc.).`
  - Line 223: `| `exclusion_labels` | `array[string]` | Issue labels that immediately disqualify from auto-fix | `["human-required",...`
- **`docs/plugin-architecture.md`**:
  - Line 496: `- Not in exclusion_labels (e.g., "human-required", "P1-blocker", "hardware-required")`
- **`scripts/configure.sh`**:
  - Line 539: `exclusion_labels = {"human-required", "hardware-required", "P1-blocker"}`
  - Line 687: `exclusion_labels = {"human-required", "hardware-required", "P1-blocker"}`
- **`scripts/install.sh`**:
  - Line 170: `"exclusion_labels": ["human-required", "P1-blocker", "hardware-required"],`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 220: `"P1-blocker", "P2-correctness", "P3-docs-dx", "found-in:", "backfill",`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 48: `| `P1-blocker` | Plugin is non-functional — sidecars crash, claims orphan, PRs fail to open |`

### `P2-correctness` (5 references)
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 110: `**Input:** Open issue "#226 OTA checkOnBoot guard" with label `P2-correctness`, single module scope, clear acceptance...`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 220: `"P1-blocker", "P2-correctness", "P3-docs-dx", "found-in:", "backfill",`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 49: `| `P2-correctness` | Plugin operates but produces wrong results — misclassification, incorrect exclusion zones, false...`
  - Line 152: `--label "P2-correctness,component:engineering,found-in:<target-repo-name>"`
  - Line 198: `--label "P2-correctness,component:engineering,found-in:<target-repo-name>,backfill" | grep -oE '[0-9]+$')`

### `P3-docs-dx` (6 references)
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 74: `"auto_fixable_max_priority": "P3-docs-dx",`
- **`README.md`**:
  - Line 226: `| `auto_fixable_max_priority` | `string` | Priority ceiling for automated fixes | `"P3-docs-dx"` |`
- **`scripts/configure.sh`**:
  - Line 544: `small_effort_labels = {"S", "S-effort", "P3-docs-dx", "P4-enhancement"}`
  - Line 690: `small_effort_labels = {"S", "S-effort", "P3-docs-dx", "P4-enhancement"}`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 220: `"P1-blocker", "P2-correctness", "P3-docs-dx", "found-in:", "backfill",`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 50: `| `P3-docs-dx` | Confusing docs, misleading skill instructions, cosmetic issues, unclear error messages |`

### `P4-enhancement` (5 references)
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 114: `**Input:** Open issue "#144 CI Pipeline for GitHub Actions" with label `P4-enhancement`, cross-cutting, architectural.`
- **`scripts/configure.sh`**:
  - Line 544: `small_effort_labels = {"S", "S-effort", "P3-docs-dx", "P4-enhancement"}`
  - Line 690: `small_effort_labels = {"S", "S-effort", "P3-docs-dx", "P4-enhancement"}`
- **`skills/bug-fixer/references/contribution-protocol.md`**:
  - Line 44: ``P4-enhancement` label.`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 51: `| `P4-enhancement` | New capability or improvement — not a defect |`

### `S / S-effort / L / L-effort / XL / XL-effort` (4 references)
- **`scripts/configure.sh`**:
  - Line 544: `small_effort_labels = {"S", "S-effort", "P3-docs-dx", "P4-enhancement"}`
  - Line 545: `large_effort_labels = {"L", "L-effort", "XL", "XL-effort"}`
  - Line 690: `small_effort_labels = {"S", "S-effort", "P3-docs-dx", "P4-enhancement"}`
  - Line 691: `large_effort_labels = {"L", "L-effort", "XL", "XL-effort"}`

### `architect-approved` (8 references)
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/wo2-architect-skill/WORKORDER.md`**:
  - Line 28: `- If the PR is perfect: Add the `architect-approved` label to the PR, mark it "Ready for review" (remove draft status...`
  - Line 54: `- Defined the exact approval flow: add `architect-approved` label, mark PR ready for review (`gh pr ready`), submit a...`
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/wo3-architect-sidecar/WORKORDER.md`**:
  - Line 28: `- Filter out any PRs that already have the `needs-iteration` or `architect-approved` labels.`
- **`scripts/architect_dispatcher.py`**:
  - Line 165: `if "needs-iteration" in labels or "architect-approved" in labels:`
- **`skills/architect-review/SKILL.md`**:
  - Line 148: `1. **Add `architect-approved` label:**`
  - Line 150: `gh pr edit <pr_number> --repo <target_repo> --add-label "architect-approved"`
  - Line 185: `2. **Remove `architect-approved` label (if present):**`
  - Line 187: `gh pr edit <pr_number> --repo <target_repo> --remove-label "architect-approved" 2>/dev/null || true`

### `auto-fixable` (97 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 22: `"auto-fixable"`
  - Line 25: `"auto-fixable",`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 68: `You MUST only fix issues labelled `auto-fixable` (T1). You MUST NOT fix issues`
  - Line 70: ``human-required` unless they have been explicitly reclassified to `auto-fixable``
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 72: `"auto-fixable"`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo4-skills/WORKORDER.md`**:
  - Line 55: `**Purpose:** Classify open issues as `auto-fixable` or `human-required` and enforce the active-PR exclusion zone.`
  - Line 63: `auto-fixable or human-required. Triggered by the triage sidecar on a`
  - Line 76: `4. **For each unclassified issue** (no `auto-fixable` or `human-required` label):`
  - Line 88: `e. **Apply label** — `gh label create auto-fixable --repo <target_repo> --force` (idempotent), then `gh issue edit <n...`
  - Line 104: `auto-fixable issue, implements the fix, runs tests, and opens a PR. Triggered`
  - Line 120: `4. **Select Issue:** Run `safe_ingest.py issue-list --repo <target_repo> --label auto-fixable --state open --json num...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo5-sidecars-and-install/WORKORDER.md`**:
  - Line 49: `"description": "Hourly triage of open GitHub issues — classifies as auto-fixable or human-required.",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 109: `## Scenario 1: Classify auto-fixable bug`
  - Line 111: `**Expected:** Agent applies `auto-fixable` label.`
  - Line 138: `**Input:** Open issue "#201 test_otaUpdateManager in test_ignore" with `auto-fixable` label, P2 priority.`
  - Line 154: `**Input:** Top-priority auto-fixable issue touches `modules/WebServer/`, maintainer's PR also touches `modules/WebSer...`
  - Line 162: `**Input:** No open issues with `auto-fixable` label.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 52: `"auto-fixable",`
  - Line 96: `- Use appropriate colours: `auto-fixable` → green, `human-required` → red, `needs-approach-review` → yellow, `project...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo8-milestones-and-routing/WORKORDER.md`**:
  - Line 13: `You are an expert senior prompt engineer rewriting the triage skill to support milestone-aware prioritisation and a 5...`
  - Line 35: `Replace the binary `auto-fixable` / `human-required` classification with 5 tiers. The triage skill must classify each...`
  - Line 39: `| **T1: Auto-fixable** | `auto-fixable` | Single module, clear acceptance criteria, testable with host tests, no arch...`
  - Line 40: `| **T2: Needs Approach Review** | `needs-approach-review` | Fixable by agent but non-trivial: multiple files in same ...`
  - Line 109: `- Add `auto-fixable` label`
  - Line 121: `**Expected:** Classified as `auto-fixable` (T1).`
  - Line 129: `**Expected:** Reclassified to `auto-fixable`, ready for tick.`
  - Line 166: `6. No references to the old binary classification (no "auto-fixable or human-required" as the only two options)`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 37: `a. **Only T1 issues are eligible for immediate fixing.** The tick skill MUST NOT pick up `needs-approach-review`, `in...`
  - Line 40: `1. First: `auto-fixable` issues in the `active_milestone` (from config), sorted by priority label then issue number`
  - Line 41: `2. Then: `auto-fixable` issues in other milestones with due dates, sorted by due date ascending`
  - Line 42: `3. Finally: `auto-fixable` issues with no milestone, sorted by priority label then issue number`
  - Line 58: `- Add `auto-fixable` label`
  - Line 61: `This ensures previously-reviewed issues enter the auto-fixable queue.`
  - Line 79: `- Issue tier: `**Classification:** T1 (auto-fixable)` or `T2 (approach approved by @maintainer)``
  - Line 97: `You MUST only fix issues labelled `auto-fixable` (T1). You MUST NOT fix issues`
  - Line 99: ``human-required` unless they have been explicitly reclassified to `auto-fixable``
  - Line 111: `**Input:** Open issue "#201 test_otaUpdateManager" with `auto-fixable` label, P2 priority.`
  - Line 127: `**Input:** Top-priority issue has `needs-approach-review` label, next has `auto-fixable`.`
  - Line 136: `**Expected:** Agent reclassifies to `auto-fixable`, then picks it up and fixes it.`
  - Line 143: `**Input:** No `auto-fixable` issues.`
  - Line 156: `1. Tick SKILL.md filters only `auto-fixable` issues (not other tiers)`
- **`README.md`**:
  - Line 68: `TriageSidecar["Triage Loop\n(Hourly cron: 0 * * * *)\nClassifies auto-fixable vs human-required"]`
  - Line 76: `TriageSidecar -- "Labels auto-fixable" --> Issues`
  - Line 99: `- Labels candidates as `auto-fixable` (bounded, single-module bugs with clear criteria) or `human-required` (architec...`
  - Line 107: `- Claims highest-priority `auto-fixable` issue from `origin/main` in a dedicated worktree.`
  - Line 224: `| `auto_fixable_labels` | `array[string]` | Labels identifying issues triaged for autonomous fixing | `["auto-fixable...`
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 10: `"auto-fixable",`
- **`docs/plugin-architecture.md`**:
  - Line 463: `| `bug-fixer-engineering-loop` | `*/10 * * * *` | `engineering_dispatcher.py` | Checks for `auto-fixable` issues, cla...`
  - Line 495: `- Has "auto-fixable" label`
- **`matthewmcneill_bug-fixer-loop-plugin_triage_preview.json`**:
  - Line 5: `"tier": "T1 auto-fixable",`
- **`rules/AGENTS.md`**:
  - Line 68: `You MUST only fix issues labelled `auto-fixable` (T1). You MUST NOT fix issues`
  - Line 70: ``human-required` unless they have been explicitly reclassified to `auto-fixable``
- **`scripts/configure.sh`**:
  - Line 354: `["auto-fixable"]="0E8A16:Issues eligible for autonomous fixing"`
  - Line 366: `"auto-fixable"`
  - Line 476: `"auto-fixable",`
  - Line 541: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 613: `tier = "T1 auto-fixable"`
  - Line 631: `tier = "T1 auto-fixable"`
  - Line 689: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 727: `label = "auto-fixable"`
  - Line 737: `label = "auto-fixable"`
- **`scripts/engineering_dispatcher.py`**:
  - Line 470: `"--repo", TARGET_REPO, "--label", "auto-fixable",`
- **`scripts/install.sh`**:
  - Line 171: `"auto_fixable_labels": ["auto-fixable"],`
  - Line 173: `"auto-fixable", "human-required", "needs-approach-review",`
- **`scripts/trajectory_dispatcher.py`**:
  - Line 120: `"Fix #", "Triage:", "auto-fixable",`
- **`scripts/triage_dispatcher.py`**:
  - Line 23: `"auto-fixable",`
- **`skills/architect-review/SKILL.md`**:
  - Line 290: `- Never add `auto-fixable` or any classification label — let the triage system handle classification`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 4: `**Input:** Open issue "#201 test_otaUpdateManager" with `auto-fixable` label, P2 priority.`
  - Line 20: `**Input:** Top-priority issue has `needs-approach-review` label, next has `auto-fixable`.`
  - Line 29: `**Expected:** Agent reclassifies to `auto-fixable`, then picks it up and fixes it.`
  - Line 36: `**Input:** No `auto-fixable` issues.`
  - Line 48: `**Input:** Two T1 issues with `batch:memory-rules-nothrow` label, both `auto-fixable`.`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 5: `auto-fixable issue respecting milestone order, implements the fix, runs tests,`
  - Line 12: `This skill executes one cycle of the autonomous bug fixer loop. It checks for review feedback and approved approaches...`
  - Line 112: `- Add `auto-fixable` label`
  - Line 115: `This ensures previously-reviewed issues enter the auto-fixable queue.`
  - Line 118: `- **Only T1 issues are eligible for immediate fixing.** The engineering skill MUST NOT pick up `needs-approach-review...`
  - Line 119: `- Query open, auto-fixable issues:`
  - Line 121: `python3 ${PLUGIN_DIR}/scripts/safe_ingest.py issue-list --repo <target_repo> --label auto-fixable --state open --json...`
  - Line 130: `1. First: `auto-fixable` issues in the `active_milestone` (from config), sorted by priority label then issue number`
  - Line 131: `2. Then: `auto-fixable` issues in other milestones with due dates, sorted by due date ascending`
  - Line 132: `3. Finally: `auto-fixable` issues with no milestone, sorted by priority label then issue number`
  - Line 135: `If it does, query for all other `auto-fixable` issues with the same batch`
  - Line 140: `--label "${BATCH_LABEL}" --label auto-fixable --state open \`
  - Line 348: `- Issue tier: `**Classification:** T1 (auto-fixable)` or `T2 (approach approved by @maintainer)``
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 5: `**Expected:** Classified as `auto-fixable` (T1).`
  - Line 13: `**Expected:** Reclassified to `auto-fixable`, ready for engineering.`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 37: `For each issue that does not yet have a classification label (`auto-fixable`, `needs-approach-review`, `input-needed`...`
  - Line 49: `e. **Small-scope override (CHECK THIS BEFORE T4/T2):** If the issue is about renaming, adding comments/docs, fixing t...`
  - Line 123: `h. **Otherwise:** Classify as `auto-fixable` (T1). Single module, clear acceptance criteria, testable with host tests...`
  - Line 133: `For **T1 (auto-fixable)** issues, post:`
  - Line 135: `[BUG-FIXER-TRIAGE] Classification: T1 (auto-fixable)`
  - Line 160: `[BUG-FIXER-TRIAGE] Classification: T1 (auto-fixable, small-scope override)`
  - Line 239: `- Add `auto-fixable` label (T1)`
  - Line 268: `- All are T1 (`auto-fixable`)`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 117: `T1 auto-fixable:          6 issue(s) — agent will fix these`

### `backfill` (2 references)
- **`skills/bug-fixer/SKILL.md`**:
  - Line 220: `"P1-blocker", "P2-correctness", "P3-docs-dx", "found-in:", "backfill",`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 185: `2. Add the `backfill` label alongside the severity and component labels.`

### `batch:* (batch:<slug>)` (9 references)
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 48: `**Input:** Two T1 issues with `batch:memory-rules-nothrow` label, both `auto-fixable`.`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 347: `- **Batch context (if batch):** `**Batch:** \`batch:<slug>\` — <count> issues` with a brief table listing each issue ...`
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 45: `**Expected:** #254 and #255 grouped into `batch:memory-rules-compliance` (same spec section, overlapping files). #256...`
  - Line 56: `**Input:** Existing batch `batch:doc-header-cleanup` with 3 issues. New T1 issue classified that matches the same pat...`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 292: ``batch:memory-rules-nothrow`, `batch:audit-script-fixes`,`
  - Line 293: ``batch:doc-header-cleanup`. Keep slugs short, lowercase, hyphenated,`
  - Line 298: `gh label create "batch:<slug>" --repo <target_repo> --color "C2E0C6" \`
  - Line 304: `gh issue edit <number> --repo <target_repo> --add-label "batch:<slug>"`
  - Line 309: `[BUG-FIXER-TRIAGE] Batch assignment: `batch:<slug>``

### `bug-fixer` (37 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 31: `"bug-fixer",`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 59: `## 13. Bug-Fixer Label`
  - Line 60: `You MUST apply the `bug-fixer` label to every PR you create. This distinguishes`
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/wo3-architect-sidecar/WORKORDER.md`**:
  - Line 27: `- Query `gh pr list --repo <target_repo> --label "bug-fixer" --state open --draft --json number,title,labels,body``
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 58: `"bug-fixer",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 16: `1. Update `bug-fixer-tick/SKILL.md` to handle T1 and T2 issues, milestone tagging, `bug-fixer` label, and approach-re...`
  - Line 68: `a. **Apply `bug-fixer` label to the PR:**`
  - Line 70: `gh pr edit <pr_number> --repo <target_repo> --add-label bug-fixer`
  - Line 81: `- The `bug-fixer` label must be listed`
  - Line 88: `## 13. Bug-Fixer Label`
  - Line 89: `You MUST apply the `bug-fixer` label to every PR you create. This distinguishes`
  - Line 112: `**Expected:** Agent branches, fixes, tests pass, PR opened with `bug-fixer` label and test output.`
  - Line 148: `**Expected:** PR has `bug-fixer` label AND milestone "v1.1" set.`
  - Line 159: `4. Tick SKILL.md applies `bug-fixer` label and milestone to PRs`
- **`README.md`**:
  - Line 116: `- Polls for un-reviewed draft PRs with the `bug-fixer` label.`
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 16: `"bug-fixer",`
- **`rules/AGENTS.md`**:
  - Line 59: `## 13. Bug-Fixer Label`
  - Line 60: `You MUST apply the `bug-fixer` label to every PR you create. This distinguishes`
- **`scripts/architect_dispatcher.py`**:
  - Line 132: `r = gh("pr", "list", "--repo", TARGET_REPO, "--label", "bug-fixer",`
- **`scripts/configure.sh`**:
  - Line 360: `["bug-fixer"]="0075CA:Managed by autonomous bug-fixer loop"`
  - Line 372: `"bug-fixer"`
  - Line 482: `"bug-fixer",`
- **`scripts/engineering_dispatcher.py`**:
  - Line 346: `"--label", "bug-fixer,needs-iteration", "--state", "open",`
- **`scripts/install.sh`**:
  - Line 175: `"bug-fixer", "bug-fixer-health", "spec-change-request"`
- **`scripts/trajectory_dispatcher.py`**:
  - Line 156: `"--label", "bug-fixer", "--state", "all",`
- **`scripts/uninstall.sh`**:
  - Line 56: `if [[ -f "$HOOKS_FILE" ]] && grep -q 'bug-fixer' "$HOOKS_FILE" 2>/dev/null; then`
  - Line 64: `if 'bug-fixer' in key:`
- **`skills/architect-review/SKILL.md`**:
  - Line 87: `- If unprovided, find the oldest open draft PR with label `bug-fixer`:`
  - Line 89: `gh pr list --repo <target_repo> --state open --draft --label "bug-fixer" --json number,title,createdAt --jq 'sort_by(...`
  - Line 298: `- **Sidecar invocation:** Triggered automatically by `architect_dispatcher.py` when an open draft PR tagged with `bug...`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 5: `**Expected:** Agent branches, fixes, tests pass, PR opened with `bug-fixer` label and test output.`
  - Line 41: `**Expected:** PR has `bug-fixer` label AND milestone "v1.1" set.`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 6: `and opens a PR with the bug-fixer label. Triggered by the engineering-loop sidecar every 2`
  - Line 350: `- The `bug-fixer` label must be listed (e.g., `**Labels:** bug-fixer`)`
  - Line 358: `- Apply `bug-fixer` label to the PR:`
  - Line 360: `gh pr edit <pr_number> --repo <target_repo> --add-label bug-fixer`

### `bug-fixer-health` (10 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 32: `"bug-fixer-health",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo4-skills/WORKORDER.md`**:
  - Line 216: `4. **Output a diagnostic report** as a structured summary. If critical patterns detected (>3 consecutive failures, st...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 59: `"bug-fixer-health",`
- **`README.md`**:
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 17: `"bug-fixer-health",`
- **`scripts/configure.sh`**:
  - Line 361: `["bug-fixer-health"]="C5DEF5:Bug fixer health, metrics, or diagnostic report"`
  - Line 373: `"bug-fixer-health"`
  - Line 483: `"bug-fixer-health",`
- **`scripts/install.sh`**:
  - Line 175: `"bug-fixer", "bug-fixer-health", "spec-change-request"`
- **`skills/bug-fixer-trajectory-review/SKILL.md`**:
  - Line 36: `Output a diagnostic report as a structured summary. If critical patterns are detected (e.g., >3 consecutive failures,...`

### `complexity:* / complexity:<tier>` (11 references)
- **`scripts/engineering_dispatcher.py`**:
  - Line 159: `if name.startswith("complexity:"):`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 198: `| T1 via small-scope override (step 4e) | `complexity:mechanical` | `flash_lite` | Rote, zero judgment |`
  - Line 199: `| T1 standard (step 4h) | `complexity:standard` | `flash` | Clear scope, concrete checklist |`
  - Line 204: `gh label create "complexity:mechanical" --repo <target_repo> --color "0E8A16" \`
  - Line 206: `gh label create "complexity:standard" --repo <target_repo> --color "1D76DB" \`
  - Line 208: `gh label create "complexity:architect" --repo <target_repo> --color "B60205" \`
  - Line 214: `gh issue edit <number> --repo <target_repo> --add-label "complexity:<tier>"`
  - Line 240: `- Add `complexity:architect` label — the triager originally assessed this as needing deep reasoning, so the engineeri...`
- **`tests/test_dispatcher_models.py`**:
  - Line 96: `resolver({"labels": [{"name": "complexity:mechanical"}]}),`
  - Line 100: `resolver({"labels": [{"name": "complexity:standard"}]}),`
  - Line 104: `resolver({"labels": [{"name": "complexity:architect"}]}),`

### `complexity:architect` (3 references)
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 208: `gh label create "complexity:architect" --repo <target_repo> --color "B60205" \`
  - Line 240: `- Add `complexity:architect` label — the triager originally assessed this as needing deep reasoning, so the engineeri...`
- **`tests/test_dispatcher_models.py`**:
  - Line 104: `resolver({"labels": [{"name": "complexity:architect"}]}),`

### `complexity:mechanical` (3 references)
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 198: `| T1 via small-scope override (step 4e) | `complexity:mechanical` | `flash_lite` | Rote, zero judgment |`
  - Line 204: `gh label create "complexity:mechanical" --repo <target_repo> --color "0E8A16" \`
- **`tests/test_dispatcher_models.py`**:
  - Line 96: `resolver({"labels": [{"name": "complexity:mechanical"}]}),`

### `complexity:standard` (3 references)
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 199: `| T1 standard (step 4h) | `complexity:standard` | `flash` | Clear scope, concrete checklist |`
  - Line 206: `gh label create "complexity:standard" --repo <target_repo> --color "1D76DB" \`
- **`tests/test_dispatcher_models.py`**:
  - Line 100: `resolver({"labels": [{"name": "complexity:standard"}]}),`

### `component:engineering` (4 references)
- **`skills/bug-fixer/SKILL.md`**:
  - Line 221: `"component:triage", "component:engineering", "component:safe-ingest".`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 58: `| `component:engineering` | Bug is in the engineering skill or engineering loop sidecar logic |`
  - Line 152: `--label "P2-correctness,component:engineering,found-in:<target-repo-name>"`
  - Line 198: `--label "P2-correctness,component:engineering,found-in:<target-repo-name>,backfill" | grep -oE '[0-9]+$')`

### `component:install-config` (1 references)
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 62: `| `component:install-config` | Bug is in `install.sh`, `configure.sh`, or `uninstall.sh` |`

### `component:pre-tool-gate` (1 references)
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 61: `| `component:pre-tool-gate` | Bug is in `pre_tool_gate.py` hook enforcement |`

### `component:rules` (2 references)
- **`skills/bug-fixer/references/contribution-protocol.md`**:
  - Line 25: `| Change to a safety rule in `AGENTS.md` | Human-only — file issue with `component:rules` |`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 64: `| `component:rules` | Bug relates to safety rules (AGENTS.md) being too strict/lenient |`

### `component:safe-ingest` (2 references)
- **`skills/bug-fixer/SKILL.md`**:
  - Line 221: `"component:triage", "component:engineering", "component:safe-ingest".`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 60: `| `component:safe-ingest` | Bug is in `safe_ingest.py` ingestion or redaction |`

### `component:test-quality` (1 references)
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 63: `| `component:test-quality` | Bug is in the test quality audit rubric |`

### `component:trajectory` (1 references)
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 59: `| `component:trajectory` | Bug is in the trajectory review skill or sidecar |`

### `component:triage` (2 references)
- **`skills/bug-fixer/SKILL.md`**:
  - Line 221: `"component:triage", "component:engineering", "component:safe-ingest".`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 57: `| `component:triage` | Bug is in the triage skill or triage sidecar logic |`

### `discovered-in-review` (6 references)
- **`scripts/configure.sh`**:
  - Line 389: `# Ensure discovered-in-review label exists on target repo (for tech debt discovery chain)`
  - Line 390: `gh label create "discovered-in-review" \`
- **`skills/architect-review/SKILL.md`**:
  - Line 277: `--label "discovered-in-review"`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 217: `j. **Discovery HITL gate:** For issues labelled `discovered-in-review`, classify as **minimum T2** (`needs-approach-r...`
  - Line 231: `This ensures every auto-filed discovery issue gets a human checkpoint before entering the automated fix pipeline. Che...`
  - Line 238: `- **If the issue also has the `discovered-in-review` label, remove it** — this is the depth reset. The issue is now h...`

### `found-in:<target-repo-name>` (5 references)
- **`skills/bug-fixer/references/contribution-protocol.md`**:
  - Line 178: ``found-in:<target-repo-name>` attribution label. Fixes should be branched`
- **`skills/bug-fixer/references/issue-management.md`**:
  - Line 70: `| `found-in:<target-repo-name>` | Issue was surfaced while the plugin operated against a specific target repo (e.g. `...`
  - Line 152: `--label "P2-correctness,component:engineering,found-in:<target-repo-name>"`
  - Line 158: `gh label create "found-in:<target-repo-name>" \`
  - Line 198: `--label "P2-correctness,component:engineering,found-in:<target-repo-name>,backfill" | grep -oE '[0-9]+$')`

### `hardware-required` (11 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 19: `"hardware-required"`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 69: `"hardware-required"`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 126: `**Input:** Open issue with `hardware-required` label (in `exclusion_labels` config).`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo8-milestones-and-routing/WORKORDER.md`**:
  - Line 140: `**Input:** Issue with `hardware-required` label.`
- **`README.md`**:
  - Line 97: `- Checks for exclusion labels (`hardware-required`, `P1-blocker`, etc.).`
  - Line 223: `| `exclusion_labels` | `array[string]` | Issue labels that immediately disqualify from auto-fix | `["human-required",...`
- **`docs/plugin-architecture.md`**:
  - Line 496: `- Not in exclusion_labels (e.g., "human-required", "P1-blocker", "hardware-required")`
- **`scripts/configure.sh`**:
  - Line 539: `exclusion_labels = {"human-required", "hardware-required", "P1-blocker"}`
  - Line 687: `exclusion_labels = {"human-required", "hardware-required", "P1-blocker"}`
- **`scripts/install.sh`**:
  - Line 170: `"exclusion_labels": ["human-required", "P1-blocker", "hardware-required"],`
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 24: `**Input:** Issue with `hardware-required` label.`

### `high` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 128: `"high": ("High", 1),`

### `human-required` (68 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 17: `"human-required",`
  - Line 26: `"human-required",`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 70: ``human-required` unless they have been explicitly reclassified to `auto-fixable``
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 67: `"human-required",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo4-skills/WORKORDER.md`**:
  - Line 55: `**Purpose:** Classify open issues as `auto-fixable` or `human-required` and enforce the active-PR exclusion zone.`
  - Line 63: `auto-fixable or human-required. Triggered by the triage sidecar on a`
  - Line 76: `4. **For each unclassified issue** (no `auto-fixable` or `human-required` label):`
  - Line 78: `a. **Skip if exclusion label present** — if the issue has any label in `exclusion_labels`, apply `human-required` lab...`
  - Line 82: `c. **Check exclusion zone** — if any estimated fix files overlap with the exclusion file set, apply `human-required` ...`
  - Line 88: `e. **Apply label** — `gh label create auto-fixable --repo <target_repo> --force` (idempotent), then `gh issue edit <n...`
  - Line 138: `- All three must pass. If any fails, debug and retry up to 2 times. If still failing after retries, unclaim the issue...`
  - Line 210: `- Same issue attempted multiple times → add `human-required` label`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo5-sidecars-and-install/WORKORDER.md`**:
  - Line 49: `"description": "Hourly triage of open GitHub issues — classifies as auto-fixable or human-required.",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 113: `## Scenario 2: Skip human-required issue`
  - Line 115: `**Expected:** Agent applies `human-required` label.`
  - Line 119: `**Expected:** Agent applies `human-required` label with comment citing the overlapping PR.`
  - Line 127: `**Expected:** Agent applies `human-required` label, skips further classification.`
  - Line 151: `**Expected:** Agent unclaims issue, adds `human-required` label, comments explaining failure.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 53: `"human-required",`
  - Line 96: `- Use appropriate colours: `auto-fixable` → green, `human-required` → red, `needs-approach-review` → yellow, `project...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo8-milestones-and-routing/WORKORDER.md`**:
  - Line 13: `You are an expert senior prompt engineer rewriting the triage skill to support milestone-aware prioritisation and a 5...`
  - Line 35: `Replace the binary `auto-fixable` / `human-required` classification with 5 tiers. The triage skill must classify each...`
  - Line 43: `| **T5: Human Required** | `human-required` | Hardware-dependent, requires physical testing, has exclusion labels, ne...`
  - Line 141: `**Expected:** Classified as `human-required` (T5), skipped.`
  - Line 145: `**Expected:** Classified as `human-required` (T5), comment citing overlapping PR.`
  - Line 166: `6. No references to the old binary classification (no "auto-fixable or human-required" as the only two options)`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 37: `a. **Only T1 issues are eligible for immediate fixing.** The tick skill MUST NOT pick up `needs-approach-review`, `in...`
  - Line 99: ``human-required` unless they have been explicitly reclassified to `auto-fixable``
  - Line 124: `**Expected:** Issue unclaimed, `human-required` label added, failure comment posted.`
- **`README.md`**:
  - Line 68: `TriageSidecar["Triage Loop\n(Hourly cron: 0 * * * *)\nClassifies auto-fixable vs human-required"]`
  - Line 77: `TriageSidecar -- "Labels human-required (e.g. overlap)" --> Issues`
  - Line 99: `- Labels candidates as `auto-fixable` (bounded, single-module bugs with clear criteria) or `human-required` (architec...`
  - Line 223: `| `exclusion_labels` | `array[string]` | Issue labels that immediately disqualify from auto-fix | `["human-required",...`
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
  - Line 260: `- **Active PR Exclusion Zone:** Triage actively inspects the maintainer's open PRs via GitHub API. If an issue requir...`
- **`config.json`**:
  - Line 11: `"human-required",`
- **`docs/plugin-architecture.md`**:
  - Line 496: `- Not in exclusion_labels (e.g., "human-required", "P1-blocker", "hardware-required")`
- **`rules/AGENTS.md`**:
  - Line 70: ``human-required` unless they have been explicitly reclassified to `auto-fixable``
- **`scripts/configure.sh`**:
  - Line 355: `["human-required"]="D93F0B:Issues requiring human intervention or architectural review"`
  - Line 367: `"human-required"`
  - Line 477: `"human-required",`
  - Line 539: `exclusion_labels = {"human-required", "hardware-required", "P1-blocker"}`
  - Line 541: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 601: `tier = "T5 human-required"`
  - Line 605: `tier = "T5 human-required"`
  - Line 687: `exclusion_labels = {"human-required", "hardware-required", "P1-blocker"}`
  - Line 689: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 721: `label = "human-required"`
  - Line 723: `label = "human-required"`
- **`scripts/install.sh`**:
  - Line 170: `"exclusion_labels": ["human-required", "P1-blocker", "hardware-required"],`
  - Line 173: `"auto-fixable", "human-required", "needs-approach-review",`
- **`scripts/triage_dispatcher.py`**:
  - Line 27: `"human-required",`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 17: `**Expected:** Issue unclaimed, `human-required` label added, failure comment posted.`
  - Line 53: `**Expected:** All 3 issues unclaimed (`status:in-progress` removed), all 3 labeled `human-required`, failure comment ...`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 118: `- **Only T1 issues are eligible for immediate fixing.** The engineering skill MUST NOT pick up `needs-approach-review...`
  - Line 321: `- All checks must pass. If any fails, debug and retry up to 2 times. If tests or lint still fail after 3 total attemp...`
- **`skills/bug-fixer-trajectory-review/SKILL.md`**:
  - Line 24: `- **Churn**: If the same issue is attempted multiple times without success, add the `human-required` label.`
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 25: `**Expected:** Classified as `human-required` (T5), skipped.`
  - Line 29: `**Expected:** Classified as `human-required` (T5), comment citing overlapping PR.`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 37: `For each issue that does not yet have a classification label (`auto-fixable`, `needs-approach-review`, `input-needed`...`
  - Line 39: `a. **Has exclusion label?** If the issue has any label found in `exclusion_labels`, classify as `human-required` (T5)...`
  - Line 41: `b. **Check exclusion zone (PR overlap):** Read the issue and estimate which files/modules would need modification. If...`
  - Line 44: `c. **Hardware dependency:** If the issue requires physical hardware access (pins, sensors, oscilloscope, soldering, a...`
  - Line 77: `> **T4 vs T5 boundary:** T5 (`human-required`) is reserved for work that an agent **physically cannot do** — needs ha...`
  - Line 181: `For **T5 (human-required)** issues that don't already have a comment from`
  - Line 184: `[BUG-FIXER-TRIAGE] Classification: T5 (human-required)`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 121: `T5 human-required:       10 issue(s) — agent skips these`
  - Line 133: `- Any `human-required` escalations`

### `input-needed` (33 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 29: `"input-needed",`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 13: `conflict, file an issue with label `spec-change-request,input-needed` and STOP.`
  - Line 69: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 101: `conflict, file an issue with label `spec-change-request,input-needed` and STOP.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo4-skills/WORKORDER.md`**:
  - Line 140: `10. **Self-Review Against Spec:** Read `spec_file_path` sections relevant to the modified modules. Verify the fix doe...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 159: `**Expected:** Agent unclaims, labels `spec-change-request,input-needed`, comments with explanation.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 56: `"input-needed",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo8-milestones-and-routing/WORKORDER.md`**:
  - Line 41: `| **T3: Input Needed** | `input-needed` | Bug report is ambiguous, lacks repro steps, or has conflicting information....`
  - Line 133: `**Expected:** Classified as `input-needed` (T3), clarification comment posted.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 37: `a. **Only T1 issues are eligible for immediate fixing.** The tick skill MUST NOT pick up `needs-approach-review`, `in...`
  - Line 98: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
  - Line 140: `**Expected:** Unclaim, label `spec-change-request,input-needed`, comment.`
- **`README.md`**:
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 14: `"input-needed",`
- **`rules/AGENTS.md`**:
  - Line 13: `conflict, file an issue with label `spec-change-request,input-needed` and STOP.`
  - Line 69: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`scripts/configure.sh`**:
  - Line 358: `["input-needed"]="D4C5F9:Waiting for human input or clarification"`
  - Line 370: `"input-needed"`
  - Line 480: `"input-needed",`
  - Line 541: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 609: `tier = "T3 input-needed"`
  - Line 689: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 725: `label = "input-needed"`
- **`scripts/install.sh`**:
  - Line 174: `"project-candidate", "input-needed", "status:in-progress",`
- **`scripts/triage_dispatcher.py`**:
  - Line 25: `"input-needed",`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 33: `**Expected:** Unclaim, label `spec-change-request,input-needed`, comment.`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 118: `- **Only T1 issues are eligible for immediate fixing.** The engineering skill MUST NOT pick up `needs-approach-review...`
  - Line 323: `10. **Self-Review Against Design Sources:** Re-read `spec_file_path` and any PA design history or additional design d...`
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 17: `**Expected:** Classified as `input-needed` (T3), clarification comment posted.`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 37: `For each issue that does not yet have a classification label (`auto-fixable`, `needs-approach-review`, `input-needed`...`
  - Line 46: `d. **Ambiguity check:** If the bug report is ambiguous, lacks repro steps, or has conflicting information, classify a...`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 68: `python3 <plugin_dir>/scripts/safe_ingest.py issue-list --repo <target_repo> --label input-needed --state open --json ...`
  - Line 119: `T3 input-needed:          0 issue(s) — waiting for clarification`

### `low` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 136: `"low": ("Low", 3),`

### `medium` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 132: `"medium": ("Medium", 2),`

### `needs-approach-review` (50 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 27: `"needs-approach-review",`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 69: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 54: `"needs-approach-review",`
  - Line 96: `- Use appropriate colours: `auto-fixable` → green, `human-required` → red, `needs-approach-review` → yellow, `project...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo8-milestones-and-routing/WORKORDER.md`**:
  - Line 40: `| **T2: Needs Approach Review** | `needs-approach-review` | Fixable by agent but non-trivial: multiple files in same ...`
  - Line 85: `When an issue is classified as T2 (`needs-approach-review`):`
  - Line 106: `3. Applies `needs-approach-review` label`
  - Line 108: `- Remove `needs-approach-review` label`
  - Line 125: `**Expected:** Classified as `needs-approach-review` (T2), approach comment posted.`
  - Line 128: `**Input:** Issue with `needs-approach-review` label, maintainer replied "approved".`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 37: `a. **Only T1 issues are eligible for immediate fixing.** The tick skill MUST NOT pick up `needs-approach-review`, `in...`
  - Line 52: `Query issues with the `needs-approach-review` label:`
  - Line 54: `python3 <plugin_dir>/scripts/safe_ingest.py issue-list --repo <target_repo> --label needs-approach-review --state ope...`
  - Line 57: `- Remove `needs-approach-review` label`
  - Line 98: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
  - Line 127: `**Input:** Top-priority issue has `needs-approach-review` label, next has `auto-fixable`.`
  - Line 135: `**Input:** Issue with `needs-approach-review` label, maintainer replied "approved".`
- **`README.md`**:
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 12: `"needs-approach-review",`
- **`rules/AGENTS.md`**:
  - Line 69: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`scripts/configure.sh`**:
  - Line 356: `["needs-approach-review"]="FBCA04:Fix approach requires human maintainer sign-off"`
  - Line 368: `"needs-approach-review"`
  - Line 478: `"needs-approach-review",`
  - Line 541: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 624: `tier = "T2 needs-approach-review"`
  - Line 627: `tier = "T2 needs-approach-review"`
  - Line 689: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 733: `label = "needs-approach-review"`
  - Line 735: `label = "needs-approach-review"`
- **`scripts/install.sh`**:
  - Line 173: `"auto-fixable", "human-required", "needs-approach-review",`
- **`scripts/triage_dispatcher.py`**:
  - Line 24: `"needs-approach-review",`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 20: `**Input:** Top-priority issue has `needs-approach-review` label, next has `auto-fixable`.`
  - Line 28: `**Input:** Issue with `needs-approach-review` label, maintainer replied "approved".`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 106: `Query issues with the `needs-approach-review` label:`
  - Line 108: `python3 ${PLUGIN_DIR}/scripts/safe_ingest.py issue-list --repo <target_repo> --label needs-approach-review --state op...`
  - Line 111: `- Remove `needs-approach-review` label`
  - Line 118: `- **Only T1 issues are eligible for immediate fixing.** The engineering skill MUST NOT pick up `needs-approach-review...`
  - Line 153: `- Note if any issue was converted from T2 (`needs-approach-review` with maintainer approval) to accurately classify i...`
  - Line 250: `- Reclassify as `needs-approach-review` (T2).`
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 9: `**Expected:** Classified as `needs-approach-review` (T2), approach comment posted.`
  - Line 12: `**Input:** Issue with `needs-approach-review` label, maintainer replied "approved".`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 37: `For each issue that does not yet have a classification label (`auto-fixable`, `needs-approach-review`, `input-needed`...`
  - Line 79: `g. **Complexity check:** Classify as `needs-approach-review` (T2) ONLY if:`
  - Line 200: `| T2 (needs-approach-review) | *(applied after approval — see Step 5)* | | |`
  - Line 217: `j. **Discovery HITL gate:** For issues labelled `discovered-in-review`, classify as **minimum T2** (`needs-approach-r...`
  - Line 219: `[BUG-FIXER-TRIAGE] Classification: T2 (needs-approach-review)`
  - Line 235: `For issues already labeled `needs-approach-review`:`
  - Line 237: `- Remove `needs-approach-review` label`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 59: `python3 <plugin_dir>/scripts/safe_ingest.py issue-list --repo <target_repo> --label needs-approach-review --state ope...`
  - Line 118: `T2 needs-approach-review: 4 issue(s) — waiting for your approval`

### `needs-human-intervention` (4 references)
- **`README.md`**:
  - Line 118: `- Tracks review rounds and escalates to `needs-human-intervention` after max rounds.`
- **`scripts/architect_dispatcher.py`**:
  - Line 193: `f"(max {MAX_REVIEW_ROUNDS}). Adding needs-human-intervention label.")`
  - Line 195: `"--add-label", "needs-human-intervention")`
- **`skills/architect-review/SKILL.md`**:
  - Line 136: `- **Round 5 (final round):** If the ONLY remaining issues are **purely cosmetic** (formatting preferences where the p...`

### `needs-iteration` (33 references)
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/PROJECT.md`**:
  - Line 22: `- **[X] [wo4-iteration-loop](wo4-iteration-loop/WORKORDER.md):** Update `scripts/tick_dispatcher.py` to detect `needs...`
  - Line 26: `- **WO2 (Architect Skill):** Completed. The `architect-review` skill was created, establishing the adversarial person...`
  - Line 28: `- **WO4 (Iteration Loop):** Completed. `tick_dispatcher.py` updated to detect `needs-iteration` labels, extract the D...`
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/wo2-architect-skill/WORKORDER.md`**:
  - Line 31: `1. Add the `needs-iteration` label.`
  - Line 55: `- Defined the exact rejection flow: add `needs-iteration` label, post actionable architectural review comment detaili...`
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/wo3-architect-sidecar/WORKORDER.md`**:
  - Line 28: `- Filter out any PRs that already have the `needs-iteration` or `architect-approved` labels.`
- **`.agents/skills/principal-architect-workspace/pr-architect-review-loop/wo4-iteration-loop/WORKORDER.md`**:
  - Line 13: `Update `scripts/tick_dispatcher.py` to add a new step: it must detect PRs with the `needs-iteration` label, extract t...`
  - Line 24: `- [ ] In `scripts/tick_dispatcher.py`, insert a new step before Step 3 (Find eligible issues), maybe call it "Step 2....`
  - Line 25: `- [ ] Query GitHub for PRs with the `bug-fixer` and `needs-iteration` labels that are open.`
  - Line 30: `- Remove the `needs-iteration` label.`
  - Line 50: `- Inserted Step 2.5 in `scripts/tick_dispatcher.py` to handle PRs with the `needs-iteration` label.`
  - Line 53: `- Resumed the developer conversation via `agentapi message` and removed the `needs-iteration` label.`
- **`scripts/architect_dispatcher.py`**:
  - Line 165: `if "needs-iteration" in labels or "architect-approved" in labels:`
- **`scripts/engineering_dispatcher.py`**:
  - Line 342: `# Step 2.5: Handle needs-iteration PRs`
  - Line 346: `"--label", "bug-fixer,needs-iteration", "--state", "open",`
  - Line 353: `log(f"Failed to parse needs-iteration PRs: {r_prs.stderr}")`
  - Line 396: `f"After pushing changes, remove the needs-iteration label:\n"`
  - Line 397: `f"  gh pr edit {pr_num} --repo {TARGET_REPO} --remove-label needs-iteration\n\n"`
  - Line 416: `log(f"PR #{pr_num} has needs-iteration but no agy_dev_conv_id. Spawning fresh developer.")`
  - Line 428: `f"After pushing changes, remove the needs-iteration label:\n"`
  - Line 429: `f"  gh pr edit {pr_num} --repo {TARGET_REPO} --remove-label needs-iteration\n\n"`
  - Line 450: `# Track the dispatch — don't remove needs-iteration label yet.`
  - Line 456: `labels_to_restore=["needs-iteration"],`
- **`scripts/liveness.py`**:
  - Line 360: `# Check if needs-iteration label is gone (agent removed it after pushing)`
  - Line 363: `'[.labels[].name] | any(. == "needs-iteration")')`
  - Line 465: `# needs-iteration label stays — it was never removed`
  - Line 468: `summary["action"] = f"state dumped to PR #{pr_num}, needs-iteration preserved"`
- **`skills/architect-review/SKILL.md`**:
  - Line 158: `3. **Remove `needs-iteration` label (if previously present):**`
  - Line 160: `gh pr edit <pr_number> --repo <target_repo> --remove-label "needs-iteration" 2>/dev/null || true`
  - Line 180: `1. **Add `needs-iteration` label:**`
  - Line 182: `gh pr edit <pr_number> --repo <target_repo> --add-label "needs-iteration"`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 100: `- After pushing iteration changes, remove the `needs-iteration` label:`
  - Line 102: `gh pr edit <pr_number> --repo <target_repo> --remove-label needs-iteration`

### `p0` (2 references)
- **`scripts/safe_ingest.py`**:
  - Line 125: `"p0": ("Urgent", 0),`
- **`tests/test_safe_ingest.py`**:
  - Line 411: `self.assertEqual(resolve_issue_priority({"labels": [{"name": "p0"}]}), ("Urgent", 0))`

### `p1` (2 references)
- **`scripts/safe_ingest.py`**:
  - Line 129: `"p1": ("High", 1),`
- **`tests/test_safe_ingest.py`**:
  - Line 412: `self.assertEqual(resolve_issue_priority({"labels": [{"name": "p1"}]}), ("High", 1))`

### `p2` (2 references)
- **`scripts/safe_ingest.py`**:
  - Line 133: `"p2": ("Medium", 2),`
- **`tests/test_safe_ingest.py`**:
  - Line 413: `self.assertEqual(resolve_issue_priority({"labels": [{"name": "p2"}]}), ("Medium", 2))`

### `p3` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 137: `"p3": ("Low", 3),`

### `priority:high` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 130: `"priority:high": ("High", 1),`

### `priority:low` (2 references)
- **`scripts/safe_ingest.py`**:
  - Line 138: `"priority:low": ("Low", 3),`
- **`tests/test_safe_ingest.py`**:
  - Line 414: `self.assertEqual(resolve_issue_priority({"labels": [{"name": "priority:low"}]}), ("Low", 3))`

### `priority:medium` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 134: `"priority:medium": ("Medium", 2),`

### `priority:p0` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 127: `"priority:p0": ("Urgent", 0),`

### `priority:p1` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 131: `"priority:p1": ("High", 1),`

### `priority:p2` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 135: `"priority:p2": ("Medium", 2),`

### `priority:p3` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 139: `"priority:p3": ("Low", 3),`

### `priority:urgent` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 126: `"priority:urgent": ("Urgent", 0),`

### `project-candidate` (30 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 28: `"project-candidate",`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 69: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 55: `"project-candidate",`
  - Line 96: `- Use appropriate colours: `auto-fixable` → green, `human-required` → red, `needs-approach-review` → yellow, `project...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo8-milestones-and-routing/WORKORDER.md`**:
  - Line 42: `| **T4: Project Candidate** | `project-candidate` | Too large for a single PR: cross-module, requires new architectur...`
  - Line 137: `**Expected:** Classified as `project-candidate` (T4), /principal-architect comment posted.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 37: `a. **Only T1 issues are eligible for immediate fixing.** The tick skill MUST NOT pick up `needs-approach-review`, `in...`
  - Line 98: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`README.md`**:
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 13: `"project-candidate",`
- **`rules/AGENTS.md`**:
  - Line 69: `labelled `needs-approach-review`, `input-needed`, `project-candidate`, or`
- **`scripts/configure.sh`**:
  - Line 357: `["project-candidate"]="5319E7:Large multi-step initiative candidate"`
  - Line 369: `"project-candidate"`
  - Line 479: `"project-candidate",`
  - Line 541: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 617: `tier = "T4 project-candidate"`
  - Line 620: `tier = "T4 project-candidate"`
  - Line 689: `auto_labels = {"auto-fixable", "needs-approach-review", "input-needed", "project-candidate", "human-required"}`
  - Line 729: `label = "project-candidate"`
  - Line 731: `label = "project-candidate"`
- **`scripts/install.sh`**:
  - Line 174: `"project-candidate", "input-needed", "status:in-progress",`
- **`scripts/triage_dispatcher.py`**:
  - Line 26: `"project-candidate",`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 118: `- **Only T1 issues are eligible for immediate fixing.** The engineering skill MUST NOT pick up `needs-approach-review...`
- **`skills/bug-fixer-triage/EVAL.md`**:
  - Line 21: `**Expected:** Classified as `project-candidate` (T4), /principal-architect comment posted.`
- **`skills/bug-fixer-triage/SKILL.md`**:
  - Line 37: `For each issue that does not yet have a classification label (`auto-fixable`, `needs-approach-review`, `input-needed`...`
  - Line 68: `f. **Size / Scope check:** Classify as `project-candidate` (T4) ONLY if ALL of these are true:`
  - Line 77: `> **T4 vs T5 boundary:** T5 (`human-required`) is reserved for work that an agent **physically cannot do** — needs ha...`
  - Line 169: `For **T4 (project-candidate)** issues, post:`
  - Line 171: `[BUG-FIXER-TRIAGE] Classification: T4 (project-candidate)`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 120: `T4 project-candidate:    13 issue(s) — too large, use /principal-architect`

### `security:prompt-injection` (5 references)
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo2-security-scripts/WORKORDER.md`**:
  - Line 76: `- Quarantine label (`security:prompt-injection`) enforcement`
- **`scripts/safe_ingest.py`**:
  - Line 40: `QUARANTINE_LABELS = frozenset({"security:prompt-injection"})`
  - Line 308: `"""Returns True if `item` carries any quarantine label (`security:prompt-injection`)."""`
  - Line 690: `# Reject any issue/PR explicitly quarantined with security:prompt-injection`
- **`tests/test_safe_ingest.py`**:
  - Line 168: `"labels": [{"name": "security:prompt-injection"}],`

### `spec-change-request` (16 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 33: `"spec-change-request"`
- **`.agents/rules/bug-fixer-AGENTS.md`**:
  - Line 13: `conflict, file an issue with label `spec-change-request,input-needed` and STOP.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo1-plugin-scaffold/WORKORDER.md`**:
  - Line 101: `conflict, file an issue with label `spec-change-request,input-needed` and STOP.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo4-skills/WORKORDER.md`**:
  - Line 140: `10. **Self-Review Against Spec:** Read `spec_file_path` sections relevant to the modified modules. Verify the fix doe...`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo6-tests-and-docs/WORKORDER.md`**:
  - Line 159: `**Expected:** Agent unclaims, labels `spec-change-request,input-needed`, comments with explanation.`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 60: `"spec-change-request"`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo9-tick-enhancements/WORKORDER.md`**:
  - Line 140: `**Expected:** Unclaim, label `spec-change-request,input-needed`, comment.`
- **`README.md`**:
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 18: `"spec-change-request"`
- **`rules/AGENTS.md`**:
  - Line 13: `conflict, file an issue with label `spec-change-request,input-needed` and STOP.`
- **`scripts/configure.sh`**:
  - Line 362: `["spec-change-request"]="F9D0C4:Proposed modification to design specification"`
  - Line 374: `"spec-change-request"`
  - Line 484: `"spec-change-request"`
- **`scripts/install.sh`**:
  - Line 175: `"bug-fixer", "bug-fixer-health", "spec-change-request"`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 33: `**Expected:** Unclaim, label `spec-change-request,input-needed`, comment.`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 323: `10. **Self-Review Against Design Sources:** Re-read `spec_file_path` and any PA design history or additional design d...`

### `status:awaiting-review` (9 references)
- **`scripts/engineering_dispatcher.py`**:
  - Line 289: `After opening the PR (with 'Closes #N' in the body), transition the issue to status:awaiting-review and delete the lo...`
  - Line 482: `if not any(l.get("name") in ("status:in-progress", "status:awaiting-review") for l in i.get("labels", []))`
  - Line 536: `After opening the PR (with 'Closes #N' in the body), transition the issue to status:awaiting-review and delete the lo...`
- **`scripts/liveness.py`**:
  - Line 353: `'[.labels[].name] | any(. == "status:awaiting-review")')`
- **`scripts/triage_dispatcher.py`**:
  - Line 131: `# Skip status:awaiting-review ONLY if a classification label is also present`
  - Line 133: `if "status:awaiting-review" in label_names and label_names & CLASSIFICATION_LABELS:`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 94: `> **Note:** The cap limits issues *being actively coded* (`status:in-progress`), not PRs waiting for review (`status:...`
  - Line 369: `gh issue edit <number> --repo <target_repo> --remove-label status:in-progress --add-label status:awaiting-review`
  - Line 372: `> **Why `status:awaiting-review`?** The issue is no longer being actively coded (frees a concurrency slot), but not d...`

### `status:in-progress` (44 references)
- **`.agents/plugins/bug-fixer-loop-workspace/config.json`**:
  - Line 30: `"status:in-progress",`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo4-skills/WORKORDER.md`**:
  - Line 122: `5. **Claim Issue:** Run `gh issue edit <number> --repo <target_repo> --add-label status:in-progress`. This acts as a ...`
  - Line 138: `- All three must pass. If any fails, debug and retry up to 2 times. If still failing after retries, unclaim the issue...`
  - Line 147: `- Remove `status:in-progress` label from the issue`
  - Line 208: `- Issues claimed (`status:in-progress`) but no PR opened within 4 hours → stale claim alert`
- **`.agents/skills/principal-architect-workspace/pr-bug-fixer-plugin/wo7-lifecycle-and-configure/WORKORDER.md`**:
  - Line 57: `"status:in-progress",`
- **`README.md`**:
  - Line 225: `| `required_labels` | `array[string]` | Standard label taxonomy maintained in target repository | `["auto-fixable", "...`
- **`config.json`**:
  - Line 15: `"status:in-progress",`
- **`docs/plugin-architecture.md`**:
  - Line 501: `b. Add "status:in-progress" label to the GitHub issue`
  - Line 560: `2. Remove the `status:in-progress` label from the GitHub issue`
  - Line 581: `2. **Finds eligible issues**, writes claim files, adds `status:in-progress` labels`
- **`scripts/configure.sh`**:
  - Line 359: `["status:in-progress"]="1D76DB:Currently being worked on by an agent or human"`
  - Line 371: `"status:in-progress"`
  - Line 481: `"status:in-progress",`
- **`scripts/engineering_dispatcher.py`**:
  - Line 244: `"--remove-label", "status:in-progress")`
  - Line 285: `The issue is already claimed (status:in-progress label applied).`
  - Line 302: `"--remove-label", "status:in-progress")`
  - Line 326: `"--label", "status:in-progress", "--state", "open",`
  - Line 482: `if not any(l.get("name") in ("status:in-progress", "status:awaiting-review") for l in i.get("labels", []))`
  - Line 510: `"--add-label", "status:in-progress")`
  - Line 532: `The issue is already claimed (status:in-progress label applied).`
  - Line 549: `"--remove-label", "status:in-progress")`
- **`scripts/install.sh`**:
  - Line 174: `"project-candidate", "input-needed", "status:in-progress",`
- **`scripts/liveness.py`**:
  - Line 453: `"--remove-label", "status:in-progress")`
- **`scripts/trajectory_dispatcher.py`**:
  - Line 150: `"--label", "status:in-progress", "--state", "open",`
- **`scripts/triage_dispatcher.py`**:
  - Line 121: `"status:in-progress",`
  - Line 128: `# Always skip if a classification label or status:in-progress is present`
- **`skills/bug-fixer-engineering/EVAL.md`**:
  - Line 53: `**Expected:** All 3 issues unclaimed (`status:in-progress` removed), all 3 labeled `human-required`, failure comment ...`
- **`skills/bug-fixer-engineering/SKILL.md`**:
  - Line 28: `List all issues tagged `status:in-progress`:`
  - Line 30: `gh issue list --repo <target_repo> --label "status:in-progress" --state open --json number,title`
  - Line 86: `After reconciliation, count issues tagged `status:in-progress` that are **owned by other active sessions** (PID alive...`
  - Line 88: `gh issue list --repo <target_repo> --label "status:in-progress" --state open --json number,title`
  - Line 94: `> **Note:** The cap limits issues *being actively coded* (`status:in-progress`), not PRs waiting for review (`status:...`
  - Line 128: `- Exclude issues with `status:in-progress` label.`
  - Line 136: `label that are not `status:in-progress`:`
  - Line 143: `Exclude any issues already labeled `status:in-progress`. The selected`
  - Line 151: `> against `status:in-progress`. If it would, reduce the batch to fit.`
  - Line 158: `gh issue edit <number> --repo <target_repo> --add-label status:in-progress`
  - Line 249: `- Unclaim the issue (`--remove-label status:in-progress`).`
  - Line 321: `- All checks must pass. If any fails, debug and retry up to 2 times. If tests or lint still fail after 3 total attemp...`
  - Line 369: `gh issue edit <number> --repo <target_repo> --remove-label status:in-progress --add-label status:awaiting-review`
  - Line 379: `- **Concurrency cap reached** → re-check `status:in-progress` count. If >= `max_concurrent_prs`, output `ENGINEERING_...`
- **`skills/bug-fixer-trajectory-review/SKILL.md`**:
  - Line 22: `- **Stale claims**: Issues claimed (`status:in-progress`) but no PR opened within 4 hours trigger a stale claim alert.`
- **`skills/bug-fixer/SKILL.md`**:
  - Line 166: `- ⚠️ Stale claims: issues with `status:in-progress` but no PR opened in >4 hours`

### `urgent` (1 references)
- **`scripts/safe_ingest.py`**:
  - Line 124: `"urgent": ("Urgent", 0),`
