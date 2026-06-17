package agentkit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditFileReplacesUniqueString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(path, []byte("alpha beta gamma"), 0o644)

	if _, err := editFile(path, "beta", "BETA"); err != nil {
		t.Fatalf("editFile: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "alpha BETA gamma" {
		t.Fatalf("content = %q", data)
	}
}

func TestEditFileRejectsMissingString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(path, []byte("hello"), 0o644)

	if _, err := editFile(path, "nope", "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestEditFileRejectsAmbiguousString(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	os.WriteFile(path, []byte("x x"), 0o644)

	if _, err := editFile(path, "x", "y"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("err = %v, want ErrAmbiguous", err)
	}
}

func TestGrepFindsMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("foo\nbar\nfoobar"), 0o644)

	out, err := grep("foo", dir)
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if want := ":1:foo"; !strings.Contains(out, want) {
		t.Fatalf("grep out = %q, missing %q", out, want)
	}
	if !strings.Contains(out, ":3:foobar") {
		t.Fatalf("grep out = %q, missing line 3", out)
	}
}

func TestGrepNoMatches(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644)

	out, err := grep("zzz", dir)
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if out != "no matches" {
		t.Fatalf("out = %q, want %q", out, "no matches")
	}
}
