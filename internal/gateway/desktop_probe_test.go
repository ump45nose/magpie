package gateway

import (
	"os"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// The model a session was on is kept on disk for Desktop's titles; once
// its provider is switched off magpie no longer serves it, and a small
// tool-less request — Desktop's Gateway credentials → Test connection, a
// one-token probe of the model it discovered — went to that model and was
// a 404 "Old is switched off in Magpie" though the model it asked for is
// served (#2473, EchoTint). A model no longer served isn't the session's.
func TestClaudeDesktopStalePickIsntTheProbesModel(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	desktopPicked.Lock()
	desktopPicked.model, desktopPicked.from = "", ""
	desktopPicked.Unlock()
	t.Cleanup(func() {
		desktopPicked.Lock()
		desktopPicked.model, desktopPicked.from = "", ""
		desktopPicked.Unlock()
	})
	for _, p := range []provider.Provider{
		{ID: "old", Name: "Old", Key: "k", Anthropic: "http://127.0.0.1:1", Models: []string{"claude-fable-5-1"}, Off: true},
		{ID: "current", Name: "Current", Key: "k", Anthropic: "http://127.0.0.1:1", Models: []string{"claude-sonnet-5-5"}},
	} {
		if err := provider.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(settings.Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(desktopPickedPath(), []byte("old/claude-fable-5-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := []byte(`{"model":"current/claude-sonnet-5-5","max_tokens":1,"messages":[{"role":"user","content":"."}]}`)
	if got := desktopTurn("current/claude-sonnet-5-5", probe, false, false); got != "current/claude-sonnet-5-5" {
		t.Fatalf("credential probe sent %q, want current/claude-sonnet-5-5", got)
	}

	// switched on again, the session's model has its titles back
	if err := provider.SetOff("old", false); err != nil {
		t.Fatal(err)
	}
	title := []byte(`{"model":"current/claude-sonnet-5-5","max_tokens":200,"system":"You write short session titles.","messages":[{"role":"user","content":"hi"}]}`)
	if got := desktopTurn("current/claude-sonnet-5-5", title, false, false); got != "old/claude-fable-5-1" {
		t.Fatalf("title sent %q, want old/claude-fable-5-1", got)
	}
}
