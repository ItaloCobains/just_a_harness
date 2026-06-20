package agentkit

import (
	"strings"
	"testing"
)

const ddgFixture = `
<div class="result">
  <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgo&amp;rut=x">The Go <b>Programming</b> Language</a>
  <a class="result__snippet" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgo">Go is an open source <b>programming</b> language.</a>
</div>
<div class="result">
  <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2Fdoc">Documentation</a>
  <a class="result__snippet" href="#">Official docs &amp; tutorials.</a>
</div>
`

func TestParseDuckDuckGo(t *testing.T) {
	results := parseDuckDuckGo(ddgFixture)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}

	first := results[0]
	if first.Title != "The Go Programming Language" {
		t.Fatalf("title = %q", first.Title)
	}
	if first.URL != "https://example.com/go" {
		t.Fatalf("url = %q, want decoded uddg", first.URL)
	}
	if first.Snippet != "Go is an open source programming language." {
		t.Fatalf("snippet = %q", first.Snippet)
	}

	if results[1].Snippet != "Official docs & tutorials." {
		t.Fatalf("entity not unescaped: %q", results[1].Snippet)
	}
}

func TestFormatResults(t *testing.T) {
	out := formatResults([]searchResult{
		{Title: "T", URL: "https://x.com", Snippet: "snip"},
		{Title: "No snippet", URL: "https://y.com"},
	})
	if !strings.Contains(out, "1. T") || !strings.Contains(out, "https://x.com") {
		t.Fatalf("missing first result: %q", out)
	}
	if !strings.Contains(out, "2. No snippet") {
		t.Fatalf("missing second result: %q", out)
	}
}

func TestParseDuckDuckGoEmpty(t *testing.T) {
	if got := parseDuckDuckGo("<html>nothing</html>"); len(got) != 0 {
		t.Fatalf("got %d results, want 0", len(got))
	}
}
