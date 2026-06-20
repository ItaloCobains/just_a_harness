package agentkit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func webFetch(t *testing.T, args map[string]any) (string, error) {
	t.Helper()
	input, _ := json.Marshal(args)
	return webFetchTool().Func(context.Background(), string(input))
}

func TestWebFetchStripsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("<html><head><style>x{}</style></head><body><h1>Title</h1><p>Hello world</p></body></html>"))
	}))
	defer srv.Close()

	out, err := webFetch(t, map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("web_fetch: %v", err)
	}
	if !strings.Contains(out, "Title") || !strings.Contains(out, "Hello world") {
		t.Fatalf("missing text: %q", out)
	}
	if strings.Contains(out, "<") || strings.Contains(out, "x{}") {
		t.Fatalf("html not stripped: %q", out)
	}
}

func TestWebFetchTruncates(t *testing.T) {
	long := strings.Repeat("a", webFetchLimit*2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(long))
	}))
	defer srv.Close()

	out, err := webFetch(t, map[string]any{"url": srv.URL})
	if err != nil {
		t.Fatalf("web_fetch: %v", err)
	}
	if !strings.HasSuffix(out, "[truncated]") {
		t.Fatalf("expected truncation marker, got len %d", len(out))
	}
}

func TestWebFetchRejectsNonHTTPScheme(t *testing.T) {
	if _, err := webFetch(t, map[string]any{"url": "file:///etc/passwd"}); err == nil {
		t.Fatal("expected error for file:// scheme")
	}
}

func TestWebFetchRequiresURL(t *testing.T) {
	if _, err := webFetch(t, map[string]any{}); err == nil {
		t.Fatal("expected error for missing url")
	}
}
