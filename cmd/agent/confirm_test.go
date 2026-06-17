package main

import (
	"io"
	"strings"
	"testing"

	"harness"
)

func TestRequireApprovalRunsToolOnYes(t *testing.T) {
	ran := false
	tool := harness.Tool{Name: "write_file", Func: func(string) string { ran = true; return "ok" }}

	gated := requireApproval(tool, strings.NewReader("y\n"), io.Discard)
	gated.Func("{}")

	if !ran {
		t.Fatal("tool should run when the user approves")
	}
}

func TestRequireApprovalSkipsToolOnNo(t *testing.T) {
	ran := false
	tool := harness.Tool{Name: "write_file", Func: func(string) string { ran = true; return "ok" }}

	out := requireApproval(tool, strings.NewReader("n\n"), io.Discard).Func("{}")

	if ran {
		t.Fatal("tool must not run when the user denies")
	}
	if out != "denied by user" {
		t.Fatalf("denied result = %q, want %q", out, "denied by user")
	}
}
