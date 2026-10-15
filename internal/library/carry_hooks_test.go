package library

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Read the serialized field so these regression tests also compile before
// bundles carry hooks: the missing behavior, not a new symbol, fails there.
func carriedHooks(t *testing.T, b *Bundle) []*Hook {
	t.Helper()
	var fields struct {
		Hooks []*Hook `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(mustJSON(t, b)), &fields); err != nil {
		t.Fatal(err)
	}
	return fields.Hooks
}

func TestCarryHooksRoundTrip(t *testing.T) {
	h := sandbox(t)
	ok(t)(SaveHook("", Hook{Name: "notify", Description: "A shared notification", Event: "turn-end", Command: "echo shared-done", Timeout: 10, Agents: []string{"codex", "claude"}}))
	ok(t)(SaveHook("", Hook{Name: "format", Event: "after-tool", Matcher: "edit", Command: "echo shared-format", Agents: []string{"claude"}}))
	write(t, filepath.Join(h, ".codex/config.toml"), "model = \"source-choice\"\n[hooks.state.source]\ntrusted_hash = \"source-only-trust\"\n")
	b, err := Collect()
	if err != nil {
		t.Fatal(err)
	}
	hooks := carriedHooks(t, b)
	if len(hooks) != 2 || hooks[0].Name != "format" || hooks[1].Name != "notify" {
		t.Fatalf("shared hook definitions are not carried in name order: %+v", hooks)
	}
	if hooks[1].Description != "A shared notification" || hooks[1].Command != "echo shared-done" || hooks[1].Timeout != 10 || !reflect.DeepEqual(hooks[1].Agents, []string{"claude", "codex"}) {
		t.Fatalf("notification lost fields or assignments: %+v", hooks[1])
	}
	if hooks[0].Event != "after-tool" || hooks[0].Matcher != "edit" {
		t.Fatalf("formatter lost its event or matcher: %+v", hooks[0])
	}
	again, err := Collect()
	if err != nil {
		t.Fatal(err)
	}
	carried := mustJSON(t, b)
	if carried != mustJSON(t, again) || strings.Contains(carried, "source-only-trust") {
		t.Fatal("a collection is unstable or carries this computer's hook trust")
	}

	// Another computer already has its own native hook and one managed
	// hook. Only the managed hook is replaced; its model and trust stay.
	h2 := sandbox(t)
	hookFile := filepath.Join(h2, ".codex/hooks.json")
	config := filepath.Join(h2, ".codex/config.toml")
	write(t, hookFile, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo my-own-done"}]}]}}`)
	localConfig := "model = \"destination-choice\"\napproval_policy = \"never\"\n\n[hooks.state." + codexKey(hookFile+":stop:0:0") + "]\ntrusted_hash = \"destination-only-trust\"\n"
	write(t, config, localConfig)
	ok(t)(SaveHook("", Hook{Name: "old", Event: "turn-end", Command: "echo old-managed", Agents: []string{"codex"}}))
	var incoming Bundle
	if err := json.Unmarshal([]byte(carried), &incoming); err != nil {
		t.Fatal(err)
	}
	ok(t)(Put(&incoming))
	if got := hooksOf(t, hookFile, "Stop"); len(got) != 2 || !strings.Contains(got[0], "my-own-done") || !strings.Contains(got[1], "shared-done") {
		t.Fatalf("restored hooks did not replace only the managed entries: %v", got)
	}
	if got := read(t, config); got != localConfig {
		t.Fatalf("restoring hooks changed the destination's model, permissions or trust:\n%s", got)
	}
	if got := read(t, filepath.Join(h2, ".claude/settings.json")); !strings.Contains(got, "shared-format") || !strings.Contains(got, "shared-done") {
		t.Fatalf("hooks were not written into Claude's native settings:\n%s", got)
	}
	back, err := Collect()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(carriedHooks(t, back), hooks) {
		t.Fatalf("hooks changed after the round trip: %+v", carriedHooks(t, back))
	}
	if res := ok(t)(Put(&incoming)); len(res.Changed) != 0 {
		t.Fatalf("restoring the same hooks again rewrote %v", res.Changed)
	}
}

func TestCarryHooksLegacyAndRemoval(t *testing.T) {
	h := sandbox(t)
	path := filepath.Join(h, ".codex/hooks.json")
	write(t, path, `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"echo my-own-done"}]}]}}`)
	ok(t)(SaveHook("", Hook{Name: "keep", Event: "turn-end", Command: "echo managed", Agents: []string{"codex"}}))
	for _, doc := range []string{`{}`, `{"hooks":null}`} {
		var legacy Bundle
		if err := json.Unmarshal([]byte(doc), &legacy); err != nil {
			t.Fatal(err)
		}
		ok(t)(Put(&legacy))
		if got := hooksOf(t, path, "Stop"); len(got) != 2 || !strings.Contains(got[1], "managed") {
			t.Fatalf("an older bundle removed hooks it cannot carry: %v", got)
		}
	}
	var empty Bundle
	if err := json.Unmarshal([]byte(`{"hooks":[]}`), &empty); err != nil {
		t.Fatal(err)
	}
	ok(t)(Put(&empty))
	if got := hooksOf(t, path, "Stop"); len(got) != 1 || !strings.Contains(got[0], "my-own-done") {
		t.Fatalf("an explicit empty hooks list did not remove only managed hooks: %v", got)
	}
	b, err := Collect()
	if err != nil {
		t.Fatal(err)
	}
	if got := carriedHooks(t, b); got == nil || len(got) != 0 {
		t.Fatalf("an empty library must carry an explicit empty hooks list: %+v", got)
	}
}

func TestCarryHooksRefusesInvalid(t *testing.T) {
	valid := &Hook{Name: "valid", Event: "turn-end", Command: "echo valid"}
	for name, hooks := range map[string][]*Hook{
		"null":      {nil},
		"duplicate": {valid, valid},
		"name":      {{Name: "../bad", Event: "turn-end", Command: "echo bad"}},
		"event":     {{Name: "bad", Event: "not-an-event", Command: "echo bad"}},
		"command":   {{Name: "bad", Event: "turn-end", Command: " "}},
		"timeout":   {{Name: "bad", Event: "turn-end", Command: "echo bad", Timeout: -1}},
		"target":    {{Name: "bad", Event: "codex:Stop", Command: "echo bad", Agents: []string{"claude"}}},
	} {
		t.Run(name, func(t *testing.T) {
			sandbox(t)
			ok(t)(SaveHook("", *valid))
			before := read(t, path())
			var incoming Bundle
			if err := json.Unmarshal([]byte(mustJSON(t, map[string]any{"hooks": hooks})), &incoming); err != nil {
				t.Fatal(err)
			}
			if _, err := Put(&incoming); err == nil {
				t.Fatal("an invalid carried hook was accepted")
			}
			if read(t, path()) != before {
				t.Fatal("a refused carried hook changed the library")
			}
		})
	}
}
