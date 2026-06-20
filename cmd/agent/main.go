package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"harness/agent"
	"harness/agentkit"
	"harness/config"
	"harness/internal/termui"
	"harness/model/ollama"
)

func main() {
	task := strings.Join(os.Args[1:], " ")
	if task == "" {
		task = "List the files in the current directory and tell me what this project does."
	}

	cfg := config.Load()
	model := ollama.New(cfg.OllamaModel, cfg.OllamaEndpoint)

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

	history := []agent.Message{
		{Role: "system", Text: agentkit.BuildSystemPrompt()},
		{Role: "user", Text: task},
	}

	_, answer, err := agent.Converse(ctx, model, tools, history, agent.Hooks{
		Observe: func(e agent.Event) {
			fmt.Printf("🔧 %s(%s)\n", e.Tool, termui.Truncate(e.Input, 80))
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("\n=== answer ===")
	fmt.Println(answer)
}
