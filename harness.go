package harness

import "errors"

const maxTurns = 10

var ErrMaxTurns = errors.New("harness: max turns exceeded")

type Message struct {
	Text string
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
	history := []Message{{Text: input}}

	for turn := 0; turn < maxTurns; turn++ {
		step := model.Next(history)

		if step.Tool != "" {
			result := tools[step.Tool](step.Input)
			history = append(history, Message{Text: result})
			continue
		}

		if step.Done {
			return step.Text, nil
		}
	}

	return "", ErrMaxTurns
}
