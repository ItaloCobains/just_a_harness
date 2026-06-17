package agentkit

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// Mutating is the set of tool names that change the filesystem or run commands.
var Mutating = map[string]bool{"write_file": true, "edit_file": true, "run_bash": true}
