package library

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Kiro CLI keeps its skills, MCP servers and instructions in ~/.kiro
// (kiro.dev/docs/cli/skills, mcp/configuration, steering); magpie wires no
// model into it, so it is one of the library's apps, found by kiro-cli
// alone. ~/.kiro without it is Kiro Crew's or the Kiro IDE's, and no
// target.
func TestKiroCLITarget(t *testing.T) {
	h := sandbox(t)
	write(t, filepath.Join(h, ".kiro", "crew", "config.json"), "{}")
	if slices.Contains(ids(Targets()), "kiro-cli") {
		t.Fatalf("kiro-cli found without its program: %v", ids(Targets()))
	}

	holdCLI(t, h, "kiro-cli")
	tg := targetByID("kiro-cli")
	if tg == nil {
		t.Fatalf("no kiro-cli target: %v", ids(Targets()))
	}
	d := filepath.Join(h, ".kiro")
	if tg.Agent.Name != "Kiro CLI" || tg.Agent.Icon != "kiro-color" {
		t.Errorf("agent: %+v", tg.Agent)
	}
	if tg.Skills != filepath.Join(d, "skills") {
		t.Errorf("skills: %q", tg.Skills)
	}
	if tg.Instructions != filepath.Join(d, "steering", "AGENTS.md") {
		t.Errorf("instructions: %q", tg.Instructions)
	}
	if tg.MCP == nil || tg.MCP.Path != filepath.Join(d, "settings", "mcp.json") || tg.MCP.Format != fmtKiro {
		t.Errorf("mcp: %+v", tg.MCP)
	}
	for _, kind := range []string{"skills", "mcp", "instructions"} {
		for _, q := range []string{"kiro-cli", "Kiro CLI"} {
			if id, err := Takes(q, kind); id != "kiro-cli" || err != nil {
				t.Errorf("Takes(%q, %s): %q %v", q, kind, id, err)
			}
		}
	}
}

// Servers go into ~/.kiro/settings/mcp.json in the documented shape:
// command/args/env or url/headers under mcpServers, no type. The user's
// own servers, and their autoApprove on magpie's, are kept; SSE, which the
// file has no way to name, isn't written; a ${NAME} is Kiro's own syntax.
func TestKiroCLIMCP(t *testing.T) {
	h := sandbox(t)
	holdCLI(t, h, "kiro-cli")
	p := filepath.Join(h, ".kiro", "settings", "mcp.json")
	write(t, p, `{"mcpServers": {
  "mine": {"command": "uvx", "args": ["mine-mcp"], "disabled": true},
  "fs": {"command": "old", "autoApprove": ["*"], "disabledTools": ["rm"]}
}}`)

	agents := []string{"kiro-cli"}
	stdio := Server{Name: "fs", Transport: "stdio", Command: "npx", Args: []string{"-y", "@mcp/fs"}, Env: map[string]string{"TOKEN": "${FS_TOKEN}"}, Agents: agents}
	web := Server{Name: "web", Transport: "http", URL: "https://example.com/mcp", Headers: map[string]string{"Authorization": "Bearer ${WEB_TOKEN}"}, Agents: agents}
	for _, s := range []Server{stdio, web} {
		ok(t)(SaveServer("", s))
	}
	if _, err := SaveServer("", Server{Name: "old", Transport: "sse", URL: "https://example.com/sse", Agents: agents}); err != nil {
		t.Fatal(err)
	}

	got := read(t, p)
	var f struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(got), &f); err != nil {
		t.Fatalf("%v:\n%s", err, got)
	}
	if m := f.Servers["mine"]; m["command"] != "uvx" || m["disabled"] != true {
		t.Errorf("lost the user's server: %v", m)
	}
	fs := f.Servers["fs"]
	if fs["command"] != "npx" || fs["type"] != nil || fs["url"] != nil {
		t.Errorf("fs: %v", fs)
	}
	if e, _ := fs["env"].(map[string]any); e["TOKEN"] != "${FS_TOKEN}" {
		t.Errorf("fs env: %v", fs)
	}
	if a, _ := fs["autoApprove"].([]any); len(a) != 1 || a[0] != "*" {
		t.Errorf("fs lost the user's autoApprove: %v", fs)
	}
	if d, _ := fs["disabledTools"].([]any); len(d) != 1 || d[0] != "rm" {
		t.Errorf("fs lost the user's disabledTools: %v", fs)
	}
	w := f.Servers["web"]
	if w["url"] != "https://example.com/mcp" || w["type"] != nil || w["command"] != nil {
		t.Errorf("web: %v", w)
	}
	if hd, _ := w["headers"].(map[string]any); hd["Authorization"] != "Bearer ${WEB_TOKEN}" {
		t.Errorf("web headers: %v", w)
	}
	if _, in := f.Servers["old"]; in || strings.Contains(got, "example.com/sse") {
		t.Errorf("an SSE server was written:\n%s", got)
	}

	// taken off again, only magpie's go
	if _, err := SaveServer("old", Server{Name: "old", Transport: "sse", URL: "https://example.com/sse", Agents: []string{}}); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"fs", "web"} {
		ok(t)(SaveServer(n, Server{Name: n, Transport: "stdio", Command: "npx", Agents: []string{}}))
	}
	f.Servers = nil
	if err := json.Unmarshal([]byte(read(t, p)), &f); err != nil {
		t.Fatal(err)
	}
	if _, in := f.Servers["mine"]; !in || f.Servers["web"] != nil {
		t.Errorf("after taking them off: %v", f.Servers)
	}
}
