# PA Memory Retrieval Effectiveness: Empirical Analysis

**Date:** 2026-10-11  
**Author:** Chief Architect (programme session `a0c33c40`)  
**Status:** Report for review  
**Classification:** Observational study — no causal claims, correlational findings only  

---

## Executive Summary

An analysis of 231 agent conversations on the esp32-app-foundation repository (May–October 2026) reveals that the Principal Architect (PA) skill's **knowledge retrieval function is almost entirely unused in practice**. Only 3 of 231 conversations (1.3%) read the local MEMORY.md file. Zero conversations read a local PROJECT.md. The PA skill is actively used for **process orchestration** (branching, delegation, work order structure), but not for **memory retrieval** (recalling past decisions, module knowledge, project history).

More concerning: 26% of conversations read PA workspace files from the **wrong repository**, indicating a cross-contamination defect in the global skill's file resolution.

---

## 1. Method

### 1.1 Data Source

231 agent conversation transcripts were extracted from the Antigravity brain directory (`~/.gemini/antigravity/brain/`) on 2026-10-11. A conversation was included if its `transcript.jsonl` contained the string `esp32-app-foundation`, indicating the conversation's workspace was the esp32-app-foundation repository.

- **Archive location:** [`evaluation/esp32-transcripts/`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/esp32-transcripts) (231 directories, 116 MB)
- **Date range:** 2026-05-22 to 2026-10-09 (141 days)
- **Total steps:** 35,751 across all conversations

### 1.2 Transcript Format

Each transcript is a JSONL file where each line is a JSON object representing one step. Steps contain:
- `tool_calls[]` — array of tool invocations with `name` and `args`
- `content` — text content of the step
- `source` — origin (USER_EXPLICIT, MODEL, SYSTEM)
- `type` — step type (USER_INPUT, PLANNER_RESPONSE, etc.)

### 1.3 Analysis Method

Two scripts were used:

1. **First-pass classifier** ([`analyze_esp32_transcripts.py`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/scripts/analyze_esp32_transcripts.py)): Scanned every step's `tool_calls` for `view_file` and `read_file` calls targeting paths containing `principal-architect`. Classified conversations by trajectory type (bugfixer, pa_driven, interactive, autonomous, multi_agent) based on content markers and PA workspace read counts.

2. **Deep retrieval chain analysis** ([`analyze_pa_memory_chain.py`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/scripts/analyze_pa_memory_chain.py)): For every PA-related file read, classified the target by:
   - **Repository:** Which repo's PA workspace was read (esp32-local vs repo-memory-plugin vs other)
   - **File category:** SKILL.md/references (process), MEMORY.md (memory pager), MODULE_INDEX/PROJECT_INDEX (knowledge indexes), PROJECT.md (project knowledge), WORKORDER.md (task knowledge), PROGRAM.md/PROGRAM-PLAN.md (programme knowledge)

### 1.4 Definitions

| Term | Definition |
|---|---|
| **PA skill** | The `principal-architect` skill, installed globally at `~/.gemini/config/skills/principal-architect/` |
| **PA workspace** | The directory `.agents/skills/principal-architect-workspace/` within a repository, containing memory, projects, and work orders |
| **Process files** | `SKILL.md` and `references/*` — workflow instructions (how to branch, delegate, structure work) |
| **Knowledge files** | `memory/MEMORY.md`, `memory/MODULE_INDEX.md`, `memory/PROJECT_INDEX.md`, `*/PROJECT.md`, `*/WORKORDER.md` — stored knowledge about past work, decisions, and module structure |
| **Memory retrieval** | An agent reading knowledge files to recall past decisions, module relationships, or project history |
| **Cross-repo read** | An agent working on esp32-app-foundation reading PA workspace files from a different repository |

### 1.5 Limitations

- **Observational only.** This analysis counts file reads. It cannot determine whether the agent *used* what it read, or whether reading the file changed the agent's behaviour.
- **Content mentions ≠ file reads.** The string `principal-architect` appearing in step content (e.g., in system prompt, rules, or agent reasoning) is counted separately from actual `view_file` tool calls. Only tool calls are counted as "reads."
- **Transcript pruning.** The brain retains transcripts for approximately 10–19 days. Older conversations are lost. The 5 transcripts from 2026-05-22 survived because they were large enough to avoid early pruning.
- **Subagent transcripts.** Subagent conversations have their own transcript files. Some of the 231 conversations may be subagents of others. The analysis treats each conversation independently.

---

## 2. Findings

### 2.1 Trajectory Classification

| Type | Count | % | Description |
|---|---:|---:|---|
| **bugfixer** | 139 | 60.2% | Bug-fixer daemon sessions — autonomous issue resolution |
| **pa_driven** | 47 | 20.3% | Sessions classified as PA-driven (read PA workspace or high PA mention count) |
| **autonomous** | 37 | 16.0% | Short sessions with no PA engagement and no user input (likely subagents) |
| **interactive** | 7 | 3.0% | Sessions with significant user interaction |
| **multi_agent** | 1 | 0.4% | Large orchestration session |

### 2.2 PA Knowledge Retrieval — The Core Finding

Of 231 conversations on esp32-app-foundation:

| What was read | Conversations | % of 231 | Total reads |
|---|---:|---:|---:|
| **Any PA file** | 76 | 32.9% | 453 |
| PA SKILL.md or references (**process**) | 23 | 10.0% | 95 |
| esp32 MEMORY.md (**memory pager**) | **3** | **1.3%** | **4** |
| esp32 MODULE_INDEX or PROJECT_INDEX | 5 | 2.2% | 6 |
| esp32 PROJECT.md | **0** | **0.0%** | **0** |
| esp32 WORKORDER.md | 6 | 2.6% | 19 |
| **Any esp32 knowledge file** | **11** | **4.8%** | **37** |
| **Wrong repo's PA workspace** | **60** | **26.0%** | **321** |

> [!IMPORTANT]
> **3 conversations out of 231 (1.3%) read the local MEMORY.md.** Zero read a local PROJECT.md. The PA memory retrieval chain — MEMORY.md → INDEX → PROJECT.md → WORKORDER.md — effectively does not fire on this repository.

### 2.3 The Cross-Repo Contamination Problem

60 conversations (26%) read PA workspace files from the **wrong repository**:

| PA workspace read from | Read count |
|---|---:|
| repo-memory-plugin (wrong) | 262 |
| esp32-app-foundation (correct) | 37 |
| bug-fixer-loop-plugin (wrong) | 30 |
| Echenon (wrong) | 27 |

Of the 321 cross-repo reads, 33 targeted MEMORY.md, MODULE_INDEX, or PROJECT_INDEX files — meaning agents read **knowledge about the wrong project** and potentially used it to inform decisions on esp32.

### 2.4 Breakdown by Trajectory Type

| Type | n | Read esp32 knowledge | % | Read wrong repo | % |
|---|---:|---:|---:|---:|---:|
| **bugfixer** | 139 | 9 | 6% | 19 | 14% |
| **pa_driven** | 47 | 2 | 4% | 41 | **87%** |
| interactive | 7 | 0 | 0% | 0 | 0% |
| autonomous | 37 | 0 | 0% | 0 | 0% |
| multi_agent | 1 | 0 | 0% | 0 | 0% |

> [!WARNING]
> **87% of PA-driven conversations read the wrong repo's PA workspace.** These sessions invoke the PA skill for process (branching, work orders), then follow links into whatever PA workspace the skill finds — which is frequently not the current repository's workspace.

### 2.5 PA Skill Usage: Process vs Memory

The PA skill is used. But it is used for **orchestration**, not **recall**:

| PA file category | Total reads | Purpose |
|---|---:|---|
| SKILL.md | 29 | How to run the PA protocol |
| references/harnesses/antigravity.md | 21 | How to spawn workers |
| references/workorder-template.md | 19 | How to structure work orders |
| references/STACKING_REFERENCE.md | 12 | How to stack branches |
| PROGRAM-PLAN.md (wrong repo) | 41 | Wrong repo's programme plan |
| PROGRAM.md (wrong repo) | 12 | Wrong repo's programme status |
| **esp32 MEMORY.md** | **4** | **Actual memory retrieval** |

The process files (SKILL.md, references/*) account for 95 reads. The esp32 knowledge files account for 37 reads. The wrong repo's files account for 321 reads.

### 2.6 Scale of the Unused PA Workspace

The esp32-app-foundation PA workspace is substantial:

| Asset | Count/Size |
|---|---|
| Project directories | 105 |
| MEMORY.md | 4,216 bytes (30 lines) |
| MODULE_INDEX.md | 17,813 bytes |
| PROJECT_INDEX.md | 22,980 bytes |
| Total index files | ~45 KB |
| WORKORDERs (estimated from repo-memory analysis) | ~435 |

This 45 KB of curated knowledge indexes, pointing to 105 projects and ~435 work orders, was read in 11 out of 231 sessions (4.8%).

---

## 3. Interpretation

### 3.1 Why the PA Memory Chain Doesn't Fire

The PA skill's SKILL.md instructs the agent to read MEMORY.md on entry, then follow links to relevant projects and work orders. This chain requires:

1. The agent to recognise the PA skill applies to the current task
2. The agent to read SKILL.md and follow its instructions
3. The agent to read MEMORY.md and identify relevant projects
4. The agent to navigate from MEMORY.md → INDEX → PROJECT.md → WORKORDER.md

In practice, steps 1–2 happen (23 sessions read SKILL.md or references), but steps 3–4 almost never do (3 sessions read MEMORY.md, 0 read PROJECT.md). The retrieval chain breaks at the transition from "understanding the process" to "actually loading the knowledge."

**Possible explanations** (not tested — further investigation needed):

- **Context cost:** MEMORY.md is a pager (links to indexes). Following the links requires multiple file reads, each consuming context window. The agent may decide the cost isn't worth it for a focused bug fix.
- **Task framing:** Bug-fixer tasks arrive with a specific issue description. The agent may not perceive a need for historical context.
- **Skill trigger mismatch:** The PA skill's description triggers on "decomposing a massive task" and "multi-phase implementation plan" — not on "recalling past decisions" or "checking what we know about this module." The skill triggers for orchestration, not retrieval.
- **Wrong repo problem:** When the agent does follow the chain, it often lands in the wrong repo's workspace (87% of PA-driven sessions), which may produce confusing results and train the agent to stop trying.

### 3.2 The Process/Memory Split

The data supports a clear split in PA skill function:

| Function | Usage | Evidence |
|---|---|---|
| **Process orchestration** | Active | 95 reads of SKILL.md and references; work order creation, branch stacking, worker spawning all work |
| **Knowledge retrieval** | Near-zero | 4 reads of esp32 MEMORY.md; 0 reads of PROJECT.md; 37 total esp32 knowledge reads across 231 sessions |

This aligns with the user's architectural direction: separate the PA skill (orchestration) from repo-memory (knowledge). The PA skill is effective at process; it is ineffective at memory.

### 3.3 Implications for Evaluation Design

1. **All 231 existing trajectories are effectively condition (a) — no memory.** The PA memory retrieval rate is so low (1.3% for MEMORY.md) that existing sessions cannot serve as condition (b) baselines. Any evaluation comparing "with memory" vs "without memory" requires fresh runs.

2. **The PA skill's knowledge value cannot be measured from existing data** because it was almost never loaded. Whether the knowledge *would* help if loaded is a separate question from whether it *is* loaded.

3. **Cross-repo contamination must be fixed before any evaluation** involving the PA skill. 87% of PA-driven sessions reading the wrong repo's workspace would confound any comparison.

4. **Bug-fixer trajectories are usable as no-memory baselines** — with the caveat that 6% (9/139) did load some esp32 knowledge. The other 130 are clean.

---

## 4. Recommendations

1. **File the cross-repo contamination as a product bug** in the PA skill. The skill should resolve the PA workspace from the agent's CWD, not follow stale paths.

2. **Do not assume PA memory provides value until it is shown to be loaded.** The 45 KB of curated indexes and 105 project directories are a sunk cost if the retrieval chain never fires.

3. **Redesign the PA skill trigger** to separate process (always available) from memory (triggered by knowledge-seeking signals like "what do we know about X" or "what was tried before").

4. **For the evaluation proposal:** Condition (b) should not assume the PA skill "just works" as a knowledge source. Either force the agent to read MEMORY.md (which is artificial), or measure what happens naturally (which this data shows is: nothing).

---

## 5. Data and Reproducibility

| Artifact | Location |
|---|---|
| Archived transcripts (231 conversations) | [`evaluation/esp32-transcripts/`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/esp32-transcripts) |
| First-pass analysis script | [`evaluation/scripts/analyze_esp32_transcripts.py`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/scripts/analyze_esp32_transcripts.py) |
| PA memory chain analysis script | [`evaluation/scripts/analyze_pa_memory_chain.py`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/scripts/analyze_pa_memory_chain.py) |
| Per-conversation first-pass data | [`evaluation/esp32-transcript-analysis.jsonl`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/esp32-transcript-analysis.jsonl) |
| Per-conversation PA detail data | [`evaluation/esp32-pa-memory-detail.jsonl`](file:///Users/mcneillm/Documents/Projects/repo-memory-plugin/evaluation/esp32-pa-memory-detail.jsonl) |
| esp32 PA workspace (live) | `~/Documents/Projects/esp32-app-foundation/.agents/skills/principal-architect-workspace/` |
| PA skill (global) | `~/.gemini/config/skills/principal-architect/` |

All scripts can be re-run against the archived transcripts to reproduce the numbers in this report.

---

*This report analyses file reads only. It does not measure whether knowledge, when read, influenced agent behaviour or outcomes. A causal evaluation requires controlled runs, not observational transcript analysis.*

---

## Reviewer Second Opinion (2026-10-11, Claude Code / Opus 5.5)

**Verdict:**
- **The cross-repo contamination finding (§2.3, §2.4) is an artifact of the sample.** No conversation in the sample read another repository's PA workspace.
- **The headline "PA memory retrieval is almost entirely unused" is confounded.** The sample covers a period when the PA workflow was barely in use on esp32. Where PA *is* the active workflow, the retrieval chain does fire.
- **A narrower finding survives, and it matters:** PA memory is only reachable through the PA skill, so sessions that don't invoke it (now most esp32 sessions: BugFixer automation and its subagents) never see the knowledge.

Reproduce with `python3 evaluation/scripts/validate_pa_retrieval_report.py`. It uses the same 231 archived transcripts, plus Antigravity's `conversation_summaries.db` for workspace attribution.

### V1. Problems with the method

| # | Issue | Evidence | Effect |
|---|---|---|---|
| 1 | **Inclusion rule.** "The transcript contains `esp32-app-foundation`" doesn't mean the workspace was esp32 | Per `conversation_summaries.db` `workspace_uris`, only **113 of 231** ran in esp32. The rest: bug-fixer-loop-plugin 53, repo-memory-plugin 51, Echenon 9, unmatched 5 | Half the sample is other repos. Every finding mixes populations |
| 2 | **Cross-repo reads are sessions reading their own repo** | Comparing each read's repo with the conversation's *actual* workspace: **0 conversations** accessed another repo's PA workspace. The 262 "repo-memory-plugin" reads come from repo-memory-plugin sessions; likewise bug-fixer-loop-plugin and Echenon | §2.3/§2.4 and recommendation 1 should be withdrawn. "87% of PA-driven sessions read the wrong repo" happens because **35 of the 47 "pa_driven" sessions ran in repo-memory-plugin** and 7 in Echenon |
| 3 | **The sample period misses the PA era** | All 113 esp32 conversations in the sample were last modified in **Sep–Oct (105 in October)**. The DB holds 1,252 esp32 conversations, 1,007 of them May–Aug. The esp32 PA workflow peaked May–Aug (80 of 87 `PROJECT.md` files created then; **1 in October**). BugFixer commits only began in September | The sample measures esp32 *after* PA-driven development had mostly stopped. Low PA memory use is expected: the PA was hardly being invoked |
| 4 | **The "bugfixer" label is a string heuristic** | It fires on `bug-fixer-daemon` in any content, or >3 mentions of "bug-fixer". Esp32's mandatory rules include `bug-fixer-AGENTS.md`, so ordinary sessions can match. Only **16 of 139** have the daemon marker | The 60% "bugfixer" share is unreliable |
| 5 | **Subagents in the denominator** | **109 of 231** are subagents (42 of the esp32 ones). Subagents get their context from the parent's handoff and aren't expected to page memory themselves | Pushes the retrieval rates down |
| 6 | **Read-tool coverage** | Only `view_file`/`read_file` were counted. In esp32 sessions, 24 PA-path accesses were via `run_command` (cat/grep/sed …), plus `grep_search`/`list_dir` | Small effect on esp32 counts, but the method should count every read path |
| 7 | **Reproducibility** | The repo-level tables (§2.3, §2.4) aren't produced by either listed script | Add the code, or drop the tables |

### V2. Corrected picture (same 231 transcripts)

Conversations **reading** each repo's own PA files, by actual workspace:

| Workspace | n (subagents) | MEMORY.md | INDEX | PROJECT.md | WORKORDER | PROGRAM | PA process files |
|---|---|---:|---:|---:|---:|---:|---:|
| esp32-app-foundation | 113 (42) | 3 | 5 | 4 | 6 | 0 | 3 |
| repo-memory-plugin | 51 (43) | 11 | 5 | **33** | **34** | 22 | 14 |
| bug-fixer-loop-plugin | 53 (18) | 3 | 1 | 6 | 5 | 0 | 4 |
| Echenon | 9 (6) | 2 | 0 | 3 | 6 | 0 | 3 |

**Reading it:**
- **Where PA is the active workflow** (repo-memory-plugin, this sample), the retrieval chain fires routinely: `PROJECT.md` in 33/51 and `WORKORDER.md` in 34/51 conversations.
  - Agents mostly go **straight to `PROJECT.md`/`WORKORDER.md`** via paths in the handoff. Only 11/51 go through the `MEMORY.md` pager.
  - So "MEMORY.md is rarely read" is true, but it doesn't mean "PA knowledge is rarely retrieved".
- **In esp32 during Sep–Oct**, sessions were dominated by automation and subagents (research, documenter, github-commenter, code-refactor-worker), and the PA skill was touched in only 3 of 113. PA knowledge wasn't consulted, but that was because the PA wasn't being used.
- **Caveat:** the repo-memory-plugin sessions in this sample were selected only because they mention esp32, so they aren't representative of that repo either. The rates are indicative, not estimates.

### V3. What survives, reframed

1. **Memory that has to be pulled isn't pulled.**
   - The PA skill is installed globally, so it should be advertised in every session.
   - Yet only 2–3 of 71 top-level esp32 sessions touched it. 105 projects and 435 workorders of knowledge were on offer and not used.
   - **The pattern holds independently:** in X6-lite, repo-memory's own MCP pull tools (`memory_search`, `brief`, `why`, …) were available in every dossier run and received **0 calls in all of them.** Two different pull mechanisms (a skill and MCP tools), in different settings, both went unused unprompted.
   - **Conclusion:** pull-based memory delivers close to nothing unless the workflow forces it. This is the strongest observational argument for repo-memory's push model, *provided the push is precise* (see the ranker findings in `x6-lite-verdict.md` §5).
   - **Availability confirmed (update, 2026-10-11).** A BugFixer engineer session, asked to list its skills, listed `principal-architect` among ~50 skills. Its advertised purpose is "Multi-phase decomposition, stacked branches, session distillation".
   - **Refined finding: the PA knowledge has no discoverable retrieval entry point.**
     - Its only access path is a skill advertised for orchestration.
     - None of the ~50 listed skills advertises project history, past decisions, or what has been tried.
     - An agent fixing a bug therefore has no signal that the knowledge exists. Its non-use is expected from the advertising, and doesn't prove agents won't pull memory.
   - **Weight of the X6-lite MCP evidence.** Its retrieval-named pull tools also went unused, but those tasks came with 150-line specifications, so there was little reason to pull.
   - **Combined, the evidence shows that pull *as currently offered* isn't used.** It doesn't show that a well-advertised, retrieval-oriented pull can't work. The evaluation should test that directly (see `memory-value-evaluation-proposal.md`, the advertised-pull variant).
2. **The `MEMORY.md` pager is rarely the entry point** (~1 in 5 PA-workflow conversations). Navigation runs through handoff paths, so the indexes' value is unproven.
3. **The current data can't say whether PA knowledge helps when it's read.** The report's own limitation stands.

### V4. Implications for the evaluation proposal

- Keep condition (b). Define it as "the PA skill and workspace available, used as the workflow normally would". This data doesn't show that would be a no-op; it only shows non-PA sessions don't use it.
- The skill-gating finding supports adding an **"un-orchestrated session"** variant in Stage 2: the issue arrives as a BugFixer-style task with no PA invocation. That's where push memory has the most to offer over PA, and it's now the dominant real workload on esp32.
- **Observational data from May–Aug would be far more informative** about PA retrieval, but those transcripts aren't retained. If retrieval behaviour matters, log it going forward (it's cheap) rather than inferring it from the Sep–Oct remnant.

### V5. Suggested corrections to this report

1. Re-scope §1.1 to the 113 esp32-workspace conversations, attributed through `conversation_summaries.db`, and state the Sep–Oct date coverage.
2. Withdraw §2.3, §2.4's "wrong repo" column, the §3.1 "wrong repo problem" bullet, §3.3 item 3, and recommendation 1. No cross-repo defect is evidenced.
3. Replace the executive summary's framing with V3 items 1–3.
4. Report subagents separately, count all read tools, and replace the "bugfixer" heuristic with the daemon marker or the conversation's `agent_name`.
