package webassets_test

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/webassets"
)

// TestRenderInterrogationStructure pins the chat-thread shape: ordered
// turns, the correct speaker label per kind, and the per-turn CSS class.
func TestRenderInterrogationStructure(t *testing.T) {
	meta := webassets.InterrogationMeta{
		InterrogationID:       "intg_1",
		RunID:                 "run_1",
		Topic:                 "threat model",
		State:                 "closed",
		InterrogatorSessionID: "sess_reviewer",
		TargetSessionID:       "sess_target",
		OpenedAt:              "2026-05-25T00:00:00Z",
		ClosedAt:              "2026-05-25T01:00:00Z",
	}
	turns := []webassets.InterrogationTurn{
		{Kind: "interrogation_question", Body: "first question"},
		{Kind: "interrogation_answer", Body: "first answer"},
	}
	html := string(mustRender(t, meta, turns))

	for _, want := range []string{
		"threat model", "sess_reviewer", "sess_target", "closed",
		"2026-05-25T00:00:00Z", "2026-05-25T01:00:00Z",
		"turn-question", "turn-answer", "Reviewer", "Target",
		"first question", "first answer",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("rendered output missing %q\n%s", want, html)
		}
	}

	// Question precedes answer (order preserved, no re-sort).
	if strings.Index(html, "first question") >= strings.Index(html, "first answer") {
		t.Fatalf("turn order not preserved:\n%s", html)
	}
	// Speaker labels track kind: the question turn is the reviewer bubble.
	qBlock := html[strings.Index(html, "turn-question"):strings.Index(html, "turn-answer")]
	if !strings.Contains(qBlock, "Reviewer") || strings.Contains(qBlock, "Target") {
		t.Fatalf("question turn mislabeled:\n%s", qBlock)
	}
}

// TestRenderInterrogationEscaping is the load-bearing XSS/D028 regression
// guard. It pins that RenderInterrogation uses html/template (not
// text/template): all three attacker-controlled body payloads must render as
// escaped, inert text. text/template would emit them raw and this test would
// fail — which is exactly the point.
func TestRenderInterrogationEscaping(t *testing.T) {
	const (
		scriptPayload = `<script>alert(1)</script>`
		imgPayload    = `<img src=x onerror=alert(1)>`
		mdLinkPayload = `[x](javascript:alert(1))`
	)
	turns := []webassets.InterrogationTurn{
		{Kind: "interrogation_question", Body: scriptPayload},
		{Kind: "interrogation_answer", Body: imgPayload + "\n" + mdLinkPayload},
	}
	html := string(mustRender(t, webassets.InterrogationMeta{Topic: "x"}, turns))

	// The raw executable forms must be absent.
	for _, raw := range []string{
		"<script>alert(1)</script>",
		"<img src=x onerror=alert(1)>",
	} {
		if strings.Contains(html, raw) {
			t.Fatalf("raw executable payload leaked into output: %q\n%s", raw, html)
		}
	}

	// The escaped forms must be present (html/template entity-encodes the
	// dangerous characters).
	for _, escaped := range []string{
		"&lt;script&gt;alert(1)&lt;/script&gt;",
		"&lt;img src=x onerror=alert(1)&gt;",
	} {
		if !strings.Contains(html, escaped) {
			t.Fatalf("expected escaped form %q not present\n%s", escaped, html)
		}
	}

	// The markdown link is rendered as literal text — no <a href> is emitted,
	// which also locks in the "no Markdown parsing" decision. The escaped
	// parentheses prove the literal text survived; no anchor tag is created.
	if strings.Contains(html, `href="javascript:`) || strings.Contains(html, "<a ") {
		t.Fatalf("markdown link became a live anchor:\n%s", html)
	}
	if !strings.Contains(html, "[x](javascript:alert(1))") {
		t.Fatalf("markdown link text not rendered as literal inert text\n%s", html)
	}
}

func mustRender(t *testing.T, meta webassets.InterrogationMeta, turns []webassets.InterrogationTurn) []byte {
	t.Helper()
	body, err := webassets.RenderInterrogation(meta, turns)
	if err != nil {
		t.Fatalf("RenderInterrogation: %v", err)
	}
	return body
}
