package agentkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"harness/agent"
)

const webFetchLimit = 10_000

const SystemPrompt = `You are a coding assistant working in the current directory.
You have tools: read_file, list_dir, write_file, edit_file, run_bash, grep, glob, web_search, web_fetch, task.

To use a tool, reply with ONLY a single JSON object and nothing else:
{"name": "<tool>", "arguments": { ... }}

Do not describe the call in prose. Do not wrap it in code fences. Emit only the JSON.
Use a tool ONLY when the user's request requires reading, searching, or changing files,
or searching/fetching the web. You CAN access the internet: call web_search with a
query to find pages when the user asks about something online, then web_fetch a result
URL to read it. Use web_fetch directly when the user gives a link.
For greetings, small talk, or questions you can answer from this conversation, reply in
plain text and do not call any tool. Never edit files unless the user explicitly asks for it.
After a tool result comes back, decide the next tool call or give your final answer.
To create a NEW file, use write_file; it creates any missing parent directories, so
you do not need mkdir first. edit_file only modifies an EXISTING file and needs an
old_string that appears exactly once; never call edit_file with an empty old_string.
Inspect files before changing them. Prefer edit_file over write_file for existing files.
Use grep and glob to search instead of shelling out. Delegate broad searches to task.
Make the smallest change that satisfies the request.
Never claim you created or changed a file unless a tool call returned success; if a tool
keeps failing, say so plainly instead of pretending it worked.
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
func CodingTools(model agent.Model) []agent.Tool {
	return []agent.Tool{
		readFileTool(),
		listDirTool(),
		writeFileTool(),
		editFileTool(),
		runBashTool(),
		grepTool(),
		globTool(),
		webSearchTool(),
		webFetchTool(),
		taskTool(model),
	}
}

func readFileTool() agent.Tool {
	return agent.Tool{
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

func listDirTool() agent.Tool {
	return agent.Tool{
		Name:        "list_dir",
		Description: "List the entries (files and directories) in a directory.",
		Schema:      objectSchema("path", "the directory path to list"),
		Func: func(_ context.Context, input string) (string, error) {
			entries, err := os.ReadDir(arg(input, "path"))
			if err != nil {
				return "", err
			}
			if len(entries) == 0 {
				return "(empty directory)", nil
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

func writeFileTool() agent.Tool {
	return agent.Tool{
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
			if dir := filepath.Dir(path); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return "", err
				}
			}
			if err := os.WriteFile(path, []byte(arg(input, "content")), 0o644); err != nil {
				return "", err
			}
			return "wrote " + path, nil
		},
	}
}

func editFileTool() agent.Tool {
	return agent.Tool{
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

func runBashTool() agent.Tool {
	return agent.Tool{
		Name:        "run_bash",
		Description: "Run a bash command in the working directory and return its combined output.",
		Schema:      objectSchema("cmd", "the bash command to run"),
		Func: func(ctx context.Context, input string) (string, error) {
			out, err := exec.CommandContext(ctx, "bash", "-c", arg(input, "cmd")).CombinedOutput()
			// Treat a non-zero exit as data, not a Go error, so the command's
			// own output reaches the model instead of being swapped for a bare
			// "error: exit status 1" by the agent loop.
			status := "exit 0"
			if err != nil {
				status = err.Error()
			}
			text := strings.TrimRight(string(out), "\n")
			if text == "" {
				return "(" + status + ", no output)", nil
			}
			return text + "\n(" + status + ")", nil
		},
	}
}

func grepTool() agent.Tool {
	return agent.Tool{
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

func globTool() agent.Tool {
	return agent.Tool{
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

func webFetchTool() agent.Tool {
	return agent.Tool{
		Name:        "web_fetch",
		Description: "Fetch a web page over HTTP(S) and return its text content (HTML stripped, truncated).",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"url":     map[string]any{"type": "string", "description": "the http or https URL to fetch"},
				"timeout": map[string]any{"type": "integer", "description": "request timeout in seconds (optional, default 10)"},
			},
			"required": []string{"url"},
		},
		Func: func(ctx context.Context, input string) (string, error) {
			raw := arg(input, "url")
			if raw == "" {
				return "", ErrMissingArg
			}
			u, err := url.Parse(raw)
			if err != nil {
				return "", err
			}
			if u.Scheme != "http" && u.Scheme != "https" {
				return "", fmt.Errorf("unsupported url scheme %q", u.Scheme)
			}

			timeout := 10 * time.Second
			if secs := argInt(input, "timeout"); secs > 0 {
				timeout = time.Duration(secs) * time.Second
			}
			client := &http.Client{Timeout: timeout}

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
			if err != nil {
				return "", err
			}
			resp, err := client.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 4*webFetchLimit))
			if err != nil {
				return "", err
			}
			return htmlToText(string(body)), nil
		},
	}
}

const (
	ddgEndpoint       = "https://html.duckduckgo.com/html/"
	ddgUserAgent      = "Mozilla/5.0 (compatible; harness/1.0)"
	defaultSearchHits = 8
)

func webSearchTool() agent.Tool {
	return agent.Tool{
		Name:        "web_search",
		Description: "Search the web via DuckDuckGo and return result titles, URLs, and snippets. Use web_fetch afterwards to read a result.",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "the search query"},
				"count": map[string]any{"type": "integer", "description": "max results (optional, default 8)"},
			},
			"required": []string{"query"},
		},
		Func: func(ctx context.Context, input string) (string, error) {
			query := arg(input, "query")
			if query == "" {
				return "", ErrMissingArg
			}
			count := argInt(input, "count")
			if count <= 0 {
				count = defaultSearchHits
			}

			form := url.Values{"q": {query}}
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, ddgEndpoint, strings.NewReader(form.Encode()))
			if err != nil {
				return "", err
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("User-Agent", ddgUserAgent)

			resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			if err != nil {
				return "", err
			}
			results := parseDuckDuckGo(string(body))
			if len(results) == 0 {
				return "no results", nil
			}
			if len(results) > count {
				results = results[:count]
			}
			return formatResults(results), nil
		},
	}
}

type searchResult struct {
	Title, URL, Snippet string
}

var (
	ddgResult  = regexp.MustCompile(`(?s)<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>(.*?)</a>`)
	ddgSnippet = regexp.MustCompile(`(?s)<a[^>]+class="result__snippet"[^>]*>(.*?)</a>`)
)

// parseDuckDuckGo extracts results from a DuckDuckGo HTML response. Result links
// point at a /l/?uddg=<encoded> redirect, so the real URL is decoded out of it.
func parseDuckDuckGo(body string) []searchResult {
	links := ddgResult.FindAllStringSubmatch(body, -1)
	snippets := ddgSnippet.FindAllStringSubmatch(body, -1)

	results := make([]searchResult, 0, len(links))
	for i, m := range links {
		snippet := ""
		if i < len(snippets) {
			snippet = cleanText(snippets[i][1])
		}
		results = append(results, searchResult{
			Title:   cleanText(m[2]),
			URL:     decodeDDGURL(m[1]),
			Snippet: snippet,
		})
	}
	return results
}

func decodeDDGURL(href string) string {
	href = html.UnescapeString(href)
	if u, err := url.Parse(href); err == nil {
		if real := u.Query().Get("uddg"); real != "" {
			return real
		}
	}
	return href
}

func cleanText(s string) string {
	s = htmlTag.ReplaceAllString(s, "")
	return strings.TrimSpace(html.UnescapeString(s))
}

func formatResults(results []searchResult) string {
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s\n   %s\n", i+1, r.Title, r.URL)
		if r.Snippet != "" {
			fmt.Fprintf(&b, "   %s\n", r.Snippet)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func argInt(input, key string) int {
	var m map[string]any
	json.Unmarshal([]byte(input), &m)
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

var (
	htmlScriptStyle = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlTag         = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlSpace       = regexp.MustCompile(`[ \t]*\n[ \t\n]*`)
)

// htmlToText strips tags from HTML for a readable, truncated plain-text view.
func htmlToText(s string) string {
	s = htmlScriptStyle.ReplaceAllString(s, "")
	s = htmlTag.ReplaceAllString(s, " ")
	s = htmlSpace.ReplaceAllString(s, "\n")
	s = strings.TrimSpace(s)
	if len(s) > webFetchLimit {
		s = s[:webFetchLimit] + "\n\n[truncated]"
	}
	return s
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
