# core/pipeline/reconcile

## Decisions

### [seed-122809ff] Enforced P13 (Evidence independence) strictly after LLM outputs a `support` operation. The reconciler looks up the matching claim ID, verifies if the corresponding episode had an exposure, and if so, ensures an outcome touched the anchored file or the `enforced_by` command before allowing the support to persist.
`decision · anchors: internal/core/pipeline/reconcile/reconciler.go, internal/core/pipeline/reconcile/reconciler_test.go, prompts/reconcile/prompt.md`
