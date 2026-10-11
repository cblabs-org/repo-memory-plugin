# Release & Test: Architect Review Loop → esp32-app-foundation

## Prerequisites
- You are in an Antigravity session (so `ANTIGRAVITY_PROJECT_ID` is set)
- `gh` CLI is authenticated

---

## Phase 1: Merge & Push the Plugin

The architect review loop lives on a stacked branch in the plugin repo. Squash-merge it to `main`.

```bash
# In the plugin repo
cd ~/Documents/Projects/bug-fixer-loop-plugin

# Merge the tip of the stack (wo4) into main
git checkout main
git merge --squash pr-architect-review-loop/wo4-iteration-loop
git commit -m "feat: architect review loop — adversarial PR review with stateless context re-ignition

- Developer agent now opens Draft PRs with embedded conversation ID
- New architect-review skill (adversarial rubric: tech debt, pointer safety, memory, concurrency)
- New architect_dispatcher.py sidecar polls for unreviewed drafts
- tick_dispatcher.py handles needs-iteration with fallback to fresh agent
- Max 3 review rounds before needs-human-intervention
- Conversation IDs stored as hidden HTML comments in PR body"

# Push to GitHub
git push origin main

# Clean up local branches
git branch -D pr-architect-review-loop/wo1-developer-skill
git branch -D pr-architect-review-loop/wo2-architect-skill
git branch -D pr-architect-review-loop/wo3-architect-sidecar
git branch -D pr-architect-review-loop/wo4-iteration-loop
```

---

## Phase 2: Update the Submodule in esp32-app-foundation

The plugin is installed as a git submodule. Pull the new version.

```bash
cd ~/Documents/Projects/esp32-app-foundation

# Update the submodule to latest main
cd .agents/plugins/bug-fixer-loop-plugin
git checkout main
git pull origin main
cd ../../..

# Stage the submodule pointer update
git add .agents/plugins/bug-fixer-loop-plugin
git commit -m "chore: update bug-fixer-loop-plugin to v0.2.0 (architect review loop)"
git push origin main
```

---

## Phase 3: Re-run `install.sh` to Provision the New Sidecar

The `install.sh` script dynamically discovers all `sidecars/*/sidecar.json` files. Re-running it will pick up the new `bug-fixer-architect` sidecar and write its stub to `~/.gemini/config/sidecars/`.

```bash
cd ~/Documents/Projects/esp32-app-foundation

.agents/plugins/bug-fixer-loop-plugin/scripts/install.sh \
  --repo matthewmcneill/esp32-app-foundation \
  --repo-dir ~/Documents/Projects/esp32-app-foundation
```

> [!IMPORTANT]
> Run this from inside an active Antigravity session so `ANTIGRAVITY_PROJECT_ID` is injected into the sidecar stub.

Verify the new sidecar was created:
```bash
ls ~/.gemini/config/sidecars/ | grep architect
# Expected: bug-fixer-architect--esp32-app-foundation
```

---

## Phase 4: Create GitHub Labels

The new loop uses two new labels. Create them if they don't exist:

```bash
gh label create "needs-iteration" \
  --repo matthewmcneill/esp32-app-foundation \
  --color "FBCA04" \
  --description "Architect review requested changes; developer must iterate" \
  --force

gh label create "architect-approved" \
  --repo matthewmcneill/esp32-app-foundation \
  --color "0E8A16" \
  --description "Passed adversarial architect review; ready for human review" \
  --force

gh label create "needs-human-intervention" \
  --repo matthewmcneill/esp32-app-foundation \
  --color "D93F0B" \
  --description "Developer/Architect loop exhausted max iterations; human needed" \
  --force
```

---

## Phase 5: Test with the 3 Existing PRs

The 3 open PRs (#266, #267, #268) were created by the old plugin version (no draft, no conversation IDs). To test the new loop:

### Option A: Convert existing PRs to Draft (recommended)

This is the gentlest approach — the architect dispatcher will see them as new drafts to review.

```bash
# Convert all 3 to draft
gh pr ready --undo 268 --repo matthewmcneill/esp32-app-foundation
gh pr ready --undo 267 --repo matthewmcneill/esp32-app-foundation
gh pr ready --undo 266 --repo matthewmcneill/esp32-app-foundation
```

Because these PRs have no `agy_dev_conv_id` or `agy_arch_conv_id` in their bodies, the dispatchers will correctly:
1. **Architect dispatcher:** See the drafts, find no `agy_arch_conv_id`, spawn a **fresh** Architect agent.
2. **If rejected:** The Architect adds `needs-iteration` + its conversation ID.
3. **Tick dispatcher:** Sees `needs-iteration`, finds no `agy_dev_conv_id`, falls back to spawning a **fresh** Developer agent with the diff + comments.

### Option B: Run the architect dispatcher manually (fastest feedback)

Don't wait for the cron. Trigger it by hand:

```bash
cd ~/Documents/Projects/esp32-app-foundation

python3 .agents/plugins/bug-fixer-loop-plugin/scripts/architect_dispatcher.py
```

Watch the output — it should log:
```
ARCHITECT_DISPATCH: ... — Polling PRs in matthewmcneill/esp32-app-foundation
ARCHITECT_DISPATCH: ... — Processing PR #268: ... (review round 1/3)
ARCHITECT_DISPATCH: ... — Spawning new architect conversation for PR #268
```

---

## Phase 6: Observe & Validate

### What to watch for

| Step | What happens | Where to see it |
|------|-------------|-----------------|
| Architect spawns | New conversation appears in Antigravity sidebar | IDE sidebar |
| Architect approves | PR gets `architect-approved` label, draft status removed | GitHub PR page |
| Architect rejects | PR gets `needs-iteration` label + `[ARCHITECT]` comment + `agy_arch_conv_id` in body | GitHub PR page |
| Developer iterates | New conversation spawns (fresh, since old IDs don't exist) | IDE sidebar |
| Developer pushes fix | New commit on PR branch | GitHub PR page |
| Max iterations hit | PR gets `needs-human-intervention` label | GitHub PR page |

### Validation checklist

- [ ] Architect dispatcher runs without errors
- [ ] At least one PR receives an `[ARCHITECT]` review comment
- [ ] If rejected: `needs-iteration` label is applied and `agy_arch_conv_id` appears in PR body
- [ ] Tick dispatcher picks up `needs-iteration` PR and spawns a developer (fresh fallback)
- [ ] Developer pushes a fix commit to the PR branch
- [ ] On re-review: Architect correctly resumes its previous conversation (or falls back gracefully)
- [ ] After 3 rejections: `needs-human-intervention` label is applied and the PR is skipped
