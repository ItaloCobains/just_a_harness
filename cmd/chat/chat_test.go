package main

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"harness/agent"
)

func sized() model {
	m, _ := initialModel("").Update(tea.WindowSizeMsg{Width: 80, Height: 24})
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

func TestChatShowsToolResult(t *testing.T) {
	next, _ := sized().Update(toolMsg(agent.Event{Tool: "web_search", Input: `{"query":"go"}`, Result: "1. The Go Language"}))
	m := next.(model)

	if !strings.Contains(m.render(), "The Go Language") {
		t.Fatalf("transcript missing tool result, got:\n%s", m.render())
	}
}

func TestChatRendersAnswerAfterStreaming(t *testing.T) {
	next, _ := sized().Update(deltaMsg("# Title"))
	next, _ = next.(model).Update(doneMsg{answer: "# Title"})
	m := next.(model)

	if !strings.Contains(m.render(), "Title") {
		t.Fatalf("rendered answer missing, got:\n%s", m.render())
	}
}

func TestChatShowsThinkingInView(t *testing.T) {
	m := sized()
	m.thinking = true

	if !strings.Contains(m.View(), "thinking") {
		t.Fatalf("view should show thinking indicator, got:\n%s", m.View())
	}
}

func TestToolPreviewWriteFile(t *testing.T) {
	out := toolPreview("write_file", `{"path":"a.txt","content":"line1\nline2"}`)
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") {
		t.Fatalf("write preview missing content: %q", out)
	}
}

func TestToolPreviewEditFileShowsBothSides(t *testing.T) {
	out := toolPreview("edit_file", `{"path":"a","old_string":"foo","new_string":"bar"}`)
	if !strings.Contains(out, "foo") || !strings.Contains(out, "bar") {
		t.Fatalf("edit preview should show old and new: %q", out)
	}
}

func TestToolPreviewEmptyForOtherTools(t *testing.T) {
	if out := toolPreview("run_bash", `{"cmd":"ls"}`); out != "" {
		t.Fatalf("expected no preview for run_bash, got %q", out)
	}
}

func TestCommandMenuFiltersByPrefix(t *testing.T) {
	all := commandMenu("/")
	if !strings.Contains(all, "/help") || !strings.Contains(all, "/sessions") {
		t.Fatalf("'/' should list all commands:\n%s", all)
	}
	re := commandMenu("/re")
	if !strings.Contains(re, "/resume") || strings.Contains(re, "/help") {
		t.Fatalf("'/re' should match only /resume:\n%s", re)
	}
	if commandMenu("hello") != "" {
		t.Fatal("non-slash input should produce no menu")
	}
}

func TestCompleteCommand(t *testing.T) {
	if got := completeCommand("/se"); got != "/sessions" {
		t.Fatalf("completeCommand(/se) = %q, want /sessions", got)
	}
	if got := completeCommand("/x"); got != "" {
		t.Fatalf("no match should return empty, got %q", got)
	}
	if got := completeCommand("hi"); got != "" {
		t.Fatalf("non-slash should return empty, got %q", got)
	}
}

func TestSubmitQueuesWhileThinking(t *testing.T) {
	m := sized()
	m.thinking = true
	m.ta.SetValue("do the next thing")

	next, _ := m.submit()
	nm := next.(model)

	if len(nm.queue) != 1 || nm.queue[0] != "do the next thing" {
		t.Fatalf("prompt not queued: %v", nm.queue)
	}
	if !strings.Contains(nm.render(), "queued") {
		t.Fatalf("no queued hint shown:\n%s", nm.render())
	}
}

func TestDequeueRunsSlashCommandInPlace(t *testing.T) {
	m := sized()
	m.queue = []string{"/help"}

	cmd := m.dequeue()

	if cmd != nil {
		t.Fatal("a slash command should not start a turn")
	}
	if len(m.queue) != 0 {
		t.Fatalf("queue not drained: %v", m.queue)
	}
	if !strings.Contains(m.render(), "/help") {
		t.Fatalf("command not shown:\n%s", m.render())
	}
}

func TestCtrlCClearsQueue(t *testing.T) {
	m := sized()
	_, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.thinking = true
	m.queue = []string{"a", "b"}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})

	if q := next.(model).queue; len(q) != 0 {
		t.Fatalf("Ctrl+C should clear the queue, got %v", q)
	}
}
