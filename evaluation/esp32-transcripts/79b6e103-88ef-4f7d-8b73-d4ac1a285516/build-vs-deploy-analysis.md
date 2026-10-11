# Build vs Deploy: Critical Analysis

## Verdict: Your observation is correct — partially

The docs and scripts **do conflate build and deploy for beta/stable**, but the
conflation is in the **documentation and prescribed workflow**, not in the
script itself. The `deploy_release.py` script already supports separation — it's
the surrounding guidance that pushes a coupled workflow.

---

## Where the Conflation Lives

### 1. Versioning Policy Decision Tree — the smoking gun

[versioning-policy.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/docs/developer/versioning-policy.md#L193-L201)

The decision tree prescribes an **atomic tag-then-deploy** sequence for both
beta and stable:

```
User says "cut a beta" →
  → Confirm PATCH number …
  → Confirm PRE_NUM …
  → Tag and push, then run deploy_release.py (it auto-discovers the tag).

User says "promote to stable" →
  → Confirm the tag matches the last beta …
  → Run deploy_release.py (it auto-discovers the stable tag from HEAD).
```

There is **no step for "flash, test, and validate on hardware"** between tagging
and deploying. The instructions say: tag → immediately deploy. There's no
"tag → build → flash → validate → then decide whether to deploy" workflow.

### 2. Stable Tag How-To — build + deploy in one command

[versioning-policy.md L125-128](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/docs/developer/versioning-policy.md#L125-L128)

```bash
git tag v0.1.4 && git push origin v0.1.4
python3 scripts/deploy_release.py --local --build --private-key private_key.pem
```

The `--build` flag bundles the PlatformIO compilation *into* the deploy script
invocation. There's no pause between "build succeeded" and "now sign + publish to
GitHub + update OTA manifest". Once you run this command, the build is
immediately deployed.

### 3. Build System Reference — reinforces the coupling

[build-system.md L96-103](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/skills/framework-expert/references/build-system.md#L94-L103)

```
`deploy_release.py` handles build → sign → deploy
```

This framing presents build-through-deploy as a single pipeline step.

### 4. Beta Rules — "ARE published via deploy_release.py"

[versioning-policy.md L110](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/docs/developer/versioning-policy.md#L110)

> Beta builds ARE published as GitHub pre-releases via `deploy_release.py`.

The word "ARE" reads as prescriptive — as if publishing is an inherent property of
cutting a beta, not a separate decision.

### 5. Release Audit — happens *before* beta, not between build and deploy

[release-audit.md](file:///Users/mcneillm/Documents/Projects/esp32-app-foundation/.agents/skills/framework-expert/guides/release-audit.md#L27-L36)

| Release type | Run audit? |
|---|---|
| `vX.Y.Z-beta.N` | **Yes** |
| `vX.Y.Z` (stable) | **Yes** |

The audit is framed as a gate before tagging, not between build and deploy.
This makes sense for code quality, but it means the **hardware validation** (flash
+ test on real device) has no explicit home in the workflow. You tag, you audit,
you deploy. There's no "tag → build → flash → soak test → *then* deploy" path.

---

## What Already Supports Separation (but isn't surfaced)

The `deploy_release.py` script itself is **already cleanly separable**. The
tooling isn't the problem — the docs are.

### `--local` (without `--build`)

```bash
python3 scripts/deploy_release.py --tag v0.1.4 --local --private-key private_key.pem
```

Uses already-built binaries from `.pio/build/`. This means you can:
1. Build via PlatformIO MCP (`build_project`)
2. Flash to device (`upload_firmware`)
3. Test on hardware
4. **Later**, run `deploy_release.py --local` to sign + publish the same binaries

### `--sign-only`

```bash
python3 scripts/deploy_release.py --tag v0.1.4 --local --sign-only --private-key private_key.pem
```

Stages, signs, and hashes the firmware but **skips** GitHub release creation and
manifest update entirely. This is a pure "prepare release artifacts" mode.

### CI Mode (`--run-id`)

Downloads binaries from a GitHub Actions run — inherently decoupled from any
local flash/test cycle.

---

## What's Missing: The Explicit Two-Phase Workflow

The docs should describe this reality:

```
Phase 1: BUILD + VALIDATE (can happen many times)
  1. Tag (dev or beta/stable)
  2. Build (pio run / build_project MCP)
  3. Flash to device (upload_firmware MCP)
  4. Test on hardware (soak test, portal check, /api/system version verify)
  5. If bad → fix, re-tag, go to step 2
  6. If good → proceed to Phase 2

Phase 2: DEPLOY (happens once, when satisfied)
  7. Run deploy_release.py --local --private-key ... (signs existing binaries)
  8. Script creates GitHub Release, uploads assets, updates OTA manifest
  9. Devices on that channel can now OTA-pull the release
```

### What this buys you:
- **No premature publication.** A beta tag doesn't immediately mean every device on
  the beta channel gets this build. You flash it, test it, and *then* decide to
  publish.
- **Cheaper iteration.** You can build and flash multiple times against the same
  tag without polluting GitHub Releases with builds you haven't validated.
- **The OTA channel is your deployment target, not your test bench.** Devices on
  `beta` get builds you've *verified*, not builds you're still testing.

---

## Specific Changes Needed

### 1. Versioning Policy Decision Tree

Current:
> → Tag and push, then run deploy_release.py

Should become:
> → Tag and push, then build, flash to hardware, and validate.
> → When satisfied, run `deploy_release.py --local` to sign and publish.

### 2. Stable How-To Example

Current:
```bash
git tag v0.1.4 && git push origin v0.1.4
python3 scripts/deploy_release.py --local --build --private-key private_key.pem
```

Should become:
```bash
# Phase 1: Tag + Build + Validate
git tag v0.1.4 && git push origin v0.1.4
pio run -e esp32dev -t clean && pio run -e esp32dev
pio run -e esp32dev --target upload
# → Test on hardware, verify /api/system shows correct version
# → Repeat for all board targets

# Phase 2: Deploy (when validated)
python3 scripts/deploy_release.py --local --private-key private_key.pem
```

### 3. Beta Rules Text

Current:
> Beta builds ARE published as GitHub pre-releases via `deploy_release.py`.

Should become:
> Beta builds are published as GitHub pre-releases via `deploy_release.py` **after
> hardware validation**. Tagging a beta does not automatically deploy it — build,
> flash, and validate on hardware first, then deploy when satisfied.

### 4. Build System Reference

Current:
> `deploy_release.py` handles build → sign → deploy

Should become:
> `deploy_release.py` handles sign → deploy (or optionally build → sign → deploy
> with `--build`). The recommended workflow is to build and validate separately,
> then deploy the validated binaries with `--local`.

### 5. Release Audit Guide — Add Hardware Validation Dimension

The release audit protocol should acknowledge that hardware validation is a
separate, post-build step that happens between building and deploying, not just
as a dimension of the audit. Consider:

> **Between build and deploy:** Flash the built firmware to representative hardware
> and validate boot, version string, portal function, and basic sensor/actuator
> operation before running `deploy_release.py`.

### 6. `03-versioning.md` (Agent Rule) — Clarify Agent Guidance

The agent rule says:
> `Run deploy_release.py (any) | ❌ No* | Yes (User runs CLI)`

This is correct (agents can't run it), but the rule should also note that the
user's deploy is a **separate decision** from the build+flash, not a mandatory
next step:

> Deploy is a separate decision from build+flash. The user builds, flashes, and
> validates on hardware. If satisfied, the user then runs `deploy_release.py`
> to sign and publish. The agent should not assume that a successful build+flash
> means the user wants to deploy.

---

## The `--build` Flag: Keep or Deprecate?

The `--build` flag on `deploy_release.py` is the **primary enabler** of the
coupled workflow. It's convenient for the case where you've *already* validated
and just want to do everything in one shot, but it encourages skipping hardware
validation.

**Recommendation:** Keep `--build` but downplay it. The docs should present
`--local` (without `--build`) as the primary path, and `--build` as a convenience
shortcut for when you've already validated and want a clean rebuild for
deployment. The `--sign-only` flag should also be more prominently documented as
the "prepare but don't publish" option.

---

## Summary

| Aspect | Current State | What Should Change |
|---|---|---|
| `deploy_release.py` script | ✅ Already supports separation (`--local`, `--sign-only`) | No code changes needed |
| Versioning policy decision tree | ❌ Tag → immediately deploy | Add flash+validate step between |
| Stable how-to example | ❌ `--build` bundles everything | Show two-phase workflow |
| Beta rules text | ❌ "ARE published" implies automatic | Add "after hardware validation" |
| Build system reference | ❌ "build → sign → deploy" as one step | Separate build from sign+deploy |
| Release audit protocol | ⚠️ No explicit hardware validation gate | Add post-build, pre-deploy validation |
| Agent versioning rule | ⚠️ Correct but could be clearer | Note deploy ≠ automatic next step |

> [!IMPORTANT]
> The tooling is already right. The documentation prescribes a workflow that's
> more coupled than the tools require. This is a **docs-only fix** — no script
> changes needed.
