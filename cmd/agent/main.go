package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"harness"
	"harness/agentkit"
)

func main() {
	task := strings.Join(os.Args[1:], " ")
	if task == "" {
		task = "List the files in the current directory and tell me what this project does."
	}

	model := harness.OllamaModel{Model: "qwen2.5-coder:7b", Endpoint: "http://localhost:11434"}

	approver := agentkit.LoadApprover()
	tools := agentkit.CodingTools(model)
	for i, tool := range tools {
		if agentkit.Mutating[tool.Name] {
			tools[i] = requireApproval(tool, approver, os.Stdin, os.Stdout)
		}
	}

	// Ctrl-C cancela a sessão inteira (aborta HTTP e tools em andamento).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	history := []harness.Message{
		{Role: "system", Text: agentkit.BuildSystemPrompt()},
		{Role: "user", Text: task},
	}

	_, answer, err := harness.Converse(ctx, model, tools, history, harness.Hooks{
		Observe: func(e harness.Event) {
			fmt.Printf("🔧 %s(%s)\n", e.Tool, truncate(e.Input, 80))
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("\n=== answer ===")
	fmt.Println(answer)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
