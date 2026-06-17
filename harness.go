package harness

type Message struct{}

type Step struct {
	Done bool
	Text string
}

func Run(model *FakeModel, input string) string {
	step := model.Next(nil)
	return step.Text
}
