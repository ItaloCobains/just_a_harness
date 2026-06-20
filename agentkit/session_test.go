package agentkit

import (
	"errors"
	"testing"
	"time"

	"harness/agent"
)

func isolateHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

func TestSaveLoadSessionRoundTrip(t *testing.T) {
	isolateHome(t)

	want := []agent.Message{
		{Role: "system", Text: "sys"},
		{Role: "user", Text: "hi"},
		{Role: "assistant", Text: "yo"},
	}
	if _, err := SaveSession("demo", want); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	got, err := LoadSession("demo")
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(got) != len(want) || got[1].Text != "hi" || got[2].Role != "assistant" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestLatestSessionPicksNewest(t *testing.T) {
	isolateHome(t)

	if _, err := SaveSession("old", []agent.Message{{Role: "user", Text: "old"}}); err != nil {
		t.Fatalf("save old: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := SaveSession("new", []agent.Message{{Role: "user", Text: "new"}}); err != nil {
		t.Fatalf("save new: %v", err)
	}

	name, history, err := LatestSession()
	if err != nil {
		t.Fatalf("LatestSession: %v", err)
	}
	if name != "new" || history[0].Text != "new" {
		t.Fatalf("latest = %q (%+v), want new", name, history)
	}
}

func TestLatestSessionEmpty(t *testing.T) {
	isolateHome(t)
	if _, _, err := LatestSession(); !errors.Is(err, ErrNoSessions) {
		t.Fatalf("err = %v, want ErrNoSessions", err)
	}
}

func TestListSessions(t *testing.T) {
	isolateHome(t)
	SaveSession("b", nil)
	SaveSession("a", nil)
	names, err := ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("names = %v, want [a b]", names)
	}
}
