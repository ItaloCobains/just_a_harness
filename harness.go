package harness

import "errors"

const maxTurns = 25

var ErrMaxTurns = errors.New("harness: max turns exceeded")

type Message struct {
	Role  string
	Text  string
	Tool  string
	Input string
}

type Step struct {
	Done  bool
	Text  string
	Tool  string
	Input string
}

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Func        func(input string) string
}

type Model interface {
	Next(messages []Message, tools []Tool) Step
}

// Event reports a tool execution so a UI can show activity as it happens.
type Event struct {
	Tool   string
	Input  string
	Result string
}

// Converse runs the agent loop over an existing conversation until the model
// gives a final answer. It returns the extended history (including the final
// assistant turn) so callers can keep a multi-turn chat going. observe, when
// non-nil, is called after each tool runs.
func Converse(model Model, tools []Tool, history []Message, observe func(Event)) ([]Message, string, error) {
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	for range maxTurns {
		step := model.Next(history, tools)

		if step.Tool != "" {
			history = append(history, Message{Role: "assistant", Tool: step.Tool, Input: step.Input})
			result := byName[step.Tool].Func(step.Input)
			history = append(history, Message{Role: "tool", Text: result})
			if observe != nil {
				observe(Event{Tool: step.Tool, Input: step.Input, Result: result})
			}
			continue
		}

		if step.Done {
			history = append(history, Message{Role: "assistant", Text: step.Text})
			return history, step.Text, nil
		}
	}

	return history, "", ErrMaxTurns
}

func Run(model Model, tools []Tool, system, input string) (string, error) {
	var history []Message
	if system != "" {
		history = append(history, Message{Role: "system", Text: system})
	}
	history = append(history, Message{Role: "user", Text: input})

	_, answer, err := Converse(model, tools, history, nil)
	return answer, err
}
