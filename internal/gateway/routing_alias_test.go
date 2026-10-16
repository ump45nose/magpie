package gateway

import (
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// The Routing page's Weighed seat carries the account's name of the user's
// own (#1515) as Alias, so the page can show "工作账号" instead of the
// masked email (#2462, faketrader). Without Alias, Who alone stays the
// vendor's identity.
func TestWeighedAccountAlias(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	p := provider.Provider{
		ID: "codex", Name: "Codex", Models: []string{"m1"},
		Account:      &provider.Account{Agent: "codex", User: "me@example.com"},
		AccountNames: map[string]string{"me@example.com": "工作账号"},
	}
	c := candidate{p: p, model: "m1", rest: "codex#me@example.com"}
	w := weighed(c, p, weighing{}, false, provider.Chat)
	if w.Kind != "account" || w.Who != "me@example.com" {
		t.Fatalf("identity %+v", w)
	}
	if w.Alias != "工作账号" {
		t.Fatalf("alias %q, want 工作账号", w.Alias)
	}
	// no name → Alias empty, Who unchanged
	p.AccountNames = nil
	c.p = p
	w = weighed(c, p, weighing{}, false, provider.Chat)
	if w.Alias != "" || w.Who != "me@example.com" {
		t.Fatalf("unnamed %+v", w)
	}
}
