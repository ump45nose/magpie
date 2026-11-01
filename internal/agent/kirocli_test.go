package agent

import (
	"os"
	"path/filepath"
	"testing"
)

// kiro-cli is found where its installer puts it, ~/.local/bin, though
// that isn't on the PATH magpie was started with; never by ~/.kiro, which
// Kiro Crew and the Kiro IDE keep too.
func TestKiroCLIDetected(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv("PATH", t.TempDir())
	if err := os.MkdirAll(filepath.Join(h, ".kiro", "crew"), 0o755); err != nil {
		t.Fatal(err)
	}
	if KiroCLI().Detected() {
		t.Fatal("detected by ~/.kiro alone")
	}
	bin := filepath.Join(h, ".local", "bin", "kiro-cli")
	if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := KiroCLI()
	if !a.Detected() || a.ID != "kiro-cli" || a.CLIMissing() {
		t.Fatalf("kiro-cli in ~/.local/bin: detected %v, %+v", a.Detected(), a)
	}
	for _, o := range All() {
		if o.ID == "kiro-cli" {
			t.Fatal("kiro-cli is one of All's agents: it would have a row on the Agents page")
		}
	}
}
