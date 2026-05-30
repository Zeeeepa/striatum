package mutations

import "testing"

// #77: run.prepare gates review/phase_synthesis edges on a clearing verdict,
// EXCEPT edges into an adjudicator (phase_synthesis), which stay ungated so the
// adjudicator can absorb a reviewer's needs_revision dissent.
func TestEdgeRequiresClearingVerdictExemptsAdjudicatorInbound(t *testing.T) {
	wf := map[string]any{"jobs": []any{
		map[string]any{"id": "cross_exam", "type": "review"},
		map[string]any{"id": "adjudicate", "type": "phase_synthesis"},
		map[string]any{"id": "implement", "type": "build"},
		map[string]any{"id": "proposal", "type": "draft"},
	}}
	cases := []struct {
		from, to string
		want     bool
	}{
		{"cross_exam", "adjudicate", false}, // review -> adjudicator: ungated (absorbs)
		{"cross_exam", "implement", true},   // review -> build: gated
		{"adjudicate", "implement", true},   // phase_synthesis -> build: gated
		{"adjudicate", "adjudicate", false}, // phase_synthesis -> phase_synthesis: ungated
		{"proposal", "cross_exam", false},   // draft -> review: not verdict-capable source
	}
	for _, tc := range cases {
		if got := edgeRequiresClearingVerdict(wf, tc.from, tc.to); got != tc.want {
			t.Errorf("edgeRequiresClearingVerdict(%s->%s) = %v, want %v", tc.from, tc.to, got, tc.want)
		}
	}
}
