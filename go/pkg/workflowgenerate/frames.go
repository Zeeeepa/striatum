package workflowgenerate

import (
	"crypto/sha256"
	"encoding/binary"
	"sort"
)

// Frame is one cognitive vantage the divergent_ideation shape binds to a
// divergence branch (RFC 0087 + RFC 0129). A frame is an *authoring input*, not
// live runner state: the generator injects the frame's vantage into a branch
// job's objective/prompt and the frame itself never survives into the workflow
// snapshot as a schema field. RFC 0129 adds `Kind`/`Category`/`Dimensions` so the
// selector can keep one run's frames structurally distinct (the anti-redundancy
// gate) instead of copying ADHD's flat persona-only table.
type Frame struct {
	ID       string
	Category string // persona | operation | temporal_forensic | risk_pricing
	Kind     string // persona | operation
	// Vantage is the one-line instruction seed injected into the branch prompt.
	Vantage string
	// Tags are ADHD's coarse selection tags (code/design/general/wild), kept for
	// backward-compatible selection and the "at least one wild frame" rule.
	Tags []string
	// Dimensions are the distortion axes the anti-redundancy gate compares. Two
	// frames that share >=2 dimension values induce the same structural
	// distortion and must not co-occur in one run (RFC 0129).
	Dimensions map[string]string
	// Transform, set only for operation frames, is the rewrite applied to the
	// problem statement *before* the branch generates (the transform-then-generate
	// path). MinStructure gates operation frames out of low-structure problems
	// where they collapse to jargon.
	Transform    string
	MinStructure string // low | medium | high (operation frames only)
}

// IsWild reports whether the frame carries ADHD's `wild` tag (the range-keeping
// vantages the selector guarantees at least one of).
func (f Frame) IsWild() bool {
	for _, tag := range f.Tags {
		if tag == "wild" {
			return true
		}
	}
	return false
}

// defaultFrames is the curated library: ADHD's 15 personas plus the three
// categories RFC 0129's multi-model divergent run surfaced (operation/transform,
// temporal-forensic, risk-pricing). The personas carry a single unique dimension
// so they never trip the anti-redundancy gate against each other; the three new
// categories carry the distortion axes the gate compares.
var defaultFrames = []Frame{
	// ---- Personas (ADHD's original library) ----
	{ID: "hardware_engineer", Category: "persona", Kind: "persona", Tags: []string{"code", "wild"}, Dimensions: map[string]string{"persona": "hardware_engineer"}, Vantage: "Re-ask this as a hardware/firmware problem: what do bus topology, cache, and the timing budget tell you?"},
	{ID: "regulator", Category: "persona", Kind: "persona", Tags: []string{"design", "general"}, Dimensions: map[string]string{"persona": "regulator"}, Vantage: "Audit the system for what must be provable, traceable, or refusable; surface failure modes."},
	{ID: "naive_ten_year_old", Category: "persona", Kind: "persona", Tags: []string{"general", "wild"}, Dimensions: map[string]string{"persona": "naive_ten_year_old"}, Vantage: "You are a curious 10-year-old who has never seen software; describe naive but unencumbered approaches, ignore convention."},
	{ID: "competitor", Category: "persona", Kind: "persona", Tags: []string{"code", "design"}, Dimensions: map[string]string{"persona": "competitor"}, Vantage: "You are a hostile competitor/attacker; generate ways to exploit or sabotage the obvious solution, then invert them into ideas."},
	{ID: "biology", Category: "persona", Kind: "persona", Tags: []string{"code", "wild"}, Dimensions: map[string]string{"persona": "biology"}, Vantage: "Transplant a mechanism from biology (immune systems, neural plasticity, evolution, gut flora) onto this problem."},
	{ID: "logistics", Category: "persona", Kind: "persona", Tags: []string{"code", "design"}, Dimensions: map[string]string{"persona": "logistics"}, Vantage: "Steal mechanisms from logistics: queues, batching, just-in-time, hub-and-spoke, returns, last-mile."},
	{ID: "game_design", Category: "persona", Kind: "persona", Tags: []string{"design", "general"}, Dimensions: map[string]string{"persona": "game_design"}, Vantage: "Approach as a game designer: loops, rewards, friction, save-states, speedrun tricks; the user is a player."},
	{ID: "markets", Category: "persona", Kind: "persona", Tags: []string{"design", "wild"}, Dimensions: map[string]string{"persona": "markets"}, Vantage: "Treat the problem as a market: buyers, sellers, market-makers, auctions, futures, clearing houses."},
	{ID: "inversion", Category: "persona", Kind: "persona", Tags: []string{"code", "design", "general"}, Dimensions: map[string]string{"persona": "inversion"}, Vantage: "Ask the OPPOSITE question: brainstorm how to guarantee NOT-X, then negate each answer back."},
	{ID: "extreme_cheap", Category: "persona", Kind: "persona", Tags: []string{"code", "general"}, Dimensions: map[string]string{"persona": "extreme_cheap"}, Vantage: "No money, no team, one hour: what is the crudest version that still does the load-bearing thing?"},
	{ID: "extreme_lavish", Category: "persona", Kind: "persona", Tags: []string{"design", "wild"}, Dimensions: map[string]string{"persona": "extreme_lavish"}, Vantage: "Infinite compute, infinite engineers, a decade: what is the maximalist version?"},
	{ID: "remove_assumption", Category: "persona", Kind: "persona", Tags: []string{"code", "design", "wild"}, Dimensions: map[string]string{"persona": "remove_assumption"}, Vantage: "Name the thing everyone treats as fixed (framework, DB, request-response, network); imagine it is gone — what is possible?"},
	{ID: "speedrunner", Category: "persona", Kind: "persona", Tags: []string{"code", "wild"}, Dimensions: map[string]string{"persona": "speedrunner"}, Vantage: "You are a speedrunner: find glitches, skips, out-of-bounds tricks, the abusive-but-legal path."},
	{ID: "ant_colony", Category: "persona", Kind: "persona", Tags: []string{"code", "wild"}, Dimensions: map[string]string{"persona": "ant_colony"}, Vantage: "No central planner: many dumb agents, local rules, pheromone trails; how does the problem solve itself emergently?"},
	{ID: "on_call_3am", Category: "persona", Kind: "persona", Tags: []string{"code", "design"}, Dimensions: map[string]string{"persona": "on_call_3am"}, Vantage: "You are the on-call engineer woken at 3am when this breaks; design so you never get paged."},

	// ---- Operation / transform frames (RFC 0129: a frame is a verb on the problem, not a persona) ----
	{ID: "lossy_compression", Category: "operation", Kind: "operation", Tags: []string{"code", "design"}, Dimensions: map[string]string{"operates_on": "representation", "min_structure": "medium"}, MinStructure: "medium", Transform: "Strip the problem to the smallest version that preserves its essential difficulty, then solve only that kernel.", Vantage: "Solve only the irreducible kernel of the problem; report which constraints were decorative versus structural."},
	{ID: "dual_problem", Category: "operation", Kind: "operation", Tags: []string{"code", "design"}, Dimensions: map[string]string{"operates_on": "objective", "min_structure": "high"}, MinStructure: "high", Transform: "Invert the optimization target: maximize the thing being avoided and describe what you would build to achieve it reliably.", Vantage: "Design the failure on purpose; the load-bearing machinery you must build to do it reliably reveals the real solution."},
	{ID: "load_bearing_wall", Category: "operation", Kind: "operation", Tags: []string{"code", "design"}, Dimensions: map[string]string{"operates_on": "constraints", "min_structure": "low"}, MinStructure: "low", Transform: "Name the single assumption that, if removed, makes the whole problem dissolve or trivialize.", Vantage: "Design as if you must remove the structural keystone assumption; what becomes possible?"},
	{ID: "conservation_law", Category: "operation", Kind: "operation", Tags: []string{"design"}, Dimensions: map[string]string{"operates_on": "invariant", "min_structure": "low"}, MinStructure: "low", Transform: "Name the quantity that must stay constant no matter what you change (attention, trust, total effort, headcount).", Vantage: "Forbid any solution that secretly violates the conserved quantity; convert vague tradeoffs into a hard ledger."},
	{ID: "resolution_sweep", Category: "operation", Kind: "operation", Tags: []string{"design", "general"}, Dimensions: map[string]string{"operates_on": "representation", "min_structure": "low"}, MinStructure: "low", Transform: "Re-state the problem at three zoom levels (one sentence, one paragraph, one page).", Vantage: "Keep only the constraints that survive all three resolutions; the rest are scale-dependent decoration."},
	{ID: "time_reverse", Category: "operation", Kind: "operation", Tags: []string{"code", "general"}, Dimensions: map[string]string{"operates_on": "causal_order", "min_structure": "medium"}, MinStructure: "medium", Transform: "Start from the finished, obviously-correct outcome and run the problem backward; narrate the last decision first.", Vantage: "Surface the hidden preconditions and ordering constraints that forward reasoning skips."},
	{ID: "adversarial_boundary", Category: "operation", Kind: "operation", Tags: []string{"code", "design"}, Dimensions: map[string]string{"operates_on": "constraints", "min_structure": "medium"}, MinStructure: "medium", Transform: "Find the input that technically satisfies every stated requirement while violating its spirit.", Vantage: "Redesign so that letter-satisfying, spirit-violating input is impossible; harden the spec, not just the code."},

	// ---- Temporal-forensic frames (RFC 0129: fix a point in time as ground truth and reason from it) ----
	{ID: "pre_mortem", Category: "temporal_forensic", Kind: "persona", Tags: []string{"design", "general"}, Dimensions: map[string]string{"time_anchor": "future", "agency": "forensic", "evidence_type": "residue"}, Vantage: "It is 18 months from now and this shipped and quietly died; write its cause-of-death report, then design backward from the morgue."},
	{ID: "decade_inheritor", Category: "temporal_forensic", Kind: "persona", Tags: []string{"design", "code"}, Dimensions: map[string]string{"time_anchor": "future", "agency": "custodial", "evidence_type": "inherited_debt"}, Vantage: "You did not build this and must keep it alive for ten years after the author quit, with no docs; design the version you would not curse."},
	{ID: "returning_exile", Category: "temporal_forensic", Kind: "persona", Tags: []string{"design", "general"}, Dimensions: map[string]string{"time_anchor": "past", "agency": "reflective", "evidence_type": "contingency"}, Vantage: "You left this domain 20 years ago and just came back; name everything the field now takes for granted that was once a live, contested choice."},
	{ID: "archaeologist_2200", Category: "temporal_forensic", Kind: "persona", Tags: []string{"general", "wild"}, Dimensions: map[string]string{"time_anchor": "deep_future", "agency": "zero", "evidence_type": "fossil_record"}, Vantage: "Excavate this artifact 174 years from now with no living witnesses; reconstruct what problem it solved from physical evidence alone."},
	{ID: "fossil_record_auditor", Category: "temporal_forensic", Kind: "persona", Tags: []string{"code", "design"}, Dimensions: map[string]string{"time_anchor": "present", "agency": "forensic", "evidence_type": "inherited_debt"}, Vantage: "Collect every workaround and duct-tape patch; treat them as the fossil record of what the spec got wrong, then write the corrected spec."},

	// ---- Risk-pricing / accountability frames (RFC 0129: price the downside / who is on the hook) ----
	{ID: "actuary", Category: "risk_pricing", Kind: "persona", Tags: []string{"design"}, Dimensions: map[string]string{"axis": "magnitude"}, Vantage: "Build the loss distribution: probability band x impact multiplier per failure; price the <0.1%, >10x tail."},
	{ID: "liability_chain_auditor", Category: "risk_pricing", Kind: "persona", Tags: []string{"design", "general"}, Dimensions: map[string]string{"axis": "ownership"}, Vantage: "Trace every decision to its last accountable owner; at which handoff does responsibility evaporate, and who is named when it fails?"},
	{ID: "short_seller", Category: "risk_pricing", Kind: "persona", Tags: []string{"design"}, Dimensions: map[string]string{"axis": "fragility"}, Vantage: "State the bull thesis in one sentence; name the single assumption whose falsity voids it; bet against it cheaply before consensus notices."},
	{ID: "liquidity_provider", Category: "risk_pricing", Kind: "persona", Tags: []string{"design"}, Dimensions: map[string]string{"axis": "hidden_subsidy"}, Vantage: "Who silently bears the volatility and what do they implicitly charge; which hidden subsidy makes the system look cheaper than it is?"},
}

// frameByID returns a frame from the default library by id.
func frameByID(id string) (Frame, bool) {
	for _, f := range defaultFrames {
		if f.ID == id {
			return f, true
		}
	}
	return Frame{}, false
}

// frameRank is a deterministic, seed-derived rank for a frame id. Selection
// orders the pool by this rank so the same workflow_id always picks the same
// frames (reproducible) while different ids explore differently — replacing
// ADHD's nondeterministic "vary across generations" with something testable.
// Date.now()/rand are unavailable to the generator by design; the seed is the
// workflow_id.
func frameRank(seed, id string) uint64 {
	sum := sha256.Sum256([]byte(seed + ":" + id))
	return binary.BigEndian.Uint64(sum[:8])
}

// sharedDimensionCount counts distortion-axis dimensions two frames share by
// value. The anti-redundancy gate (RFC 0129) refuses a run that would place two
// frames sharing >=2 dimensions in the same diverge pool, because they induce
// the same structural distortion and would produce paraphrase branches — the
// silent-frame-failure ADHD's paper admits its harness cannot catch.
func sharedDimensionCount(a, b Frame) int {
	n := 0
	for key, value := range a.Dimensions {
		if other, ok := b.Dimensions[key]; ok && other == value {
			n++
		}
	}
	return n
}

func violatesAntiRedundancy(picked []Frame, candidate Frame) bool {
	for _, p := range picked {
		if sharedDimensionCount(p, candidate) >= 2 {
			return true
		}
	}
	return false
}

func containsFrame(frames []Frame, id string) bool {
	for _, f := range frames {
		if f.ID == id {
			return true
		}
	}
	return false
}

func anyWild(frames []Frame) bool {
	for _, f := range frames {
		if f.IsWild() {
			return true
		}
	}
	return false
}

// selectFrames picks `count` frames from the curated library for one run,
// deterministically seeded by `seed` (the workflow_id). It enforces three
// policies from RFC 0129:
//
//   - min-structure gate: operation frames are excluded on a "low"-structure
//     problem, where a transform has nothing to bite into and degrades to jargon;
//   - anti-redundancy: no two selected frames share >=2 distortion-axis
//     dimensions, so the run's branches are provably distinct;
//   - at least one wild frame, to keep range (ADHD's selection heuristic).
//
// If the anti-redundancy gate makes `count` unreachable from the eligible pool,
// the remainder is backfilled ignoring the gate (a smaller library should never
// produce an empty run); the gate is a quality preference, not a hard cap.
func selectFrames(seed string, count int, problemShape string) []Frame {
	if count < 1 {
		count = 1
	}
	pool := make([]Frame, 0, len(defaultFrames))
	for _, f := range defaultFrames {
		if problemShape == "low" && f.Kind == "operation" {
			continue // min-structure gate
		}
		pool = append(pool, f)
	}
	sort.SliceStable(pool, func(i, j int) bool {
		return frameRank(seed, pool[i].ID) < frameRank(seed, pool[j].ID)
	})

	picked := make([]Frame, 0, count)
	for _, f := range pool {
		if len(picked) >= count {
			break
		}
		if violatesAntiRedundancy(picked, f) {
			continue
		}
		picked = append(picked, f)
	}
	// Guarantee at least one wild frame (range), preferring to swap in a wild
	// frame that does not break the anti-redundancy invariant.
	if len(picked) > 0 && !anyWild(picked) {
		for _, f := range pool {
			if !f.IsWild() || containsFrame(picked, f.ID) {
				continue
			}
			trial := append([]Frame{}, picked[:len(picked)-1]...)
			if violatesAntiRedundancy(trial, f) {
				continue
			}
			picked = append(trial, f)
			break
		}
	}
	// Backfill if the gate left us short of count (relax the gate last).
	if len(picked) < count {
		for _, f := range pool {
			if len(picked) >= count {
				break
			}
			if containsFrame(picked, f.ID) {
				continue
			}
			picked = append(picked, f)
		}
	}
	if len(picked) > count {
		picked = picked[:count]
	}
	return picked
}

// distinctModelFamilies counts the distinct lane display models among the given
// lane ids in the compiled lanes map. The divergent_ideation multi-model
// convergence signal (RFC 0129) is only meaningful when branches span >=2
// families; a single-family run degrades to ADHD's stated same-model judging
// limitation and earns a generator warning.
func distinctModelFamilies(lanes map[string]any, laneIDs []string) int {
	seen := map[string]struct{}{}
	for _, id := range laneIDs {
		lane := mapFrom(lanes[id])
		display, _ := lane["display_model"].(string)
		if display == "" {
			display = id
		}
		seen[display] = struct{}{}
	}
	return len(seen)
}
