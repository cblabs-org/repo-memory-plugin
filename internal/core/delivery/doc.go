// Package delivery implements the v2 delivery engine: the injection ledger,
// ranking engine, budget enforcement, anchor verification, and exposure tracking.
// It is harness-agnostic; hook handlers in internal/harness/ call this engine.
package delivery
