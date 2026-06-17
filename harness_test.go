package harness

import "testing"

type endlessModel struct{}

type FakeModel struct {
	steps []Step
	calls int
	seen  [][]Message
}

func (endlessModel) Next(_ []Message, _ []Tool) Step { return Step{} }

func (m *FakeModel) Next(msgs []Message, _ []Tool) Step {
	m.seen = append(m.seen, msgs)
	step := m.steps[m.calls]
	m.calls++
	return step
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
	tools := []Tool{{
		Name: "echo",
		Func: func(input string) string {
			gotInput = input
			return input
		},
	}}

	model := &FakeModel{steps: []Step{
		{Tool: "echo", Input: "hi"},
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "", "oi")

	if gotInput != "hi" {
		t.Fatalf("tool received %q, want %q", gotInput, "hi")
	}
}

func TestRunFeedsToolResultBackToModel(t *testing.T) {
	tools := []Tool{{
		Name: "echo",
		Func: func(input string) string { return input },
	}}
	model := &FakeModel{steps: []Step{
		{Tool: "echo", Input: "hi"},
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
		t.Fatalf("model did not see tool result %q on secound turn, saw %v", "hi", secondTurn)
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
	tools := []Tool{{
		Name: "echo",
		Func: func(input string) string { return input },
	}}
	model := &FakeModel{steps: []Step{
		{Tool: "echo", Input: "hi"},
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
	tools := []Tool{{
		Name: "echo",
		Func: func(input string) string { return input },
	}}
	model := &FakeModel{steps: []Step{
		{Tool: "echo", Input: "hi"},
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "", "oi")

	// Esperado na segunda volta: [user "oi", assistant pede echo, tool "hi"]
	secondTurn := model.seen[1]
	assistant := secondTurn[len(secondTurn)-2]
	if assistant.Role != "assistant" || assistant.Tool != "echo" {
		t.Fatalf("expected assistant tool-request for %q, got %+v", "echo", assistant)
	}
}
