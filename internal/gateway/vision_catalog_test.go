package gateway

import (
	"testing"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
)

// A custom provider whose /models lists only ids has a model models.dev
// knows sees images: Providers shows it "Accepts images" and the served
// entry takes images, so Automatic image recognition picks it too — unless
// the vendor's list says it takes none and the user didn't say otherwise
// (#2460, qiangxinglin).
func TestAutoVisionTakesTheCatalogsWordForAnIDOnlyModel(t *testing.T) {
	fresh(t)
	writeCatalog(t, `{"openai": {"models": {"gpt-5.5": {"id":"gpt-5.5","name":"GPT-5.5","modalities":{"input":["text","image"],"output":["text"]}}}}}`)
	if !catalog.SeesImages("gpt-5.5") {
		t.Fatal("the catalog doesn't say gpt-5.5 sees images")
	}
	if err := provider.Save(provider.Provider{ID: "custom", Name: "Custom", Chat: "http://custom.invalid/v1", Key: "key", Models: []string{"gpt-5.5"}}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveLive("custom", "http://custom.invalid/v1", []catalog.Model{{ID: "gpt-5.5"}}); err != nil {
		t.Fatal(err)
	}
	if e, ok := provider.ServedEntryOf("custom/gpt-5.5"); !ok || !e.Images {
		t.Fatalf("served entry %+v %v, want one that takes images", e, ok)
	}
	if m := AutoVision(); m != "custom/gpt-5.5" {
		t.Fatalf("AutoVision = %q, want custom/gpt-5.5, which the catalog says sees", m)
	}

	// the vendor's own no is kept over the catalog's yes
	if err := catalog.SaveLive("custom", "http://custom.invalid/v1", []catalog.Model{{ID: "gpt-5.5", ImageInput: imageInputBool(false)}}); err != nil {
		t.Fatal(err)
	}
	if m := AutoVision(); m != "" {
		t.Fatalf("AutoVision = %q, a model whose vendor says it takes no images", m)
	}

	// and the user's answer over the vendor's, as the served entry has it
	yes := true
	if err := provider.SetModelImage("custom/gpt-5.5", &yes); err != nil {
		t.Fatal(err)
	}
	if m := AutoVision(); m != "custom/gpt-5.5" {
		t.Fatalf("AutoVision = %q, a model the user says sees images", m)
	}
}
