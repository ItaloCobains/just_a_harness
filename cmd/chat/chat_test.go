package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"harness"
)

func sized() model {
	m, _ := initialModel().Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m.(model)
}

func TestChatRendersAgentAnswer(t *testing.T) {
	next, _ := sized().Update(doneMsg{answer: "hello world"})
	m := next.(model)

	if m.thinking {
		t.Fatal("thinking must be false after doneMsg")
	}
	if !strings.Contains(m.render(), "hello world") {
		t.Fatalf("transcript missing answer, got:\n%s", m.render())
	}
}

func TestChatRendersToolCall(t *testing.T) {
	next, _ := sized().Update(toolMsg(harness.Event{Tool: "read_file", Input: `{"path":"go.mod"}`}))
	m := next.(model)

	if !strings.Contains(m.render(), "read_file") {
		t.Fatalf("transcript missing tool call, got:\n%s", m.render())
	}
}

func TestChatShowsThinkingInView(t *testing.T) {
	m := sized()
	m.thinking = true

	if !strings.Contains(m.View(), "thinking") {
		t.Fatalf("view should show thinking indicator, got:\n%s", m.View())
	}
}
