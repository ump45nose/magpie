package sessions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// OpenCode 2 keeps a child session's agent in session_v2.agent and titles
// it with its task alone (#2290, infinitr0us: 132 children, every one with
// an agent, none with "(@general subagent)" in its title), so the Sessions
// page takes its type from there. A 1.x title still gives it when the
// column is empty, and a session_v2 with no agent column still lists the
// subagent, without a type. Each change bumps time_updated, as OpenCode
// does when it writes a session.
func TestOpenCodeSubagentKindFromAgentColumn(t *testing.T) {
	inZone(t, 0)
	data, _ := setupMore(t)
	db := ocMakeV2(t, filepath.Join(data, "opencode", "opencode.db"))
	sub := func(step string) Subagent {
		t.Helper()
		Reset()
		s := find(t, List(0), "opencode", ocMain)
		if len(s.Subagents) != 1 || s.Subagents[0].ID != ocChild {
			t.Fatalf("%s: subagents %+v", step, s.Subagents)
		}
		return s.Subagents[0]
	}
	if _, err := db.Exec(`UPDATE session_v2 SET agent = 'explore', title = 'Port Phase2 physics', time_updated = 1790420000000 WHERE id = ?`, ocChild); err != nil {
		t.Fatal(err)
	}
	if sa := sub("agent column"); sa.Kind != "explore" || sa.Title != "Port Phase2 physics" {
		t.Fatalf("agent column: %+v, want explore, Port Phase2 physics", sa)
	}
	if _, err := db.Exec(`UPDATE session_v2 SET agent = NULL, title = 'Find the flaky test (@general subagent)', time_updated = 1790420001000 WHERE id = ?`, ocChild); err != nil {
		t.Fatal(err)
	}
	if sa := sub("1.x title"); sa.Kind != "general" || sa.Title != "Find the flaky test" {
		t.Fatalf("1.x title: %+v, want general, Find the flaky test", sa)
	}
	if _, err := db.Exec(`ALTER TABLE session_v2 DROP COLUMN agent`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE session_v2 SET title = 'Port Phase2 physics', time_updated = 1790420002000 WHERE id = ?`, ocChild); err != nil {
		t.Fatal(err)
	}
	if sa := sub("no agent column"); sa.Kind != "" || sa.Title != "Port Phase2 physics" {
		t.Fatalf("no agent column: %+v, want no kind, Port Phase2 physics", sa)
	}
}

// In a Kimi Code subagent's own wire the first turn.prompt is the task it
// was given, origin {kind: system_trigger, name: subagent} (#2290). It shows
// as the subagent's prompt, as Claude Code's and OpenCode's tasks do, not
// folded as context. Another system_trigger there, and the same prompt in
// the session's main wire, stay folded.
func TestKimiSubagentTaskIsItsPrompt(t *testing.T) {
	inZone(t, 0)
	claude, _ := setup(t)
	dir := filepath.Join(filepath.Dir(claude), "home", ".kimi-code")
	kimiSessionsIn(t, dir)
	first := func(s Session, sub string) Part {
		t.Helper()
		Reset()
		tr, err := SubagentTranscript(s, sub)
		if err != nil || len(tr.Parts) == 0 {
			t.Fatalf("subagent transcript %+v %v", tr, err)
		}
		return tr.Parts[0]
	}
	s := find(t, List(0), "kimi", kimiJune)
	if len(s.Subagents) != 1 {
		t.Fatalf("subagents %+v", s.Subagents)
	}
	wire := s.Subagents[0].Path
	if p := first(s, "agent-0"); p.Kind != "context" {
		t.Fatalf("another system_trigger: %+v, want it folded", p)
	}
	b, err := os.ReadFile(wire)
	if err != nil {
		t.Fatal(err)
	}
	const other, task = `"origin":{"kind":"system_trigger","name":"[redacted]"}`, `"origin":{"kind":"system_trigger","name":"subagent"}`
	if !strings.Contains(string(b), other) {
		t.Fatalf("fixture changed: no %s in %s", other, wire)
	}
	if err := os.WriteFile(wire, []byte(strings.Replace(string(b), other, task, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	s = find(t, List(0), "kimi", kimiJune)
	if p := first(s, "agent-0"); p.Role != "user" || p.Kind != "text" || p.Text != "[redacted]" {
		t.Fatalf("the subagent's task: %+v, want its prompt", p)
	}

	main := filepath.Join(filepath.Dir(filepath.Dir(wire)), "main", "wire.jsonl")
	f, err := os.OpenFile(main, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(`{"type":"turn.prompt","input":[{"type":"text","text":"Subagent news"}],` + task + `,"time":1781503099000}` + "\n")
	f.Close()
	Reset()
	tr, err := TranscriptOf(find(t, List(0), "kimi", kimiJune))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range tr.Parts {
		if p.Text == "Subagent news" && p.Kind != "context" {
			t.Fatalf("main wire: %+v, want it folded", p)
		}
	}
	if !strings.Contains(transcriptText(t, tr), "Subagent news") {
		t.Fatalf("main wire lost the prompt:\n%s", transcriptText(t, tr))
	}
}
