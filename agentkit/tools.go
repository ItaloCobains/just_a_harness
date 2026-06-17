package agentkit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"harness"
)

const SystemPrompt = `You are a coding assistant working in the current directory.
You have tools: read_file, list_dir, write_file, run_bash.

To use a tool, reply with ONLY a single JSON object and nothing else:
{"name": "<tool>", "arguments": { ... }}

Do not describe the call in prose. Do not wrap it in code fences. Emit only the JSON.
After a tool result comes back, decide the next tool call or give your final answer.
Inspect files before changing them. Make the smallest change that satisfies the request.
Only when the task is fully complete, reply with a plain-text summary (no JSON).`

func arg(input, key string) string {
	var m map[string]any
	json.Unmarshal([]byte(input), &m)
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// CodingTools returns the file-system and shell tools for a coding agent.
func CodingTools() []harness.Tool {
	return []harness.Tool{
		{
			Name:        "read_file",
			Description: "Read the full contents of a file at the given path.",
			Schema:      objectSchema("path", "the file path to read"),
			Func: func(input string) string {
				data, err := os.ReadFile(arg(input, "path"))
				if err != nil {
					return "error: " + err.Error()
				}
				return string(data)
			},
		},
		{
			Name:        "list_dir",
			Description: "List the entries (files and directories) in a directory.",
			Schema:      objectSchema("path", "the directory path to list"),
			Func: func(input string) string {
				entries, err := os.ReadDir(arg(input, "path"))
				if err != nil {
					return "error: " + err.Error()
				}
				names := make([]string, 0, len(entries))
				for _, e := range entries {
					name := e.Name()
					if e.IsDir() {
						name += "/"
					}
					names = append(names, name)
				}
				return strings.Join(names, "\n")
			},
		},
		{
			Name:        "write_file",
			Description: "Write content to a file, creating or overwriting it.",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string", "description": "the file path to write"},
					"content": map[string]any{"type": "string", "description": "the full content to write"},
				},
				"required": []string{"path", "content"},
			},
			Func: func(input string) string {
				path := arg(input, "path")
				if err := os.WriteFile(path, []byte(arg(input, "content")), 0o644); err != nil {
					return "error: " + err.Error()
				}
				return "wrote " + path
			},
		},
		{
			Name:        "run_bash",
			Description: "Run a bash command in the working directory and return its combined output.",
			Schema:      objectSchema("cmd", "the bash command to run"),
			Func: func(input string) string {
				out, err := exec.Command("bash", "-c", arg(input, "cmd")).CombinedOutput()
				if err != nil {
					return fmt.Sprintf("exit error: %v\n%s", err, out)
				}
				return string(out)
			},
		},
	}
}

func objectSchema(name, desc string) map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			name: map[string]any{"type": "string", "description": desc},
		},
		"required": []string{name},
	}
}
