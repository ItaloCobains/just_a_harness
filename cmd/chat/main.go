package main

import (
	"context"
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

type deltaMsg string

// approvalReq is sent by a gated tool goroutine and answered by the UI thread.
type approvalReq struct {
	name  string
	input string
	resp  chan string
}

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
	approver   *agentkit.Approver
	history    []harness.Message
	transcript []string
	sub        chan tea.Msg
	cancel     context.CancelFunc
	pending    *approvalReq
	streaming  bool
	thinking   bool
	ready      bool
}

func initialModel() model {
	ta := textarea.New()
	ta.Placeholder = "Ask the agent... (Enter to send, Ctrl+C to quit, /help for commands)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 4000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	llm := harness.OllamaModel{Model: "qwen2.5-coder:7b", Endpoint: "http://localhost:11434"}
	approver := agentkit.LoadApprover()
	sub := make(chan tea.Msg, 64)

	tools := agentkit.CodingTools(llm)
	for i, t := range tools {
		if agentkit.Mutating[t.Name] {
			tools[i] = gate(t, approver, sub)
		}
	}

	return model{
		ta:         ta,
		llm:        llm,
		tools:      tools,
		approver:   approver,
		history:    []harness.Message{{Role: "system", Text: agentkit.BuildSystemPrompt()}},
		transcript: []string{hintStyle.Render("Coding agent ready. Type a message and press Enter.")},
		sub:        sub,
	}
}

// gate wraps a mutating tool so it asks the UI thread for approval and blocks
// until the user answers, reusing the persistent Approver allowlist.
func gate(tool harness.Tool, approver *agentkit.Approver, sub chan tea.Msg) harness.Tool {
	inner := tool.Func
	tool.Func = func(ctx context.Context, input string) (string, error) {
		if approver.Allowed(tool.Name) {
			return inner(ctx, input)
		}
		resp := make(chan string, 1)
		sub <- approvalReq{name: tool.Name, input: input, resp: resp}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case ans := <-resp:
			switch ans {
			case "a":
				approver.Always(tool.Name)
				return inner(ctx, input)
			case "y":
				return inner(ctx, input)
			default:
				return "denied by user", nil
			}
		}
	}
	return tool
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

func (m *model) appendToLast(s string) {
	if len(m.transcript) == 0 {
		m.transcript = append(m.transcript, s)
	} else {
		m.transcript[len(m.transcript)-1] += s
	}
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
		if m.pending != nil {
			return m.answerApproval(msg)
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			if m.thinking && m.cancel != nil {
				m.cancel()
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyEnter:
			return m.submit()
		}

	case approvalReq:
		m.pending = &msg
		m.push(hintStyle.Render(fmt.Sprintf("Allow %s(%s)? [y/N/a]", msg.name, truncate(msg.input, 80))))
		return m, waitFor(m.sub)

	case deltaMsg:
		if !m.streaming {
			m.push(botStyle.Render("Agent") + "\n")
			m.streaming = true
		}
		m.appendToLast(string(msg))
		return m, waitFor(m.sub)

	case toolMsg:
		m.streaming = false
		m.push(toolStyle.Render(fmt.Sprintf("🔧 %s(%s)", msg.Tool, truncate(msg.Input, 80))))
		return m, waitFor(m.sub)

	case doneMsg:
		m.thinking = false
		m.streaming = false
		m.cancel = nil
		if msg.err != nil {
			m.push(errStyle.Render("error: " + msg.err.Error()))
		} else {
			m.history = msg.history
		}
		return m, nil
	}

	var tcmd, vcmd tea.Cmd
	m.ta, tcmd = m.ta.Update(msg)
	m.vp, vcmd = m.vp.Update(msg)
	return m, tea.Batch(tcmd, vcmd)
}

func (m model) answerApproval(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	ans := strings.ToLower(msg.String())
	if ans != "y" && ans != "a" {
		ans = "n"
	}
	m.pending.resp <- ans
	m.pending = nil
	return m, waitFor(m.sub)
}

func (m model) submit() (tea.Model, tea.Cmd) {
	if m.thinking {
		return m, nil
	}
	input := strings.TrimSpace(m.ta.Value())
	if input == "" {
		return m, nil
	}
	m.ta.Reset()
	m.push(userStyle.Render("You") + "\n" + input)

	if res := agentkit.HandleCommand(input, m.history, m.tools); res.Handled {
		if res.History != nil {
			m.history = res.History
		}
		m.push(hintStyle.Render(res.Reply))
		return m, nil
	}

	m.thinking = true
	m.history = append(m.history, harness.Message{Role: "user", Text: input})

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	sub, llm, tools, hist := m.sub, m.llm, m.tools, m.history
	go func() {
		out, answer, err := harness.Converse(ctx, llm, tools, hist, harness.Hooks{
			Observe: func(e harness.Event) { sub <- toolMsg(e) },
			Delta:   func(s string) { sub <- deltaMsg(s) },
		})
		sub <- doneMsg{out, answer, err}
	}()
	return m, waitFor(m.sub)
}

func (m model) View() string {
	if !m.ready {
		return "loading..."
	}
	status := ""
	if m.pending != nil {
		status = hintStyle.Render(" awaiting approval...")
	} else if m.thinking {
		status = hintStyle.Render(" thinking... (Ctrl+C to interrupt)")
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
