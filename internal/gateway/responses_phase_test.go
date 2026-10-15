package gateway

import (
	"bufio"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// eventsFrom runs Events through a Responses encoder and returns every
// JSON data payload written (SSE data lines).
func eventsFrom(t *testing.T, phases bool, evs ...Event) []map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	enc := &responsesEncoder{w: newSSEWriter(rec), model: "m", phases: phases}
	for _, ev := range evs {
		enc.event(ev)
	}
	enc.finish()
	var out []map[string]any
	sc := bufio.NewScanner(strings.NewReader(rec.Body.String()))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			var m map[string]any
			if json.Unmarshal([]byte(data), &m) == nil {
				out = append(out, m)
			}
		}
	}
	return out
}

func assistantPhases(events []map[string]any) []string {
	var phases []string
	for _, ev := range events {
		if ev["type"] != "response.output_item.done" {
			continue
		}
		item, _ := ev["item"].(map[string]any)
		if item == nil || item["type"] != "message" || item["role"] != "assistant" {
			continue
		}
		ph, _ := item["phase"].(string)
		phases = append(phases, ph)
	}
	return phases
}

// A Codex turn with progress text, a tool call, then a final answer labels
// the first message commentary and the last final_answer (#2437).
func TestCodexPhasesLabelCommentaryThenFinal(t *testing.T) {
	out := eventsFrom(t, true,
		Event{Kind: KStart, MsgID: "r1", Model: "m"},
		Event{Kind: KText, Text: "Starting."},
		Event{Kind: KToolStart, ID: "c1", Name: "time"},
		Event{Kind: KToolArgs, Text: `{}`},
		Event{Kind: KText, Text: "Done."},
		Event{Kind: KStop, Stop: "stop"},
	)
	got := assistantPhases(out)
	if len(got) != 2 || got[0] != "commentary" || got[1] != "final_answer" {
		t.Fatalf("phases %v, want [commentary final_answer]", got)
	}
	// text deltas must still stream before the held done frame
	var sawDeltaBeforeDone bool
	var sawFirstDone bool
	for _, ev := range out {
		switch ev["type"] {
		case "response.output_text.delta":
			if !sawFirstDone {
				sawDeltaBeforeDone = true
			}
		case "response.output_item.done":
			item, _ := ev["item"].(map[string]any)
			if item["type"] == "message" && !sawFirstDone {
				sawFirstDone = true
			}
		}
	}
	if !sawDeltaBeforeDone {
		t.Fatal("text delta did not stream before the held message done")
	}
}

// Without phases (WorkBuddy and others), assistant messages stay unlabelled.
func TestNonCodexPhasesLeaveMessagesUnlabelled(t *testing.T) {
	out := eventsFrom(t, false,
		Event{Kind: KStart, MsgID: "r1", Model: "m"},
		Event{Kind: KText, Text: "Starting."},
		Event{Kind: KToolStart, ID: "c1", Name: "time"},
		Event{Kind: KToolArgs, Text: `{}`},
		Event{Kind: KText, Text: "Done."},
		Event{Kind: KStop, Stop: "stop"},
	)
	for _, ph := range assistantPhases(out) {
		if ph != "" {
			t.Fatalf("non-Codex message got phase %q", ph)
		}
	}
}

// Trailing reasoning after the last message does not turn that message into
// commentary; it remains the final answer.
func TestCodexPhasesTrailingReasoningKeepsFinalAnswer(t *testing.T) {
	out := eventsFrom(t, true,
		Event{Kind: KStart, MsgID: "r1", Model: "m"},
		Event{Kind: KText, Text: "Answer."},
		Event{Kind: KThink, Text: "hmm"},
		Event{Kind: KStop, Stop: "stop"},
	)
	got := assistantPhases(out)
	if len(got) != 1 || got[0] != "final_answer" {
		t.Fatalf("phases %v, want [final_answer]", got)
	}
}

// An incomplete (length) turn does not invent final_answer on a held message.
func TestCodexPhasesIncompleteReleasesWithoutFinal(t *testing.T) {
	rec := httptest.NewRecorder()
	enc := &responsesEncoder{w: newSSEWriter(rec), model: "m", phases: true}
	enc.event(Event{Kind: KStart, MsgID: "r1", Model: "m"})
	enc.event(Event{Kind: KText, Text: "Cut off"})
	enc.event(Event{Kind: KStop, Stop: "length"})
	enc.finish()
	got := assistantPhases(func() []map[string]any {
		var out []map[string]any
		sc := bufio.NewScanner(strings.NewReader(rec.Body.String()))
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var m map[string]any
				if json.Unmarshal([]byte(data), &m) == nil {
					out = append(out, m)
				}
			}
		}
		return out
	}())
	if len(got) != 1 || got[0] != "" {
		t.Fatalf("incomplete phases %v, want one unlabelled message", got)
	}
}

func TestLabelResponsesPhasesNonStream(t *testing.T) {
	body := []byte(`{"id":"resp_1","output":[
		{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"output_text","text":"a"}]},
		{"id":"fc_1","type":"function_call","call_id":"c","name":"t","arguments":"{}"},
		{"id":"msg_2","type":"message","role":"assistant","content":[{"type":"output_text","text":"b"}]}
	]}`)
	out := labelResponsesPhases(body)
	var q map[string]any
	if err := json.Unmarshal(out, &q); err != nil {
		t.Fatal(err)
	}
	items, _ := q["output"].([]any)
	p0 := items[0].(map[string]any)["phase"]
	p2 := items[2].(map[string]any)["phase"]
	if p0 != "commentary" || p2 != "final_answer" {
		t.Fatalf("phases %v %v", p0, p2)
	}
	// preserve an upstream phase
	body2 := []byte(`{"output":[{"id":"msg_1","type":"message","role":"assistant","phase":"commentary","content":[]}]}`)
	out2 := labelResponsesPhases(body2)
	var q2 map[string]any
	json.Unmarshal(out2, &q2)
	if q2["output"].([]any)[0].(map[string]any)["phase"] != "commentary" {
		t.Fatal("upstream phase was rewritten")
	}
}
