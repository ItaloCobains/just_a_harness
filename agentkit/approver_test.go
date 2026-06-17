package agentkit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApproverAlwaysPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".harness/allow.json")

	a := &Approver{path: path, allowed: map[string]bool{}}
	if a.Allowed("run_bash") {
		t.Fatal("nothing should be allowed initially")
	}
	a.Always("run_bash")
	if !a.Allowed("run_bash") {
		t.Fatal("run_bash should be allowed after Always")
	}

	// A fresh Approver pointed at the same file must reload the decision.
	reloaded := &Approver{path: path, allowed: map[string]bool{}}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("allow file not written: %v", err)
	}
	loadInto(reloaded, data)
	if !reloaded.Allowed("run_bash") {
		t.Fatal("reloaded approver should remember run_bash")
	}
}

func TestLoadApproverMissingFileIsEmpty(t *testing.T) {
	a := &Approver{path: filepath.Join(t.TempDir(), "nope.json"), allowed: map[string]bool{}}
	if a.Allowed("run_bash") {
		t.Fatal("missing file must yield empty allowlist")
	}
}
