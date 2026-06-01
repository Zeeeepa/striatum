package adapterconformance

import "testing"

// TestEvaluateTierAAllAdaptersPass asserts the end-to-end Tier-A evaluation:
// every declared adapter's hermetic clauses (C0, C2) pass and the live clauses
// are reported as deferred, with profile scoping honored (agy SingleShot has
// fewer clauses than the Full adapters).
func TestEvaluateTierAAllAdaptersPass(t *testing.T) {
	for _, spec := range Adapters() {
		res := EvaluateTierA(spec, nil)
		if len(res.ContractFailures()) != 0 {
			t.Errorf("adapter %s has Tier-A contract failures: %+v", spec.Name, res.ContractFailures())
		}
		var hermeticPasses, deferred int
		for _, c := range res.Clauses {
			switch {
			case IsHermetic(c.Clause):
				if c.Status != StatusPass {
					t.Errorf("adapter %s clause %s should pass, got %s: %s", spec.Name, c.Clause, c.Status, c.Message)
				}
				hermeticPasses++
			default:
				if c.Status != StatusDeferred {
					t.Errorf("adapter %s clause %s should be deferred, got %s", spec.Name, c.Clause, c.Status)
				}
				deferred++
			}
		}
		if hermeticPasses != 2 {
			t.Errorf("adapter %s should have 2 hermetic clause passes (C0,C2), got %d", spec.Name, hermeticPasses)
		}
		if deferred == 0 {
			t.Errorf("adapter %s should have deferred live clauses", spec.Name)
		}
	}
}

// TestEvaluateTierAProfileClauseCount asserts Full adapters carry more clauses
// than the agy SingleShot profile (the multi-turn clauses are scoped out).
func TestEvaluateTierAProfileClauseCount(t *testing.T) {
	full := EvaluateTierA(AdapterSpec{Name: "claude_code", Command: []string{"claude"}, Profile: Full}, nil)
	single := EvaluateTierA(AdapterSpec{Name: "agy", Command: []string{"agy"}, Profile: SingleShot}, nil)
	if len(single.Clauses) >= len(full.Clauses) {
		t.Fatalf("SingleShot (%d clauses) must run fewer clauses than Full (%d)",
			len(single.Clauses), len(full.Clauses))
	}
}
