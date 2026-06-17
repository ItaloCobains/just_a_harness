package main

import (
	"fmt"
	"os"
	"strings"

	"harness"
)

const systemPrompt = `You are a coding assistant working in the current directory.
You have tools: read_file, list_dir, write_file, run_bash.

To use a tool, reply with ONLY a single JSON object and nothing else:
{"name": "<tool>", "arguments": { ... }}

Do not describe the call in prose. Do not wrap it in code fences. Emit only the JSON.
After a tool result comes back, decide the next tool call or give your final answer.
Inspect files before changing them. Make the smallest change that satisfies the request.
Only when the task is fully complete, reply with a plain-text summary (no JSON).`

func main() {
	task := strings.Join(os.Args[1:], " ")
	if task == "" {
		task = "List the files in the current directory and tell me what this project does."
	}

	tools := codingTools()
	mutating := map[string]bool{"write_file": true, "run_bash": true}
	for i, tool := range tools {
		if mutating[tool.Name] {
			tools[i] = requireApproval(tool, os.Stdin, os.Stdout)
		}
	}

	model := harness.OllamaModel{Model: "qwen2.5-coder:7b", Endpoint: "http://localhost:11434"}

	answer, err := harness.Run(model, tools, systemPrompt, task)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("\n=== answer ===")
	fmt.Println(answer)
}
