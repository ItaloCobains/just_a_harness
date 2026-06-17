package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"harness"
	"harness/agentkit"
)

func okTool(name string, run *bool) harness.Tool {
	return harness.Tool{Name: name, Func: func(_ context.Context, _ string) (string, error) {
		*run = true
		return "ok", nil
	}}
}

func TestRequireApprovalRunsToolOnYes(t *testing.T) {
	ran := false
	gated := requireApproval(okTool("write_file", &ran), &agentkit.Approver{}, strings.NewReader("y\n"), io.Discard)
	gated.Func(context.Background(), "{}")

	if !ran {
		t.Fatal("tool should run when the user approves")
	}
}

func TestRequireApprovalSkipsToolOnNo(t *testing.T) {
	ran := false
	gated := requireApproval(okTool("write_file", &ran), &agentkit.Approver{}, strings.NewReader("n\n"), io.Discard)
	out, _ := gated.Func(context.Background(), "{}")

	if ran {
		t.Fatal("tool must not run when the user denies")
	}
	if out != "denied by user" {
		t.Fatalf("denied result = %q, want %q", out, "denied by user")
	}
}
