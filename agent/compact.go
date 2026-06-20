package agent

import "context"

// compactThreshold is the message count above which Converse summarises the
// older middle of the conversation to keep the context window from overflowing.
const compactThreshold = 40

// keepRecent is how many of the most recent messages survive compaction intact.
const keepRecent = 12

const summaryPrompt = `Summarise the conversation so far into a concise note that preserves
decisions, file paths, and open tasks. Reply with the summary text only, no preamble.`

// Compact summarises the old middle of history via the model, preserving the
// leading system message and the most recent turns. If history is short or the
// summary call fails, it returns history unchanged.
func Compact(ctx context.Context, model Model, history []Message) []Message {
	if len(history) <= compactThreshold {
		return history
	}

	head := 0
	if len(history) > 0 && history[0].Role == "system" {
		head = 1
	}
	cut := len(history) - keepRecent
	if cut <= head {
		return history
	}

	middle := history[head:cut]
	convo := append([]Message{}, middle...)
	convo = append(convo, Message{Role: "user", Text: summaryPrompt})

	step, err := model.Next(ctx, convo, nil, nil)
	if err != nil || !step.Done || step.Text == "" {
		return history
	}

	out := make([]Message, 0, head+1+keepRecent)
	out = append(out, history[:head]...)
	out = append(out, Message{Role: "system", Text: "Summary of earlier conversation:\n" + step.Text})
	out = append(out, history[cut:]...)
	return out
}
