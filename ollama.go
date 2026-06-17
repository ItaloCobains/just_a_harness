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

func (m OllamaModel) Next(messages []Message) Step {
	chat := make([]map[string]any, 0, len(messages))

	for _, msg := range messages {
		chat = append(chat, map[string]any{"role": msg.Role, "content": msg.Text})
	}

	body, _ := json.Marshal(map[string]any{
		"model":    m.Model,
		"messages": chat,
		"stream":   false,
	})

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
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.Unmarshal(body, &resp); err != nil {
		return Step{}, err
	}

	return Step{Done: true, Text: resp.Message.Content}, nil
}
