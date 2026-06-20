package main

import (
	"context"
	"flag"
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
	resume := flag.String("resume", "", "resume a saved session: a name, or \"latest\"")
	flag.Parse()

	task := strings.Join(flag.Args(), " ")
	if task == "" {
		task = "List the files in the current directory and tell me what this project does."
	}

	cfg := config.Load()
	model := ollama.New(cfg.OllamaModel, cfg.OllamaEndpoint)
	model.HTTPClient = ollama.StreamingClient(cfg.HTTPTimeout)
	model.MaxRetries = cfg.HTTPMaxRetries

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

	history := loadHistory(*resume)
	history = append(history, agent.Message{Role: "user", Text: task})

	out, answer, err := agent.Converse(ctx, model, tools, history, agent.Hooks{
		Observe: func(e agent.Event) {
			fmt.Printf("🔧 %s(%s)\n", e.Tool, termui.Truncate(e.Input, 80))
		},
	})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if name := *resume; name != "" {
		if name == "latest" {
			name, _, _ = agentkit.LatestSession()
		}
		if name != "" {
			agentkit.SaveSession(name, out)
		}
	}
	fmt.Println("\n=== answer ===")
	fmt.Println(answer)
}

// loadHistory returns a resumed session's messages, or a fresh system prompt.
func loadHistory(resume string) []agent.Message {
	if resume != "" {
		var (
			h   []agent.Message
			err error
		)
		if resume == "latest" {
			_, h, err = agentkit.LatestSession()
		} else {
			h, err = agentkit.LoadSession(resume)
		}
		if err == nil {
			return h
		}
		fmt.Println("resume failed:", err)
	}
	return []agent.Message{{Role: "system", Text: agentkit.BuildSystemPrompt()}}
}
