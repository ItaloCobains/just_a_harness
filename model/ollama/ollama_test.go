package ollama

import (
	"encoding/json"
	"strings"
	"testing"

	"harness/agent"
)

func chatBody(content string) string {
	body, _ := json.Marshal(map[string]any{
		"message": map[string]any{"role": "assistant", "content": content},
		"done":    true,
	})
	return string(body)
}

func parse(t *testing.T, ndjson string) agent.Step {
	t.Helper()
	step, err := parseStream(strings.NewReader(ndjson), nil)
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	return step
}

func firstCall(t *testing.T, step agent.Step) agent.ToolCall {
	t.Helper()
	if len(step.ToolCalls) == 0 {
		t.Fatalf("expected a tool call, got %+v", step)
	}
	return step.ToolCalls[0]
}

func TestParseStepToolCallEmbeddedInProse(t *testing.T) {
	content := "Sure, let's write it:\n\n```json\n{\"name\":\"write_file\",\"arguments\":{\"path\":\"x.md\",\"content\":\"hi\"}}\n```\nDone."

	call := firstCall(t, parse(t, chatBody(content)))

	if call.Name != "write_file" {
		t.Fatalf("Name = %q, want %q", call.Name, "write_file")
	}
}

func TestParseStepPlainAnswer(t *testing.T) {
	step := parse(t, `{"message":{"role":"assistant","content":"4"},"done":true}`)

	if !step.Done || step.Text != "4" {
		t.Fatalf("got %+v, want Done with Text %q", step, "4")
	}
}

func TestParseStepToolCallInContent(t *testing.T) {
	// qwen2.5-coder emite a chamada como texto no content, sem tool_calls estruturado.
	step := parse(t, `{"message":{"role":"assistant","content":"{\"name\":\"read_file\",\"arguments\":{\"path\":\"go.mod\"}}"},"done":true}`)

	call := firstCall(t, step)
	if call.Name != "read_file" {
		t.Fatalf("Name = %q, want %q", call.Name, "read_file")
	}
	if call.Input != `{"path":"go.mod"}` {
		t.Fatalf("Input = %q, want %q", call.Input, `{"path":"go.mod"}`)
	}
}

func TestParseStepToolCallInFencedContent(t *testing.T) {
	body := "{\"message\":{\"role\":\"assistant\",\"content\":\"```json\\n{\\\"name\\\":\\\"list_dir\\\",\\\"arguments\\\":{\\\"path\\\":\\\".\\\"}}\\n```\"},\"done\":true}"

	call := firstCall(t, parse(t, body))
	if call.Name != "list_dir" {
		t.Fatalf("Name = %q, want %q", call.Name, "list_dir")
	}
}

func TestParseStepToolCallInBashFence(t *testing.T) {
	// qwen2.5-coder narra passos com fences ```bash em vez de emitir só JSON.
	body := "{\"message\":{\"role\":\"assistant\",\"content\":\"Vamos começar:\\n```bash\\n{\\\"name\\\":\\\"run_bash\\\",\\\"arguments\\\":{\\\"cmd\\\":\\\"gem install rails\\\"}}\\n```\"},\"done\":true}"

	call := firstCall(t, parse(t, body))
	if call.Name != "run_bash" {
		t.Fatalf("Name = %q, want %q", call.Name, "run_bash")
	}
	if call.Input != `{"cmd":"gem install rails"}` {
		t.Fatalf("Input = %q", call.Input)
	}
}

func TestParseStepPlainAnswerNotMistakenForToolCall(t *testing.T) {
	step := parse(t, `{"message":{"role":"assistant","content":"The module is named harness."},"done":true}`)

	if !step.Done {
		t.Fatalf("plain prose must be a final answer, got %+v", step)
	}
}

func TestOllamaToolDefFromTool(t *testing.T) {
	tool := agent.Tool{
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
	step := parse(t, `{"message":{"role":"assistant","tool_calls":[{"function":{"name":"add","arguments":{"a":2,"b":2}}}]},"done":true}`)

	call := firstCall(t, step)
	if call.Name != "add" {
		t.Fatalf("Name = %q, want %q", call.Name, "add")
	}
	if call.Input != `{"a":2,"b":2}` {
		t.Fatalf("Input = %q, want %q", call.Input, `{"a":2,"b":2}`)
	}
}

func TestParseStreamAccumulatesDeltas(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"message":{"role":"assistant","content":"Hel"},"done":false}`,
		`{"message":{"role":"assistant","content":"lo"},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true}`,
	}, "\n")

	var deltas strings.Builder
	step, err := parseStream(strings.NewReader(ndjson), func(s string) { deltas.WriteString(s) })
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	if !step.Done || step.Text != "Hello" {
		t.Fatalf("got %+v, want final text %q", step, "Hello")
	}
	if deltas.String() != "Hello" {
		t.Fatalf("deltas = %q, want %q", deltas.String(), "Hello")
	}
}
