/*
 * repo-memory-plugin (c) 2026 cblabs-org
 * https://github.com/cblabs-org/repo-memory-plugin
 * SPDX-License-Identifier: Apache-2.0
 */

package extractor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractor(t *testing.T) {
	tempDir := t.TempDir()

	// Setup some files
	os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(`
rules:
  topics: ["networking", "bluetooth"]
  tools: ["gcc", "clang"]
  errors: ["panic", "segfault"]
`), 0644)

	os.WriteFile(filepath.Join(tempDir, "sdkconfig"), []byte(`
CONFIG_WIFI_ENABLED=y
# Some comment
CONFIG_BT_ENABLED=n
`), 0644)

	os.WriteFile(filepath.Join(tempDir, "main.c"), []byte(`
int main(int argc, char** argv) {
	return 0;
}
void test_func() {}
`), 0644)

	os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(`
ignore_me.c
`), 0644)

	os.WriteFile(filepath.Join(tempDir, "ignore_me.c"), []byte(`
void ignored_func() {}
`), 0644)

	stagedPluginDir := filepath.Join(tempDir, ".agents", "plugins", "repo-memory")
	if err := os.MkdirAll(stagedPluginDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(stagedPluginDir, "staged_ignore.c"), []byte(`
int staged_plugin_func(void) {
	return 0;
}
`), 0644)

	realAgentsDir := filepath.Join(tempDir, "agents")
	if err := os.MkdirAll(realAgentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(realAgentsDir, "real_agent.c"), []byte(`
int real_agent_func(void) {
	return 0;
}
`), 0644)

	ext := New(tempDir)
	entities, err := ext.Run()
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	foundTopic := false
	foundConfig := false
	foundSymbol := false
	foundIgnored := false
	foundStagedPlugin := false
	foundRealAgent := false

	for _, e := range entities {
		if e.ID == "topic:networking" {
			foundTopic = true
		}
		if e.ID == "config:CONFIG_WIFI_ENABLED" {
			foundConfig = true
		}
		if e.ID == "symbol:main" {
			foundSymbol = true
		}
		if e.ID == "symbol:ignored_func" {
			foundIgnored = true
		}
		if e.ID == "symbol:staged_plugin_func" {
			foundStagedPlugin = true
		}
		if e.ID == "symbol:real_agent_func" {
			foundRealAgent = true
		}
	}

	if !foundTopic {
		t.Error("Expected to find topic:networking")
	}
	if !foundConfig {
		t.Error("Expected to find config:CONFIG_WIFI_ENABLED")
	}
	if !foundSymbol {
		t.Error("Expected to find symbol:main")
	}
	if foundIgnored {
		t.Error("Expected ignored_func to be ignored")
	}
	if foundStagedPlugin {
		t.Error("Expected staged_plugin_func under .agents/plugins to be ignored")
	}
	if !foundRealAgent {
		t.Error("Expected real_agent_func under agents/ to be extracted")
	}
}
