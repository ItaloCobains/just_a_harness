package agentkit

import (
	"os"
	"os/exec"
	"strings"
)

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

	if status := gitStatus(); status != "" {
		b.WriteString("\nGit status:\n" + status + "\n")
	}

	if guide, name := projectGuide(); guide != "" {
		b.WriteString("\nProject guide (" + name + "):\n" + guide + "\n")
	}

	return strings.TrimSpace(b.String())
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
