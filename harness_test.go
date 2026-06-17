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

	got := Run(model, "oi")

	if got != "olá" {
		t.Fatalf("Run = %q, want %q", got, "olá")
	}
}
