package gateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/provider"
)

// #2582: bare Codex model names remapped by codexAccounts must consult
// nativeCompactionFor. With the toggle off, a compaction_trigger must not
// reach ChatGPT as-is; magpie returns a portable magpie1: summary instead.
func TestCodexOwnModelCompactionHonoursProviderToggle(t *testing.T) {
	codexSignedIn(t, "spare@example.com")

	var hits []string
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		hits = append(hits, r.URL.Path+"\n"+string(b))
		if strings.Contains(string(b), "compaction_trigger") {
			w.WriteHeader(400)
			io.WriteString(w, `{"error":{"message":"native compaction_trigger reached ChatGPT with toggle off"}}`)
			return
		}
		// Summariser turn from codexCompact / serveAgent.
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, sse(
			`data: {"type":"response.output_item.done","output_index":0,"item":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PORTABLE SUMMARY"}]}}`,
			`data: {"type":"response.completed","response":{"id":"resp_s","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"PORTABLE SUMMARY"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`))
	}))
	t.Cleanup(up.Close)
	was := provider.CodexBase
	provider.CodexBase = up.URL + "/backend-api/codex"
	t.Cleanup(func() { provider.CodexBase = was })

	code, body := codexPost(t, strings.Replace(compactTurn, "%s", "gpt-6.1-sol", 1))
	if code != 200 || !strings.Contains(body, magpieCompaction) {
		t.Fatalf("toggle off: %d %s\nhits: %v", code, body, hits)
	}
	for _, h := range hits {
		if strings.Contains(h, "compaction_trigger") {
			t.Fatalf("compaction_trigger reached ChatGPT with toggle off:\n%s", h)
		}
	}

	// Toggle on: native trigger is kept for the remapped pool.
	p, model, ok := provider.Resolve("codex/gpt-6.1-sol")
	if !ok {
		t.Fatal("expected codex/gpt-6.1-sol to resolve after sign-in")
	}
	_ = model
	p.NativeCompaction = true
	if err := provider.Save(p); err != nil {
		t.Fatal(err)
	}
	hits = nil
	up2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		hits = append(hits, string(b))
		if !strings.Contains(string(b), "compaction_trigger") {
			t.Errorf("toggle on should keep compaction_trigger: %s", b)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, nativeSealedReply("gAAAAB-sealed-by-codex-pool"))
	}))
	t.Cleanup(up2.Close)
	provider.CodexBase = up2.URL + "/backend-api/codex"

	code, body = codexPost(t, strings.Replace(compactTurn, "%s", "gpt-6.1-sol", 1))
	if code != 200 {
		t.Fatalf("toggle on: %d %s\nhits: %v", code, body, hits)
	}
	if !strings.Contains(body, `"encrypted_content":"gAAAAB-sealed-by-codex-pool"`) || strings.Contains(body, magpieCompaction) {
		t.Fatalf("toggle on should keep native seal: %s", body)
	}
}
