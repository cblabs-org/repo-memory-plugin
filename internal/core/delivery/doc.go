/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/doc.go
 * Description: Package documentation for the harness-agnostic v2 repository
 *              memory push-delivery and workorder brief engine.
 *
 * Exported Types / Functions:
 * - (None — package documentation file)
 */

// Package delivery implements the v2 delivery engine: the injection ledger,
// ranking engine, budget enforcement, anchor verification, and exposure tracking.
// It is harness-agnostic; hook handlers in internal/harness/ call this engine.
package delivery
