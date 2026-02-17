package agent

import "testing"

func TestToLLMToolsIncludesRequiredFields(t *testing.T) {
	defs := []ToolDef{{
		Name:        "look",
		Description: "desc",
		Parameters: map[string]ParamDef{
			"direction": {Type: "string", Required: true, Enum: []string{"forward", "left"}},
		},
	}}

	out := ToLLMTools(defs)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
	if out[0].Name != "look" {
		t.Fatalf("name=%q want look", out[0].Name)
	}
	if out[0].Parameters["type"] != "object" {
		t.Fatalf("schema type=%v want object", out[0].Parameters["type"])
	}
	props, _ := out[0].Parameters["properties"].(map[string]any)
	if _, ok := props["direction"]; !ok {
		t.Fatalf("direction property missing: %+v", props)
	}
}
