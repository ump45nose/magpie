package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Claude Code writes the /exit and /quit it answers itself into
// history.jsonl as the bare word, with no slash (#3224). Leaving a session
// is not a use: counted as one, it put LastUsed after the gateway's last
// request and Drift said "bypassed".
func TestClaudeLastUsedSkipsExit(t *testing.T) {
	home, _ := claudeSettings(t, `{}`)
	cc := claude(home)
	asked := time.Now().Add(-10 * time.Minute).Truncate(time.Second)
	var lines string
	for i, display := range []string{"fix the test", "exit", " quit ", "/model"} {
		lines += fmt.Sprintf(`{"display":%q,"timestamp":%d}`+"\n", display, asked.Add(time.Duration(i)*time.Minute).UnixMilli())
	}
	if err := os.WriteFile(filepath.Join(home, ".claude", "history.jsonl"), []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := cc.LastUsed(); !got.Equal(asked) {
		t.Fatalf("LastUsed = %v, want the prompt at %v", got, asked)
	}
}
