package adapterconformance

// TierAResult is the per-adapter outcome of the hermetic Tier-A evaluation: the
// applicable clause results, with the live clauses (C1, C3..C12) reported as
// StatusDeferred and the hermetic clauses (C0, C2) carrying real Pass/
// ContractFail outcomes.
type TierAResult struct {
	Adapter string         `json:"adapter"`
	Profile string         `json:"profile"`
	Clauses []ClauseResult `json:"clauses"`
}

// HermeticClauses are the clause ids asserted in Slice 1 with no live CLI,
// daemon, or PostgreSQL.
func HermeticClauses() []ClauseID { return []ClauseID{C0, C2} }

// IsHermetic reports whether a clause has a real Slice-1 hermetic assertion.
func IsHermetic(id ClauseID) bool {
	for _, h := range HermeticClauses() {
		if h == id {
			return true
		}
	}
	return false
}

// EvaluateTierA evaluates every clause that applies to the adapter's profile,
// in C0..C12 order. The hermetic clauses (C0, C2) run their real asserts; the
// rest report StatusDeferred. baseEnv is the controlled base environment for
// the C2 lane-env golden (nil uses the C2 golden's own battery).
func EvaluateTierA(spec AdapterSpec, baseEnv []string) TierAResult {
	in := AssertInput{Adapter: spec, BaseEnv: baseEnv}
	out := TierAResult{Adapter: spec.Name, Profile: string(spec.Profile)}
	for _, clause := range Contract() {
		if !clause.AppliesTo(spec.Profile) {
			continue
		}
		out.Clauses = append(out.Clauses, clause.Assert(in))
	}
	return out
}

// ContractFailures returns the contract-fail clause results in a TierAResult.
func (r TierAResult) ContractFailures() []ClauseResult {
	var fails []ClauseResult
	for _, c := range r.Clauses {
		if c.Status == StatusContractFail {
			fails = append(fails, c)
		}
	}
	return fails
}
