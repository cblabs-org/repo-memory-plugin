# Memory Retrieval Quality & Eval Scope — Follow-on Report

**Date:** 2026-10-11
**Author:** Claude Code (Opus 5.5), reviewer
**Status:** **Deferred. Start only after X6-lite is complete** (scoring fix + Block 1 re-run, per the batch report's W6). The only items to act on during X6-lite are in §7.

---

## 1. Summary

X6-lite measures memory on a small codebase with detailed, spec-style tasks. That setup has two consequences:

1. **It can't show the benefits that grow with codebase size,** such as navigation and finding the relevant prior knowledge among thousands of entries.
2. **It can't expose the failure that grows with size:** injecting the wrong entries.

Now that delivery works, the open question is no longer "does content reach the model" but **"is the right content selected."** That is a retrieval-and-ranking problem, essentially RAG with an injection policy on top. It should be evaluated the way RAG systems are: **offline retrieval quality first** (cheap, many queries, fast iteration), then **end-to-end agent impact** (expensive, few runs).

The X6-lite batch data already shows retrieval problems worth fixing before any larger eval (§4).

---

## 2. Scope of the current eval

| Dimension | X6-lite | Typical large-repo use |
|---|---|---|
| Codebase | ~17–19K lines of Go (non-test), ~190 Go files, ~30 packages | 10⁵–10⁶+ lines, thousands of files |
| Agent exploration | Reads 8–22 distinct files per run; a few greps find anything | Can't read the relevant area; must search or recall |
| Memory index | 147–172 entries, all seeded | Thousands of entries, seeded + learned |
| Task type | New packages from 150–180-line WOs that already contain the design | Bug fixes, changes to existing code, vague issues, known pitfalls |
| Memory source | Seeded from PA docs/code at the cut | Seeded + captured from sessions (the learning loop) |

**What this hides:**
- **Navigation value.** "Where does X live, what touches Y" is cheap to rediscover here. A null result for navigation doesn't generalise to large repos.
- **Value of knowledge that isn't in the code** (decisions, ruled-out approaches, gotchas). This doesn't depend on size, but it is suppressed by spec-style WOs: the prompt already carries the design.
- **Ranking at scale.** With ~150–170 entries, a poor ranker still lands on something plausible. With thousands, precision decides whether injection helps or distracts.

**Scope statement for the X6-lite verdict** (paste in):
> X6-lite measured seeded-memory delivery on a ~18K-line Go repository, using new-package work orders with detailed specifications. It does not measure navigation benefit on large codebases, retrieval precision at large index sizes, the session-learning loop, or tasks dominated by historical pitfalls. A null result should not be generalised to those settings.

---

## 3. Framing: this is RAG with a push-based injection policy

| RAG stage | repo-memory equivalent | Where it lives today |
|---|---|---|
| Query formation | **Implicit.** No user query; built from the moment's context: task prompt (task start), touched file path (first touch), command output (error), edit target (gate) | `internal/harness/antigravity/preinvocation.go`, `posttooluse.go`, `pretooluse.go` |
| Candidate retrieval | FTS5/bm25 over prompt terms (top 20) + anchor lookup by path | `internal/core/delivery/engine.go:113-150` (`TaskStartEntries`), `AnchorEntries`, `ErrorEntries` |
| Re-ranking | Rule-based score: anchor match, entry type, verified, evidence count | `internal/core/delivery/ranker.go:26-99` (`RankEntries`) |
| Context budget | Per-moment token budgets | `internal/core/delivery/types.go:22` (`DefaultBudgets`: 500 task start, 150 first touch, 50 about-to-edit, 200 error/gate) |
| Generation | The coding agent | — |

**How it differs from classic RAG** (and why the evaluation must differ):
- **Push, not pull.** Entries arrive unrequested and interrupt the agent's context, so **precision matters more than recall**. An irrelevant or false injection is an active cost. Classic RAG tolerates noisy top-k because the user asked a question.
- **No explicit query.** Query construction from implicit context is itself a design problem, and probably the biggest lever.
- **Structural signals as well as text.** File-path anchors give a strong non-semantic retrieval channel that classic RAG lacks.
- **Validity over time.** Entries can be superseded or become false. Freshness and contradiction handling are part of retrieval quality, not just corpus hygiene.
- **A pull channel exists but went unused.** The MCP tools (`memory_search`, `brief`, …) received **0 calls in every X6-lite run.** Whether agents pull when they could is a separate question from push quality.

---

## 4. What the X6-lite data already shows about retrieval

Taken from the batch archives (`evaluation/results/archive/*-dossiers-rep1`) and code reading.

1. **Task-start ranking ignores relevance.**
   - `TaskStartEntries` retrieves up to 20 bm25 matches for the whole WO's prompt terms. `RankEntries` then re-scores them **without the bm25 score**: anchor match, entry type order, verified flag, evidence count.
   - At task start there are no diff paths, so no entry gets an anchor match, and the final order is effectively **by entry type.** That matches the pilot's injections being grouped as ruled-out, then decision, then open-issue.
   - The query is ~150 lines of WO text, so it matches broadly. Candidate quality is weak, and the re-ranker then discards what relevance signal there was.
2. **Duplicate injections.** The same entries are re-injected across first-touch moments:

   | Run | Injections | Unique entries |
   |---|---:|---:|
   | wo3 (c) | 38 | 15 |
   | wo2 (c) | 32 | 14 |
   | wo4-gates (c) | 57 | 34 |
   | wo6-pull-tools (c) | 60 | 39 |

   Roughly 35–60% of injections are repeats.
3. **Task-start injection sometimes doesn't happen.** wo2 and wo3 (c) recorded **0** task-start injections, only first-touch ones. FTS errors are swallowed silently (`engine.go:118-130`), so the cause is unknown: an empty query after stop-word filtering, an FTS syntax error, or no matches.
4. **Off-topic and false entries in the pilot:**
   - placeholders (`(pending)`, `N/A`, `None.`), now filtered at source by `3036eb4`;
   - off-topic entries for a gates task (Windows path escaping, origin-URL canonicalisation);
   - one now-false entry, `seed-b1617c95` ("Antigravity completely ignores systemMessage and userMessage …"), contradicted by `8d7ad7b`.
5. **Volume isn't the problem yet; precision is.** Injected text totals only ~1.0–2.1K tokens per run (4–8.5K chars), negligible against 250–600K tokens per run. Overload is a scale risk, not a current one.
6. **No measured uptake.** In the pilot (c) run, the model's reasoning never referenced an injected entry. There's no direct evidence yet that the agent *uses* what it's given.

---

## 5. Proposed evaluation approach (after X6-lite)

### Layer 1: offline retrieval eval (cheap; no agent runs)
- **Queries:** reconstruct real retrieval moments from archived runs: task prompt at task start, file path at first touch, command and output at error. They should come from X6-lite runs (see §7 for the logging needed) and from real dogfood sessions.
- **Labels:** for each query, grade candidate entries 0/1/2 for relevance. Also flag *harmful* entries (stale, false, placeholder). Use human labels for a seed set, and an LLM judge (spot-checked) to scale.
- **Metrics:**
  - precision@k and nDCG@k at the actual injection cutoff;
  - **harm rate** (harmful entries injected per moment);
  - duplicate rate;
  - **no-good-answer rate** (moments where nothing relevant exists; the right action there is to inject nothing);
  - tokens injected.
- **Use:** a regression suite for the ranker. Every ranking change is scored offline in seconds before any agent run.

### Layer 2: scale test (cheap; no agent runs)
- Grow the index with realistic distractors (entries from other repos, or seeded from a large open-source repo) at 1×, 10× and 100×. Re-run the Layer 1 queries and plot precision@k and harm rate against index size.
- This directly tests the large-codebase concern without needing a large-codebase agent eval yet.

### Layer 3: end-to-end agent eval (expensive; only once Layer 1/2 look healthy)
- **Reuse the X6-lite v3 harness:** isolation, hidden tests with API contracts, interleaved paired design, pre-registered analysis.
- **Task mix:**
  - (a) the current new-package WOs;
  - (b) **history-dependent tasks** replayed from real git history with issue-style prompts. Examples: the PR-08 "address all N review findings" commits, and the `systemMessage → ephemeralMessage` fix (`8d7ad7b`);
  - (c) at least one **larger repository** with real history, time-cut and seeded the same way.
- **Add uptake measures:** whether injected entries are referenced in the agent's reasoning or actions, and pull-tool usage.

---

## 6. Candidate ranking improvements to test (offline first)

These are hypotheses to score in Layer 1, not commitments:
1. **Keep relevance in the final score.** Carry the bm25 (or similarity) score into `RankEntries`, and stop entry-type ordering from dominating.
2. **Better query construction.** Use the WO title, objective and key nouns rather than the full prompt. Build per-moment queries: path plus nearby symbols at first touch, normalised error text at error.
3. **Hybrid retrieval.** Lexical bm25 plus embeddings plus the anchor graph (exact path, directory, symbol), then a reranker.
4. **Relevance threshold.** Inject nothing when nothing clears the bar, rather than filling the budget.
5. **Session-level dedupe and caps.** Don't re-inject an entry already shown in the conversation (the ledger already has the data). Cap total injections per session.
6. **Entry validity.** Superseded and contradicted entries should be demoted or removed. Add a check for entries contradicted by later ops or commits (e.g. `seed-b1617c95`).
7. **Surface errors.** Log FTS query failures instead of swallowing them (`engine.go:118-130`).
8. **Pull channel.** Test whether a one-line hint at task start ("memory tools available: …") increases pull-tool use, and whether pulled entries are better targeted than pushed ones.

---

## 7. Do now, during the X6-lite re-run (small, enables Layer 1 later)

To avoid needing new agent runs for Layer 1, have the harness archive the retrieval inputs and outputs it currently throws away:
- For each delivery moment:
  - the **query** (prompt terms / path / error text),
  - the **candidate list with bm25 and final scores**,
  - the **selected entries** and **budget used**.

  A debug log from the hook (gated by an env var in the per-run `hooks.json` command) is enough.
- Archive each run's **agent diff** (already recommended in W6).
- Keep the per-run `index.db` (already archived for (c)).

Nothing else in this report should change X6-lite's design or delay its completion.

---

## 8. Sequencing

1. **Finish X6-lite:** API-contract prompts, test-set replacement fix, re-run Block 1, apply the pre-registered rule. Add the §2 scope statement to its verdict.
2. **Layer 1 harness,** built from the X6-lite archives and dogfood sessions; establish baseline ranker metrics.
3. **Ranker iterations** (§6), scored offline.
4. **Layer 2 scale test.**
5. **Layer 3 (X6-full):** mixed task types and a larger repo, if Layers 1–2 show precision holds up.

## 9. Open questions

- Which large repository? It needs real history, decent tests to serve as oracles, and ideally design docs or ADRs to seed from.
- What does a good injection look like? Agree a relevance rubric before labelling.
- Should the learning loop (captured sessions → entries) be in scope for X6-full, or kept separate?
- What precision target justifies push injection at all, versus pull-only with a hint?
