package catalog

import "testing"

// #1520 (liuweifeng): models.dev lists DeepSeek V4.1 Flash as deepseek-flash
// (and two more ids) with canonical_model_id deepseek/deepseek-v4.1-flash,
// the id WorkBuddy serves it by, and no row has that id: it had no price.
const canonicalCatalog = `{
  "deepseek": {"name":"DeepSeek","models": {
    "deepseek-flash": {"id":"deepseek-flash","name":"DeepSeek V4.1 Flash",
      "canonical_model_id":"deepseek/deepseek-v4.1-flash",
      "cost":{"input":0.15,"output":0.6,"cache_read":0.003}},
    "deepseek-v4-flash": {"id":"deepseek-v4-flash","name":"DeepSeek V4 Flash",
      "canonical_model_id":"deepseek/deepseek-v4.1-flash",
      "cost":{"input":0.15,"output":0.6,"cache_read":0.003}},
    "deepseek-v4-pro": {"id":"deepseek-v4-pro","name":"DeepSeek V4 Pro",
      "canonical_model_id":"deepseek/deepseek-v4-pro-0813",
      "cost":{"input":0.66,"output":1.98,"cache_read":0.022}},
    "deepseek-chat": {"id":"deepseek-chat","name":"DeepSeek Chat"}}}
}`

func TestCanonicalIDPricesAVendorsAlias(t *testing.T) {
	writeCatalog(t, canonicalCatalog)

	p, ok := PricedBy([]string{"deepseek"}, "deepseek-v4.1-flash")
	if !ok || p.Input != 0.15 || p.Output != 0.6 || p.CacheRead != 0.003 {
		t.Errorf("PricedBy(deepseek-v4.1-flash) = %+v, %v; want deepseek-flash's 0.15/0.6/0.003", p, ok)
	}
	// a path or case on the id is matched as the other levels match it
	if p, ok := PricedBy([]string{"deepseek"}, "workbuddy/DeepSeek-V4.1-Flash"); !ok || p.Input != 0.15 {
		t.Errorf("PricedBy(workbuddy/DeepSeek-V4.1-Flash) = %+v, %v", p, ok)
	}
	// an id listed as given keeps its own row's price
	if p, ok := PricedBy([]string{"deepseek"}, "deepseek-v4-pro"); !ok || p.Input != 0.66 {
		t.Errorf("PricedBy(deepseek-v4-pro) = %+v, %v", p, ok)
	}
	// a canonical id is not a prefix match: no row names this one
	if p, ok := PricedBy([]string{"deepseek"}, "deepseek-v4"); ok {
		t.Errorf("PricedBy(deepseek-v4) = %+v, want no price", p)
	}
	// only a provider asked is read: one not listed gives nothing
	if p, ok := PricedBy([]string{"anthropic"}, "deepseek-v4.1-flash"); ok {
		t.Errorf("PricedBy(anthropic, deepseek-v4.1-flash) = %+v, want no price", p)
	}
}
