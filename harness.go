package harness

import "errors"

const maxTurns = 10

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

func Run(model Model, tools []Tool, input string) (string, error) {
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	history := []Message{{Role: "user", Text: input}}

	for range maxTurns {
		step := model.Next(history, tools)

		if step.Tool != "" {
			history = append(history, Message{Role: "assistant", Tool: step.Tool, Input: step.Input})
			result := byName[step.Tool].Func(step.Input)
			history = append(history, Message{Role: "tool", Text: result})
			continue
		}

		if step.Done {
			return step.Text, nil
		}
	}

	return "", ErrMaxTurns
}
