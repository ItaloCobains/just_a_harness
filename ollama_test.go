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
