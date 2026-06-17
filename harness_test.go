package harness

import "testing"

type FakeModel struct {
	steps []Step
	calls int
	seen  [][]Message
}

func (m *FakeModel) Next(msgs []Message) Step {
	m.seen = append(m.seen, msgs)
	step := m.steps[m.calls]
	m.calls++
	return step
}

func TestRunReturnsFinalTextWhenModelStops(t *testing.T) {
	model := &FakeModel{steps: []Step{{Done: true, Text: "olá"}}}

	got := Run(model, nil, "oi")

	if got != "olá" {
		t.Fatalf("Run = %q, want %q", got, "olá")
	}
}

func TestRunLoopsUntilModelStops(t *testing.T) {
	model := &FakeModel{steps: []Step{
		{Done: false, Text: "pensando"},
		{Done: true, Text: "pronto"},
	}}

	got := Run(model, nil, "oi")

	if got != "pronto" {
		t.Fatalf("Run = %q, want %q", got, "pronto")
	}
}

func TestRunExecutesRequestedTool(t *testing.T) {
	var gotInput string
	tools := map[string]Tool{
		"echo": func(input string) string {
			gotInput = input
			return input
		},
	}

	model := &FakeModel{steps: []Step{
		{Tool: "echo", Input: "hi"},
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "oi")

	if gotInput != "hi" {
		t.Fatalf("tool received %q, want %q", gotInput, "hi")
	}
}

func TestRunFeedsToolResultBackToModel(t *testing.T) {
	tools := map[string]Tool{
		"echo": func(input string) string { return input },
	}
	model := &FakeModel{steps: []Step{
		{Tool: "echo", Input: "hi"},
		{Done: true, Text: "ok"},
	}}

	Run(model, tools, "oi")

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
