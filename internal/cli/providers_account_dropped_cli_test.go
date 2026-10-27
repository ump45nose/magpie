package cli

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// #3940: provider set on a signed-in account must refuse pairs Save drops,
// instead of printing "saved" for a change that never sticks.
func TestApplyPairsRefusesAccountDroppedFields(t *testing.T) {
	a := provider.Provider{ID: "codex", Name: "ChatGPT", Account: &provider.Account{Agent: "codex", User: "me@example.com"}}
	dropped := []string{
		"name=Nope",
		"icon=star",
		"catalog=https://example.com/c",
		"website=https://example.com",
		"url=https://example.com/v1",
		"chat=https://example.com/v1",
		"openai=https://example.com/v1",
		"responses=https://example.com/v1",
		"anthropic=https://example.com/v1",
		"gemini=https://example.com/v1beta",
		"decide=https://example.com/systemone",
		"key=sk-secret",
		"keys=https://example.com/keys",
		"balance=https://example.com/bal",
		"balance.path=$.x",
		"balance.method=POST",
		"balance.token=tok",
		"access.key=ak",
		"access.secret=sk",
		"models.url=https://example.com/models",
		"codex=as-sent",
		"search=yes",
		"unmasked=yes",
		"header.X-Test=1",
	}
	for _, pair := range dropped {
		err := applyPairs(&a, []string{pair})
		if err == nil {
			t.Fatalf("%s: want refusal on a signed-in account, got nil", pair)
		}
		k, _, _ := strings.Cut(pair, "=")
		if !strings.Contains(err.Error(), k) && !strings.Contains(strings.ToLower(err.Error()), "account") {
			t.Fatalf("%s: error %q should name the field or account", pair, err)
		}
	}
}

func TestApplyPairsKeepsAccountPicks(t *testing.T) {
	a := provider.Provider{ID: "codex", Name: "ChatGPT", Account: &provider.Account{Agent: "codex", User: "me@example.com"}}
	if err := applyPairs(&a, []string{"models=gpt-5", "family=work", "compaction=magpie"}); err != nil {
		t.Fatalf("kept picks refused: %v", err)
	}
	if len(a.Models) != 1 || a.Models[0] != "gpt-5" {
		t.Fatalf("models not set: %#v", a.Models)
	}
	if a.Family != "work" {
		t.Fatalf("family: got %q", a.Family)
	}
	if a.NativeCompaction {
		t.Fatalf("compaction=magpie should clear NativeCompaction")
	}
}

func TestApplyPairsAllowsKiroAccountKey(t *testing.T) {
	a := provider.Provider{ID: "kiro", Name: "Kiro", Account: &provider.Account{Agent: "kiro"}}
	if err := applyPairs(&a, []string{"key=abc"}); err != nil {
		t.Fatalf("kiro key should be kept: %v", err)
	}
	if a.Key != "abc" {
		t.Fatalf("key: got %q", a.Key)
	}
}
