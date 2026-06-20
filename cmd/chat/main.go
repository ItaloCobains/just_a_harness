package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"harness/agent"
	"harness/agentkit"
	"harness/config"
	"harness/internal/termui"
	"harness/model/ollama"
)

type toolMsg agent.Event

type deltaMsg string

// approvalReq is sent by a gated tool goroutine and answered by the UI thread.
type approvalReq struct {
	name  string
	input string
	resp  chan string
}

type doneMsg struct {
	history []agent.Message
	answer  string
	err     error
}

var (
	userStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	botStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	toolStyle   = lipgloss.NewStyle().Faint(true).Italic(true)
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	hintStyle   = lipgloss.NewStyle().Faint(true)
	statusStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
)

type model struct {
	vp          viewport.Model
	ta          textarea.Model
	spinner     spinner.Model
	llm         ollama.Model
	tools       []agent.Tool
	approver    *agentkit.Approver
	history     []agent.Message
	sessionName string
	transcript  []string
	sub         chan tea.Msg
	cancel      context.CancelFunc
	pending     *approvalReq
	start       time.Time
	agentIdx    int // transcript index of the current Agent bubble, -1 when none
	streaming   bool
	thinking    bool
	ready       bool
}

func initialModel(resume string) model {
	ta := textarea.New()
	ta.Placeholder = "Ask the agent... (Enter to send, Ctrl+C to quit, /help for commands)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 4000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	cfg := config.Load()
	llm := ollama.New(cfg.OllamaModel, cfg.OllamaEndpoint)
	llm.HTTPClient = &http.Client{Timeout: cfg.HTTPTimeout}
	llm.MaxRetries = cfg.HTTPMaxRetries
	approver := agentkit.LoadApprover()
	sub := make(chan tea.Msg, 64)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = statusStyle

	tools := agentkit.CodingTools(llm)
	for i, t := range tools {
		if agentkit.Mutating[t.Name] {
			tools[i] = gate(t, approver, sub)
		}
	}

	history := []agent.Message{{Role: "system", Text: agentkit.BuildSystemPrompt()}}
	banner := "Coding agent ready. Type a message and press Enter."
	if resume != "" {
		if loaded, name, err := loadResume(resume); err != nil {
			banner = "resume failed: " + err.Error()
		} else {
			history = loaded
			resume = name
			banner = "resumed session " + name
		}
	}

	return model{
		ta:          ta,
		spinner:     sp,
		llm:         llm,
		tools:       tools,
		approver:    approver,
		history:     history,
		sessionName: sessionName(resume),
		transcript:  []string{hintStyle.Render(banner)},
		sub:         sub,
		agentIdx:    -1,
	}
}

// loadResume loads a named session, or the latest when name is "latest".
func loadResume(name string) (history []agent.Message, resolved string, err error) {
	if name == "latest" {
		resolved, history, err = agentkit.LatestSession()
		return history, resolved, err
	}
	history, err = agentkit.LoadSession(name)
	return history, name, err
}

// sessionName returns the resumed name or a fresh timestamped one.
func sessionName(resumed string) string {
	if resumed != "" {
		return resumed
	}
	return "session-" + time.Now().Format("20060102-150405")
}

// gate wraps a mutating tool so it asks the UI thread for approval and blocks
// until the user answers, reusing the persistent Approver allowlist.
func gate(tool agent.Tool, approver *agentkit.Approver, sub chan tea.Msg) agent.Tool {
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
			if run, denied := approver.Decide(tool.Name, ans); !run {
				return denied, nil
			}
			return inner(ctx, input)
		}
	}
	return tool
}

func (m model) Init() tea.Cmd { return textarea.Blink }

func waitFor(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-sub }
}

// toolSummary renders a tool call as "name · key: value, ..." instead of raw
// JSON, truncating long values so a URL or query stays readable.
func toolSummary(name, input string) string {
	var args map[string]any
	if json.Unmarshal([]byte(input), &args) != nil || len(args) == 0 {
		return name
	}
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, termui.Truncate(fmt.Sprint(args[k]), 70)))
	}
	return name + " · " + strings.Join(parts, ", ")
}

// renderMarkdown pretty-prints an agent answer. It falls back to the raw text
// if glamour is unavailable or the render fails.
func renderMarkdown(s string, width int) string {
	if width <= 6 {
		width = 80
	}
	r, err := glamour.NewTermRenderer(glamour.WithAutoStyle(), glamour.WithWordWrap(width-4))
	if err != nil {
		return s
	}
	out, err := r.Render(s)
	if err != nil {
		return s
	}
	return strings.Trim(out, "\n")
}

// agentBubble renders an "Agent" turn with its markdown-formatted answer.
func (m *model) agentBubble(answer string) string {
	return botStyle.Render("Agent") + "\n" + renderMarkdown(answer, m.vp.Width)
}

func (m *model) render() string {
	body := strings.Join(m.transcript, "\n\n")
	if m.vp.Width > 0 {
		body = lipgloss.NewStyle().Width(m.vp.Width).Render(body)
	}
	return body
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
		height := msg.Height - m.ta.Height() - 3
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

	case spinner.TickMsg:
		if !m.thinking {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case approvalReq:
		m.pending = &msg
		m.push(statusStyle.Render("Allow " + toolSummary(msg.name, msg.input) + " ? [y/N/a]"))
		return m, waitFor(m.sub)

	case deltaMsg:
		if !m.streaming {
			m.push(botStyle.Render("Agent") + "\n")
			m.agentIdx = len(m.transcript) - 1
			m.streaming = true
		}
		m.appendToLast(string(msg))
		return m, waitFor(m.sub)

	case toolMsg:
		m.streaming = false
		m.agentIdx = -1
		m.push(toolStyle.Render("🔧 " + toolSummary(msg.Tool, msg.Input)))
		if result := strings.TrimSpace(msg.Result); result != "" {
			m.push(toolStyle.Render("  ↳ " + termui.Truncate(strings.ReplaceAll(result, "\n", " "), 200)))
		}
		return m, waitFor(m.sub)

	case doneMsg:
		streamed := m.streaming
		m.thinking = false
		m.streaming = false
		m.cancel = nil
		if msg.err != nil {
			m.push(errStyle.Render("error: " + msg.err.Error()))
		} else {
			answer := strings.TrimSpace(msg.answer)
			if streamed && m.agentIdx >= 0 && m.agentIdx < len(m.transcript) {
				m.transcript[m.agentIdx] = m.agentBubble(answer)
				m.vp.SetContent(m.render())
				m.vp.GotoBottom()
			} else if answer != "" {
				m.push(m.agentBubble(answer))
			} else {
				m.push(hintStyle.Render("(agent finished without a message)"))
			}
			m.agentIdx = -1
			m.history = msg.history
			if _, err := agentkit.SaveSession(m.sessionName, m.history); err != nil {
				m.push(hintStyle.Render("session not saved: " + err.Error()))
			}
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
	m.start = time.Now()
	m.history = append(m.history, agent.Message{Role: "user", Text: input})

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	sub, llm, tools, hist := m.sub, m.llm, m.tools, m.history
	go func() {
		out, answer, err := agent.Converse(ctx, llm, tools, hist, agent.Hooks{
			Observe: func(e agent.Event) { sub <- toolMsg(e) },
			Delta:   func(s string) { sub <- deltaMsg(s) },
		})
		sub <- doneMsg{out, answer, err}
	}()
	return m, tea.Batch(waitFor(m.sub), m.spinner.Tick)
}

func (m model) View() string {
	if !m.ready {
		return "loading..."
	}
	status := hintStyle.Render("ready")
	if m.pending != nil {
		status = statusStyle.Render("● awaiting approval")
	} else if m.thinking {
		elapsed := int(time.Since(m.start).Seconds())
		status = m.spinner.View() + statusStyle.Render(fmt.Sprintf("thinking %ds · Ctrl+C to interrupt", elapsed))
	}
	return fmt.Sprintf("%s\n%s\n%s", m.vp.View(), status, m.ta.View())
}

func main() {
	resume := flag.String("resume", "", "resume a saved session: a name, or \"latest\"")
	flag.Parse()

	p := tea.NewProgram(initialModel(*resume), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Println("error:", err)
	}
}
