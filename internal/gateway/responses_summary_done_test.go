package gateway

import (
	"strings"
	"testing"
)

// A Responses upstream that writes reasoning only in output_item.done
// summary[] (no reasoning_*_text.delta) must still emit thinking (#2382).
func TestResponsesSummaryOnlyOnItemDone(t *testing.T) {
	var d responsesDecoder
	var think strings.Builder
	var kinds []EventKind
	stream := []string{
		`{"type":"response.created","response":{"id":"r1","model":"m"}}`,
		`{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`,
		`{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"plan A then B"}]}}`,
		`{"type":"response.output_item.added","output_index":1,"item":{"id":"m1","type":"message","role":"assistant","content":[]}}`,
		`{"type":"response.output_text.delta","delta":"ok"}`,
		`{"type":"response.completed","response":{"id":"r1","status":"completed","usage":{"output_tokens_details":{"reasoning_tokens":12}}}}`,
	}
	for _, c := range stream {
		if err := d.decode(c, func(e Event) {
			kinds = append(kinds, e.Kind)
			if e.Kind == KThink {
				think.WriteString(e.Text)
			}
		}); err != nil {
			t.Fatal(err)
		}
	}
	if think.String() != "plan A then B" {
		t.Fatalf("thinking = %q, want summary text", think.String())
	}
	// a stream that already sent deltas must not double the summary
	d = responsesDecoder{}
	think.Reset()
	stream2 := []string{
		`{"type":"response.created","response":{"id":"r2"}}`,
		`{"type":"response.output_item.added","item":{"id":"rs_2","type":"reasoning","summary":[]}}`,
		`{"type":"response.reasoning_summary_text.delta","delta":"already"}`,
		`{"type":"response.output_item.done","item":{"id":"rs_2","type":"reasoning","summary":[{"type":"summary_text","text":"already streamed"}]}}`,
	}
	for _, c := range stream2 {
		_ = d.decode(c, func(e Event) {
			if e.Kind == KThink {
				think.WriteString(e.Text)
			}
		})
	}
	if think.String() != "already" {
		t.Fatalf("doubled thinking = %q", think.String())
	}
}
