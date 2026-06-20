package ollama

import (
	"encoding/json"
	"strings"
	"testing"
)

// chunk renders one streaming NDJSON line carrying a content fragment.
func chunk(content string) string {
	body, _ := json.Marshal(map[string]any{
		"message": map[string]any{"role": "assistant", "content": content},
		"done":    false,
	})
	return string(body)
}

func collectDeltas(t *testing.T, ndjson string) (string, string) {
	t.Helper()
	var seen strings.Builder
	step, err := parseStream(strings.NewReader(ndjson), func(s string) { seen.WriteString(s) })
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	return seen.String(), step.Text
}

func TestStreamSuppressesJSONToolCall(t *testing.T) {
	stream := chunk(`{"name":"web_fetch",`) + "\n" + chunk(`"arguments":{"url":"https://x"}}`) + "\n"

	seen, _ := collectDeltas(t, stream)
	if seen != "" {
		t.Fatalf("streamed tool-call JSON to UI: %q", seen)
	}
}

func TestStreamEmitsProse(t *testing.T) {
	stream := chunk("Hello") + "\n" + chunk(", world") + "\n"

	seen, text := collectDeltas(t, stream)
	if seen != "Hello, world" {
		t.Fatalf("deltas = %q, want full prose", seen)
	}
	if text != "Hello, world" {
		t.Fatalf("text = %q", text)
	}
}
