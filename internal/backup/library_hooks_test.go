package backup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/library"
)

// Exercise the sealed backup path as well as Library's own round trip:
// the portable definition survives while no local Applied state travels.
func TestLibraryHooksSealedRoundTrip(t *testing.T) {
	home(t)
	if _, err := library.SaveHook("", library.Hook{Name: "notify", Description: "Shared notification", Event: "turn-end", Command: "echo shared-notification", Timeout: 10}); err != nil {
		t.Fatal(err)
	}
	b, err := Collect(true, "hook-test")
	if err != nil {
		t.Fatal(err)
	}
	data, err := Seal(b, "synthetic-test-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "shared-notification") {
		t.Fatal("the hook command is readable in a sealed backup")
	}
	home(t)
	in, err := Open(data, "synthetic-test-passphrase")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Restore(in, Parts{Library: true})
	if err != nil || !result.Library {
		t.Fatalf("restore: %+v %v", result, err)
	}
	back, err := library.Collect()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(back)
	if err != nil {
		t.Fatal(err)
	}
	var fields struct {
		Hooks   []*library.Hook `json:"hooks"`
		Applied json.RawMessage `json:"applied"`
	}
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields.Hooks) != 1 || fields.Hooks[0].Command != "echo shared-notification" || fields.Hooks[0].Description != "Shared notification" || fields.Hooks[0].Timeout != 10 {
		t.Fatalf("the sealed backup lost the hook: %s", raw)
	}
	if len(fields.Applied) != 0 {
		t.Fatal("a portable library carries local ownership state")
	}
}
