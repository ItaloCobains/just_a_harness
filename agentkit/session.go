package agentkit

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"harness/agent"
)

// ErrNoSessions is returned by LatestSession when no session has been saved yet.
var ErrNoSessions = errors.New("no saved sessions")

// sessionDir returns ~/.harness/sessions, creating it if missing. Sessions are
// stored globally (unlike the CWD-relative approver allowlist) so a conversation
// can be resumed from anywhere.
func sessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".harness", "sessions")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// SaveSession writes the conversation history to <sessionDir>/<name>.json and
// returns the file path.
func SaveSession(name string, history []agent.Message) (string, error) {
	dir, err := sessionDir()
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(history)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// LoadSession reads a previously saved session by name.
func LoadSession(name string) ([]agent.Message, error) {
	dir, err := sessionDir()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return nil, err
	}
	var history []agent.Message
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// LatestSession loads the most recently modified session, returning its name and
// history. It returns ErrNoSessions when none exist.
func LatestSession() (string, []agent.Message, error) {
	dir, err := sessionDir()
	if err != nil {
		return "", nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", nil, err
	}

	var newest string
	var newestMod int64 = -1
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if mod := info.ModTime().UnixNano(); mod > newestMod {
			newestMod = mod
			newest = e.Name()
		}
	}
	if newest == "" {
		return "", nil, ErrNoSessions
	}

	name := newest[:len(newest)-len(".json")]
	history, err := LoadSession(name)
	if err != nil {
		return "", nil, err
	}
	return name, history, nil
}

// ListSessions returns the names of all saved sessions, sorted.
func ListSessions() ([]string, error) {
	dir, err := sessionDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, e.Name()[:len(e.Name())-len(".json")])
	}
	sort.Strings(names)
	return names, nil
}
