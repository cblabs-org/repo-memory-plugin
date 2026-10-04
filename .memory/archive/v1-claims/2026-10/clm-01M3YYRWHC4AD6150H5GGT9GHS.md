---
id: clm-01M3YYRWHC4AD6150H5GGT9GHS
type: decision
title: Transcript parser utilizes bufio.Scanner configured with a 50MB buffer over an io.MultiReader to reliably parse large chunks and handle JSON boundary straddling.
relevant_when: Parsing Antigravity transcript files into memory episodes.
apply: Configure bufio.Scanner with a 50MB max capacity on an io.MultiReader stream, and enforce byte continuity when streaming chunks.
status: accepted
flags: []
created: 2026-10-02T19:42:06.508562+01:00
updated: 2026-10-02T19:42:06.508562+01:00
---


