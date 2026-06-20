package agentkit

import (
	"slices"
	"strings"
	"testing"

	"harness/agent"
)

func TestHandleCommandPassesThroughOrdinaryInput(t *testing.T) {
	if res := HandleCommand("hello there", nil, nil); res.Handled {
		t.Fatalf("ordinary input should not be handled, got %+v", res)
	}
}

func TestHandleClearKeepsSystem(t *testing.T) {
	history := []agent.Message{
		{Role: "system", Text: "sys"},
		{Role: "user", Text: "oi"},
		{Role: "assistant", Text: "ola"},
	}

	res := HandleCommand("/clear", history, nil)
	if !res.Handled {
		t.Fatal("/clear must be handled")
	}
	if len(res.History) != 1 || res.History[0].Role != "system" {
		t.Fatalf("history after /clear = %+v, want only system", res.History)
	}
}

func TestHandleUnknownCommandShowsHelp(t *testing.T) {
	res := HandleCommand("/bogus", nil, nil)
	if !res.Handled || res.Reply == "" {
		t.Fatalf("unknown command should be handled with help, got %+v", res)
	}
}

func TestHandleToolsListsNames(t *testing.T) {
	tools := []agent.Tool{{Name: "read_file"}, {Name: "grep"}}
	res := HandleCommand("/tools", nil, tools)
	if !res.Handled {
		t.Fatal("/tools must be handled")
	}
	lines := strings.Split(res.Reply, "\n")
	for _, name := range []string{"read_file", "grep"} {
		if !slices.Contains(lines, name) {
			t.Fatalf("/tools reply %q missing %q", res.Reply, name)
		}
	}
}
