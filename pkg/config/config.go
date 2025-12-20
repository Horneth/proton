package config

import (
	"encoding/json"
	"os"
)

type Mapping struct {
	Type           string `json:"type"`            // Source message type
	Field          string `json:"field"`           // Field name of type bytes
	TargetType     string `json:"target_type"`     // Target message type to decode/encode
	Versioned      bool   `json:"versioned"`       // Whether it uses UntypedVersionedMessage
	DefaultVersion int32  `json:"default_version"` // Version to use for generate
}

type Config struct {
	Aliases  map[string]string `json:"aliases"`
	Mappings []Mapping         `json:"mappings"`
}

func (c *Config) ResolveAlias(name string) string {
	if c == nil || c.Aliases == nil {
		return name
	}
	if full, ok := c.Aliases[name]; ok {
		return full
	}
	return name
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
