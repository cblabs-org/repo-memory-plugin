# core/pipeline/verify

## Decisions

### [seed-dc1318b1] Used local shell `exec.Command` calls for `git branch --contains` and `git rev-parse` directly inside the Verifier since the `gitops` package was strictly out-of-scope for editing.
`decision · anchors: internal/core/pipeline/verify/verifier.go, internal/core/pipeline/verify/verifier_test.go, prompts/verify/prompt.md`

## Open issues

### [seed-6c04bb11] The git querying for merged vs abandoned branches had to be duplicated within `verifier.go` because we lacked suitable methods in `gitops/branch.go` and modifying `gitops` was restricted. The Principal Architect might want to consider adding these utility methods into `gitops` for future reuse.
`open-issue · anchors: internal/core/pipeline/verify/verifier.go, internal/core/pipeline/verify/verifier_test.go, prompts/verify/prompt.md`
