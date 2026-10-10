package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

// #1520: a subscription serving DeepSeek V4.1 Flash as deepseek-v4.1-flash
// gets the maker's list price, though DeepSeek's models.dev row has the id
// deepseek-flash and names deepseek/deepseek-v4.1-flash only as its
// canonical_model_id.
func TestMakerPriceReadsCanonicalModelID(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0700); err != nil {
		t.Fatal(err)
	}
	body := `{"deepseek":{"id":"deepseek","name":"DeepSeek","models":{
	  "deepseek-flash":{"id":"deepseek-flash","name":"DeepSeek V4.1 Flash",
	    "canonical_model_id":"deepseek/deepseek-v4.1-flash",
	    "cost":{"input":0.15,"output":0.6,"cache_read":0.003}}}}}`
	if err := os.WriteFile(catalog.CachePath(), []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(catalog.Reset)

	pr, ok := MakerPrice("deepseek-v4.1-flash")
	if !ok || pr.Input != 0.15 || pr.Output != 0.6 || pr.CacheRead != 0.003 {
		t.Fatalf("MakerPrice(deepseek-v4.1-flash) = %+v, %v; want 0.15/0.6/0.003", pr, ok)
	}
}
