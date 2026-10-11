# B5 Evaluation: Pipeline Knowledge vs PA Knowledge

## Recall Test (Did the pipeline find what the PA knew?)

10 specific knowledge statements from PA completion reports, checked against 125 pipeline claims:

| # | PA Knowledge Statement | Pipeline Found? | Claim Quality |
|---|---|---|---|
| 1 | bufio.Scanner 50MB buffer for transcripts | ✅ Yes — 5 claims | **Excellent.** `clm-..RWHC`: "bufio.Scanner configured with a 50MB buffer over an io.MultiReader to reliably parse large chunks" |
| 2 | Pure-Go zstd compression with atomic writes | ✅ Yes — 3 claims | **Excellent.** `clm-..RWHH`: "klauspost/compress/zstd with atomic temporary-file-and-rename writes" |
| 3 | CommitLinker with pluggable GitRunner | ✅ Yes — 2 claims | **Excellent.** `clm-..RHKY`: "pluggable GitRunner interface defaulting to system Git execution" |
| 4 | PreToolUse `{}` = implicit deny | ⚠️ Partial — claim about probe responses, not the undocumented deny behaviour | **Missed the key insight.** Found probe mechanics, not the dangerous silent-deny discovery |
| 5 | Hook command path relative to hooks.json dir | ❌ No | **Missed entirely.** 0 claims mention hooks.json |
| 6 | Kill switch REPO_MEMORY_DISABLED=1 (~2.8ms) | ✅ Yes | **Good.** "kill switches (REPO_MEMORY_DISABLED=1)" with fast no-op |
| 7 | `agy -p` fixed overhead ~17K tokens | ❌ No | **Missed.** Has latency claim (6-7s) but not the token overhead |
| 8 | HOUSEKEEPING exclusion mechanism | ✅ Yes | **Excellent.** "isolation requires dedicated run directories and REPO_MEMORY_HOUSEKEEPING=1" |
| 9 | 5.4× compaction ratio | ✅ Yes | **Excellent.** "5.4x compaction ratio... satisfying the PR-02a exit criterion" |
| 10 | Subagents fire ALL hooks with own conversationId | ✅ Yes | **Excellent.** "Subagents trigger all Antigravity hooks with their own conversationId" |

### Recall Score: **8/10** (80%)
- 7 excellent matches, 1 partial, 2 missed
- The misses are notable: both are **discoveries about external system behaviour** (hooks.json path resolution, agy -p token overhead) — things the pipeline's compressed episodes don't contain enough context to extract

## Precision Test (Are the pipeline's claims accurate?)

10 randomly sampled claims from the 125:

| Claim | Accurate? | Notes |
|---|---|---|
| PreToolUse deny retries (3 advisory, 2 policy) | ✅ | Matches E09 experiment findings |
| bufio.Scanner 50MB buffer | ✅ | Matches implementation |
| Probe `--probe-decision` JSON response format | ✅ | Matches cmd_hook.go code |
| Cobra CLI modular cmd_\<name\>.go structure | ✅ | Matches project structure |
| `agy -p` ignores `--project` flag | ✅ | Confirmed by E7 experiment |
| Canary tests for transcript format bumps | ✅ | Good practice recommendation |
| transcript_full.jsonl is append-only | ✅ | Matches Antigravity behaviour |
| `agy -p` 6-7s startup latency | ✅ | Matches E5 measurement |
| Hook latency p50 ~57ms non-enrolled vs ~3ms enrolled | ✅ | Matches E5 data |
| Tool traffic = 78% of transcript footprint | ✅ | Matches E3 measurement |

### Precision Score: **10/10** (100%)
All sampled claims are factually accurate.

## Coverage: What the Pipeline Found That the PA Didn't Record

The pipeline extracted knowledge the PA never explicitly wrote down:

| Claim | Source |
|---|---|
| `agy -p` startup latency = 6-7s per call | Implicit in experiment data, never stated as a fact |
| Canary tests for transcript format changes | Best practice the worker mentioned but PA didn't surface |
| transcript_full.jsonl append-only behaviour | Implementation detail the PA didn't record |
| Hook latency p50: 57ms non-enrolled vs 3ms enrolled | Raw experiment data, PA recorded the 5ms target but not the p50 |
| Token estimation standardized at chars/4 | Convention used but never stated |

---

## The Verdict

### Quality: **B+ — The claims are accurate but miss key discoveries**

| Metric | Score |
|---|---|
| Precision | **100%** — every claim checked is factually accurate |
| Recall | **80%** — finds most PA knowledge, misses cross-system discoveries |
| Coverage | **Adds ~5-10 knowledge items** the PA never recorded |
| Depth | **Shallow** — atomic statements vs PA's rich narrative context |

### But at what cost?

| | PA | Pipeline | Winner |
|---|---|---|---|
| Precision | ~100% (human-curated) | 100% (LLM-extracted) | Tie |
| Recall of own sessions | ~100% | 80% | **PA** |
| Coverage of non-PA sessions | 0% | 100% | **Pipeline** |
| Knowledge density | ~4KB rich context per item | 537 bytes atomic per item | **PA** |
| Marginal token cost | ~2,400/session (in-session) | ~154,000/session (new calls) | **PA by 64×** |
| Machine-searchable | ❌ | ✅ | **Pipeline** |

### Bottom Line

> [!IMPORTANT]
> **The pipeline produces accurate but shallow knowledge at 64× the cost of the PA's in-session approach.** Its only unique value is capturing knowledge from non-PA sessions and making it machine-searchable. That value is real — but the cost model must change to justify it.
>
> **The conversation-injection approach would get the pipeline's coverage benefit at the PA's cost point.** The agent that did the work distils its own knowledge (high quality, full context, zero extra LLM calls), and the system just stores, deduplicates, and serves it.
