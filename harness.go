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

type Tool func(input string) string

type Model interface {
	Next(messages []Message) Step
}

func Run(model Model, tools map[string]Tool, input string) (string, error) {
	history := []Message{{Role: "user", Text: input}}

	for range maxTurns {
		step := model.Next(history)

		if step.Tool != "" {
			history = append(history, Message{Role: "assistant", Tool: step.Tool, Input: step.Input})
			result := tools[step.Tool](step.Input)
			history = append(history, Message{Role: "tool", Text: result})
			continue
		}

		if step.Done {
			return step.Text, nil
		}
	}

	return "", ErrMaxTurns
}
