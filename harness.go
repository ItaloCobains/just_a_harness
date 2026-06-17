package harness

type Message struct{}

type Step struct {
	Done bool
	Text string
}

func Run(model *FakeModel, input string) string {
	for {
		step := model.Next(nil)

		if step.Done {
			return step.Text
		}
	}
}
