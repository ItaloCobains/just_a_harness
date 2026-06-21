// Package ollama implements agent.Model against an Ollama HTTP endpoint.
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"harness/agent"
)

const defaultBackoffBase = 200 * time.Millisecond
const defaultIdleTimeout = 90 * time.Second

// errStreamStalled is returned when the server stops sending data mid-stream.
var errStreamStalled = errors.New("ollama: stream stalled")

// StreamingClient builds an HTTP client suited to a streaming chat endpoint.
// connectTimeout bounds dialing and the wait for response headers (so a dead or
// unresponsive server still fails fast), but the client has NO total timeout:
// a long generation must not be killed mid-stream while tokens are flowing.
// Callers cancel via the request context (e.g. Ctrl+C) instead.
func StreamingClient(connectTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: connectTimeout}).DialContext,
			ResponseHeaderTimeout: connectTimeout,
		},
	}
}

// Model talks to an Ollama server's /api/chat endpoint.
type Model struct {
	Model       string
	Endpoint    string
	HTTPClient  *http.Client
	MaxRetries  int
	BackoffBase time.Duration
	IdleTimeout time.Duration // max gap between streamed chunks before aborting
	Temperature float64       // sampling temperature; low values steady tool calls
}

// New builds a Model for the given model name and endpoint with built-in
// network-resilience defaults.
func New(model, endpoint string) Model {
	return Model{
		Model:       model,
		Endpoint:    endpoint,
		HTTPClient:  StreamingClient(120 * time.Second),
		MaxRetries:  3,
		BackoffBase: defaultBackoffBase,
		IdleTimeout: defaultIdleTimeout,
	}
}

func (m Model) idleTimeout() time.Duration {
	if m.IdleTimeout > 0 {
		return m.IdleTimeout
	}
	return defaultIdleTimeout
}

func (m Model) client() *http.Client {
	if m.HTTPClient != nil {
		return m.HTTPClient
	}
	return http.DefaultClient
}

func (m Model) backoffBase() time.Duration {
	if m.BackoffBase > 0 {
		return m.BackoffBase
	}
	return defaultBackoffBase
}

// doRequest POSTs body to /api/chat, retrying transient failures (transport
// errors and 5xx responses) with exponential backoff. Context cancellation and
// 4xx responses abort immediately without a retry. The final failure is wrapped
// with agent.ErrUpstreamUnavailable.
func (m Model) doRequest(ctx context.Context, body []byte) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= m.MaxRetries; attempt++ {
		if attempt > 0 {
			delay := m.backoffBase() << (attempt - 1)
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint+"/api/chat", bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := m.client().Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			continue
		}
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
			continue
		}
		if resp.StatusCode >= 400 {
			resp.Body.Close()
			return nil, fmt.Errorf("ollama: status %d", resp.StatusCode)
		}
		return resp, nil
	}
	return nil, fmt.Errorf("%w: %v", agent.ErrUpstreamUnavailable, lastErr)
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
		"options":  map[string]any{"temperature": m.Temperature},
	}
	if len(tools) > 0 {
		defs := make([]map[string]any, 0, len(tools))
		for _, tool := range tools {
			defs = append(defs, ollamaToolDef(tool))
		}
		payload["tools"] = defs
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return agent.Step{}, err
	}

	resp, err := m.doRequest(ctx, body)
	if err != nil {
		return agent.Step{}, err
	}
	defer resp.Body.Close()

	return parseStream(resp.Body, m.idleTimeout(), onDelta)
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
	Done            bool  `json:"done"`
	PromptEvalCount int   `json:"prompt_eval_count"`
	EvalCount       int   `json:"eval_count"`
	EvalDuration    int64 `json:"eval_duration"`
}

func parseStream(r io.Reader, idle time.Duration, onDelta func(string)) (agent.Step, error) {
	var content strings.Builder
	var calls []agent.ToolCall

	// Some models (qwen2.5-coder) emit a tool call as a JSON object in the
	// content channel. Withhold streaming until the first non-empty text
	// reveals whether this turn is prose or a tool call, so the raw JSON never
	// leaks into the UI.
	decided, suppress := false, false
	var usage agent.Step // accumulates token counts reported in the final chunk
	emit := func(s string) {
		if onDelta != nil && !suppress {
			onDelta(s)
		}
	}

	process := func(line []byte) error {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			return nil
		}
		var chunk chatChunk
		if err := json.Unmarshal(line, &chunk); err != nil {
			return err
		}
		if chunk.PromptEvalCount > 0 {
			usage.PromptTokens = chunk.PromptEvalCount
		}
		if chunk.EvalCount > 0 {
			usage.EvalTokens = chunk.EvalCount
		}
		if chunk.EvalDuration > 0 {
			usage.EvalNanos = chunk.EvalDuration
		}
		if chunk.Message.Content != "" {
			content.WriteString(chunk.Message.Content)
			if !decided {
				if trimmed := strings.TrimSpace(content.String()); trimmed != "" {
					decided = true
					suppress = strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "```")
					emit(content.String()) // flush what buffered before the decision
				}
			} else {
				emit(chunk.Message.Content)
			}
		}
		for _, tc := range chunk.Message.ToolCalls {
			calls = append(calls, agent.ToolCall{
				ID:    "call_" + strconv.Itoa(len(calls)),
				Name:  tc.Function.Name,
				Input: string(tc.Function.Arguments),
			})
		}
		return nil
	}

	if err := scanLines(r, idle, process); err != nil {
		return agent.Step{}, err
	}

	if len(calls) > 0 {
		usage.ToolCalls = calls
		return usage, nil
	}

	// Alguns modelos (qwen2.5-coder) emitem a chamada como texto no content
	// em vez de usar o canal nativo. Tolera esse formato.
	if call, ok := toolCallFromText(content.String()); ok {
		usage.ToolCalls = []agent.ToolCall{call}
		return usage, nil
	}

	usage.Done = true
	usage.Text = content.String()
	return usage, nil
}

// scanLines reads NDJSON lines from r, calling process for each. When idle > 0,
// it aborts with errStreamStalled if no line arrives within that window, so a
// server that goes silent mid-generation does not hang the caller forever.
func scanLines(r io.Reader, idle time.Duration, process func([]byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	if idle <= 0 {
		for scanner.Scan() {
			if err := process(scanner.Bytes()); err != nil {
				return err
			}
		}
		return scanner.Err()
	}

	lines := make(chan []byte)
	done := make(chan struct{})
	defer close(done)
	errc := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			b := append([]byte(nil), scanner.Bytes()...)
			select {
			case lines <- b:
			case <-done:
				return
			}
		}
		errc <- scanner.Err()
	}()

	timer := time.NewTimer(idle)
	defer timer.Stop()
	for {
		select {
		case b := <-lines:
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(idle)
			if err := process(b); err != nil {
				return err
			}
		case err := <-errc:
			return err
		case <-timer.C:
			return fmt.Errorf("%w: no data for %s", errStreamStalled, idle)
		}
	}
}

func toolCallFromText(content string) (agent.ToolCall, bool) {
	for _, candidate := range jsonCandidates(content) {
		if call, ok := decodeCall(candidate); ok {
			return call, true
		}
	}
	// Last resort: the model wrote prose and then a bare JSON tool call. Try
	// decoding from each '{' until one yields a valid call. The name+arguments
	// check keeps stray braces in prose from matching.
	for i := 0; i < len(content); i++ {
		if content[i] == '{' {
			if call, ok := decodeCall(content[i:]); ok {
				return call, true
			}
		}
	}
	return agent.ToolCall{}, false
}

// decodeCall reads the first JSON value from s and returns it as a tool call if
// it has a name and arguments. It tolerates trailing junk after the object.
func decodeCall(s string) (agent.ToolCall, bool) {
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.NewDecoder(strings.NewReader(s)).Decode(&call); err != nil {
		return agent.ToolCall{}, false
	}
	if call.Name != "" && len(call.Arguments) > 0 {
		return agent.ToolCall{ID: "call_0", Name: call.Name, Input: string(call.Arguments)}, true
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
			// Unclosed fence: the model opened ```json but never closed it.
			// Keep the remainder so the JSON body is still recoverable.
			candidates = append(candidates, strings.TrimSpace(rest))
			break
		}
		candidates = append(candidates, strings.TrimSpace(rest[:end]))
		rest = rest[end+3:]
	}
	return candidates
}
