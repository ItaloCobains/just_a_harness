package main

import (
	"fmt"
	"os"
	"strings"

	"harness"
	"harness/agentkit"
)

func main() {
	task := strings.Join(os.Args[1:], " ")
	if task == "" {
		task = "List the files in the current directory and tell me what this project does."
	}

	tools := agentkit.CodingTools()
	mutating := map[string]bool{"write_file": true, "run_bash": true}
	for i, tool := range tools {
		if mutating[tool.Name] {
			tools[i] = requireApproval(tool, os.Stdin, os.Stdout)
		}
	}

	model := harness.OllamaModel{Model: "qwen2.5-coder:7b", Endpoint: "http://localhost:11434"}

	answer, err := harness.Run(model, tools, agentkit.SystemPrompt, task)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("\n=== answer ===")
	fmt.Println(answer)
}
