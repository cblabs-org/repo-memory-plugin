---
id: clm-01M3YZZ8GCF8G59MVMEZQHNSWK
type: decision
title: Releases are automated via .github/workflows/release.yml on v* tag pushes, building 5 cross-compiled target binaries (darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64.exe) using make build VERSION=${{ github.ref_name }} and uploading assets using softprops/action-gh-release@v2.
relevant_when: Publishing new version tags or updating binary release packaging.
apply: Build all 5 cross-compilation targets with the tag ref injected and publish artifacts via softprops/action-gh-release@v2.
status: active
flags: []
anchors:
  - kind: file
    ref: .github/workflows/release.yml
created: 2026-10-02T20:03:03.948419+01:00
updated: 2026-10-02T20:11:52.518847+01:00
---


