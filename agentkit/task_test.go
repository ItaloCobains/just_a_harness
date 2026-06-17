package agentkit

import (
	"context"
	"testing"

	"harness"
)

// scriptModel returns canned steps, ignoring input.
type scriptModel struct {
	steps []harness.Step
	i     int
}

func (m *scriptModel) Next(_ context.Context, _ []harness.Message, _ []harness.Tool, _ func(string)) (harness.Step, error) {
	s := m.steps[m.i]
	m.i++
	return s, nil
}

func TestTaskToolReturnsSubagentAnswer(t *testing.T) {
	model := &scriptModel{steps: []harness.Step{{Done: true, Text: "found it"}}}
	tool := taskTool(model)

	out, err := tool.Func(context.Background(), `{"prompt":"where is X"}`)
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	if out != "found it" {
		t.Fatalf("out = %q, want %q", out, "found it")
	}
}

func TestTaskToolRequiresPrompt(t *testing.T) {
	tool := taskTool(&scriptModel{})
	if _, err := tool.Func(context.Background(), `{}`); err == nil {
		t.Fatal("empty prompt should error")
	}
}

func TestReadOnlyToolsExcludeMutators(t *testing.T) {
	for _, tool := range ReadOnlyTools() {
		if Mutating[tool.Name] {
			t.Fatalf("read-only set leaked mutating tool %q", tool.Name)
		}
	}
}
