package workflowgenerate

import (
	"fmt"
	"sort"
	"strings"
)

// compileDivergentIdeation compiles the RFC 0087 divergent_ideation shape: a
// diverge -> converge -> deepen -> final-synthesis graph that widens a design
// space before narrowing it. It is the striatum-native, provenance-backed,
// provider-portable port of the ADHD method (UditAkhourii/adhd, MIT) — the model
// calls live in agent lanes, never in a state transition, and isolation is the
// existing fresh_session + disjoint-write-scope machinery, not a new sandbox.
//
// It is structured exactly like compileImplementationPanel (flat
// striatum.workflow.v1 with parallel_group fan-out + edges), the closest shipped
// shape, rather than RFC 0087's proposed v1.1 phases: the diverge and deepen
// fan-outs are parallel_groups and ordinary edges enforce diverge -> converge ->
// deepen -> final, which the scheduler already understands.
//
// RFC 0129 supplies the frame layer: branches round-robin across the lane ring so
// different models carry different frames (the multi-model signal ADHD's
// Claude-only loop cannot produce), frames are selected with the anti-redundancy
// + min-structure policy in frames.go, and the convergence critic can run on a
// different lane/family than the generators.
func compileDivergentIdeation(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, error) {
	branchCount, err := divergentBranchCount(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	deepenCount, err := divergentDeepenCount(spec, branchCount)
	if err != nil {
		return nil, nil, nil, err
	}
	ideasPerBranch, err := divergentIdeasPerBranch(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	problemShape, err := divergentProblemShape(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	convergenceLane, err := divergentConvergenceLane(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	frames := selectFrames(spec.WorkflowID, branchCount, problemShape)
	weights := divergentScoreWeights(spec)
	base := spec.ArtifactRoot

	jobs := []map[string]any{
		job(
			"frame_problem",
			"synthesis",
			"Frame problem",
			"problem_framer",
			divergentPrimaryLane(spec),
			base,
			"PROBLEM_BRIEF.md",
			"handoff",
			"problem_brief",
			"frame_problem",
			"Publish a concise problem brief — the open-ended question to ideate on, plus constraints, goals, non-goals, and decision criteria. Do not propose solutions.",
		),
	}

	branchIDs := make([]string, 0, len(frames))
	for idx, frame := range frames {
		branchID := fmt.Sprintf("branch_%d_%s", idx+1, frame.ID)
		branchIDs = append(branchIDs, branchID)
		branch := job(
			branchID,
			"build",
			fmt.Sprintf("Diverge under frame: %s", frame.ID),
			"diverger",
			divergentBranchLane(spec, idx),
			base+"/branches/"+branchID,
			"IDEAS.md",
			"handoff",
			branchID,
			"diverge",
			divergentBranchObjective(frame, ideasPerBranch),
		)
		branch["parallel_group"] = "diverge"
		branch["fresh_session_required"] = true
		jobs = append(jobs, branch)
	}

	jobs = append(jobs, job(
		"converge",
		"synthesis",
		"Converge: score, cluster, trap-detect",
		"convergence_critic",
		convergenceLane,
		base,
		"CONVERGENCE.md",
		"findings_ledger",
		"convergence_ledger",
		"converge",
		divergentConvergeObjective(deepenCount, weights),
	))

	deepenIDs := make([]string, 0, deepenCount)
	for idx := 0; idx < deepenCount; idx++ {
		deepenID := fmt.Sprintf("deepen_%d", idx+1)
		deepenIDs = append(deepenIDs, deepenID)
		deepen := job(
			deepenID,
			"build",
			fmt.Sprintf("Deepen pick %d", idx+1),
			"deepener",
			divergentBranchLane(spec, idx),
			base+"/deepened/"+deepenID,
			"DEEPENED.md",
			"synthesis",
			deepenID,
			"deepen",
			fmt.Sprintf("Take the idea ranked #%d in the convergence ledger. Sketch how it would actually work in 4-8 sentences, name the load-bearing risk, name the first concrete step a builder would take, then generate 3-5 child ideas (variations, hybrids, unlocks).", idx+1),
		)
		deepen["parallel_group"] = "deepen"
		deepen["fresh_session_required"] = true
		jobs = append(jobs, deepen)
	}

	jobs = append(jobs, job(
		"final_synthesis",
		"synthesis",
		"Final synthesis",
		"final_synthesizer",
		convergenceLane,
		base,
		"IDEATION_SYNTHESIS.md",
		"synthesis",
		"ideation_synthesis",
		"final_synthesis",
		"Assemble the operator-facing result from the deepened picks and the convergence ledger: the shortlist (2-4 ideas) with why each is on it, the non-obvious-but-viable pick marked with a star, the trap list (each with its one-line reason), and one wildcard provocation that opens a new direction.",
	))

	edges := []map[string]any{}
	for _, branchID := range branchIDs {
		edges = append(edges, map[string]any{"from": "frame_problem", "to": branchID, "on": "completed"})
	}
	for _, branchID := range branchIDs {
		edges = append(edges, map[string]any{"from": branchID, "to": "converge", "on": "completed"})
	}
	for _, deepenID := range deepenIDs {
		edges = append(edges, map[string]any{"from": "converge", "to": deepenID, "on": "completed"})
	}
	for _, deepenID := range deepenIDs {
		edges = append(edges, map[string]any{"from": deepenID, "to": "final_synthesis", "on": "completed"})
	}

	return jobs, edges, nil, nil
}

// divergentBranchObjective injects the frame's vantage into the branch job's
// objective. For operation frames it prepends the transform-then-generate step:
// the branch first rewrites the problem statement, then ideates against the
// rewrite (RFC 0129). The generator-only instruction mirrors ADHD's divergent
// system prompt — no evaluation during divergence.
func divergentBranchObjective(frame Frame, ideasPerBranch int) string {
	var b strings.Builder
	if frame.Kind == "operation" && frame.Transform != "" {
		fmt.Fprintf(&b, "FIRST apply this transform to the problem statement, then ideate against the transformed problem (keep the original in view too): %s\n\n", frame.Transform)
	}
	fmt.Fprintf(&b, "Vantage frame (%s): %s\n\n", frame.ID, frame.Vantage)
	fmt.Fprintf(&b, "You are in DIVERGENT mode — a generator, not a critic. Produce %d short, distinct ideas under this frame. Do not evaluate, rank, or hedge. The first three obvious answers everyone would give are banned; push past them into the awkward middle. You cannot see the other branches.", ideasPerBranch)
	if frame.Kind == "operation" {
		b.WriteString(" Include the checkable intermediate your transform produced (the kernel / dual objective / invariant / restated problem) so the critic can confirm the transform did real work.")
	}
	return b.String()
}

func divergentConvergeObjective(deepenCount int, weights map[string]float64) string {
	return fmt.Sprintf(
		"You are the CRITIC. Read every branch's ideas (each branch ran a different frame, and branches may have run on different models). Score every idea on novelty, viability, and fit (0-10 each; weighted novelty %.2f / viability %.2f / fit %.2f for ranking). Cluster the ideas by their underlying angle, not surface keywords. Flag traps — ideas that look attractive but are a hidden cost, false economy, or won't scale — each with a one-line reason. Select the top %d picks by weighted score, excluding traps. CROSS-MODEL SIGNAL: explicitly note any idea independently surfaced by branches running on DIFFERENT model families — convergence across families is a confidence boost; record it. Publish the scored, clustered ledger.",
		weights["novelty"], weights["viability"], weights["fit"], deepenCount,
	)
}

func divergentBranchCount(spec Spec) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["branch_count"], 5))
	if !ok || value < 2 || value > 8 {
		return 0, genErr("branch_count must be an integer between 2 and 8 for divergent_ideation", "spec.options.branch_count")
	}
	return value, nil
}

func divergentDeepenCount(spec Spec, branchCount int) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["deepen_count"], 3))
	if !ok || value < 1 || value > 5 {
		return 0, genErr("deepen_count must be an integer between 1 and 5 for divergent_ideation", "spec.options.deepen_count")
	}
	if value > branchCount {
		return 0, genErr("deepen_count must not exceed branch_count", "spec.options.deepen_count")
	}
	return value, nil
}

func divergentIdeasPerBranch(spec Spec) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["ideas_per_branch"], 6))
	if !ok || value < 1 || value > 12 {
		return 0, genErr("ideas_per_branch must be an integer between 1 and 12 for divergent_ideation", "spec.options.ideas_per_branch")
	}
	return value, nil
}

func divergentProblemShape(spec Spec) (string, error) {
	value := fmt.Sprint(defaultAny(spec.Options["problem_shape"], "medium"))
	switch value {
	case "low", "medium", "high":
		return value, nil
	default:
		return "", genErr("problem_shape must be one of low, medium, high", "spec.options.problem_shape")
	}
}

func divergentScoreWeights(spec Spec) map[string]float64 {
	weights := map[string]float64{"novelty": 0.35, "viability": 0.40, "fit": 0.25}
	raw, ok := spec.Options["score_weights"].(map[string]any)
	if !ok {
		return weights
	}
	for _, key := range []string{"novelty", "viability", "fit"} {
		if value, ok := raw[key].(float64); ok {
			weights[key] = value
		}
	}
	return weights
}

// divergentLaneRing is the ordered set of lanes the diverge/deepen fan-outs
// round-robin across, so different models carry different frames when the lane
// set is multi-model (e.g. a custom claude/codex/agy set = Opus/GPT/Gemini).
func divergentLaneRing(spec Spec) []string {
	switch spec.LaneSet {
	case "local":
		return []string{"local"}
	case "single_agent":
		return []string{"agent"}
	case "author_reviewer":
		return []string{"author", "reviewer"}
	case "multi_review":
		ring := []string{"author"}
		for idx := 1; idx <= reviewerCount(spec); idx++ {
			ring = append(ring, fmt.Sprintf("reviewer_%d", idx))
		}
		return ring
	case "custom":
		ids := laneIDsFor(spec)
		sort.Strings(ids)
		if len(ids) == 0 {
			return []string{authorLane(spec)}
		}
		return ids
	default:
		return []string{authorLane(spec)}
	}
}

func divergentBranchLane(spec Spec, idx int) string {
	ring := divergentLaneRing(spec)
	if len(ring) == 0 {
		return authorLane(spec)
	}
	return ring[idx%len(ring)]
}

// divergentPrimaryLane is the lane the frame_problem brief and the coordinator
// bind to: the first lane in the ring. For the built-in lane sets this equals
// authorLane(spec) (author/agent/local), but for a custom multi-model lane set
// (e.g. claude/codex/agy) it resolves to a real declared lane instead of the
// literal "author", which would not exist.
func divergentPrimaryLane(spec Spec) string {
	ring := divergentLaneRing(spec)
	if len(ring) == 0 {
		return authorLane(spec)
	}
	return ring[0]
}

// divergentConvergenceLane resolves the lane the convergence critic and final
// synthesis run on. An explicit `convergence_lane_id` option binds the critic to
// a chosen lane (e.g. a different model family than the generators — the RFC 0129
// cross-family split); otherwise it defaults to the last lane in the ring (often
// a different model than branch 1) so the critic is not locked to the first
// generator's model by default.
func divergentConvergenceLane(spec Spec) (string, error) {
	if raw, ok := spec.Options["convergence_lane_id"]; ok {
		laneID, ok := raw.(string)
		if !ok || laneID == "" {
			return "", genErr("convergence_lane_id must be a non-empty string", "spec.options.convergence_lane_id")
		}
		if spec.LaneSet == "local" {
			return "local", nil
		}
		if _, known := stringSet(laneIDsFor(spec))[laneID]; !known {
			return "", genErr("convergence_lane_id references a lane not in the lane set", "spec.options.convergence_lane_id")
		}
		return laneID, nil
	}
	ring := divergentLaneRing(spec)
	if len(ring) > 1 {
		return ring[len(ring)-1], nil
	}
	return authorLane(spec), nil
}
