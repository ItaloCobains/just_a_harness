package agentkit

import (
	"fmt"
	"sort"
	"strings"

	"harness/agent"
)

// CommandResult is the outcome of interpreting a user line as a slash command.
type CommandResult struct {
	Handled bool            // true if the line was a slash command
	Reply   string          // text to show the user
	History []agent.Message // replacement history, when the command mutates it (e.g. /clear, /compact)
}

// HandleCommand interprets lines beginning with "/". It returns Handled=false
// for ordinary input so the caller forwards it to the model unchanged.
func HandleCommand(line string, history []agent.Message, tools []agent.Tool) CommandResult {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return CommandResult{}
	}

	fields := strings.Fields(line)
	switch fields[0] {
	case "/help":
		return CommandResult{Handled: true, Reply: helpText()}
	case "/tools":
		return CommandResult{Handled: true, Reply: toolList(tools)}
	case "/clear":
		return CommandResult{Handled: true, Reply: "context cleared", History: keepSystem(history)}
	case "/resume":
		return resumeCommand(fields[1:])
	case "/sessions":
		return sessionsCommand()
	default:
		return CommandResult{Handled: true, Reply: "unknown command. " + helpText()}
	}
}

func resumeCommand(args []string) CommandResult {
	var (
		name    string
		history []agent.Message
		err     error
	)
	if len(args) > 0 {
		name = args[0]
		history, err = LoadSession(name)
	} else {
		name, history, err = LatestSession()
	}
	if err != nil {
		return CommandResult{Handled: true, Reply: "resume failed: " + err.Error()}
	}
	return CommandResult{Handled: true, Reply: "resumed session " + name, History: history}
}

func sessionsCommand() CommandResult {
	names, err := ListSessions()
	if err != nil {
		return CommandResult{Handled: true, Reply: "sessions failed: " + err.Error()}
	}
	if len(names) == 0 {
		return CommandResult{Handled: true, Reply: "no saved sessions"}
	}
	return CommandResult{Handled: true, Reply: strings.Join(names, "\n")}
}

// Command describes a slash command for help text and TUI autocomplete.
type Command struct {
	Name string
	Desc string
}

// Commands is the canonical list of slash commands, shared by help and the TUI
// menu so both stay in sync.
func Commands() []Command {
	return []Command{
		{"/help", "show this help"},
		{"/tools", "list available tools"},
		{"/clear", "reset the conversation (keeps the system prompt)"},
		{"/resume", "resume the latest session, or /resume <name>"},
		{"/sessions", "list saved sessions"},
	}
}

func helpText() string {
	lines := make([]string, 0, len(Commands()))
	for _, c := range Commands() {
		lines = append(lines, fmt.Sprintf("%-9s - %s", c.Name, c.Desc))
	}
	return strings.Join(lines, "\n")
}

func toolList(tools []agent.Tool) string {
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return strings.Join(names, "\n")
}

func keepSystem(history []agent.Message) []agent.Message {
	if len(history) > 0 && history[0].Role == "system" {
		return []agent.Message{history[0]}
	}
	return nil
}
