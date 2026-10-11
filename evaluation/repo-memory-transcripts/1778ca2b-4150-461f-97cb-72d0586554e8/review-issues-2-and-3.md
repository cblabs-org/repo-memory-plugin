# Review: Issue #3 (v2 Design) and Issue #2 (Platform Questions)

## Issue #3 — Design review: repo-memory v2 distillation and retrieval

I've read the v2 design doc, the SPEC v0.9, the dogfood evaluation (including Appendix C's literature review), and the E01–E12 experiment summary. Here are my thoughts, structured around the seven reviewer questions from §14, plus observations on the cost model and risks.

---

### Q1. Turn notes or sidecar replay?

**Turn notes should be primary. The evidence is clear.**

The cache-lifetime measurement (94% → 0% after 5 min idle) is the single strongest piece of data in the whole document. Distilling in-context costs a few thousand uncached tokens; replaying cold costs 55–135k. That's a 20–50× multiplier, and it's the user's own model and quota.

The concern about visible noise is real but manageable:
- The gate (§5.1) already demands multiple conditions to hold — that's the right shape.
- The note block is self-contained and appears *after* the agent's answer. Users will learn to ignore it the way they ignore tool-call details.
- The fallback-only setting is a good safety valve for users who do mind.

The real risk isn't noise — it's **quality variance across models**. A Flash session will produce weaker notes than an Opus session. The design tags `model` on every note (§13), which is good — consolidation should weight accordingly, and X2 should measure the gap explicitly.

> **My answer: Turn notes primary, sidecar fallback. Ship it with an easy opt-out.**

---

### Q2. Gate defaults

The gate signals and thresholds look reasonable as starting points, with two adjustments:

1. **The 30-minute cooldown may be too aggressive for long multi-turn sessions.** A developer who works through a complex debugging session over 90 minutes with three distinct phases (diagnosis → root-cause → fix) would get at most 2 notes when 3 would be warranted. Consider tying the cooldown to topic shifts (e.g., a new file cluster touched) rather than pure wall-clock time.

2. **20 notes/day is generous for a single developer.** At 3–10 notes/day projected steady-state (§9), a cap of 20 gives 2–7× headroom. That's fine — it protects against runaway without biting in practice. But track it from day one; if the median active developer hits 15+, something is wrong with the gate.

The milestone waiver (commit, PR opened, work order completed) is a smart touch — these are natural knowledge-crystallisation points.

> **My answer: Ship with these defaults, but consider making cooldown topic-aware rather than purely time-based.**

---

### Q3. What qualifies (§5.2)?

The "would a capable engineer new to this module get it wrong without being told" test is excellent. It directly operationalises the VibeMemBench finding that the useful record is about 5 lines covering *what went wrong and how to fix it*.

**What it would correctly exclude:**
- Code restatements ("the search command uses tabwriter") — these were 34.4% of v1's claims.
- Progress narration and plans.
- Facts discoverable by reading the code.

**What it might wrongly exclude:**
- **Cross-module interaction patterns.** "Module A must call B's `Init()` before C starts listening" — a new engineer *might* figure this out from the code, but it could take 30+ minutes. The "30 seconds to rediscover" threshold may be too strict for these.
- **Configuration gotchas with non-obvious defaults.** "Setting `triage.min_relevance` below 0.2 causes a quadratic blowup in reconcile" — the code doesn't make this obvious.

The eight note types (§5.2) cover the right categories. I'd keep `convention` in scope but police it during consolidation — conventions tend to be low-value individually but valuable in aggregate.

> **My answer: The test is strong. Relax the "30 seconds" heuristic to something like "would require reading more than two files to understand", which captures cross-module knowledge.**

---

### Q4. Dossiers as the source of truth

This is a good decision, and well-justified by the evidence:

- The field converged on files (§2.2, Appendix C.4.2) — Codex, Claude Code, Letta all use markdown.
- The ACE finding that repeated whole-page rewrites degrade memory is critical. Entry-level operations (add, amend, supersede) avoid this.
- Stable IDs enable delivery, verification, and evidence linking per-entry.

**What you lose vs. one-file-per-claim:**
- Fine-grained git blame per claim. With dossiers, blame is per-section at best. This is a minor loss because the `source` field on each entry covers provenance.
- Independent lifecycle per claim. In a dossier, retiring entry [P-003] is a diff to a shared file. With separate files, it's a deletion. The dossier approach is operationally cleaner.

**The 3k-token cap per dossier (§7.3) deserves scrutiny.** For a complex module like the transcript parser (which you yourself describe as "architect-complexity work"), 3k tokens might be tight once you have invariants, decisions, ruled-out approaches, failure signatures, and procedures. Measure on the first 5 real dossiers and adjust.

> **My answer: Good decision. Watch the cap — 3k may need to be 4–5k for complex modules.**

---

### Q5. Delivery on Antigravity

This is the question where the platform constraints bite hardest.

**First-read push (§8.2 "first read in a dossier's scope") is the right primary path.** The evidence:
- Coherence Debt: "a fact helps only while it is in context when the agent edits." Touch-on-read puts it there at the right time.
- `ephemeralMessage` is per-invocation transient (E01), but delivering on the `PreInvocation` *following* the file read (via `PostToolUse` → pending → `PreInvocation`) means it's present for the invocation where the agent reasons about what it just read. That's the right moment.

**Deny-once on edit (the `gate: edit` entries) should be reserved for genuine invariants.** E09 showed that deny-with-reason is delivered verbatim and is isolated within parallel tool calls. But it costs an extra model step per gated edit — the agent re-reads the denial, reasons about it, and re-issues. That's real latency and tokens.

Reserve it for entries where violating the invariant would cause a hard-to-debug failure:
- Architectural boundaries ("internal/core must not import adapters")
- Concurrency invariants ("always acquire lock A before lock B")
- Data format invariants ("step_index may restart at 0 after session reset")

Everything else should be first-read push only.

**On `userMessage` persistence (Issue #2 Q2):** Until this is confirmed, the task-start push should point at `memory.brief` rather than carrying content. The design already hedges this in §8.2 ("it decides whether the task-start push can carry content, or only point at `memory.brief`"). That's the right hedge.

> **My answer: First-read push as primary. Deny-once only for entries explicitly marked `gate: edit`, restricted to true invariants. Task-start push should be a pointer until `userMessage` persistence is confirmed.**

---

### Q6. Subagents

**Don't note subagents. The design gets this right.**

E08 confirmed subagents fire all hooks with their own `conversationId`. But:
- We don't know if `Stop` `continue` changes what the subagent returns to its parent (Issue #2 Q22–Q23). Until we do, the risk of changing subagent behavior is unacceptable.
- Subagent turns are typically narrow and task-focused — less likely to produce the kind of relational, architectural knowledge that notes capture best.
- The sidecar fallback covers subagent transcripts.

**However: subagents *should* receive delivery.** E08 showed hooks fire for subagent tool calls. A subagent editing `internal/core/compact/compactor.go` should see the invariants for that module just as a parent would. The delivery hooks should not filter out subagent conversations — only the note-taking hooks should.

> **My answer: No notes for subagents (correct as designed). Yes to delivery for subagents. Revisit after Q22–Q23 are answered.**

---

### Q7. The kill criterion (X6)

"Dossiers must beat no memory in a work-order replay" is **necessary but not sufficient**.

It's necessary because Appendix C.4.1 cites VibeMemBench showing 11/12 system/solver pairings performed at or below no memory. If dossiers can't clear this bar, nothing else matters.

It's not sufficient because:
1. **The comparison should also include PA documents.** The evaluation showed PA distillations are richer. The kill criterion should be: "dossiers beat no memory, and are not worse than PA documents at fewer tokens." If they're worse than PA docs, you need to understand why before proceeding.
2. **"Beat no memory" on what metric?** Tests passed is necessary but not sufficient — an agent can pass tests while making bad architectural decisions. Steps and tokens are secondary. I'd add: "a blind reviewer judges the code changes as at least as good."
3. **6–8 work orders may be too few.** Appendix C.3.4 notes the small-sample problem. At 6 work orders per condition, you can only detect very large effects. If budget allows, aim for 12–15. If it doesn't, accept that a pass is a weak positive, and plan for a larger follow-up.

> **My answer: Strengthen to "beat no memory AND match PA documents at ≤½ the recall tokens, on tests passed + blind code-quality review." 12+ work orders if feasible.**

---

### Cost model (§9) — Flaws

1. **The "~1–5k uncached" estimate for turn notes is optimistic.** The `reason` injection is 150 tokens, but the agent will also re-read its recent output to understand the context for the note. In practice, uncached tokens will depend on what changed since the last cache hit. Measure this in X1 before committing to the gate thresholds.

2. **"The main unknown is how the quota counts cached tokens" (§9).** This is not just an unknown — it's a design-critical dependency. If cached tokens count at full rate, the entire turn-note architecture becomes expensive: a 75k cached context + 5k uncached = 80k tokens per note at full rate. The gate would need to fire 2–3× less often, and the fallback would carry most of the load. **Recommend: answer Q6 from Issue #2 before committing to turn-note-primary.**

3. **Consolidation cost is underestimated.** "~25–50k a day" assumes 1–2 calls at 10k overhead + 0.5k per note. But the consolidation prompt needs the target dossier sections (1–3k each) plus the notes. With 10 notes touching 5 different dossiers, you need the sections for all 5 — that's 5–15k of dossier context on top of the overhead and notes. More realistic: 40–80k per day.

---

### Risks (§13) — Missing items

1. **Interaction with other plugins.** Other plugins may also use `Stop` hooks. If two plugins both return `continue`, the agent gets two continuation requests. The design doesn't address hook ordering or conflict. This is also an Issue #2 question (Q26).

2. **Model instruction-following on the note format.** The note request is a structured-output prompt delivered as a system message via `Stop` `reason`. Smaller models (Flash) may not reliably produce the `repo-memory-note` fenced block format. The harvest-side fallback (keep raw text with parse error, §5.1) is good, but if the parse-error rate is high (>20% on Flash), the signal-to-noise ratio degrades.

3. **Dossier merge conflicts in multi-machine setups.** The design says dossiers are in `.memory/` (git-tracked). If two machines run consolidation on the same repo, their dossier changes will conflict. The "one memory branch" approach (§7.4, §6 step 5) mitigates this, but the design should explicitly state that consolidation is single-writer.

---

## Issue #2 — Antigravity Platform Questions

I've reviewed all the questions (Q1–Q39) against the Antigravity customization docs, the hooks reference, and the experiment findings. Here's what I can confirm, what I can infer, and what truly needs an answer from the platform team.

### What the docs already answer

| Question | Answer from docs/experiments | Evidence |
|:---|:---|:---|
| **Q1** (`ephemeralMessage` per-invocation?) | **Yes, per-invocation transient.** | E01 confirmed; hooks.md §3 calls it "transient system message" |
| **Q27** (undocumented behaviours) | **All confirmed by experiments.** `{}` = deny (E08), exit non-zero = deny (E09), paths relative to `hooks.json` (E09), `overwrite` rewrites with notice (E09, hooks.md §1), malformed `injectSteps` silently ignored (E01) | The question is whether they're *intended and stable*, not whether they happen. That still needs a platform answer. |
| **Q28** (`PostToolUse` payload) | **Partially answered.** hooks.md §2 shows the payload has `stepIdx` and `error`. The matcher does filter by tool name. Tool name/input/output are NOT in the payload — the hook must read the transcript. | hooks.md lines 225–236 |
| **Q29** (hook timeout) | **30 seconds default**, configurable per handler via `timeout` field. Behaviour on timeout is not documented. | hooks.md line 128 |
| **Q30** (hooks in `agy -p`?) | **Yes.** E07 and E10 show hooks fire in `agy -p` sessions. | Experiment evidence |
| **Q39** (`glob` trigger) | **Effectively deprecated.** Only `always_on` and `model_decision` are documented in rules.md. `activation:` is ignored (E04). `glob` was not observed to fire. | rules.md, E04 |

### What can be inferred but should be confirmed

| Question | Inference | Confidence | Why confirm |
|:---|:---|:---|:---|
| **Q2** (`userMessage` persistence) | Likely persists across invocations — `userMessage` would be modeled as a user turn in the conversation history, which persists. `ephemeralMessage` is explicitly transient; the distinction implies `userMessage` is not. | Medium | Critical for delivery architecture. Must test (E1b). |
| **Q3** (`toolCall` persistence) | Likely persists — synthetic tool calls should join the conversation history. | Medium | Less critical if `userMessage` works. |
| **Q24** (reliable "turn is over" signal) | `fullyIdle == true && terminationReason == "model_stop"` is the best available test. E08 showed `Stop` also fires when parent idles for a subagent, but `fullyIdle` would be false in that case. | High | Gate correctness depends on this. |
| **Q33** (append-only transcript) | E03 and E12 confirmed append-only. Editing/branching was not tested. | High for append; unknown for edit/branch | Parser correctness |

### Questions that truly need platform answers

These are the ones where the docs and experiments provide no answer, and the design depends on the answer:

#### Critical for v2

| Question | Why it matters | Impact if answer is bad |
|:---|:---|:---|
| **Q2** (`userMessage` persistence) | Determines whether task-start push can carry content or must be pointer-only | Design works either way, but pull-only is worse UX |
| **Q5–Q7** (cache lifetime, quota accounting) | If cached tokens count at full rate, turn notes are expensive | Gate fires less often; fallback carries more load |
| **Q23** (`Stop` `continue` behaviour) | The entire turn-note mechanism depends on this: cache, visibility, second `Stop`, loop protection | If `continue` doesn't reuse cache, turn notes lose their cost advantage. If there's no loop protection, the gate must implement its own (it already does — one request per stop sequence). |
| **Q25** (prompt against context without joining history) | Would enable "inspect-only" distillation that doesn't pollute the conversation | Nice-to-have, not critical — the current design adds the note to the conversation, which is acceptable |
| **Q32** (conversation folder deletion) | Durable state in `brain/<id>/` is deleted. Must copy artifacts before deletion. | Design already stores in plugin data dir. But if deletion happens without warning, artifacts can be lost. |

#### Important but not blocking

| Question | Why it matters |
|:---|:---|
| **Q4** (recommended pattern for persistent context) | Confirms or denies the `userMessage` approach |
| **Q8–Q12** (context-window structure) | Understanding ordering helps optimise injection placement |
| **Q13–Q15** (`agy -p` details) | Affects sidecar/fallback implementation |
| **Q16–Q22** (subagent behaviour) | Needed before enabling subagent notes |
| **Q26** (`PostInvocation` vs `Stop` for continuation) | Design uses `Stop`; should verify it's the right choice |
| **Q31** (transcript as supported interface) | Risk of format changes breaking the parser |
| **Q34** (`thinking` field availability) | Affects what the fallback can extract |
| **Q35** (injected messages in transcript) | Affects harvest: can the hook find its own `reason` in the transcript? |
| **Q36** (artifact stability) | Affects artifact copying at capture |
| **Q37–Q38** (sidecar headless, enabled check) | Affects fallback reliability |

---

## Summary of Key Recommendations

### For Issue #3 (v2 design):

1. **Ship turn-notes-primary** — the cache evidence is overwhelming
2. **Strengthen the kill criterion** (X6) to also compare against PA documents
3. **Answer Q5–Q7 (quota accounting) before finalising gate thresholds** — this is the biggest financial risk
4. **Watch the 3k dossier cap** — may need 4–5k for complex modules
5. **Reserve deny-once delivery for true invariants only** — first-read push for everything else
6. **No subagent notes until Q22–Q23 are answered** — but do deliver to subagents

### For Issue #2 (platform questions):

1. **Run E1b immediately** — `userMessage` persistence is the most impactful open question
2. **Prototype X1** (Stop continue behaviour) — the whole architecture depends on it
3. **Q5–Q7 are design-critical** — escalate to the Antigravity team
4. **Many questions are already answered** by experiments — consider closing those or marking them confirmed to reduce noise
5. **Group the remaining questions by urgency** — the "Critical for v2" table above gives a starting prioritisation
