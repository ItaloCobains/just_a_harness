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

func TestApproverDecide(t *testing.T) {
	cases := []struct {
		answer     string
		wantRun    bool
		wantDenied string
	}{
		{"y", true, ""},
		{" Y\n", true, ""},
		{"n", false, "denied by user"},
		{"", false, "denied by user"},
		{"garbage", false, "denied by user"},
	}
	for _, c := range cases {
		a := &Approver{path: filepath.Join(t.TempDir(), "allow.json"), allowed: map[string]bool{}}
		run, denied := a.Decide("run_bash", c.answer)
		if run != c.wantRun || denied != c.wantDenied {
			t.Errorf("Decide(%q) = (%v, %q), want (%v, %q)", c.answer, run, denied, c.wantRun, c.wantDenied)
		}
	}
}

func TestApproverDecideAlwaysPersists(t *testing.T) {
	a := &Approver{path: filepath.Join(t.TempDir(), ".harness/allow.json"), allowed: map[string]bool{}}
	if run, _ := a.Decide("run_bash", "a"); !run {
		t.Fatal(`"a" should approve the call`)
	}
	if !a.Allowed("run_bash") {
		t.Fatal(`"a" should remember the tool via Always`)
	}
}

func TestLoadApproverMissingFileIsEmpty(t *testing.T) {
	a := &Approver{path: filepath.Join(t.TempDir(), "nope.json"), allowed: map[string]bool{}}
	if a.Allowed("run_bash") {
		t.Fatal("missing file must yield empty allowlist")
	}
}
