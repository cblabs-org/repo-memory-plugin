/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package core_test

import (
	"testing"

	"github.com/cblabs-org/repo-memory-plugin/internal/harness/fake"
)

func TestCoreWithFakeHarness(t *testing.T) {
	h := fake.New()
	if h.Name() != "fake" {
		t.Fatalf("expected harness name %q, got %q", "fake", h.Name())
	}
}
