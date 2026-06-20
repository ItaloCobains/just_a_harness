// Package ollama implements agent.Model against an Ollama HTTP endpoint.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"harness/agent"
)

// Model talks to an Ollama server's /api/chat endpoint.
type Model struct {
	Model    string
	Endpoint string
}

// New builds a Model for the given model name and endpoint.
func New(model, endpoint string) Model {
	return Model{Model: model, Endpoint: endpoint}
}

func ollamaToolDef(tool agent.Tool) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Schema,
		},
	}
}

func (m Model) Next(ctx context.Context, messages []agent.Message, tools []agent.Tool, onDelta func(string)) (agent.Step, error) {
	chat := make([]map[string]any, 0, len(messages))
	for _, msg := range messages {
		entry := map[string]any{"role": msg.Role, "content": msg.Text}
		if len(msg.ToolCalls) > 0 {
			calls := make([]map[string]any, 0, len(msg.ToolCalls))
			for _, c := range msg.ToolCalls {
				calls = append(calls, map[string]any{"function": map[string]any{
					"name":      c.Name,
					"arguments": json.RawMessage(c.Input),
				}})
			}
			entry["tool_calls"] = calls
		}
		chat = append(chat, entry)
	}

	payload := map[string]any{
		"model":    m.Model,
		"messages": chat,
		"stream":   true,
	}
	if len(tools) > 0 {
		defs := make([]map[string]any, 0, len(tools))
		for _, tool := range tools {
			defs = append(defs, ollamaToolDef(tool))
		}
		payload["tools"] = defs
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return agent.Step{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return agent.Step{}, err
	}
	defer resp.Body.Close()

	return parseStream(resp.Body, onDelta)
}

// chatChunk is one NDJSON line of an Ollama streaming response.
type chatChunk struct {
	Message struct {
		Content   string `json:"content"`
		ToolCalls []struct {
			Function struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"function"`
		} `json:"tool_calls"`
	} `json:"message"`
	Done bool `json:"done"`
}

func parseStream(r io.Reader, onDelta func(string)) (agent.Step, error) {
	var content strings.Builder
	var calls []agent.ToolCall

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var chunk chatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			return agent.Step{}, err
		}
		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if onDelta != nil {
				onDelta(chunk.Message.Content)
			}
		}
		for _, tc := range chunk.Message.ToolCalls {
			calls = append(calls, agent.ToolCall{
				ID:    "call_" + strconv.Itoa(len(calls)),
				Name:  tc.Function.Name,
				Input: string(tc.Function.Arguments),
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return agent.Step{}, err
	}

	if len(calls) > 0 {
		return agent.Step{ToolCalls: calls}, nil
	}

	// Alguns modelos (qwen2.5-coder) emitem a chamada como texto no content
	// em vez de usar o canal nativo. Tolera esse formato.
	if call, ok := toolCallFromText(content.String()); ok {
		return agent.Step{ToolCalls: []agent.ToolCall{call}}, nil
	}

	return agent.Step{Done: true, Text: content.String()}, nil
}

func toolCallFromText(content string) (agent.ToolCall, bool) {
	for _, candidate := range jsonCandidates(content) {
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal([]byte(candidate), &call); err != nil {
			continue
		}
		if call.Name != "" && len(call.Arguments) > 0 {
			return agent.ToolCall{ID: "call_0", Name: call.Name, Input: string(call.Arguments)}, true
		}
	}
	return agent.ToolCall{}, false
}

// jsonCandidates returns the trimmed content plus the inner text of every
// fenced ```...``` block, so an embedded tool call can be recovered from prose.
func jsonCandidates(content string) []string {
	content = strings.TrimSpace(content)
	candidates := []string{content}

	rest := content
	for {
		start := strings.Index(rest, "```")
		if start == -1 {
			break
		}
		rest = rest[start+3:]
		// Drop the fence's language line (```json, ```bash, ```ruby, ...) so the
		// JSON body underneath is left as the candidate. A line that already starts
		// with '{' is the body itself, not a tag.
		if i := strings.IndexByte(rest, '\n'); i != -1 {
			if tag := strings.TrimSpace(rest[:i]); !strings.HasPrefix(tag, "{") {
				rest = rest[i+1:]
			}
		}
		end := strings.Index(rest, "```")
		if end == -1 {
			break
		}
		candidates = append(candidates, strings.TrimSpace(rest[:end]))
		rest = rest[end+3:]
	}
	return candidates
}
