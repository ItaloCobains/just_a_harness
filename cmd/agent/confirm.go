package main

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"harness/agent"
	"harness/agentkit"
)

// requireApproval wraps a tool so it runs only after the user approves the call.
// It reads a y/N/a answer from in and prompts on out, leaving the harness untouched.
// "a" (always) is remembered by the Approver so later calls to the same tool skip the prompt.
func requireApproval(tool agent.Tool, approver *agentkit.Approver, in io.Reader, out io.Writer) agent.Tool {
	inner := tool.Func
	reader := bufio.NewReader(in)

	tool.Func = func(ctx context.Context, input string) (string, error) {
		if approver.Allowed(tool.Name) {
			return inner(ctx, input)
		}
		fmt.Fprintf(out, "Allow %s(%s)? [y/N/a] ", tool.Name, input)
		answer, _ := reader.ReadString('\n')
		if run, denied := approver.Decide(tool.Name, answer); !run {
			return denied, nil
		}
		return inner(ctx, input)
	}
	return tool
}
