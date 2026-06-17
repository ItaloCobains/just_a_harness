package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"harness"
	"harness/agentkit"
)

// requireApproval wraps a tool so it runs only after the user approves the call.
// It reads a y/N/a answer from in and prompts on out, leaving the harness untouched.
// "a" (always) is remembered by the Approver so later calls to the same tool skip the prompt.
func requireApproval(tool harness.Tool, approver *agentkit.Approver, in io.Reader, out io.Writer) harness.Tool {
	inner := tool.Func
	reader := bufio.NewReader(in)

	tool.Func = func(ctx context.Context, input string) (string, error) {
		if approver.Allowed(tool.Name) {
			return inner(ctx, input)
		}
		fmt.Fprintf(out, "Allow %s(%s)? [y/N/a] ", tool.Name, input)
		answer, _ := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "a":
			approver.Always(tool.Name)
			return inner(ctx, input)
		case "y":
			return inner(ctx, input)
		default:
			return "denied by user", nil
		}
	}
	return tool
}
