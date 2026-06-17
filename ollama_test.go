package harness

import (
	"encoding/json"
	"testing"
)

func chatBody(content string) []byte {
	body, _ := json.Marshal(map[string]any{
		"message": map[string]any{"role": "assistant", "content": content},
	})
	return body
}

func TestParseStepToolCallEmbeddedInProse(t *testing.T) {
	content := "Sure, let's write it:\n\n```json\n{\"name\":\"write_file\",\"arguments\":{\"path\":\"x.md\",\"content\":\"hi\"}}\n```\nDone."

	step, err := parseStep(chatBody(content))

	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if step.Tool != "write_file" {
		t.Fatalf("Tool = %q, want %q", step.Tool, "write_file")
	}
}

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

func TestParseStepToolCallInContent(t *testing.T) {
	// qwen2.5-coder emite a chamada como texto no content, sem tool_calls estruturado.
	body := []byte(`{"message":{"role":"assistant","content":"{\"name\":\"read_file\",\"arguments\":{\"path\":\"go.mod\"}}"}}`)

	step, err := parseStep(body)

	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if step.Done {
		t.Fatalf("should detect embedded tool call, got Done %+v", step)
	}
	if step.Tool != "read_file" {
		t.Fatalf("Tool = %q, want %q", step.Tool, "read_file")
	}
	if step.Input != `{"path":"go.mod"}` {
		t.Fatalf("Input = %q, want %q", step.Input, `{"path":"go.mod"}`)
	}
}

func TestParseStepToolCallInFencedContent(t *testing.T) {
	body := []byte("{\"message\":{\"role\":\"assistant\",\"content\":\"```json\\n{\\\"name\\\":\\\"list_dir\\\",\\\"arguments\\\":{\\\"path\\\":\\\".\\\"}}\\n```\"}}")

	step, err := parseStep(body)

	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if step.Tool != "list_dir" {
		t.Fatalf("Tool = %q, want %q", step.Tool, "list_dir")
	}
}

func TestParseStepPlainAnswerNotMistakenForToolCall(t *testing.T) {
	body := []byte(`{"message":{"role":"assistant","content":"The module is named harness."}}`)

	step, err := parseStep(body)

	if err != nil {
		t.Fatalf("parseStep: %v", err)
	}
	if !step.Done {
		t.Fatalf("plain prose must be a final answer, got %+v", step)
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
