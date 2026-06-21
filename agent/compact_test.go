package agent

import (
	"context"
	"strings"
	"testing"
)

// each message carries ~200 chars so token estimates are meaningful.
var msgText = strings.Repeat("x", 200)

func bigHistory(n int) []Message {
	h := []Message{{Role: "system", Text: "sys"}}
	for i := 0; i < n; i++ {
		h = append(h, Message{Role: "user", Text: msgText})
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
	// 300 messages * ~200 chars / 4 ≈ 15000 tokens, over the budget.
	h := bigHistory(300)
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

func TestCapResultTruncatesLargeOutput(t *testing.T) {
	out := capResult(strings.Repeat("a", maxToolResult+500))
	if len(out) >= maxToolResult+500 {
		t.Fatalf("output not truncated: len %d", len(out))
	}
	if !strings.Contains(out, "truncated") {
		t.Fatalf("missing truncation marker: %q", out[len(out)-40:])
	}
}

func TestCapResultLeavesSmallOutput(t *testing.T) {
	if got := capResult("small"); got != "small" {
		t.Fatalf("small output changed: %q", got)
	}
}
