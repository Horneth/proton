package engine

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"buf-lib-poc/pkg/config"
)

func TestEngine_EndToEnd(t *testing.T) {
	imagePath := os.Getenv("PROTO_IMAGE")
	if imagePath == "" {
		t.Skip("PROTO_IMAGE not set, skipping engine integration tests")
	}

	cfg := &config.Config{
		Aliases: map[string]string{
			"TopologyTransaction": "com.digitalasset.canton.protocol.v30.TopologyTransaction",
		},
	}
	e := NewEngine(cfg)
	e.Loader.ImportPaths = []string{"../../"}
	ctx := context.Background()

	// 1. Test Template
	tmpl, err := e.Template(ctx, imagePath, "TopologyTransaction")
	if err != nil {
		t.Fatalf("Template() error = %v", err)
	}
	if tmpl == nil {
		t.Fatal("Template() returned nil")
	}

	// 2. Test Generate
	jsonData := []byte(`{"operation": "TOPOLOGY_CHANGE_OP_ADD_REPLACE", "serial": 42}`)
	version := int32(30)
	binaryData, err := e.Generate(ctx, imagePath, "TopologyTransaction", jsonData, &version)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 3. Test Decode
	decoded, err := e.Decode(ctx, imagePath, "TopologyTransaction", binaryData, true)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedMap := decoded.(map[string]interface{})
	if decodedMap["operation"] != "TOPOLOGY_CHANGE_OP_ADD_REPLACE" {
		t.Errorf("expected operation ADD_REPLACE, got %v", decodedMap["operation"])
	}
	if decodedMap["serial"].(float64) != 42 {
		t.Errorf("expected serial 42, got %v", decodedMap["serial"])
	}
}

func TestEngine_NestedRecursion(t *testing.T) {
	imagePath := os.Getenv("PROTO_IMAGE")
	if imagePath == "" {
		t.Skip("PROTO_IMAGE not set")
	}

	// Configure mapping for recurisve expansion/compression
	// Note: In real usage, these come from the config file.
	cfg := &config.Config{
		Aliases: map[string]string{
			"TopologyTransaction": "com.digitalasset.canton.protocol.v30.TopologyTransaction",
		},
		Mappings: []config.Mapping{
			{
				Type:       "com.digitalasset.canton.protocol.v30.TopologyTransaction",
				Field:      "mapping",
				TargetType: "com.digitalasset.canton.protocol.v30.TopologyMapping",
			},
		},
	}
	e := NewEngine(cfg)
	e.Loader.ImportPaths = []string{"../../"}
	ctx := context.Background()

	// Build a JSON with nested mapping
	jsonData := map[string]interface{}{
		"operation": "TOPOLOGY_CHANGE_OP_ADD_REPLACE",
		"serial":    1,
		"mapping": map[string]interface{}{
			"namespaceDelegation": map[string]interface{}{
				"namespace": "MY_NS",
			},
		},
	}
	jsonBytes, _ := json.Marshal(jsonData)

	// Generate
	version := int32(30)
	binary, err := e.Generate(ctx, imagePath, "TopologyTransaction", jsonBytes, &version)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Decode
	decoded, err := e.Decode(ctx, imagePath, "TopologyTransaction", binary, true)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}

	decodedMap := decoded.(map[string]interface{})
	mapping := decodedMap["mapping"].(map[string]interface{})
	nsDel := mapping["namespaceDelegation"].(map[string]interface{})
	if nsDel["namespace"] != "MY_NS" {
		t.Errorf("expected MY_NS, got %v", nsDel["namespace"])
	}
}
