package agent

import (
	"context"
	"testing"
)

func bigHistory(n int) []Message {
	h := []Message{{Role: "system", Text: "sys"}}
	for i := 0; i < n; i++ {
		h = append(h, Message{Role: "user", Text: "msg"})
	}
	return h
}

func TestCompactLeavesShortHistoryUntouched(t *testing.T) {
	h := bigHistory(3)
	model := &FakeModel{steps: []Step{{Done: true, Text: "summary"}}}

	out := Compact(context.Background(), model, h)
	if len(out) != len(h) || model.calls != 0 {
		t.Fatalf("short history must be untouched; calls=%d len=%d", model.calls, len(out))
	}
}

func TestCompactSummarisesOldMiddle(t *testing.T) {
	h := bigHistory(compactThreshold + 5)
	model := &FakeModel{steps: []Step{{Done: true, Text: "the summary"}}}

	out := Compact(context.Background(), model, h)

	if out[0].Role != "system" || out[0].Text != "sys" {
		t.Fatalf("system prompt must survive, got %+v", out[0])
	}
	if out[1].Role != "system" || out[1].Text != "Summary of earlier conversation:\nthe summary" {
		t.Fatalf("expected summary block, got %+v", out[1])
	}
	if len(out) != 2+keepRecent {
		t.Fatalf("len = %d, want %d", len(out), 2+keepRecent)
	}
}
