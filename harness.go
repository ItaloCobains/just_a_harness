package harness

type Message struct{}

type Tool func(input string) string

type Step struct {
	Done  bool
	Text  string
	Tool  string
	Input string
}

func Run(model *FakeModel, tools map[string]Tool, input string) string {
	for {
		step := model.Next(nil)

		if step.Tool != "" {
			tools[step.Tool](step.Input)
			continue
		}

		if step.Done {
			return step.Text
		}
	}
}
