# core/pipeline

## Ruled out, failure signatures and gotchas

### [seed-d900dbd5] N/A
`ruled-out · anchors: internal/core/pipeline/verify/verifier.go, internal/core/pipeline/verify/verifier_test.go, fake.NewFakeHousekeeper(), prompts/verify/prompt.md`

## Decisions

### [seed-122809ff] Enforced P13 (Evidence independence) strictly after LLM outputs a `support` operation. The reconciler looks up the matching claim ID, verifies if the corresponding episode had an exposure, and if so, ensures an outcome touched the anchored file or the `enforced_by` command before allowing the support to persist.
`decision · anchors: internal/core/pipeline/reconcile/reconciler.go, internal/core/pipeline/reconcile/reconciler_test.go, prompts/reconcile/prompt.md`

### [seed-dc1318b1] Used local shell `exec.Command` calls for `git branch --contains` and `git rev-parse` directly inside the Verifier since the `gitops` package was strictly out-of-scope for editing.
`decision · anchors: internal/core/pipeline/verify/verifier.go, internal/core/pipeline/verify/verifier_test.go, fake.NewFakeHousekeeper(), prompts/verify/prompt.md`

## Open issues

### [seed-6c04bb11] The git querying for merged vs abandoned branches had to be duplicated within `verifier.go` because we lacked suitable methods in `gitops/branch.go` and modifying `gitops` was restricted. The Principal Architect might want to consider adding these utility methods into `gitops` for future reuse.
`open-issue · anchors: internal/core/pipeline/verify/verifier.go, internal/core/pipeline/verify/verifier_test.go, fake.NewFakeHousekeeper(), prompts/verify/prompt.md`
