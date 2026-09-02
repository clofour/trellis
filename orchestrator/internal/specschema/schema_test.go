package specschema

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestGenerateDeterministic(t *testing.T) {
	apiA, yamlA, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	apiB, yamlB, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(apiA, apiB) || !bytes.Equal(yamlA, yamlB) {
		t.Fatal("schema generation is not deterministic")
	}
}

func TestYAMLSchemaOnlyAddsAuthoringRepresentation(t *testing.T) {
	apiRaw, yamlRaw, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	var api, yaml map[string]any
	if err := json.Unmarshal(apiRaw, &api); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(yamlRaw, &yaml); err != nil {
		t.Fatal(err)
	}

	apiDefs := api["$defs"].(map[string]any)
	yamlDefs := yaml["$defs"].(map[string]any)
	apiMemory := property(t, apiDefs, "ResourcesSpec", "memory")
	yamlMemory := property(t, yamlDefs, "ResourcesSpec", "memory")
	if apiMemory["type"] != "integer" {
		t.Fatalf("API memory schema = %#v", apiMemory)
	}
	if _, ok := yamlMemory["oneOf"]; !ok {
		t.Fatalf("YAML memory schema does not accept human quantities: %#v", yamlMemory)
	}

	apiMode := property(t, apiDefs, "TaskNetworkingSpec", "mode")
	yamlMode := property(t, yamlDefs, "TaskNetworkingSpec", "mode")
	if !contains(apiMode["enum"].([]any), "") || contains(yamlMode["enum"].([]any), "") {
		t.Fatalf("default network mode representation was not derived correctly: API=%#v YAML=%#v", apiMode, yamlMode)
	}
	if !contains(yamlMode["enum"].([]any), "namespace") {
		t.Fatalf("YAML network modes = %#v, want namespace", yamlMode["enum"])
	}
}

func property(t *testing.T, defs map[string]any, definition, name string) map[string]any {
	t.Helper()
	def, ok := defs[definition].(map[string]any)
	if !ok {
		t.Fatalf("definition %s missing", definition)
	}
	properties, ok := def["properties"].(map[string]any)
	if !ok {
		t.Fatalf("definition %s has no properties", definition)
	}
	property, ok := properties[name].(map[string]any)
	if !ok {
		t.Fatalf("property %s.%s missing", definition, name)
	}
	return property
}

func contains(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
