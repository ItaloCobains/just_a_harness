package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"harness"
	"harness/agentkit"
)

type toolMsg harness.Event

type doneMsg struct {
	history []harness.Message
	answer  string
	err     error
}

var (
	userStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	botStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	toolStyle = lipgloss.NewStyle().Faint(true).Italic(true)
	errStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	hintStyle = lipgloss.NewStyle().Faint(true)
)

type model struct {
	vp         viewport.Model
	ta         textarea.Model
	llm        harness.OllamaModel
	tools      []harness.Tool
	history    []harness.Message
	transcript []string
	sub        chan tea.Msg
	thinking   bool
	ready      bool
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Ask the agent... (Enter to send, Ctrl+C to quit)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 4000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	return model{
		ta:         ta,
		llm:        harness.OllamaModel{Model: "qwen2.5-coder:7b", Endpoint: "http://localhost:11434"},
		tools:      agentkit.CodingTools(),
		history:    []harness.Message{{Role: "system", Text: agentkit.SystemPrompt}},
		transcript: []string{hintStyle.Render("Coding agent ready. Type a message and press Enter.")},
		sub:        make(chan tea.Msg, 16),
	}
}

func (m model) Init() tea.Cmd { return textarea.Blink }

func waitFor(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-sub }
}

func (m *model) render() string {
	return strings.Join(m.transcript, "\n\n")
}

func (m *model) push(line string) {
	m.transcript = append(m.transcript, line)
	m.vp.SetContent(m.render())
	m.vp.GotoBottom()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		height := msg.Height - m.ta.Height() - 2
		if !m.ready {
			m.vp = viewport.New(msg.Width, height)
			m.vp.SetContent(m.render())
			m.ready = true
		} else {
			m.vp.Width = msg.Width
			m.vp.Height = height
		}
		m.ta.SetWidth(msg.Width)

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if m.thinking {
				return m, nil
			}
			input := strings.TrimSpace(m.ta.Value())
			if input == "" {
				return m, nil
			}
			m.ta.Reset()
			m.push(userStyle.Render("You") + "\n" + input)
			m.thinking = true

			next := append([]harness.Message(nil), m.history...)
			next = append(next, harness.Message{Role: "user", Text: input})
			m.history = next

			sub, llm, tools := m.sub, m.llm, m.tools
			go func() {
				hist, answer, err := harness.Converse(llm, tools, next, func(e harness.Event) {
					sub <- toolMsg(e)
				})
				sub <- doneMsg{hist, answer, err}
			}()
			return m, waitFor(m.sub)
		}

	case toolMsg:
		m.push(toolStyle.Render(fmt.Sprintf("🔧 %s(%s)", msg.Tool, truncate(msg.Input, 80))))
		return m, waitFor(m.sub)

	case doneMsg:
		m.thinking = false
		if msg.err != nil {
			m.push(errStyle.Render("error: " + msg.err.Error()))
		} else {
			m.history = msg.history
			m.push(botStyle.Render("Agent") + "\n" + msg.answer)
		}
		return m, nil
	}

	var tcmd, vcmd tea.Cmd
	m.ta, tcmd = m.ta.Update(msg)
	m.vp, vcmd = m.vp.Update(msg)
	return m, tea.Batch(tcmd, vcmd)
}

func (m model) View() string {
	if !m.ready {
		return "loading..."
	}
	status := ""
	if m.thinking {
		status = hintStyle.Render(" thinking...")
	}
	return fmt.Sprintf("%s\n%s%s", m.vp.View(), m.ta.View(), status)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
