package agentkit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"harness"
)

const SystemPrompt = `You are a coding assistant working in the current directory.
You have tools: read_file, list_dir, write_file, edit_file, run_bash, grep, glob, task.

To use a tool, reply with ONLY a single JSON object and nothing else:
{"name": "<tool>", "arguments": { ... }}

Do not describe the call in prose. Do not wrap it in code fences. Emit only the JSON.
Use a tool ONLY when the user's request requires reading, searching, or changing files.
For greetings, small talk, or questions you can answer from this conversation, reply in
plain text and do not call any tool. Never edit files unless the user explicitly asks for it.
After a tool result comes back, decide the next tool call or give your final answer.
Inspect files before changing them. Prefer edit_file over write_file for existing files.
Use grep and glob to search instead of shelling out. Delegate broad searches to task.
Make the smallest change that satisfies the request.
Only when the task is fully complete, reply with a plain-text summary (no JSON).`

var (
	ErrMissingArg = errors.New("missing or empty argument")
	ErrNotFound   = errors.New("old_string not found in file")
	ErrAmbiguous  = errors.New("old_string is not unique in file")
)

func arg(input, key string) string {
	var m map[string]any
	json.Unmarshal([]byte(input), &m)
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// CodingTools returns the file-system and shell tools for a coding agent. The
// model is used by the task tool to spawn read-only search subagents.
func CodingTools(model harness.Model) []harness.Tool {
	return []harness.Tool{
		readFileTool(),
		listDirTool(),
		writeFileTool(),
		editFileTool(),
		runBashTool(),
		grepTool(),
		globTool(),
		taskTool(model),
	}
}

func readFileTool() harness.Tool {
	return harness.Tool{
		Name:        "read_file",
		Description: "Read the full contents of a file at the given path.",
		Schema:      objectSchema("path", "the file path to read"),
		Func: func(_ context.Context, input string) (string, error) {
			data, err := os.ReadFile(arg(input, "path"))
			if err != nil {
				return "", err
			}
			return string(data), nil
		},
	}
}

func listDirTool() harness.Tool {
	return harness.Tool{
		Name:        "list_dir",
		Description: "List the entries (files and directories) in a directory.",
		Schema:      objectSchema("path", "the directory path to list"),
		Func: func(_ context.Context, input string) (string, error) {
			entries, err := os.ReadDir(arg(input, "path"))
			if err != nil {
				return "", err
			}
			names := make([]string, 0, len(entries))
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() {
					name += "/"
				}
				names = append(names, name)
			}
			return strings.Join(names, "\n"), nil
		},
	}
}

func writeFileTool() harness.Tool {
	return harness.Tool{
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
		Func: func(_ context.Context, input string) (string, error) {
			path := arg(input, "path")
			if path == "" {
				return "", ErrMissingArg
			}
			if err := os.WriteFile(path, []byte(arg(input, "content")), 0o644); err != nil {
				return "", err
			}
			return "wrote " + path, nil
		},
	}
}

func editFileTool() harness.Tool {
	return harness.Tool{
		Name:        "edit_file",
		Description: "Replace an exact substring in a file. old_string must appear exactly once.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path":       map[string]any{"type": "string", "description": "the file path to edit"},
				"old_string": map[string]any{"type": "string", "description": "the exact text to replace (must be unique)"},
				"new_string": map[string]any{"type": "string", "description": "the replacement text"},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
		Func: func(_ context.Context, input string) (string, error) {
			return editFile(arg(input, "path"), arg(input, "old_string"), arg(input, "new_string"))
		},
	}
}

func editFile(path, old, new string) (string, error) {
	if path == "" || old == "" {
		return "", ErrMissingArg
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	text := string(data)
	switch strings.Count(text, old) {
	case 0:
		return "", ErrNotFound
	case 1:
		updated := strings.Replace(text, old, new, 1)
		if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
			return "", err
		}
		return "edited " + path, nil
	default:
		return "", ErrAmbiguous
	}
}

func runBashTool() harness.Tool {
	return harness.Tool{
		Name:        "run_bash",
		Description: "Run a bash command in the working directory and return its combined output.",
		Schema:      objectSchema("cmd", "the bash command to run"),
		Func: func(ctx context.Context, input string) (string, error) {
			out, err := exec.CommandContext(ctx, "bash", "-c", arg(input, "cmd")).CombinedOutput()
			if err != nil {
				return string(out), err
			}
			return string(out), nil
		},
	}
}

func grepTool() harness.Tool {
	return harness.Tool{
		Name:        "grep",
		Description: "Search files for a regular expression. Returns path:line:text matches.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"pattern": map[string]any{"type": "string", "description": "the regular expression to search for"},
				"path":    map[string]any{"type": "string", "description": "directory to search (defaults to .)"},
			},
			"required": []string{"pattern"},
		},
		Func: func(_ context.Context, input string) (string, error) {
			return grep(arg(input, "pattern"), arg(input, "path"))
		},
	}
}

func grep(pattern, root string) (string, error) {
	if pattern == "" {
		return "", ErrMissingArg
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "", err
	}
	if root == "" {
		root = "."
	}

	var matches []string
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || isBinary(data) {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if re.MatchString(line) {
				matches = append(matches, path+":"+strconv.Itoa(i+1)+":"+line)
			}
		}
		return nil
	})
	if walkErr != nil {
		return "", walkErr
	}
	if len(matches) == 0 {
		return "no matches", nil
	}
	return strings.Join(matches, "\n"), nil
}

func globTool() harness.Tool {
	return harness.Tool{
		Name:        "glob",
		Description: "List file paths matching a glob pattern (e.g. **/*.go via cmd/*.go style).",
		Schema:      objectSchema("pattern", "the glob pattern to match"),
		Func: func(_ context.Context, input string) (string, error) {
			pattern := arg(input, "pattern")
			if pattern == "" {
				return "", ErrMissingArg
			}
			paths, err := filepath.Glob(pattern)
			if err != nil {
				return "", err
			}
			if len(paths) == 0 {
				return "no matches", nil
			}
			sort.Strings(paths)
			return strings.Join(paths, "\n"), nil
		},
	}
}

func isBinary(data []byte) bool {
	n := min(len(data), 8000)
	for _, b := range data[:n] {
		if b == 0 {
			return true
		}
	}
	return false
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
