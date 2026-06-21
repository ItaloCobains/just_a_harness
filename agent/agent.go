package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const maxTurns = 25

// maxToolResult caps how many characters of a tool's output are kept in the
// conversation history. A single big read_file or web_fetch would otherwise
// flood a small context window. The UI still observes the full output.
const maxToolResult = 6000

// capResult truncates oversized tool output stored in history, leaving a marker
// so the model knows the result was cut.
func capResult(s string) string {
	if len(s) <= maxToolResult {
		return s
	}
	return s[:maxToolResult] + "\n... [truncated, " + strconv.Itoa(len(s)-maxToolResult) + " more chars]"
}

var (
	ErrMaxTurns            = errors.New("harness: max turns exceeded")
	ErrUpstreamUnavailable = errors.New("harness: upstream unavailable after retries")
	ErrLoop                = errors.New("harness: tool-call loop detected")
)

// loopLimit is how many times the model may repeat the exact same tool-call turn
// before Converse gives up.
const loopLimit = 3

const loopNudge = "You already ran this exact tool call and got the same result. " +
	"Try a different approach or give your final answer."

// ToolCall is a single request from the model to run a tool.
type ToolCall struct {
	ID    string
	Name  string
	Input string
}

type Message struct {
	Role      string
	Text      string
	ToolCalls []ToolCall // assistant turn requesting one or more tools
	ToolID    string     // tool turn: the call this result answers
}

type Step struct {
	Done      bool
	Text      string
	ToolCalls []ToolCall

	// Usage reported by the backend for this step, when available.
	PromptTokens int
	EvalTokens   int
	EvalNanos    int64
}

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Func        func(ctx context.Context, input string) (string, error)
}

// Model produces the next step given the conversation so far. onDelta, when
// non-nil, receives streamed text chunks as they arrive.
type Model interface {
	Next(ctx context.Context, messages []Message, tools []Tool, onDelta func(string)) (Step, error)
}

// Event reports a tool execution so a UI can show activity as it happens.
type Event struct {
	Tool   string
	Input  string
	Result string
}

// Hooks bundles the optional callbacks Converse fires during a run. A nil hook
// is simply skipped. PreTool may veto a call before it runs.
type Hooks struct {
	Observe  func(Event)
	Delta    func(string)
	PreTool  func(call ToolCall) (deny bool, reason string)
	PostTool func(call ToolCall, result string)
	Usage    func(step Step) // fires after each model turn with token counts
}

type toolResult struct {
	call   ToolCall
	output string
}

// Converse runs the agent loop over an existing conversation until the model
// gives a final answer. It returns the extended history (including the final
// assistant turn) so callers can keep a multi-turn chat going.
func Converse(ctx context.Context, model Model, tools []Tool, history []Message, h Hooks) ([]Message, string, error) {
	byName := make(map[string]Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}

	var lastSig string
	repeat := 0

	for range maxTurns {
		if err := ctx.Err(); err != nil {
			return history, "", err
		}

		history = Compact(ctx, model, history)

		step, err := model.Next(ctx, history, tools, h.Delta)
		if err != nil {
			return history, "", err
		}
		if h.Usage != nil {
			h.Usage(step)
		}

		if len(step.ToolCalls) > 0 {
			if sig := callSignature(step.ToolCalls); sig == lastSig {
				repeat++
				if repeat >= loopLimit {
					return history, "", ErrLoop
				}
				if repeat == 1 {
					history = append(history, Message{Role: "system", Text: loopNudge})
				}
			} else {
				lastSig, repeat = sig, 0
			}

			history = append(history, Message{Role: "assistant", ToolCalls: step.ToolCalls})
			results := runTools(ctx, byName, step.ToolCalls, h)
			for _, r := range results {
				history = append(history, Message{Role: "tool", ToolID: r.call.ID, Text: capResult(r.output)})
				if h.Observe != nil {
					h.Observe(Event{Tool: r.call.Name, Input: r.call.Input, Result: r.output})
				}
			}
			continue
		}

		if step.Done {
			history = append(history, Message{Role: "assistant", Text: step.Text})
			return history, step.Text, nil
		}
	}

	return history, "", ErrMaxTurns
}

// runTools executes the requested calls concurrently and returns their results
// in the original order, so the model sees a deterministic transcript.
func runTools(ctx context.Context, byName map[string]Tool, calls []ToolCall, h Hooks) []toolResult {
	results := make([]toolResult, len(calls))
	done := make(chan int, len(calls))

	for i, call := range calls {
		go func(i int, call ToolCall) {
			results[i] = toolResult{call: call, output: runOne(ctx, byName, call, h)}
			done <- i
		}(i, call)
	}
	for range calls {
		<-done
	}
	return results
}

func runOne(ctx context.Context, byName map[string]Tool, call ToolCall, h Hooks) string {
	if h.PreTool != nil {
		if deny, reason := h.PreTool(call); deny {
			return "denied: " + reason
		}
	}

	tool, ok := byName[call.Name]
	if !ok {
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)
		return fmt.Sprintf("error: unknown tool %q. Available tools: %s. To run a shell command (compilers, scripts, etc.) use run_bash.",
			call.Name, strings.Join(names, ", "))
	}

	if err := validateInput(tool, call.Input); err != nil {
		return "error: " + err.Error()
	}

	out, err := tool.Func(ctx, call.Input)
	if err != nil {
		out = "error: " + err.Error()
	}
	if h.PostTool != nil {
		h.PostTool(call, out)
	}
	return out
}

// callSignature joins a turn's tool calls into a stable string so repeated,
// identical turns can be detected.
func callSignature(calls []ToolCall) string {
	var b strings.Builder
	for _, c := range calls {
		b.WriteString(c.Name)
		b.WriteByte(0)
		b.WriteString(c.Input)
		b.WriteByte('\n')
	}
	return b.String()
}

// validateInput checks that a tool call supplies every required argument before
// the tool runs, so the model gets an actionable error instead of an opaque
// failure deep inside the tool.
func validateInput(tool Tool, input string) error {
	required := requiredFields(tool.Schema)
	if len(required) == 0 {
		return nil
	}
	var args map[string]any
	if json.Unmarshal([]byte(input), &args) != nil {
		return fmt.Errorf("invalid JSON arguments for %s", tool.Name)
	}
	var missing []string
	for _, key := range required {
		v, ok := args[key]
		if !ok || v == nil || v == "" {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s requires argument(s): %s", tool.Name, strings.Join(missing, ", "))
	}
	return nil
}

// requiredFields reads the JSON-schema "required" list, tolerating both the
// []string literals used in this codebase and the []any form from decoded JSON.
func requiredFields(schema map[string]any) []string {
	switch r := schema["required"].(type) {
	case []string:
		return r
	case []any:
		out := make([]string, 0, len(r))
		for _, v := range r {
			if s, ok := v.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func Run(model Model, tools []Tool, system, input string) (string, error) {
	var history []Message
	if system != "" {
		history = append(history, Message{Role: "system", Text: system})
	}
	history = append(history, Message{Role: "user", Text: input})

	_, answer, err := Converse(context.Background(), model, tools, history, Hooks{})
	return answer, err
}
