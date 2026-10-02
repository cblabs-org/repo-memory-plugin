package extractor

import (
	"os"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/entities"
	"gopkg.in/yaml.v3"
)

type ConfigRules struct {
	Rules struct {
		Topics []string `yaml:"topics"`
		Tools  []string `yaml:"tools"`
		Errors []string `yaml:"errors"`
	} `yaml:"rules"`
}

func ExtractFromRules(configPath string) ([]entities.Entity, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Return empty if no config.yaml
		}
		return nil, err
	}

	var cfg ConfigRules
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

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
