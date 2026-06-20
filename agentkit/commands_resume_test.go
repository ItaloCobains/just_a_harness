package agentkit

import (
	"testing"

	"harness/agent"
)

func TestResumeCommandReturnsHistory(t *testing.T) {
	isolateHome(t)
	saved := []agent.Message{{Role: "system", Text: "sys"}, {Role: "user", Text: "hi"}}
	if _, err := SaveSession("demo", saved); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	res := HandleCommand("/resume demo", nil, nil)
	if !res.Handled || res.History == nil {
		t.Fatalf("resume not handled or history nil: %+v", res)
	}
	if len(res.History) != 2 || res.History[1].Text != "hi" {
		t.Fatalf("history = %+v", res.History)
	}
}

func TestResumeCommandUnknownSession(t *testing.T) {
	isolateHome(t)
	res := HandleCommand("/resume nope", nil, nil)
	if !res.Handled || res.History != nil {
		t.Fatalf("expected handled with nil history: %+v", res)
	}
}
