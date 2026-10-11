# PR-01b: Experiment Spike — Bridge Prompts

> PR-01 is ✅ complete and merged. PR-01b requires **your hands** for 3 interactive work orders.

## How This Works

WO1–WO3 are `interactive` complexity — they need you to observe real Antigravity behavior (probe hook output, sidecar logs, deny responses). Copy each bridge prompt below into a **new Antigravity session** and work through the checklist. When done, return here and I'll spawn WO4 (spec reconciliation) autonomously.

## Recommended Order

| WO | Experiments | Time Estimate |
|---|---|---|
| **WO1** — Transcript & Hooks | E12 (remaining), E3, E1 | ~30 min |
| **WO2** — Delivery Surfaces | E2, E4, E9, E8 | ~45 min |
| **WO3** — Runtime & Cost | E6, E5, E7, E10, E11 | ~60 min |

> [!TIP]
> You can run WO1 and WO2 in any order. WO3 is independent of both. All three can be done in parallel across separate sessions if you like.

---

## WO1 Bridge Prompt

Copy this into a new Antigravity session:

```
# pr-01b-experiment-spike / WO1 - transcript-and-hooks

You are an expert senior software engineer and execution-focused worker agent. **DO NOT trigger or act as the principal-architect.** I need you to completely execute Work Order 1.
Read your exact strict-instruction manual here via view_file: `.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/wo1-transcript-and-hooks/WORKORDER.md`

Execute the checklist. For each experiment, we'll work together interactively — I'll provide observations from the Antigravity UI and you'll record the findings. Write each experiment report to `docs/experiments/`. When all 3 experiments (E12 remaining, E3, E1) are documented, append your Completion Report to the WORKORDER.md file.
```

---

## WO2 Bridge Prompt

Copy this into a new Antigravity session:

```
# pr-01b-experiment-spike / WO2 - delivery-surfaces

You are an expert senior software engineer and execution-focused worker agent. **DO NOT trigger or act as the principal-architect.** I need you to completely execute Work Order 2.
Read your exact strict-instruction manual here via view_file: `.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/wo2-delivery-surfaces/WORKORDER.md`

Execute the checklist. For each experiment, we'll work together interactively — I'll provide observations from the Antigravity UI and you'll record the findings. Write each experiment report to `docs/experiments/`. When all 4 experiments (E2, E4, E9, E8) are documented, append your Completion Report to the WORKORDER.md file.
```

---

## WO3 Bridge Prompt

Copy this into a new Antigravity session:

```
# pr-01b-experiment-spike / WO3 - runtime-and-cost

You are an expert senior software engineer and execution-focused worker agent. **DO NOT trigger or act as the principal-architect.** I need you to completely execute Work Order 3.
Read your exact strict-instruction manual here via view_file: `.agents/skills/principal-architect-workspace/pr-01b-experiment-spike/wo3-runtime-and-cost/WORKORDER.md`

Execute the checklist. For each experiment, we'll work together interactively — I'll provide observations and measurements and you'll record the findings. Write each experiment report to `docs/experiments/`. When all 5 experiments (E6, E5, E7, E10, E11) are documented, append your Completion Report to the WORKORDER.md file.
```

---

## When You're Done

Come back to this session and say **"experiments complete"**. I'll then:
1. Spawn WO4 (spec reconciliation) autonomously to summarize findings and propose SPEC changes
2. Once you approve the SPEC changes, merge PR-01b and start PR-02a

> [!IMPORTANT]
> **Human task from the plan:** You also need to run E5 on a **Windows machine** if possible. If not available now, that's fine — it will be re-confirmed at PR-07.
