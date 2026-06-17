package harness

type Message struct {
	Text string
}

type Tool func(input string) string

type Step struct {
	Done  bool
	Text  string
	Tool  string
	Input string
}

func Run(model *FakeModel, tools map[string]Tool, input string) string {
	history := []Message{{Text: input}}

	for {
		step := model.Next(history)

		if step.Tool != "" {
			result := tools[step.Tool](step.Input)
			history = append(history, Message{Text: result})
			continue
		}

		if step.Done {
			return step.Text
		}
	}
}
