package provider

import (
	"os"
	"testing"
	"time"
)

// A command that only reads, run as a new process each time, looks at the
// agents' sign-ins again: when all it would change in logins.json is a
// Seen a few minutes old, it leaves the file as it is. A long-running
// magpie may have just renewed a saved account's token there, and writing
// back what was read before would put the old token back (#1674).
func TestRememberLoginsLeavesFreshSeenUnwritten(t *testing.T) {
	claudeAuthFixture(t, "")
	newProcess := func() {
		forgetClaudeCredential()
		loginsMu.Lock()
		loginsSeenAt = time.Time{}
		loginsMu.Unlock()
		rememberLogins(false)
	}
	newProcess() // the own account is saved and seen now
	before, err := os.Stat(loginsPath())
	if err != nil {
		t.Fatal(err)
	}
	b0, _ := os.ReadFile(loginsPath())

	newProcess()
	after, err := os.Stat(loginsPath())
	if err != nil {
		t.Fatal(err)
	}
	if b1, _ := os.ReadFile(loginsPath()); !os.SameFile(before, after) || string(b0) != string(b1) {
		t.Fatalf("logins.json rewritten when only Seen would change")
	}
}

// Seen long ago is still brought up to date: renewalDue counts from it.
func TestRememberLoginsUpdatesStaleSeen(t *testing.T) {
	claudeAuthFixture(t, "")
	rememberLogins(true)
	ls := readLogins()
	old := time.Now().Add(-2 * loginsSeenEvery).UTC().Truncate(time.Second)
	found := false
	for i := range ls {
		if ls[i].User == "own@example.com" {
			ls[i].Seen, found = old, true
		}
	}
	if !found {
		t.Fatalf("own account not saved: %+v", ls)
	}
	if err := writeLogins(ls); err != nil {
		t.Fatal(err)
	}
	forgetClaudeCredential()
	loginsMu.Lock()
	loginsSeenAt = time.Time{}
	loginsMu.Unlock()
	rememberLogins(false)
	for _, l := range readLogins() {
		if l.User == "own@example.com" && !l.Seen.After(old) {
			t.Fatalf("stale Seen kept: %v", l.Seen)
		}
	}
}
