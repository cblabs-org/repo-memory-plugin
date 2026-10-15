# prompts

## Decisions

### [seed-122809ff] Enforced P13 (Evidence independence) strictly after LLM outputs a `support` operation. The reconciler looks up the matching claim ID, verifies if the corresponding episode had an exposure, and if so, ensures an outcome touched the anchored file or the `enforced_by` command before allowing the support to persist.
`decision · kind: episodic · anchors: prompts/reconcile/prompt.md`

### [seed-7fa7cddc] P13 checks were hardened directly into `extractor.go` rather than purely relying on the LLM, rejecting any cited evidence that isn't `tool_outcome` or `human`.
`decision · kind: episodic · anchors: prompts/extract/prompt.md`

### [seed-dc1318b1] Used local shell `exec.Command` calls for `git branch --contains` and `git rev-parse` directly inside the Verifier since the `gitops` package was strictly out-of-scope for editing.
`decision · kind: episodic · anchors: prompts/verify/prompt.md`

### [seed-e96d91d7] Used heuristic token counting (`bytes / 4`) as standard across the project.
`decision · kind: episodic · anchors: prompts/extract/prompt.md`

## Open issues

### [seed-6c04bb11] The git querying for merged vs abandoned branches had to be duplicated within `verifier.go` because we lacked suitable methods in `gitops/branch.go` and modifying `gitops` was restricted. The Principal Architect might want to consider adding these utility methods into `gitops` for future reuse.
`open-issue · kind: semantic · anchors: prompts/verify/prompt.md`
