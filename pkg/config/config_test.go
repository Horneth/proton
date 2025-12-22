package config

import (
	"os"
	"reflect"
	"testing"
)

func TestResolveAlias(t *testing.T) {
	cfg := &Config{
		Aliases: map[string]string{
			"User": "example.v1.User",
		},
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"existing alias", "User", "example.v1.User"},
		{"non-existing alias", "Post", "Post"},
		{"nil config", "User", "User"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c *Config
			if tt.name != "nil config" {
				c = cfg
			}
			if got := c.ResolveAlias(tt.input); got != tt.expected {
				t.Errorf("ResolveAlias() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	content := `{
		"aliases": {"U": "User"},
		"mappings": [{"type": "A", "field": "f", "target_type": "B"}]
	}`
	tmpfile, err := os.CreateTemp("", "config-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())
	if _, err := tmpfile.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := LoadConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	expected := &Config{
		Aliases: map[string]string{"U": "User"},
		Mappings: []Mapping{
			{Type: "A", Field: "f", TargetType: "B"},
		},
	}

	if !reflect.DeepEqual(cfg, expected) {
		t.Errorf("got %v, want %v", cfg, expected)
	}
}
