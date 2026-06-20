package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"harness/agent"
)

func sized() model {
	m, _ := initialModel().Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return m.(model)
}

func TestChatStreamsAgentAnswer(t *testing.T) {
	next, _ := sized().Update(deltaMsg("hello "))
	next, _ = next.(model).Update(deltaMsg("world"))
	m := next.(model)

	if !strings.Contains(m.render(), "hello world") {
		t.Fatalf("transcript missing streamed answer, got:\n%s", m.render())
	}
}

func TestChatDoneClearsThinking(t *testing.T) {
	m := sized()
	m.thinking = true
	next, _ := m.Update(doneMsg{answer: "done"})

	if next.(model).thinking {
		t.Fatal("thinking must be false after doneMsg")
	}
}

func TestChatRendersToolCall(t *testing.T) {
	next, _ := sized().Update(toolMsg(agent.Event{Tool: "read_file", Input: `{"path":"go.mod"}`}))
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
