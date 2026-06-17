package harness

import "testing"

type FakeModel struct {
	steps []Step
	calls int
}

func (m *FakeModel) Next(_ []Message) Step {
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
