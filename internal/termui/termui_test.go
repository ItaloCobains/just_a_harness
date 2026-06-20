package termui

import "testing"

func TestTruncate(t *testing.T) {
	cases := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"short", "hello", 80, "hello"},
		{"newlines collapsed", "a\nb\nc", 80, "a b c"},
		{"capped with ellipsis", "abcdef", 3, "abc…"},
		{"exact length kept", "abc", 3, "abc"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Truncate(c.in, c.n); got != c.want {
				t.Errorf("Truncate(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
			}
		})
	}
}
