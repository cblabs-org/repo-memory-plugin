---
id: clm-01M3YYRWHGDKZ191MKF5EF6FGA
type: architecture_rule
title: The core package (internal/core) must remain completely independent of concrete adapters, interacting only via abstract interfaces and fake adapters during tests.
relevant_when: Defining domain models, parsers, and interfaces for repository memory pipelines.
apply: Ensure internal/core imports no concrete adapter packages; mock adapter behavior in tests using dedicated fake adapters.
status: accepted
flags: []
created: 2026-10-02T19:42:06.51203+01:00
updated: 2026-10-02T19:42:06.51203+01:00
---


