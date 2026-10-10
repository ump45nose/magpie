package agent

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// A Codex CLI magpie routes as its model_provider reads its models from
// magpie-models.json, not from the gateway's /v1/codex/models. The
// gateway's lists mark a GPT model on a provider the user added by its
// address with the tier Codex offers /fast on (OwnTier, #1234), but
// magpieModels didn't, so every model on disk had service_tiers: [] and
// the CLI had no /fast at all (#1530).
func TestCodexCatalogOnDiskOffersOwnTiers(t *testing.T) {
	home := syncHome(t)
	if err := provider.Save(provider.Provider{ID: "sub", Name: "Sub", Key: "k", Chat: "http://127.0.0.1:1/v1", Models: []string{"gpt-6.1-sol"}}); err != nil {
		t.Fatal(err)
	}
	cx := codex(home)
	if err := cx.Fields[0].Set("sub/gpt-6.1-sol"); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Models []struct {
			Slug  string `json:"slug"`
			Tiers []struct {
				ID string `json:"id"`
			} `json:"service_tiers"`
		} `json:"models"`
	}
	raw := readFile(filepath.Join(home, ".codex", "magpie-models.json"))
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("%v: %s", err, raw)
	}
	tiers := map[string][]string{}
	for _, m := range got.Models {
		ids := []string{}
		for _, tr := range m.Tiers {
			ids = append(ids, tr.ID)
		}
		tiers[m.Slug] = ids
	}
	gpt, ok := tiers["sub/gpt-6.1-sol"]
	if !ok {
		t.Fatalf("sub/gpt-6.1-sol is not in the on-disk catalog: %v", tiers)
	}
	if len(gpt) != 1 || gpt[0] != "priority" {
		t.Errorf("sub/gpt-6.1-sol service_tiers = %v, want [priority] as /v1/codex/models has it", gpt)
	}
	// a model that isn't OpenAI's gets no tier
	if glm := tiers["relay/glm-4.6"]; len(glm) != 0 {
		t.Errorf("relay/glm-4.6 service_tiers = %v, want none", glm)
	}
}
