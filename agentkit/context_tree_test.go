package agentkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileTreeRespectsDepthAndSkips(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "src", "deep", "deeper"), 0o755)
	os.MkdirAll(filepath.Join(root, ".git"), 0o755)
	os.MkdirAll(filepath.Join(root, "node_modules", "x"), 0o755)
	os.WriteFile(filepath.Join(root, "main.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(root, ".git", "config"), []byte("x"), 0o644)

	tree := fileTree(root)

	if !strings.Contains(tree, "main.go") || !strings.Contains(tree, "a.go") {
		t.Fatalf("missing expected files:\n%s", tree)
	}
	if strings.Contains(tree, ".git") || strings.Contains(tree, "node_modules") {
		t.Fatalf("should skip noise dirs:\n%s", tree)
	}
	if strings.Contains(tree, "deeper") {
		t.Fatalf("should respect depth limit:\n%s", tree)
	}
}
