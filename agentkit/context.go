package agentkit

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	treeMaxDepth   = 2
	treeMaxEntries = 200
)

// skipDir holds directory names that add noise without helping orientation.
var skipDir = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "bin": true,
	"target": true, "dist": true, "build": true, ".harness": true,
}

// BuildSystemPrompt augments the base prompt with live project context: the
// working directory, git status, and the project's CLAUDE.md/AGENTS.md if present.
func BuildSystemPrompt() string {
	ctx := ProjectContext()
	if ctx == "" {
		return SystemPrompt
	}
	return SystemPrompt + "\n\n" + ctx
}

// ProjectContext gathers ambient facts about the current project. Every source
// is best-effort: missing git or files simply contribute nothing.
func ProjectContext() string {
	var b strings.Builder

	if cwd, err := os.Getwd(); err == nil {
		b.WriteString("Working directory: " + cwd + "\n")
	}

	if tree := fileTree("."); tree != "" {
		b.WriteString("\nProject files (depth " + strconv.Itoa(treeMaxDepth) + "):\n" + tree + "\n")
	}

	if status := gitStatus(); status != "" {
		b.WriteString("\nGit status:\n" + status + "\n")
	}

	if guide, name := projectGuide(); guide != "" {
		b.WriteString("\nProject guide (" + name + "):\n" + guide + "\n")
	}

	return strings.TrimSpace(b.String())
}

// fileTree returns a compact, depth-limited listing of root, skipping hidden
// and noisy directories, so the model can orient itself without spending turns
// on list_dir. It caps the entry count to keep the prompt small.
func fileTree(root string) string {
	type entry struct {
		path string
		dir  bool
	}
	var entries []entry

	filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		depth := strings.Count(rel, string(filepath.Separator)) + 1

		name := d.Name()
		if d.IsDir() {
			if skipDir[name] || strings.HasPrefix(name, ".") || depth > treeMaxDepth {
				return fs.SkipDir
			}
		} else if depth > treeMaxDepth {
			return nil
		}
		entries = append(entries, entry{rel, d.IsDir()})
		return nil
	})

	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })
	if len(entries) > treeMaxEntries {
		entries = entries[:treeMaxEntries]
	}

	var b strings.Builder
	for _, e := range entries {
		indent := strings.Repeat("  ", strings.Count(e.path, string(filepath.Separator)))
		name := filepath.Base(e.path)
		if e.dir {
			name += "/"
		}
		b.WriteString(indent + name + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func gitStatus() string {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func projectGuide() (content, name string) {
	for _, candidate := range []string{"CLAUDE.md", "AGENTS.md"} {
		if data, err := os.ReadFile(candidate); err == nil {
			return strings.TrimSpace(string(data)), candidate
		}
	}
	return "", ""
}
