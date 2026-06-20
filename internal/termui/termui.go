// Package termui holds small terminal-rendering helpers shared by the CLIs.
package termui

import "strings"

// Truncate collapses newlines and caps s at n bytes, appending an ellipsis when
// it overflows. Used to keep tool-call previews on a single line.
func Truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
