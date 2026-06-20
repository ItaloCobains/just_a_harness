package agentkit

import (
	"context"
	"strings"
	"testing"
)

func runBash(t *testing.T, cmd string) string {
	t.Helper()
	out, err := runBashTool().Func(context.Background(), `{"cmd":`+jsonString(cmd)+`}`)
	if err != nil {
		t.Fatalf("run_bash returned a Go error (should surface exit as data): %v", err)
	}
	return out
}

func jsonString(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func TestRunBashEmptyOutputReportsSuccess(t *testing.T) {
	out := runBash(t, "true")
	if !strings.Contains(out, "exit 0") || !strings.Contains(out, "no output") {
		t.Fatalf("empty success should report status, got %q", out)
	}
}

func TestRunBashFailureKeepsOutputAndStatus(t *testing.T) {
	out := runBash(t, "echo boom >&2; exit 3")
	if !strings.Contains(out, "boom") {
		t.Fatalf("stderr lost: %q", out)
	}
	if !strings.Contains(out, "exit status 3") {
		t.Fatalf("exit status lost: %q", out)
	}
}

func TestRunBashNormalOutput(t *testing.T) {
	out := runBash(t, "echo hello")
	if !strings.Contains(out, "hello") || !strings.Contains(out, "exit 0") {
		t.Fatalf("unexpected output: %q", out)
	}
}
