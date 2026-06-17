package main

import (
	"fmt"

	"harness"
)

func main() {
	model := harness.OllamaModel{Model: "llama3.2:3b", Endpoint: "http://localhost:11434"}

	answer, err := harness.Run(model, nil, "What is 2+2? Reply with only the number.")
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("answer:", answer)
}
