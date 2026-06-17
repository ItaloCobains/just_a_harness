package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"harness"
)

func arg(input, key string) string {
	var m map[string]any
	json.Unmarshal([]byte(input), &m)
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func logCall(name, input string) {
	fmt.Fprintf(os.Stderr, "[%s] %s\n", name, input)
}

func codingTools() []harness.Tool {
	return []harness.Tool{
		{
			Name:        "read_file",
			Description: "Read the full contents of a file at the given path.",
			Schema:      objectSchema("path", "the file path to read"),
			Func: func(input string) string {
				logCall("read_file", input)
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
				logCall("list_dir", input)
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
				logCall("write_file", input)
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
				logCall("run_bash", input)
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
