package harness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

type OllamaModel struct {
	Model    string
	Endpoint string
}

func ollamaToolDef(tool Tool) map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"parameters":  tool.Schema,
		},
	}
}

func (m OllamaModel) Next(messages []Message, tools []Tool) Step {
	chat := make([]map[string]any, 0, len(messages))

	for _, msg := range messages {
		entry := map[string]any{"role": msg.Role, "content": msg.Text}
		if msg.Tool != "" {
			entry["tool_calls"] = []map[string]any{
				{"function": map[string]any{
					"name":      msg.Tool,
					"arguments": json.RawMessage(msg.Input),
				}},
			}
		}
		chat = append(chat, entry)
	}

	payload := map[string]any{
		"model":    m.Model,
		"messages": chat,
		"stream":   false,
	}
	if len(tools) > 0 {
		defs := make([]map[string]any, 0, len(tools))
		for _, tool := range tools {
			defs = append(defs, ollamaToolDef(tool))
		}
		payload["tools"] = defs
	}
	body, _ := json.Marshal(payload)

	resp, err := http.Post(m.Endpoint+"/api/chat", "application/json", bytes.NewReader(body))
	if err != nil {
		return Step{Done: true, Text: "http error: " + err.Error()}
	}

	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	step, err := parseStep(data)
	if err != nil {
		return Step{Done: true, Text: "parse error: " + err.Error()}
	}

	return step
}

func parseStep(body []byte) (Step, error) {
	var resp struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return Step{}, err
	}

	if len(resp.Message.ToolCalls) > 0 {
		call := resp.Message.ToolCalls[0].Function
		return Step{Tool: call.Name, Input: string(call.Arguments)}, nil
	}

	return Step{Done: true, Text: resp.Message.Content}, nil
}
