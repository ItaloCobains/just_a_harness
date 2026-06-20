package agent

import (
	"context"
	"errors"
	"fmt"
)

const maxTurns = 25

var ErrMaxTurns = errors.New("harness: max turns exceeded")

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

	for range maxTurns {
		if err := ctx.Err(); err != nil {
			return history, "", err
		}

		history = Compact(ctx, model, history)

		step, err := model.Next(ctx, history, tools, h.Delta)
		if err != nil {
			return history, "", err
		}

		if len(step.ToolCalls) > 0 {
			history = append(history, Message{Role: "assistant", ToolCalls: step.ToolCalls})
			results := runTools(ctx, byName, step.ToolCalls, h)
			for _, r := range results {
				history = append(history, Message{Role: "tool", ToolID: r.call.ID, Text: r.output})
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
		return fmt.Sprintf("error: unknown tool %q", call.Name)
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

func Run(model Model, tools []Tool, system, input string) (string, error) {
	var history []Message
	if system != "" {
		history = append(history, Message{Role: "system", Text: system})
	}
	history = append(history, Message{Role: "user", Text: input})

	_, answer, err := Converse(context.Background(), model, tools, history, Hooks{})
	return answer, err
}
