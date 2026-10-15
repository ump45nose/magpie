package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

// With redact on, /v1/systemone must mask secrets before postDecide (#2389).
// serve() already did; serveSystemOne did not, so a token in state reached
// the decision provider as written.
func TestSystemOneRedactsStateBeforeUpstream(t *testing.T) {
	setHome(t, t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	const secret = "sk-proj-abcdEFGH1234ijklMNOP5678qrst"
	var mu sync.Mutex
	var got []byte
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		got = append([]byte(nil), b...)
		mu.Unlock()
		if r.URL.Path != "/v1/systemone" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"model":   "jev-latest",
			"answers": map[string]any{"intent": map[string]any{"choice": "bug"}},
			"usage":   map[string]int{"input_tokens": 10, "output_tokens": 2},
		})
	}))
	t.Cleanup(up.Close)
	if err := provider.Save(provider.Provider{ID: "load-a", Name: "A", Key: "ka", Decide: up.URL + "/v1"}); err != nil {
		t.Fatal(err)
	}
	if err := settings.Save(settings.Settings{Redact: true}); err != nil {
		t.Fatal(err)
	}
	s := New()
	body := `{"model":"load-a/jev-latest","state":{"message":"token ` + secret + `"},"questions":{"intent":{"type":"choice"}}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/v1/systemone", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer magpie")
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	mu.Lock()
	upstream := string(got)
	mu.Unlock()
	if strings.Contains(upstream, secret) {
		t.Fatalf("upstream saw the raw secret: %s", upstream)
	}
	if !strings.Contains(upstream, "{{API_KEY_") {
		t.Fatalf("upstream body was not masked: %s", upstream)
	}
	if !json.Valid(got) {
		t.Fatalf("upstream body not JSON: %s", upstream)
	}
}
