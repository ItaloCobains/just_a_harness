package agentkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesParentDirs(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a", "b", "c", "index.html")
	input := `{"path":` + jsonString(path) + `,"content":"<h1>hi</h1>"}`

	out, err := writeFileTool().Func(context.Background(), input)
	if err != nil {
		t.Fatalf("write_file: %v", err)
	}
	if out == "" {
		t.Fatal("expected confirmation message")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(data) != "<h1>hi</h1>" {
		t.Fatalf("content = %q", data)
	}
}

func TestListDirEmptyReportsEmpty(t *testing.T) {
	dir := t.TempDir()
	out, err := listDirTool().Func(context.Background(), `{"path":`+jsonString(dir)+`}`)
	if err != nil {
		t.Fatalf("list_dir: %v", err)
	}
	if out != "(empty directory)" {
		t.Fatalf("out = %q, want empty marker", out)
	}
}
