# core

## Invariants & contracts

### [docseed-14f65441] 5 — **Blobs** — Inline base64 images and binaries over 1 KiB, replaced by `«blob:image/png 23KiB sha256:ab12…»`
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-30433ad4] 4 — **Repo rules** — Label-to-regex rules, plus allow patterns, from `.memory/scrub.yaml`
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-4915d23b] 1 — **Structural** — Private-key blocks; credentialed URLs (`scheme://user:pass@host`); database connection strings; `Authorization:` headers; assignments to secret-like names (`*_TOKEN`, `*_SECRET`, `*_KEY`, `PASSWORD`, …); provider key formats with fixed prefixes. The provider list is our own table, maintained in the package.
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-4fc216d6] 3 — **PII** (opt-in) — Email addresses and phone numbers
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-6d01ea5d] op publish (the existing second scan, SPEC §11.3).
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-7d83b4ee] input prepared for the lean agent (v2 §5.6);
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

### [docseed-f9127884] 2 — **Entropy** — Unlabelled tokens of 24 or more characters over base64 or hex alphabets, above an entropy threshold. **Allowed through:** UUIDs and ULIDs in the known-ID allow-set (conversations, steps, notes, entries); hex strings of 7–40 characters that resolve as commit objects in this repository (`git cat-file -e`, batched); content from files matched by `scrub.skip_paths` (for exam...
`invariant · kind: semantic · anchors: internal/core/scrub · source_doc: docs/addendum-a-provenance-and-hardening.md · evidence: doc:docs/addendum-a-provenance-and-hardening.md`

## Decisions

### [seed-01e79d6d] Symlinks & Subdirectory Enrolment: Registered paths are indexed both as raw paths and evaluated symlink paths (e.g. `/var` vs `/private/var` on macOS). `FindRegistryEntry` also supports subdirectories within enrolled repositories so hooks invoked from subfolders match correctly.
`decision · kind: episodic · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`

### [seed-4367d3cc] Origin URL Canonicalization: Normalized SSH (`git@host:owner/repo.git`), schemes (`ssh://`, `git://`), trailing slashes, and `.git` suffixes to `https://host/owner/repo` so all clone protocols produce matching Repo IDs.
`decision · kind: episodic · anchors: internal/core/gitops/repoid.go, internal/core/gitops/repoid_test.go, internal/core/config/registry.go, internal/core/config/config.go, internal/core/config/registry_test.go, internal/harness/fastpath.go, internal/harness/fastpath_test.go, cmd/repo-memory/cmd_enable.go, cmd/repo-memory/cmd_disable.go, cmd/repo-memory/cmd_hook.go, cmd/repo-memory/main.go, cmd/repo-memory/enrolment_test.go`
