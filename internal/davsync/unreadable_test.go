package davsync

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/yetone/magpie/internal/backup"
	"github.com/yetone/magpie/internal/lastgood"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// A providers.json that can't be read is never pushed as no providers: a
// sync stops on it, the server keeps the providers, and another computer
// mirroring the server keeps them too.
func TestSyncStopsOnUnreadableProviders(t *testing.T) {
	fake := &fakeDAV{files: map[string][]byte{}, etags: map[string]string{}, dirs: map[string]bool{"/dav": true}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	ctx := context.Background()
	cfg := Config{URL: srv.URL + "/dav/", User: "me", Password: "pw", Passphrase: "correct horse", Keys: true}

	a, b := newComputer(t), newComputer(t)
	b.use(t)
	if err := Configure(cfg); err != nil {
		t.Fatal(err)
	}
	a.use(t)
	if err := provider.Save(provider.Provider{ID: "deepseek", Name: "DeepSeek", Chat: "https://api.deepseek.com/v1", Key: "k1"}); err != nil {
		t.Fatal(err)
	}
	if err := Configure(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	b.use(t)
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	if got := ids(); !slices.Equal(got, []string{"deepseek=k1"}) {
		t.Fatalf("b's providers: %v", got)
	}

	// a's file is cut short
	a.use(t)
	whole, err := os.ReadFile(provider.Path())
	if err != nil {
		t.Fatal(err)
	}
	broken := whole[:len(whole)/2]
	if err := os.WriteFile(provider.Path(), broken, 0o600); err != nil {
		t.Fatal(err)
	}
	puts := fake.puts
	if err := Now(ctx); !errors.Is(err, provider.ErrUnreadable) {
		t.Fatalf("a synced a broken providers.json: %v", err)
	}
	if fake.puts != puts {
		t.Fatal("a pushed a broken providers.json")
	}
	if after, _ := os.ReadFile(provider.Path()); !bytes.Equal(after, broken) {
		t.Fatal("a's broken providers.json was written over")
	}
	if _, err := backup.Collect(true, "test"); !errors.Is(err, provider.ErrUnreadable) {
		t.Fatalf("a backup of a broken providers.json: %v", err)
	}
	remote, err := backup.Open(fake.files["/dav/magpie/magpie.magpie-backup"], "correct horse")
	if err != nil || len(remote.Providers) != 1 {
		t.Fatalf("on the server: %v %+v", err, remote.Providers)
	}
	b.use(t)
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	if got := ids(); !slices.Equal(got, []string{"deepseek=k1"}) {
		t.Fatalf("b lost its providers to a's broken file: %v", got)
	}
}

// A settings.json that can't be read is never pushed as the defaults: a
// sync stops on it and says so, the server keeps the settings, and another
// computer mirroring the server keeps its own (#2269).
func TestSyncStopsOnUnreadableSettings(t *testing.T) {
	fake := &fakeDAV{files: map[string][]byte{}, etags: map[string]string{}, dirs: map[string]bool{"/dav": true}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	ctx := context.Background()
	cfg := Config{URL: srv.URL + "/dav/", User: "me", Password: "pw", Passphrase: "correct horse", Keys: true}

	a, b := newComputer(t), newComputer(t)
	b.use(t)
	if err := Configure(cfg); err != nil {
		t.Fatal(err)
	}
	a.use(t)
	if err := settings.Save(settings.Settings{Theme: "dark", Lang: "ja", TextSize: 125}); err != nil {
		t.Fatal(err)
	}
	if err := Configure(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	b.use(t)
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	if s := settings.Load(); s.Theme != "dark" || s.Lang != "ja" {
		t.Fatalf("b's settings: %s %s", s.Theme, s.Lang)
	}

	// a's file is cut short, with no last good generation to fall back to
	a.use(t)
	whole, err := os.ReadFile(settings.Path())
	if err != nil {
		t.Fatal(err)
	}
	broken := whole[:len(whole)/2]
	if err := os.WriteFile(settings.Path(), broken, 0o600); err != nil {
		t.Fatal(err)
	}
	os.Remove(lastgood.Bak(settings.Path()))
	puts := fake.puts
	if err := Now(ctx); err == nil {
		t.Fatal("a synced a broken settings.json")
	}
	if fake.puts != puts {
		t.Fatal("a pushed a broken settings.json")
	}
	if after, _ := os.ReadFile(settings.Path()); !bytes.Equal(after, broken) {
		t.Fatal("a's broken settings.json was written over")
	}
	if st := Status(); st.Error == "" {
		t.Fatal("a's sync status doesn't say it stopped")
	}
	remote, err := backup.Open(fake.files["/dav/magpie/magpie.magpie-backup"], "correct horse")
	if err != nil || remote.Settings == nil || remote.Settings.Theme != "dark" {
		t.Fatalf("on the server: %v %+v", err, remote.Settings)
	}
	b.use(t)
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	if s := settings.Load(); s.Theme != "dark" || s.Lang != "ja" || s.TextSize != 125 {
		t.Fatalf("b lost its settings to a's broken file: %s %s %d", s.Theme, s.Lang, s.TextSize)
	}
}

// A plugins.json that can't be read is never pushed as no plugins: a sync
// stops on it and says so, and the server keeps the plugin list, so a
// computer that joins meanwhile still gets the plugins (#2287).
func TestSyncStopsOnUnreadablePlugins(t *testing.T) {
	fake := &fakeDAV{files: map[string][]byte{}, etags: map[string]string{}, dirs: map[string]bool{"/dav": true}}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	ctx := context.Background()
	cfg := Config{URL: srv.URL + "/dav/", User: "me", Password: "pw", Passphrase: "correct horse", Keys: true}

	a := newComputer(t)
	a.use(t)
	if err := os.MkdirAll(settings.Dir(), 0o700); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(settings.Dir(), "plugins.json")
	whole := []byte(`{"plugins":[{"spec":"@magpie-community/opencode-cursor-auth"}]}`)
	if err := os.WriteFile(list, whole, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Configure(cfg); err != nil {
		t.Fatal(err)
	}
	if err := Now(ctx); err != nil {
		t.Fatal(err)
	}
	carried := func() []string {
		t.Helper()
		remote, err := backup.Open(fake.files["/dav/magpie/magpie.magpie-backup"], "correct horse")
		if err != nil || remote.Plugins == nil {
			t.Fatalf("on the server: %v %+v", err, remote.Plugins)
		}
		var specs []string
		for _, e := range *remote.Plugins {
			specs = append(specs, e.Spec)
		}
		return specs
	}
	if got := carried(); !slices.Equal(got, []string{"@magpie-community/opencode-cursor-auth"}) {
		t.Fatalf("the server's plugins: %v", got)
	}

	// a's file is cut short, with no last good generation to fall back to
	broken := whole[:len(whole)/2]
	if err := os.WriteFile(list, broken, 0o600); err != nil {
		t.Fatal(err)
	}
	os.Remove(lastgood.Bak(list))
	puts := fake.puts
	if err := Now(ctx); err == nil {
		t.Fatal("a synced a broken plugins.json")
	}
	if fake.puts != puts {
		t.Fatal("a pushed a broken plugins.json")
	}
	if after, _ := os.ReadFile(list); !bytes.Equal(after, broken) {
		t.Fatal("a's broken plugins.json was written over")
	}
	if st := Status(); st.Error == "" {
		t.Fatal("a's sync status doesn't say it stopped")
	}
	if got := carried(); !slices.Equal(got, []string{"@magpie-community/opencode-cursor-auth"}) {
		t.Fatalf("the server lost the plugins to a's broken file: %v", got)
	}
}
