package harness

import "testing"

func TestParseStepPlainAnswer(t *testing.T) {
	body := []byte(`{"message":{"role":"assistant","content":"4"}}`)

	step, err := parseStep(body)

	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if !step.Done || step.Text != "4" {
		t.Fatalf("got %+v, want Done with Text %q", step, "4")
	}
}

func TestOllamaToolDefFromTool(t *testing.T) {
	tool := Tool{
		Name:        "read_file",
		Description: "Read a file",
		Schema:      map[string]any{"type": "object"},
	}

	def := ollamaToolDef(tool)

	fn := def["function"].(map[string]any)
	if fn["name"] != "read_file" {
		t.Fatalf("name = %v, want %q", fn["name"], "read_file")
	}
	if fn["description"] != "Read a file" {
		t.Fatalf("description = %v, want %q", fn["description"], "Read a file")
	}
}

func TestParseStepToolCall(t *testing.T) {
	body := []byte(`{"message":{"role":"assistant","tool_calls":[{"function":{"name":"add","arguments":{"a":2,"b":2}}}]}}`)

	step, err := parseStep(body)

	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if step.Done {
		t.Fatalf("a tool call must not be Done, got %+v", step)
	}
	if step.Tool != "add" {
		t.Fatalf("Tool = %q, want %q", step.Tool, "add")
	}
	if step.Input != `{"a":2,"b":2}` {
		t.Fatalf("Input = %q, want %q", step.Input, `{"a":2,"b":2}`)
	}
}
