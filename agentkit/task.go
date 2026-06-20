package agentkit

import (
	"context"

	"harness/agent"
)

const subagentPrompt = `You are a search subagent. Use read_file, list_dir, grep, and glob to
investigate the codebase and answer the task. You cannot modify files. When done,
reply with a concise plain-text answer.`

// taskTool spawns a fresh, read-only Converse to answer a focused sub-question,
// isolating its exploration from the main conversation's context.
func taskTool(model agent.Model) agent.Tool {
	return agent.Tool{
		Name:        "task",
		Description: "Delegate a focused search/investigation to a read-only subagent. Returns its final answer.",
		Schema:      objectSchema("prompt", "the task for the subagent to investigate"),
		Func: func(ctx context.Context, input string) (string, error) {
			prompt := arg(input, "prompt")
			if prompt == "" {
				return "", ErrMissingArg
			}
			history := []agent.Message{
				{Role: "system", Text: subagentPrompt},
				{Role: "user", Text: prompt},
			}
			_, answer, err := agent.Converse(ctx, model, ReadOnlyTools(), history, agent.Hooks{})
			if err != nil {
				return "", err
			}
			return answer, nil
		},
	}
}

// ReadOnlyTools returns the subset of tools that never mutate the filesystem.
func ReadOnlyTools() []agent.Tool {
	return []agent.Tool{
		readFileTool(),
		listDirTool(),
		grepTool(),
		globTool(),
	}
}
