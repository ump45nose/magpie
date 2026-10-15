package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/lastgood"
	"github.com/yetone/magpie/internal/settings"
)

// A plugins.json that is there but can't be read, with no good .bak, stops
// the backup, as an unreadable providers.json or settings.json does: carried
// as no plugins, it would leave the server's bundle and an exported backup
// with none, and a computer that joins in between would get none (#2287).
func TestCollectStopsOnUnreadablePlugins(t *testing.T) {
	home(t)
	if err := os.MkdirAll(settings.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(settings.Dir(), "plugins.json")
	whole := []byte(`{"plugins":[{"spec":"@magpie-community/opencode-cursor-auth"}]}`)
	if err := os.WriteFile(list, whole[:len(whole)/2], 0o600); err != nil {
		t.Fatal(err)
	}
	os.Remove(lastgood.Bak(list))
	b, err := Collect(true, "test")
	if err == nil || !strings.Contains(err.Error(), "plugins.json") {
		var got any
		if b.Plugins != nil {
			got = *b.Plugins
		}
		t.Fatalf("Collect read a plugins.json that can't be read as no plugins: %v %+v", err, got)
	}

	// its last good generation is read in its place
	if err := os.WriteFile(lastgood.Bak(list), whole, 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := Collect(true, "test"); err != nil || b.Plugins == nil || len(*b.Plugins) != 1 {
		t.Fatalf("plugins.json with a good .bak: %v %+v", err, b.Plugins)
	}

	// a missing file is no plugins, not an error
	for _, p := range []string{list, lastgood.Bak(list)} {
		if err := os.Remove(p); err != nil {
			t.Fatal(err)
		}
	}
	if b, err := Collect(true, "test"); err != nil || b.Plugins == nil || len(*b.Plugins) != 0 {
		t.Fatalf("no plugins.json: %v %+v", err, b.Plugins)
	}
}
