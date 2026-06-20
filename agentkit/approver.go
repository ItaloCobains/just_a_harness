package agentkit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const allowFile = ".harness/allow.json"

// Approver remembers which tools the user has granted "always allow", persisting
// the decisions to .harness/allow.json so they survive across sessions.
type Approver struct {
	mu      sync.Mutex
	path    string
	allowed map[string]bool
}

// LoadApprover reads any previously saved allowlist. A missing or corrupt file
// is treated as an empty allowlist.
func LoadApprover() *Approver {
	a := &Approver{path: allowFile, allowed: map[string]bool{}}
	if data, err := os.ReadFile(a.path); err == nil {
		loadInto(a, data)
	}
	return a
}

func loadInto(a *Approver, data []byte) {
	var names []string
	if json.Unmarshal(data, &names) == nil {
		for _, n := range names {
			a.allowed[n] = true
		}
	}
}

func (a *Approver) Allowed(tool string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.allowed[tool]
}

func (a *Approver) Always(tool string) {
	a.mu.Lock()
	a.allowed[tool] = true
	names := make([]string, 0, len(a.allowed))
	for n := range a.allowed {
		names = append(names, n)
	}
	a.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return
	}
	if data, err := json.Marshal(names); err == nil {
		os.WriteFile(a.path, data, 0o644)
	}
}

// Decide applies a user's y/N/a answer for a tool call. "y" approves once, "a"
// approves and remembers the tool via Always, anything else denies. It returns
// whether the call should run and, when denied, the message to surface instead.
func (a *Approver) Decide(tool, answer string) (run bool, denied string) {
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "a":
		a.Always(tool)
		return true, ""
	case "y":
		return true, ""
	default:
		return false, "denied by user"
	}
}

// Mutating is the set of tool names gated behind user approval: those that
// change the filesystem, run commands, or reach external networks.
var Mutating = map[string]bool{"write_file": true, "edit_file": true, "run_bash": true, "web_fetch": true}
