---
id: clm-01M3YZXY7QSH3TPN110SHX0VTV
type: pattern
title: Incremental transcript copying verifies existing prefix integrity up to the cursor offset and pipes newly appended bytes through an io.TeeReader to compute the updated prefix hash in a single pass.
relevant_when: Performing incremental file copies with prefix hash verification.
apply: Hash bytes up to state.ByteOffset first, then tee new appended bytes to the hasher during transfer to avoid a second pass over the file.
status: candidate
flags: []
anchors:
  - kind: file
    ref: internal/core/spool/copy.go
created: 2026-10-02T20:02:20.663243+01:00
updated: 2026-10-02T20:02:20.663243+01:00
---


