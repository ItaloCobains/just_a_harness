package ollama

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
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
	step, err := parseStream(strings.NewReader(ndjson), 0, func(s string) { seen.WriteString(s) })
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

// stallingReader returns one line then blocks forever, simulating a server that
// goes silent mid-stream.
type stallingReader struct {
	line string
	sent bool
	gate chan struct{}
}

func (s *stallingReader) Read(p []byte) (int, error) {
	if !s.sent {
		s.sent = true
		return copy(p, s.line), nil
	}
	<-s.gate // block until the test closes it
	return 0, nil
}

func TestStreamIdleTimeout(t *testing.T) {
	r := &stallingReader{line: chunk("Hel") + "\n", gate: make(chan struct{})}
	defer close(r.gate)

	_, err := parseStream(r, 50*time.Millisecond, nil)
	if !errors.Is(err, errStreamStalled) {
		t.Fatalf("err = %v, want errStreamStalled", err)
	}
}

func TestParseStreamCapturesUsage(t *testing.T) {
	ndjson := strings.Join([]string{
		`{"message":{"role":"assistant","content":"hi"},"done":false}`,
		`{"message":{"role":"assistant","content":""},"done":true,"prompt_eval_count":1200,"eval_count":42,"eval_duration":2000000000}`,
	}, "\n")

	step, err := parseStream(strings.NewReader(ndjson), 0, nil)
	if err != nil {
		t.Fatalf("parseStream: %v", err)
	}
	if step.PromptTokens != 1200 || step.EvalTokens != 42 {
		t.Fatalf("usage = %d/%d, want 1200/42", step.PromptTokens, step.EvalTokens)
	}
	if step.EvalNanos != 2000000000 {
		t.Fatalf("EvalNanos = %d", step.EvalNanos)
	}
}
