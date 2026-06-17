package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"harness"
)

// requireApproval wraps a tool so it runs only after the user approves the call.
// It reads a y/N answer from in and prompts on out, leaving the harness untouched.
func requireApproval(tool harness.Tool, in io.Reader, out io.Writer) harness.Tool {
	inner := tool.Func
	reader := bufio.NewReader(in)

	tool.Func = func(input string) string {
		fmt.Fprintf(out, "Allow %s(%s)? [y/N] ", tool.Name, input)
		answer, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" {
			return "denied by user"
		}
		return inner(input)
	}
	return tool
}
