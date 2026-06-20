package agentkit

import (
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

	switch strings.Fields(line)[0] {
	case "/help":
		return CommandResult{Handled: true, Reply: helpText()}
	case "/tools":
		return CommandResult{Handled: true, Reply: toolList(tools)}
	case "/clear":
		return CommandResult{Handled: true, Reply: "context cleared", History: keepSystem(history)}
	default:
		return CommandResult{Handled: true, Reply: "unknown command. " + helpText()}
	}
}

func helpText() string {
	return strings.Join([]string{
		"/help  - show this help",
		"/tools - list available tools",
		"/clear - reset the conversation (keeps the system prompt)",
	}, "\n")
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
