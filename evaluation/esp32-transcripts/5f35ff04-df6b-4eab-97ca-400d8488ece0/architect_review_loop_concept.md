# Concept & Design: Principal Architect Review Loop

## 1. Executive Summary

The `bug-fixer-loop` plugin currently automates triaging, fixing, testing, and opening PRs. While this successfully offloads routine work, an analysis of the generated PRs reveals that the AI "junior developer" sometimes accrues technical debt or makes functionally correct but structurally suboptimal choices. 

This paper proposes implementing a **Principal Architect Review Loop**—an automated adversarial code review step executed by a high-capability model (e.g., Gemini 3.1 Pro High / Opus-class) before a human ever sees the PR. If the Architect detects technical debt, it kicks the PR back to the Developer agent for iteration, ensuring only high-quality, architecturally sound code is presented to human maintainers.

---

## 2. Observations from Open PRs (esp32-app-foundation)

A review of the recently opened `[BugFixer]` PRs on `matthewmcneill/esp32-app-foundation` (e.g., PR #268: Removing `std::function`) highlights exactly the problem this loop solves:

*   **Correctness vs. Debt**: The developer agent successfully refactored `std::function` to function pointers (`void* ctx`), and all 30 unit tests passed. 
*   **The Architectural Miss**: The Principal Architect review caught that while `std::vector::push_back` was used correctly in this narrow context, it creates a risk of dangling pointers if reallocation occurs. It suggested adding a defensive `reserve()` before the loop. It also flagged the lifetime of the `getProviderId()` string pointer.
*   **Conclusion**: The initial agent solves the *local* problem to pass tests, but lacks the *global* skepticism required to prevent subtle architectural debt. 

---

## 3. Industry Best Practices for AI Code Review

Research into multi-agent systems and LLM-driven software engineering reveals several established patterns for this workflow:

### The "Adversarial Review" Pattern
Having the same agent that wrote the code review its own PR suffers from **Context Collapse** and **Leniency Bias**. The agent is anchored to its original reasoning and will typically "rubber-stamp" its own work. Industry best practice is an **Adversarial Review (AR)**:
1.  A fresh, independent Architect agent is instantiated.
2.  Its explicit system prompt defines its role as a "skeptic" whose goal is to find edge cases, structural flaws, and security risks.
3.  Because it does not share the internal chain-of-thought of the builder, it evaluates the code purely on its objective merits.

---

## 4. Re-igniting the Conversation (Stateless Context Management)

When the Architect rejects a PR, the Developer agent must fix it. When the Developer fixes it, the Architect must re-review it. To prevent rebuilding context from scratch, both agents must "remember" their previous sessions. 

The most robust, **industry-standard best practice** is to store the Antigravity `conversation_id`s directly on the GitHub PR using **hidden HTML comments**. 

### The PR Metadata Pattern
By injecting hidden state into the PR body, the automation loop becomes entirely stateless and resilient to local daemon restarts.

**Example PR Body:**
```markdown
## Summary
Removed `std::function` allocations to fix #254.

## Testing
- All 30 unit tests pass.

<!-- agy_dev_conv_id: 5f35ff04-df6b-4eab-97ca-400d8488ece0 -->
<!-- agy_arch_conv_id: 9a82cd11-1a2b-3c4d-5e6f-7g8h9i0j1k2l -->
```

### Why Re-using Conversation IDs is Efficient
1. **Institutional Memory**: When the Developer agent is awoken via its ID, it has perfect recall of its internal chain-of-thought for *why* it wrote the code. When the Architect is awoken, it remembers exactly what it asked the Developer to fix.
2. **Context Caching**: Re-loading a massive transcript from disk sounds expensive, but **Gemini Context Caching** automatically caches the massive prefix of the conversation. Re-igniting the agent utilizes the heavily discounted cached-token rate (~75-80% cheaper) and drops time-to-first-token to near-zero. 

---

## 5. Proposed Architecture & Design

### The 4-Loop Workflow
1.  **Loop 1 (Triage)**: Identifies issue.
2.  **Loop 2 (Trajectory/Fix)**: Developer Agent (Gemini Flash) fixes the bug, passes tests, and opens a **Draft PR**.
    *   *Action*: Injects `<!-- agy_dev_conv_id: <uuid> -->` into the PR body.
3.  **Loop 3 (Architect Review)**: The `architect_dispatcher.py` sidecar wakes up and detects un-reviewed Draft PRs.
    *   It extracts the `agy_arch_conv_id` (if it exists) to re-ignite the Architect, or starts a new Architect agent if this is the first review.
    *   *Pass*: Architect approves, changes PR to "Ready for Review", adds `architect-approved` label.
    *   *Fail*: Architect comments on the PR, updates the PR body with its `agy_arch_conv_id`, and adds a `needs-iteration` label.
4.  **Loop 4 (Iteration)**: The `trajectory_dispatcher.py` sees the `needs-iteration` label. 
    *   It parses the PR body for the `agy_dev_conv_id` and re-ignites the Developer agent, passing in the new Architect comments.
    *   The Developer applies fixes, pushes to the branch, and removes the `needs-iteration` label. 
    *   GOTO Step 3.

### Failsafes
*   **Max Iterations**: Hardcode a maximum iteration limit (e.g., 3 loops). If the Developer and Architect cannot agree after 3 tries, the PR is flagged as `needs-human-intervention` to prevent an infinite token-burning loop.
