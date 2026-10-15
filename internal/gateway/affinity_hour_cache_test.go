package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/provider"
)

// hourCacheWriter is an Anthropic Messages upstream whose usage splits the
// cache it wrote by how long it is kept, as Anthropic's cache_creation
// does when it was asked for a 1-hour ttl.
type hourCacheWriter struct {
	mu                     sync.Mutex
	read, wrote5m, wrote1h int
}

func (c *hourCacheWriter) set(read, wrote5m, wrote1h int) {
	c.mu.Lock()
	c.read, c.wrote5m, c.wrote1h = read, wrote5m, wrote1h
	c.mu.Unlock()
}

func (c *hourCacheWriter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	io.ReadAll(r.Body)
	c.mu.Lock()
	read, w5, w1 := c.read, c.wrote5m, c.wrote1h
	c.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	usage := `"cache_read_input_tokens":` + itoa(read) + `,"cache_creation_input_tokens":` + itoa(w5+w1) +
		`,"cache_creation":{"ephemeral_5m_input_tokens":` + itoa(w5) + `,"ephemeral_1h_input_tokens":` + itoa(w1) + `}`
	for _, ev := range []string{
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"m","content":[],"usage":{"input_tokens":12,"output_tokens":1,` + usage + `}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"ok"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`,
		`{"type":"message_stop"}`,
	} {
		var t struct {
			Type string `json:"type"`
		}
		json.Unmarshal([]byte(ev), &t)
		io.WriteString(w, "event: "+t.Type+"\ndata: "+ev+"\n\n")
	}
}

// aged moves every remembered answer back by d, as if d had passed since.
func aged(d time.Duration) {
	sticks.Lock()
	for k, st := range sticks.m {
		st.at = st.at.Add(-d)
		sticks.m[k] = st
	}
	sticks.Unlock()
}

// A cache Claude reported writing for an hour (ephemeral_1h_input_tokens)
// keeps Stays: Auto on its account for that hour, not only for the five
// minutes a default cache lasts (#2302, j3r0lin). Across a restart too, as
// the file has it; a turn that read that cache again and wrote only a
// little keeps the hour; a 5-minute cache still goes cold after five
// minutes, and an hour cache after an hour.
func TestAffinityKeepsAnHourCacheForItsHour(t *testing.T) {
	turns := []string{
		`{"model":"hc/m","messages":[{"role":"user","content":"hi"}]}`,
		`{"model":"hc/m","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"ok"},{"role":"user","content":"more"}]}`,
		`{"model":"hc/m","messages":[{"role":"user","content":"hi"},{"role":"assistant","content":"ok"},{"role":"user","content":"more"},{"role":"assistant","content":"ok"},{"role":"user","content":"again"}]}`,
	}
	type reply struct{ read, wrote5m, wrote1h int }
	for _, c := range []struct {
		name    string
		replies []reply // what each turn but the last reports
		gap     time.Duration
		restart bool
		why     string
	}{
		{"hour cache, 10 minutes on", []reply{{0, 0, 3000}}, 10 * time.Minute, false, "cache"},
		{"hour cache, 10 minutes on, restarted", []reply{{0, 0, 3000}}, 10 * time.Minute, true, "cache"},
		{"hour cache read again, 10 minutes on", []reply{{0, 0, 3000}, {3000, 0, 40}}, 10 * time.Minute, false, "cache"},
		{"hour cache, past the hour", []reply{{0, 0, 3000}}, 61 * time.Minute, false, "cold"},
		{"5-minute cache, 10 minutes on", []reply{{0, 3000, 0}}, 10 * time.Minute, false, "cold"},
		{"5-minute cache read again, 10 minutes on", []reply{{0, 3000, 0}, {3000, 40, 0}}, 10 * time.Minute, false, "cold"},
		{"5-minute cache, 2 minutes on", []reply{{0, 3000, 0}}, 2 * time.Minute, false, "cache"},
	} {
		fresh(t)
		v := &hourCacheWriter{}
		up := httptest.NewServer(v)
		t.Cleanup(up.Close)
		p := provider.Provider{ID: "hc", Name: "HC", Key: "k1", Models: []string{"m"}, Anthropic: up.URL + "/v1",
			Keys: []provider.KeyAccount{{Key: "k2"}}}
		if err := provider.Save(p); err != nil {
			t.Fatal(err)
		}
		s := New()
		for i, r := range c.replies {
			v.set(r.read, r.wrote5m, r.wrote1h)
			if code, body := postAs(t, s, "s1", turns[i]); code != 200 {
				t.Fatalf("%s: turn %d: %d %s", c.name, i+1, code, body)
			}
		}
		aged(c.gap)
		if c.restart {
			saveSticks()
			sticks.Lock()
			sticks.m, sticks.from, sticks.mod = map[string]stick{}, "", time.Time{}
			sticks.Unlock()
			s = New()
		}
		v.set(3000, 0, 0)
		if code, body := postAs(t, s, "s1", turns[len(c.replies)]); code != 200 {
			t.Fatalf("%s: last turn %d %s", c.name, code, body)
		}
		a := s.trace.routes[len(s.trace.routes)-1].Affinity
		if a == nil || a.Why != c.why || a.Kept != (c.why == "cache") {
			t.Fatalf("%s: affinity %+v, want %s", c.name, a, c.why)
		}
	}
}
