/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/entities/extractor/rules.go
 * Description: Parses declarative YAML rule files (config.yaml) to extract
 *              configured topic, tool, and error entities.
 *
 * Exported Types / Functions:
 * - ConfigRules:        YAML schema for rule-defined entity lists.
 * - ExtractFromRules(): Reads a YAML config file and emits topic, tool, and error entities.
 */

package extractor

import (
	"os"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
	"gopkg.in/yaml.v3"
)

// ConfigRules represents the top-level YAML structure for declarative entity extraction rules.
type ConfigRules struct {
	// Rules groups the lists of static entity names by category.
	Rules struct {
		Topics []string `yaml:"topics"` // Domain topic names to register as KindTopic entities.
		Tools  []string `yaml:"tools"`  // Tool names to register as KindTool entities.
		Errors []string `yaml:"errors"` // Error signatures to register as KindError entities.
	} `yaml:"rules"`
}

// ExtractFromRules reads the YAML configuration at configPath and converts declared
// topics, tools, and errors into entities. It returns nil, nil if configPath does not exist.
func ExtractFromRules(configPath string) ([]entities.Entity, error) {
	// --- Step 1: Read configuration file if present ---
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return empty if no config.yaml.
		}
		return nil, err
	}

	// --- Step 2: Unmarshal YAML rules payload ---
	var cfg ConfigRules
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// --- Step 3: Project declared topics, tools, and errors into Entity records ---
	var extracted []entities.Entity
	for _, topic := range cfg.Rules.Topics {
		extracted = append(extracted, entities.Entity{
			ID:   "topic:" + topic,
			Kind: entities.KindTopic,
			Name: topic,
		})
	}
	for _, tool := range cfg.Rules.Tools {
		extracted = append(extracted, entities.Entity{
			ID:   "tool:" + tool,
			Kind: entities.KindTool,
			Name: tool,
		})
	}
	for _, errItem := range cfg.Rules.Errors {
		extracted = append(extracted, entities.Entity{
			ID:   "error:" + errItem,
			Kind: entities.KindError,
			Name: errItem,
		})
	}
	return extracted, nil
}
