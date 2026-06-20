package agent

import (
	"context"
	"testing"
)

type endlessModel struct{}

type FakeModel struct {
	steps []Step
	calls int
	seen  [][]Message
}

func (endlessModel) Next(_ context.Context, _ []Message, _ []Tool, _ func(string)) (Step, error) {
	return Step{}, nil
}

func (m *FakeModel) Next(_ context.Context, msgs []Message, _ []Tool, _ func(string)) (Step, error) {
	m.seen = append(m.seen, msgs)
	step := m.steps[m.calls]
	m.calls++
	return step, nil
}

func okTool(name string, fn func(string) string) Tool {
	return Tool{Name: name, Func: func(_ context.Context, input string) (string, error) {
		return fn(input), nil
	}}
}

func callStep(name, input string) Step {
	return Step{ToolCalls: []ToolCall{{ID: "c0", Name: name, Input: input}}}
}

func TestConverseAppendsFinalAnswerToHistory(t *testing.T) {
	model := &FakeModel{steps: []Step{{Done: true, Text: "hi"}}}

	history, answer, err := Converse(context.Background(), model, nil, []Message{{Role: "user", Text: "oi"}}, Hooks{})

	if err != nil {
		t.Fatalf("Converse: %v", err)
	}
	if answer != "hi" {
		t.Fatalf("answer = %q, want %q", answer, "hi")
	}
	last := history[len(history)-1]
	if last.Role != "assistant" || last.Text != "hi" {
		t.Fatalf("last message = %+v, want assistant %q", last, "hi")
	}
}

func TestConverseObservesToolCalls(t *testing.T) {
	tools := []Tool{okTool("echo", func(input string) string { return input })}
	model := &FakeModel{steps: []Step{
		callStep("echo", "x"),
		{Done: true, Text: "ok"},
	}}

	var events []Event
	Converse(context.Background(), model, tools, []Message{{Role: "user", Text: "oi"}}, Hooks{
		Observe: func(e Event) { events = append(events, e) },
	})

	if len(events) != 1 || events[0].Tool != "echo" || events[0].Result != "x" {
		t.Fatalf("events = %+v, want one echo call returning %q", events, "x")
	}
}

func TestRunReturnsFinalTextWhenModelStops(t *testing.T) {
	model := &FakeModel{steps: []Step{{Done: true, Text: "olá"}}}

	got, _ := Run(model, nil, "", "oi")

	if got != "olá" {
		t.Fatalf("Run = %q, want %q", got, "olá")
	}
}

func TestRunLoopsUntilModelStops(t *testing.T) {
	model := &FakeModel{steps: []Step{
		{Done: false, Text: "pensando"},
		{Done: true, Text: "pronto"},
	}}

	got, _ := Run(model, nil, "", "oi")

	if got != "pronto" {
		t.Fatalf("Run = %q, want %q", got, "pronto")
	}
}

func TestRunExecutesRequestedTool(t *testing.T) {
	var gotInput string
	tools := []Tool{okTool("echo", func(input string) string {
		gotInput = input
		return input
	})}

	model := &FakeModel{steps: []Step{
		callStep("echo", "hi"),
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "", "oi")

	if gotInput != "hi" {
		t.Fatalf("tool received %q, want %q", gotInput, "hi")
	}
}

func TestRunFeedsToolResultBackToModel(t *testing.T) {
	tools := []Tool{okTool("echo", func(input string) string { return input })}
	model := &FakeModel{steps: []Step{
		callStep("echo", "hi"),
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "", "oi")

	secondTurn := model.seen[1]
	found := false
	for _, msg := range secondTurn {
		if msg.Text == "hi" {
			found = true
		}
	}

	if !found {
		t.Fatalf("model did not see tool result %q on second turn, saw %v", "hi", secondTurn)
	}
}

func TestRunStopsAfterMaxTurns(t *testing.T) {
	_, err := Run(endlessModel{}, nil, "", "oi")

	if err == nil {
		t.Fatalf("Run should return an error when the model never stops")
	}
}

func TestRunTagsUserInputWithUserRole(t *testing.T) {
	model := &FakeModel{steps: []Step{{Done: true, Text: "ok"}}}

	Run(model, nil, "", "oi")

	first := model.seen[0][0]
	if first.Role != "user" {
		t.Fatalf("first message role = %q, want %q", first.Role, "user")
	}
}

func TestRunTagsToolResultWithToolRole(t *testing.T) {
	tools := []Tool{okTool("echo", func(input string) string { return input })}
	model := &FakeModel{steps: []Step{
		callStep("echo", "hi"),
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "", "oi")

	secondTurn := model.seen[1]
	last := secondTurn[len(secondTurn)-1]
	if last.Role != "tool" {
		t.Fatalf("tool result role = %q, want %q", last.Role, "tool")
	}
}

func TestRunRecordsAssistantToolRequest(t *testing.T) {
	tools := []Tool{okTool("echo", func(input string) string { return input })}
	model := &FakeModel{steps: []Step{
		callStep("echo", "hi"),
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "", "oi")

	// Esperado na segunda volta: [user "oi", assistant pede echo, tool "hi"]
	secondTurn := model.seen[1]
	assistant := secondTurn[len(secondTurn)-2]
	if assistant.Role != "assistant" || len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].Name != "echo" {
		t.Fatalf("expected assistant tool-request for %q, got %+v", "echo", assistant)
	}
}

func TestConverseUnknownToolDoesNotPanic(t *testing.T) {
	model := &FakeModel{steps: []Step{
		callStep("ghost", "{}"),
		{Done: true, Text: "ok"},
	}}

	history, _, err := Converse(context.Background(), model, nil, []Message{{Role: "user", Text: "oi"}}, Hooks{})
	if err != nil {
		t.Fatalf("Converse: %v", err)
	}

	// O resultado da tool fantasma deve voltar como erro para o modelo.
	second := model.seen[1]
	last := second[len(second)-1]
	if last.Role != "tool" || last.Text == "" {
		t.Fatalf("expected tool error result, got %+v", last)
	}
	_ = history
}

func TestConverseRunsParallelToolsInOrder(t *testing.T) {
	tools := []Tool{
		okTool("a", func(string) string { return "ra" }),
		okTool("b", func(string) string { return "rb" }),
	}
	model := &FakeModel{steps: []Step{
		{ToolCalls: []ToolCall{
			{ID: "0", Name: "a", Input: "{}"},
			{ID: "1", Name: "b", Input: "{}"},
		}},
		{Done: true, Text: "ok"},
	}}

	Converse(context.Background(), model, tools, []Message{{Role: "user", Text: "oi"}}, Hooks{})

	second := model.seen[1]
	// Últimas duas mensagens são os resultados, na ordem original a,b.
	ra := second[len(second)-2]
	rb := second[len(second)-1]
	if ra.Text != "ra" || ra.ToolID != "0" || rb.Text != "rb" || rb.ToolID != "1" {
		t.Fatalf("parallel results out of order: %+v %+v", ra, rb)
	}
}

func TestConverseStopsOnCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	model := &FakeModel{steps: []Step{{Done: true, Text: "nope"}}}

	_, _, err := Converse(ctx, model, nil, []Message{{Role: "user", Text: "oi"}}, Hooks{})
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if model.calls != 0 {
		t.Fatalf("model should not be called on cancelled context, calls=%d", model.calls)
	}
}

func TestConversePreToolCanDeny(t *testing.T) {
	ran := false
	tools := []Tool{okTool("danger", func(string) string { ran = true; return "did it" })}
	model := &FakeModel{steps: []Step{
		callStep("danger", "{}"),
		{Done: true, Text: "ok"},
	}}

	Converse(context.Background(), model, tools, []Message{{Role: "user", Text: "oi"}}, Hooks{
		PreTool: func(ToolCall) (bool, string) { return true, "blocked" },
	})

	if ran {
		t.Fatalf("denied tool should not run")
	}
	second := model.seen[1]
	if last := second[len(second)-1]; last.Text != "denied: blocked" {
		t.Fatalf("expected denial result, got %q", last.Text)
	}
}
