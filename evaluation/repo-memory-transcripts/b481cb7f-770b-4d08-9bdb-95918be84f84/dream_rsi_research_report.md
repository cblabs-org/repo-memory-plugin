# Research Report: Dream-RSI (Recursive Self-Improvement through Evolving Worlds)

**Paper Details:**
- **Title:** *Dream-RSI: Recursive Self-Improvement through Evolving Worlds*
- **Authors:** Tong Zheng, Xidong Wu, Zheng Zhang, Zhankui He, Chaoyi Zhang, Benjamin Coleman, Ruoqiao Wei, Di Bai, Haolin Liu, Rui Liu, Xue Wang, Yue Zhuan, Wang-Cheng Kang, Renkai Xiang, Heng Huang, Xinwu Cheng, and Yunsong Guo
- **Affiliations:** Google, University of Maryland College Park, Google DeepMind, University of Virginia
- **Date/Identifier:** September 2026 / arXiv:2609.14858v1 [cs.CL]
- **Document Source:** `/Users/mcneillm/Documents/Projects/repo-memory-plugin/docs/2609.14858v1.pdf`

---

## 1. What is Dream-RSI? What Problem Does It Solve?

### The Core Problem: The Meta-Exploration Dilemma
In autonomous AI agents targeting open-ended, long-horizon scientific discovery, algorithm engineering, and complex programming tasks, the primary engine of progress is **exploration**. An exploration strategy must decide:
- Which initial hypotheses/roots to branch out into,
- Which intermediate candidate solutions to refine further,
- How many attempts to schedule in parallel,
- When a search direction is exhausted and should be pruned, and
- When to terminate the search altogether.

Existing approaches to agentic search face a **fundamental dilemma**:
1. **Fixed Exploration Strategies:** Most current discovery frameworks (e.g., AlphaEvolve, SimpleTES, CodeEvolve, ShinkaEvolve) rely on manually designed, static heuristics (e.g., standard parallel refining, fixed branching ratios, static genetic algorithms). These strategies cannot learn from experience; they repeat identical search patterns and allocate expensive compute to unproductive or saturated directions.
2. **Online Policy Optimization:** Attempting to optimize exploration strategies *online* during real-world discovery hits two severe bottlenecks:
   - **Delayed and Costly Meta-Feedback:** Evaluating an individual candidate solution provides immediate feedback (e.g., test pass/fail, benchmark runtime). However, evaluating an *exploration policy* requires observing its cumulative effect over dozens of proposal–evaluation cycles and long horizons.
   - **Vast Meta-Policy Search Space:** A candidate exploration policy may fail completely or starve promising leads. Testing thousands of meta-policy variations online would require tens of thousands of expensive LLM calls, long code compilation pipelines, and GPU benchmarks—making online meta-learning economically and computationally infeasible.

### The Dream-RSI Solution
**Dream-RSI** introduces a framework for **scalable, recursive self-improvement of exploration strategies**. It decouples the system into two clean layers:
1. **Underlying Coding/Discovery Agent & Evaluator:** Remains fixed (e.g., Gemini-3.1 Pro or Gemini-3.7 Flash) to generate and evaluate code solutions.
2. **Lightweight Orchestration Layer:** Formulates exploration as an explicit, programmable policy in Python.

**The Key Insight:** Accumulated discovery history is not merely passive text context or fine-tuning data; it inherently forms an **interactive, grounded replay simulator (an empirical world model)** over the portion of the search space that has already been explored. 

By running **offline "dreaming"** inside this replay simulator, Dream-RSI can evaluate thousands of candidate exploration strategies at **zero execution cost and with instantaneous feedback**, without invoking the underlying LLM coding agent or running real code evaluations. The verified best exploration policy is then redeployed online to drive the next round of real discovery, creating a closed recursive self-improvement loop.

```
+-----------------------------------------------------------------------------------+
|                            Dream-RSI Recursive Loop                               |
|                                                                                   |
|      +--------------------------------------------------------------------+       |
|      | 1. ONLINE EXPLORATION                                              |       |
|      |    Deploy current policy pi_t -> Guides fixed coding agent         |       |
|      |    Generates new discovery tree T_t with real executions/evals     |       |
|      +---------------------------------+----------------------------------+       |
|                                        | Logs execution traces                    |
|                                        v                                          |
|      +--------------------------------------------------------------------+       |
|      | 2. CONSTRUCT REPLAY SIMULATOR                                      |       |
|      |    Convert T_t into replay simulator; add to simulator pool:      |       |
|      |    H_t = H_{t-1} U {T_t} ("Evolving Worlds")                       |       |
|      +---------------------------------+----------------------------------+       |
|                                        | Replayable historical outcomes           |
|                                        v                                          |
|      +--------------------------------------------------------------------+       |
|      | 3. DREAMING-BASED POLICY IMPROVEMENT                               |       |
|      |    Policy-development agent "dreams" up candidate policies         |       |
|      |    Evaluates pi_t^0 ... pi_t^{M-1} across all replay worlds in H_t |       |
|      |    Zero-cost instant replay; selects pi_{t+1} with highest reward   |       |
|      +---------------------------------+----------------------------------+       |
|                                        | Deploy upgraded policy                   |
|                                        +-------------------> Loop back to 1       |
+-----------------------------------------------------------------------------------+
```

---

## 2. How the 'Dreaming' Process Works

### Conceptual Roots: Model-Based RL and World Models
Dream-RSI draws direct inspiration from Model-Based Reinforcement Learning (MBRL) and the **Dreamer** line of work (Ha & Schmidhuber, 2018; Hafner et al., 2019, 2020, 2023, 2025). In Dreamer, an agent learns a world model from environmental transitions and learns optimal behaviors by "imagining" or "dreaming" rollouts entirely inside the learned latent world model.

In long-horizon code and scientific discovery, the search space is vast and execution is expensive. Dream-RSI recognizes that the **realized tree of historical exploration traces** serves as an empirical, non-parametric world model.

### The Tripartite Execution Process
Dream-RSI operates across three continuous stages:

1. **Online Exploration:**
   - Policy $\pi_t$ controls the scheduling of parallel workers $W$.
   - In each decision round $k \le K_1$, the policy selects a batch of nodes $C_k^t$ to explore or refine.
   - The coding agent generates code candidates for each chosen node, and the evaluator executes and scores them.
   - The final output is an expanded discovery tree $T_t$.
2. **Replay Simulator Construction:**
   - The tree $T_t$ records every attempt: workspace snapshots, code diffs, compiler errors, test results, and objective metrics $s_v$.
   - This tree is converted into an executable simulator and added to the pool: $\mathcal{H}_t = \mathcal{H}_{t-1} \cup \{T_t\}$.
3. **Dreaming-Based Policy Improvement:**
   - Offline, without making a single real LLM generation or code run, an LLM-based policy-development agent proposes alternative exploration policies.
   - Each policy "dreams" through the historical trees in $\mathcal{H}_t$, making branching, batching, and stopping decisions.
   - The simulator simply reveals the pre-computed outcomes along chosen paths.
   - Thousands of policy variants can be evaluated in seconds.

### The Concept of "Evolving Worlds"
Why does the paper refer to **"Evolving Worlds"**?
- In classical RL, the world model is either stationary or parameterized by a static neural network.
- In Dream-RSI, the "world" is the empirical replay simulator pool $\mathcal{H}_t$. With every outer round $t$, a real online rollout explores unmapped territory, discovering new breakthroughs, encountering new edge-case bugs, and revealing deeper branches.
- Thus, the world **evolves non-stationarily** over time: $\mathcal{H}_0 \subset \mathcal{H}_1 \subset \mathcal{H}_2 \dots \subset \mathcal{H}_t$.
- Candidate policies are continuously evaluated against the **entire accumulated history of evolving worlds**, forcing policies to avoid overfitting to a single rollout and ensuring they generalize across both early-stage exploratory phases and late-stage exploitation regimes.

---

## 3. The Replay Mechanism: Technical Architecture

### Formal Definition of Discovery Trees
- **Root ($r$):** Represents the initial repository or workspace state.
- **Node ($v$):** Represents a specific attempt (candidate solution).
  - Every non-root node $v$ has exactly one parent in the tree from which it inherited workspace state and context.
  - Each node preserves the generated code artifact, execution stdout/stderr, failure classification, and task score $s_v$.
- **Action Frontier ($A(T)$):** At any step, the set of legally selectable nodes is:
  $$A(T) = \{r\} \cup \{v \in T : v \text{ is a leaf}\}$$
  - Selecting $r$ opens a brand-new exploration branch.
  - Selecting a leaf $v$ refines an ongoing branch.
- **Batch Action:** Given $W$ parallel workers, the policy selects batch $C \subseteq A(T)$ with $|C| \le W$.

### The Deterministic Replay Step Transition
During offline replay of policy $\pi_m$ on historical tree $T_i$:
- The policy starts with an observed subtree $T_i^{m,0} = \{r\}$.
- At step $k$, it selects a legal batch $C_i^{m,k} \in A(T_i^{m,k}; W)$.
- The simulator reveals the unobserved children for each selected node:
  $$T_i^{m, k+1} = T_i^{m,k} \cup \bigcup_{v \in C_i^{m,k}} \text{Child}(v; T_i, T_i^{m,k})$$
  - If $v \ne r$: Replay returns the unique recorded child of $v$ in $T_i$ (if it exists).
  - If $v = r$: Replay returns the earliest-created child of $r$ that is not yet revealed in $T_i^{m,k}$ (opening the next historical branch).
  - If no child was recorded in the historical run, it returns $\emptyset$.
- Replay terminates when the policy chooses $C = \emptyset$, reaches round limit $K_2$, or fully reveals $T_i$.

### Strict Prefix-Only Observability (Preventing Data Leakage)
A critical technical safeguard in Dream-RSI is **prefix-only observability**:
- The policy $\pi$ only receives the revealed prefix observations $T_i^{m,k}$.
- It **never** has access to unrevealed nodes, future downstream scores, the maximum historical score of the tree, or ground-truth best cell IDs.
- Any decision (to prune, widen, deepen, batch, or stop) must be derived purely from historical and revealed prefix statistics:
  - `delta_vs_parent`, `delta_vs_baseline`, `fail_class`, `error`, `n_valid`, `n_total`.
- This ensures that a policy optimized in the simulator transfers directly to real online discovery without distribution shift or cheating.

### The Replay Objective Function
The replay score $V_i^m$ of policy $\pi_m$ on world $T_i$ balances three distinct objectives:

$$V_i^m = \underbrace{\max_{v \in T_i^{m, k_{i}^{m,*}}} s_v}_{\text{Discovery Quality}} - \underbrace{\beta_1 N_i^m}_{\text{Execution Cost}} + \underbrace{\beta_2 \frac{N_i^m}{\max\{1, k_i^{m,*}\}}}_{\text{Parallelism Bonus}}$$

Where:
- $N_i^m = |T_i^{m, k_{i}^{m,*}}| - 1$ is the total number of non-root nodes revealed (the number of generation-evaluation requests incurred).
- $k_i^{m,*}$ is the number of decision rounds taken.
- $\beta_1$ penalizes computational expenditure (fewer total probes).
- $\beta_2$ rewards high parallelism (favoring policies that pack full batches of size $W$ per round, minimizing wall-clock sequential latency).

In practical implementations (Appendix B.2), this is formalized as a **Pareto Reward**:
$$\text{pareto.reward} = \text{pareto.auc} - \lambda \times \text{parallel\_penalty}$$
where `parallel_penalty` is the mean of $(\text{effective\_sequential\_rounds} / \text{total\_probes})$.

---

## 4. The Policy Improvement Loop

### Exploration Policy Representation as Executable Python Code
Rather than parameterizing exploration as a black-box prompt or neural network, Dream-RSI represents the exploration policy as an **executable Python class**:
```python
class OptimalPolicy(LLMDesignedMethod):
    def __init__(self, config):
        self.beta = float(config.get("beta", 0.6))
        
    def plan_grid(self, context: GridPlanningContext) -> GridPlan:
        # Decides branch count (W) and refinement depth (R) for the upcoming live run
        ...
        return GridPlan(branch_count=W, refine_count=R, reason="...")

    def solve(self, question, budget=None) -> SimResult:
        # Prefix-only dynamic portfolio batch selection
        question.reset()
        while not _budget_done(question, budget):
            prefix = question.observed()
            batch = self.select_batch(prefix, question)
            if not batch:
                break
            question.probe_batch(batch)
        return finalize_result(question)
```

### The Policy Improvement Cycle
1. **Initial Baseline (Round $t=1$):** Starts with a hand-crafted parallel-refining policy $\pi_1$.
2. **Replay Feedback Collection:** After rollout $t$, historical traces are stored. Replay sweeps are executed across various $\beta$ hyperparameters (controlling exploration patience vs. exploitation selectivity).
3. **Meta-Optimization via Policy-Development Agent:**
   - An LLM (e.g. Gemini) acts as the policy-development agent.
   - Input to the agent:
     - The current Python implementation of `OptimalPolicy`.
     - Step-by-step replay diagnostic logs (`policy_execution_traces.jsonl`), highlighting where previous policies made suboptimal batching, over-pruned viable branches, or suffered premature starvation.
     - Beta sweep frontier curves (`beta_sweep.json`).
     - Live cycle manifests (`live_cycle_manifest.json`).
   - The agent writes an updated Python implementation $\pi_t^{m+1}$.
4. **Offline Evaluation Across $M$ Revisions:**
   - Over $M$ revision iterations, candidate policies $\pi_t^0, \dots, \pi_t^{M-1}$ are benchmarked against all historical replay worlds $i \in \{1, \dots, t\}$.
   - The mean score across all worlds is:
     $$V^m = \frac{1}{t} \sum_{i=1}^t V_i^m$$
5. **Monotonic Selection Guarantee:**
   - The next deployed policy is chosen via:
     $$\pi_{t+1} = \arg\max_{\pi_t^m} V^m$$
   - Since the existing policy $\pi_t$ is always included in the evaluation pool ($m=0$), $V^{m^*} \ge V^0$.
   - **This mathematically guarantees that the newly deployed policy is never worse than the incumbent policy on historical replays.**

### Key Policy Mechanisms Discovered by the Meta-Agent
Through dreaming, the meta-agent converged on sophisticated algorithmic patterns:
- **Dynamic Portfolio Batching:** Each parallel batch of $W$ workers is split across three distinct roles:
  1. *Exploitation:* Refinements on established high-performing anchors.
  2. *Exploration:* Probing unopened roots or underexplored diverse branches.
  3. *Recovery:* Allocating at most one slot to repair a promising direction that failed due to a transient bug.
- **Nuanced Failure Classification:** Distinguishing hard algorithmic dead ends from repairable errors (syntax slips, compilation errors, out-of-memory, tensor shape mismatches). One repairable error does not trigger branch closure.
- **Adaptive Grid Planning (`plan_grid`):** Dynamically adjusting exploration width ($W$) versus depth ($R$) depending on historical terrain difficulty.

---

## 5. Key Experimental Results and Domains

Dream-RSI was evaluated across **8 challenging scientific and systems discovery tasks** in 3 domains:

| Domain | Tasks Evaluated | Primary Metrics | Key Baselines |
| :--- | :--- | :--- | :--- |
| **Algorithm Engineering** | Complete Lasso Regularization Path Solver (17 synthetic search tasks, 6 held-out test datasets) | Wall-clock runtime (ms), numerical correctness check (tolerance $\le 10^{-6}$) | sklearn, glmnet, SimpleTES (GPT-OSS-120B), Recursive Fixed Exploration |
| **Mathematical Optimization** | 1. Sum-Difference Problem<br>2. Circle Packing ($n=26, 32$)<br>3. Autocorrelation Inequalities | Normalized set ratio $\Gamma(A)$, total packed radii $\sum r_i$, autoconvolution $\Phi(f)$ | AlphaEvolve, AlphaEvolveV2, OpenEvolve, CodeEvolve, ShinkaEvolve, TTS-Discovery, ThetaEvolve, EvoX, SimpleTES |
| **GPU Kernel Engineering** | KernelBench tasks: VGG16, LayerNorm, ConvDiv, ConvMax | Kernel throughput / inverse runtime ($1/\text{ms}$) | Recursive Fixed Exploration, reference PyTorch implementations |

### 1. Algorithm Engineering: Lasso Regularization Path
- **Downstream Generalization:** Solvers were evaluated on 6 held-out real-world datasets: Non-biological (*Gisette, RCV1*) and Biological (*DNA, Leukemia, Colon, Duke Breast*).
- **Runtime Performance:**
  - Standard libraries: `sklearn` averaged 44,180.3 ms; `glmnet` averaged 13,767.5 ms.
  - SimpleTES (with 51,200 agent generations): averaged 3,804.8 ms.
  - **Dream-RSI (Gemini-3.1 Pro):** achieved **2,931.0 ms** with only **317 discovery calls** (compared to 3,587.1 ms / 550 calls for Fixed Exploration, and 51,200 calls for SimpleTES — an efficiency gain of **up to 162×**).
  - **Dream-RSI (Gemini-3.7 Flash):** achieved **2,350.6 ms** with **1,879 calls** (compared to 2,516.7 ms / 3,200 calls for Fixed Exploration).
- **Algorithmic Innovations Discovered:** The solver generated by Dream-RSI introduced:
  - Adaptive active-set optimization with Cauchy-Schwarz KKT pruning,
  - Strong-rule screening,
  - Branch-free soft thresholding with `std::copysign`,
  - 64-byte aligned SIMD data structures and padding,
  - Lazy Gram-matrix computation.

### 2. Mathematical Optimization
- **Sum-Difference Problem:** Dream-RSI achieved **1.145427**, surpassing SimpleTES (1.143975) and AlphaEvolveV2 (1.121936).
- **Circle Packing:** Reached **2.635983**, matching the absolute theoretical best in the literature across all evolution systems.
- **Autocorrelation Inequalities:** Achieved **1.456375** in fewer than **1,000 generations**, matching SimpleTES (1.453675) which required **51,200 generations** (a **>50× compute reduction**).

### 3. GPU Kernel Engineering (KernelBench)
- **VGG16:** Reached target performance using **2.43× fewer generations**.
- **LayerNorm:** Reached target performance using **1.79× fewer generations**.
- **ConvDiv:** Achieved **2.09× higher performance** under identical discovery budget.
- **ConvMax:** Achieved **1.44× higher performance** under identical discovery budget.

### 4. Critical Empirical Analyses

#### A. The Pitfall of Prompt-Level Semantic Guidance (Section 5.1, Figure 5)
A common intuition in agent architecture is to summarize prior history into high-level textual advice or directional guidelines and inject them into the system prompt.
- The authors explicitly tested this: **Prompt Semantic Guidance vs. Unguided Replay-Based Exploration**.
- **Finding:** Explicit semantic prompt guidance **consistently underperformed** unguided exploration across all settings!
- **Reason:** Injecting strong textual biases regarding "where to search" over-constrains the LLM, causes premature convergence to local optima, and stifles the exploration diversity required for long-horizon breakthroughs. Organizing history as an *interactive simulator* rather than prescriptive advice was vastly superior.

#### B. Dynamic Compute Allocation Across Recursive Rounds (Section 5.2, Figure 6)
On ConvDiv, tracking exploration effort across recursive execution rounds revealed:
- When early performance was rising rapidly, the learned policy **conserved compute** (dropping attempts from 110 down to 50).
- When progress plateaued, the policy automatically **ramped up exploration effort** (increasing attempts back up to 92 and 91), escaping stagnation and triggering subsequent performance jumps.

---

## 6. Applying 'Dreaming' to a Repo-Level Memory System for Coding Agents

How can these findings revolutionize repository-level memory architectures (like `repo-memory-plugin`)?

### A. Offline "Dream" Sessions for Memory Consolidation
*Current Paradigm:* Memory plugins typically perform extraction and retrieval synchronously during interactive coding sessions. This incurs heavy token costs, adds latency, and produces noisy, fragmented memory entries.
*Dream-RSI Application:*
- **Decoupled Memory Consolidation:** Implement background "dreaming" sessions (triggered during idle time, overnight cron jobs, or post-PR git hooks).
- **Synthetic Re-indexing & Compression:** During a dream session, the memory system reviews recent session logs, resolves contradictory memory items, prunes stale/invalid patterns, merges duplicate architectural guidelines, and creates hierarchical abstraction summaries.
- **Counterfactual Testing:** Dream sessions can test whether existing memory entries would correctly guide an agent to solve known historical issues without hallucinating deprecated APIs.

### B. Adapting the Replay Mechanism to Coding Sessions
*Current Paradigm:* Past agent session traces (terminal commands, file edits, git diffs, linter/test errors) are treated as dead textual logs or discarded.
*Dream-RSI Application:*
- **Session Trees as Replay Simulators:** Every complex coding task (e.g. debugging an issue, refactoring a module) forms an execution tree:
  - Root: Initial repository state and task prompt.
  - Branches: Edit attempts, terminal commands run, compiler errors encountered, unit tests executed.
- **Zero-Cost Retrieval Evaluation:** We can replay these historical coding trees to test new retrieval policies:
  - *"If our memory retriever had injected snippet X (e.g. database migration rules) at step 3, would the agent have avoided the test failure at step 4?"*
  - Because the ground-truth outcomes of subsequent steps are already logged, we can benchmark alternative retrieval strategies without issuing expensive new LLM rollouts or re-running test suites.

### C. Policy Improvement for Memory Extraction and Retrieval
*Current Paradigm:* Retrieval algorithms (vector search top-k, BM25, hybrid weights, prompt stuffing) use hardcoded, static heuristics.
*Dream-RSI Application:*
- **Programmable Retrieval Policy:** Formulate repository memory retrieval as an explicit Python policy:
  ```python
  class RepoMemoryPolicy:
      def retrieve(self, task_context, revealed_history) -> list[MemoryItem]: ...
  ```
- **Multi-Objective Policy Optimization:** Evaluate the policy using a Dream-RSI style objective:
  $$\text{Reward} = \text{Task Success} - \alpha \cdot (\text{Context Tokens Injected}) + \beta \cdot (\text{Retrieval Precision})$$
- **Meta-Optimization Loop:** An offline policy-development agent uses replay traces over past coding sessions to iteratively improve the retrieval strategy:
  - Dynamically learning when to fetch architectural guidelines vs. concrete code examples.
  - Avoiding context bloat by pruning memories that cause distraction.
  - Adjusting retrieval breadth vs. depth based on repo structure.

### D. Analogy: Evolving Worlds vs. Evolving Codebases
| Dream-RSI Concept | Repo-Level Memory Analogy |
| :--- | :--- |
| **Discovery Tree $T_t$** | A developer/agent task session or PR branch (hypotheses, edits, test passes/fails). |
| **Simulator Pool $\mathcal{H}_t = \mathcal{H}_{t-1} \cup \{T_t\}$** | The repository's historical commit graph, closed issues, and CI/CD test runs. |
| **Evolving Worlds** | **The Evolving Codebase.** As a repository matures, its architecture, APIs, and conventions evolve. A memory system cannot assume a static world. |
| **Prefix-Only Observability** | **Temporal Causality in Memory.** When replaying or testing memory retrieval on past commits, the agent must *only* observe code and memories that existed *at or before that commit*, preventing data leakage and anachronistic recommendations. |
| **Failure Classification & Recovery** | Distinguishing between permanent architectural anti-patterns vs. simple syntax/typo mistakes in coding sessions, ensuring the memory system doesn't permanently ban viable architectural approaches due to a transient implementation bug. |
| **Anti-Prompt Guidance Finding** | **Warning against heavy prescriptive text in prompts.** Forcing rigid, high-level summaries into system prompts often degrades agent problem-solving. Instead, structured, context-grounded memory retrieval with dynamic exploration yields far better results. |

---

## 7. Strategic Recommendations for `repo-memory-plugin`

1. **Build a Structured Trace Logger:** Record coding interactions as branch trees (parent commit $\to$ edit action $\to$ compiler/linter feedback $\to$ score/test result) rather than flat markdown logs.
2. **Implement an Offline Memory Dreamer Daemon:** Create a background consolidation worker (using the `schedule` cron capability) that replays recent session trees, prunes stale memory entries, and tests memory relevance.
3. **Formulate Memory Retrieval as a Tunable Policy:** Replace rigid hardcoded top-$k$ RAG with an explicit retrieval policy that balances precision, context token budget, and diverse source coverage.
4. **Enforce Temporal Prefix-Safety:** Ensure memory indexing respects git timestamps, preventing future code knowledge from corrupting historical benchmarks or historical task replays.
