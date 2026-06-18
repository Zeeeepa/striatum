package workflowgenerate

import "fmt"

// compileFogOfWarReview compiles the RFC 0094 §3 fog_of_war_review shape: a
// phased striatum.workflow.v1.1 graph that distributes DISJOINT spec fragments to
// each reconstructor lane, forces peer interrogation to recover the hidden
// constraints, and WITHHOLDS the proposal until a judge lane (holding the full
// spec) scores reconstruction coverage through the substance gate.
//
// It is the mirror of divergent_ideation's enforced isolation: here the lanes are
// deliberately given PARTIAL context they must reconstruct THROUGH dialog (RFC
// 0094 §3 vs RFC 0087). The load-bearing new mechanism is RFC 0094 §2 work-packet
// TYPE sequencing — the `proposal`-typed job lives in a phase whose `gate`
// declares `withhold_packet_types: [proposal]` / `until_verdict_clears:
// coverage_gate`, compiled into an ordinary RFC 0045 cross-phase dependency the
// daemon already enforces (validatePhaseGateSequencing in pkg/workflowauthoring
// proves the withholding is structurally real; no new daemon method/route).
//
// Phases (each carries exactly one phase_synthesis job + ≥1 peer, so the shared
// ValidatePhaseShapes rules accept it):
//   - distribution: the coordinator partitions the spec into disjoint fragments
//     (one per reconstructor, fixed at prepare time); the judge holds the full
//     spec. The fragment_map synthesis closes the phase.
//   - reconstruction: each reconstructor holds one fragment, stays interrogable,
//     and interrogates its peers to recover the constraints it was NOT given;
//     reconstruction_rollup closes the phase.
//   - coverage: the judge (full-spec ground truth) scores reconstructed /
//     hallucinated / missed and publishes the collaboration_ledger verdict
//     (coverage_gate is the phase_synthesis). A lane that claims coverage it never
//     reconstructed scores missed/hallucinated → needs_revision.
//   - proposal: the proposal job (work-packet type `proposal`) is withheld until
//     coverage_gate clears; proposal_synthesis closes the run.
//
// The coverage→reconstruction revision cycle re-opens reconstruction on
// needs_revision, mirroring the falsification_gate / ACE bounded-cycle pattern.
func compileFogOfWarReview(spec Spec, maxCycles int) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	topic := collaborationTopic(spec)
	authorL := authorLane(spec)
	judgeLane := collaborationAdjudicatorLane(spec)

	reconstructors, err := fogReconstructorCount(spec)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	jobs := []map[string]any{}
	edges := []map[string]any{}
	phases := []map[string]any{}

	// --- distribution ---------------------------------------------------------
	distScan := job("fragment_scan", "build", "Partition spec into disjoint fragments", "coordinator", authorL, base+"/distribution/scan", "FRAGMENTS.md", "handoff", "fragment_scan", "fog_fragment_scan", "Partition the spec for "+topic+" into disjoint fragments — one per reconstructor lane. Each lane receives ONLY its fragment; the full spec is held only by the judge. Record the partition so each reconstructor's hidden constraints are well-defined.")
	distScan["phase_id"] = "distribution"
	distSynth := job("fragment_map", "phase_synthesis", "Publish the fragment map", "coordinator", authorL, base+"/distribution/synthesis", "FRAGMENT_MAP.md", "synthesis", "fragment_map", "fog_fragment_map", "Publish the fixed fragment-to-lane map for "+topic+". This synthesis frames which constraints each reconstructor must recover THROUGH peer interrogation, not from its own fragment.")
	distSynth["phase_id"] = "distribution"
	jobs = append(jobs, distScan, distSynth)
	edges = append(edges, map[string]any{"from": "fragment_scan", "to": "fragment_map", "on": "completed"})
	phases = append(phases, map[string]any{"id": "distribution", "name": "Fragment distribution", "description": "Partition the spec into disjoint fragments; the judge alone holds the full spec", "synthesis_job_id": "fragment_map"})

	// --- reconstruction -------------------------------------------------------
	// Each reconstructor stays interrogable so its PEERS can interrogate it to
	// recover the constraints carried in its fragment (RFC 0094 §3 peer
	// interrogation). The peer interrogation is the runtime round-robin the
	// coordinator drives through the packet commands (interrogation.open against a
	// still-live sibling, serialized per holder under round-robin, RFC 0094 §6);
	// `interrogable: true` is the static projection that lets a peer open the
	// window. The rollup fans the reconstructors in (it does not interrogate them —
	// that would be a redundant interrogation of its own already-completed
	// dependency, the ACE cross_examiner→convener_draft prior-phase pattern keeps
	// interrogation and direct dependency disjoint).
	reconstructorIDs := []string{}
	for idx := 1; idx <= reconstructors; idx++ {
		id := fmt.Sprintf("reconstructor_%d", idx)
		reconstructorIDs = append(reconstructorIDs, id)
		recon := job(id, "build", fmt.Sprintf("Reconstructor %d", idx), "reconstructor", collaborationReviewerLane(spec, idx), fmt.Sprintf("%s/reconstruction/%s", base, id), "RECONSTRUCTION.md", "handoff", id, "fog_reconstructor", "You hold ONE disjoint fragment of the spec for "+topic+". Interrogate your peers to recover the constraints you were NOT given, and record which constraints you reconstructed (with the peer turn that revealed each) vs which you could not. Do not invent constraints you cannot source from a peer answer. Stay live for interrogation.")
		recon["phase_id"] = "reconstruction"
		recon["parallel_group"] = "reconstruction"
		recon["interrogable"] = true
		jobs = append(jobs, recon)
	}
	rollup := job("reconstruction_rollup", "phase_synthesis", "Roll up reconstruction coverage", "coordinator", judgeLane, base+"/reconstruction/synthesis", "RECONSTRUCTION_ROLLUP.md", "findings_ledger", "reconstruction_rollup", "fog_reconstruction_rollup", "Roll up every reconstructor's recovered-constraint record for "+topic+" into one findings ledger. Preserve which constraints each lane reconstructed vs missed, and carry unanswered peer interrogations forward as evidence for the coverage judge.")
	rollup["phase_id"] = "reconstruction"
	jobs = append(jobs, rollup)
	for _, id := range reconstructorIDs {
		edges = append(edges, map[string]any{"from": id, "to": "reconstruction_rollup", "on": "completed"})
	}
	phases = append(phases, map[string]any{"id": "reconstruction", "name": "Reconstruction", "description": "Reconstructors interrogate peers to recover the constraints their fragment omitted", "synthesis_job_id": "reconstruction_rollup"})

	// --- coverage -------------------------------------------------------------
	// The judge holds full-spec ground truth and scores reconstructed /
	// hallucinated / missed. coverage_gate is the RFC 0093 substance gate; its
	// clearing verdict is what the proposal phase's type-sequencing gate awaits.
	coverageIntake := job("coverage_intake", "build", "Stage coverage inputs", "judge", judgeLane, base+"/coverage/intake", "COVERAGE_INTAKE.md", "handoff", "coverage_intake", "fog_coverage_intake", "Assemble the reconstruction rollup and the full spec for coverage adjudication of "+topic+". Do not add new constraints; stage the curated trajectory only.")
	coverageIntake["phase_id"] = "coverage"
	coverageGate := job("coverage_gate", "phase_synthesis", "Score reconstruction coverage", "judge", judgeLane, base+"/coverage/judge", "COLLABORATION_LEDGER_${cycle}.md", "collaboration_ledger", "fog_coverage_ledger_${cycle}", "fog_coverage_gate", "You alone hold the full spec for "+topic+". Read only the curated reconstruction trajectory and score each hidden constraint reconstructed / hallucinated / missed against your ground truth; publish the collaboration_ledger (shape fog_of_war_review) verdict. A lane that CLAIMED coverage it never reconstructed scores hallucinated/missed → needs_revision; the proposal stays withheld until this verdict clears.")
	coverageGate["phase_id"] = "coverage"
	coverageGate["fresh_session_required"] = true
	jobs = append(jobs, coverageIntake, coverageGate)
	edges = append(edges, map[string]any{"from": "coverage_intake", "to": "coverage_gate", "on": "completed"})
	phases = append(phases, map[string]any{"id": "coverage", "name": "Coverage gate", "description": "The full-spec judge scores reconstructed vs hallucinated vs missed and gates the proposal", "synthesis_job_id": "coverage_gate"})

	// --- proposal (type-sequenced; RFC 0094 §2) -------------------------------
	// The proposal job's work-packet TYPE is `proposal`; it is unreachable until
	// the coverage_gate verdict clears. The phase declares the type-sequencing
	// gate, and the proposal job declares the compiled cross-phase dependency edge
	// from coverage_gate (a verdict-emitting phase_synthesis) so the withholding
	// is a real RFC 0045 dependency, not a new daemon authority.
	proposal := job("proposal", "proposal", "Author the proposal", "proposer", authorL, base+"/proposal/draft", "PROPOSAL.md", "handoff", "proposal", "fog_proposal", "Author the proposal for "+topic+" only after the coverage gate cleared. Build on the reconstructed constraints the judge confirmed, not on any constraint a reconstructor hallucinated.")
	proposal["phase_id"] = "proposal"
	proposalSynth := job("proposal_synthesis", "phase_synthesis", "Finalize the fog-of-war run", "coordinator", judgeLane, base+"/proposal/synthesis", "FINAL_SUMMARY.md", "synthesis", "proposal_synthesis", "fog_proposal_synthesis", "Summarize the cleared coverage gate and the published proposal for "+topic+".")
	proposalSynth["phase_id"] = "proposal"
	jobs = append(jobs, proposal, proposalSynth)
	edges = append(edges,
		// The type-sequencing dependency: proposal is withheld behind the gate.
		map[string]any{"from": "coverage_gate", "to": "proposal", "on": "completed"},
		map[string]any{"from": "proposal", "to": "proposal_synthesis", "on": "completed"},
	)
	phases = append(phases, map[string]any{
		"id":               "proposal",
		"name":             "Proposal",
		"description":      "The proposal is withheld until the coverage gate clears (RFC 0094 §2 work-packet type sequencing)",
		"synthesis_job_id": "proposal_synthesis",
		"gate": map[string]any{
			"withhold_packet_types": []any{"proposal"},
			"until_verdict_clears":  "coverage_gate",
		},
	})

	// --- cross-phase sequencing edges (synthesis -> next phase entry) ----------
	edges = append(edges,
		map[string]any{"from": "fragment_map", "to": reconstructorIDs[0], "on": "completed"},
		map[string]any{"from": "reconstruction_rollup", "to": "coverage_intake", "on": "completed"},
	)
	// fragment_map fans out to the remaining reconstructors so the whole
	// reconstruction panel opens together (parallel_group reconstruction).
	for idx := 1; idx < len(reconstructorIDs); idx++ {
		edges = append(edges, map[string]any{"from": "fragment_map", "to": reconstructorIDs[idx], "on": "completed"})
	}

	// --- coverage -> reconstruction revision cycle (RFC 0083 bounded) ---------
	cycle := map[string]any{
		"from":            "coverage_gate",
		"to":              reconstructorIDs[0],
		"on_verdict":      "needs_revision",
		"max_iterations":  maxCycles,
		"allow_same_lane": true,
	}
	if spec.LaneSet == "local" {
		cycle["allow_same_model"] = true
	}
	cycles := []map[string]any{cycle}
	return jobs, edges, cycles, phases, nil
}

// compileSynapticPrune compiles the RFC 0094 §4 synaptic_prune shape: a forum
// closes, a post_dialog_hook (RFC 0094 §1) emits a prune packet to the
// coordinator BEFORE the participant lanes' preserved-context window is released,
// the coordinator fans out prune-nomination interrogations to each still-live
// participant, and a ≥2-vote claim is retired into a durable collaboration_ledger
// injected as a NEGATIVE PREAMBLE into future runs on the same topic.
//
// The load-bearing new mechanism is RFC 0094 §1 emit-before-teardown: without it
// the nomination fan-out races lane exit. The hook is declared at RUNTIME on the
// coordinator's `conversation.open` call (deliver_to a participant session,
// packet_type prune) — the generated forum job's commands/prompt instruct the
// coordinator lane to open the conversation WITH the post_dialog_hook, run the
// round-robin forum, then close. The graph below is the job/phase scaffold; the
// dialog + hook + interrogation calls live in the packet commands (D028: curated
// authored text only, the hook payload carries session ids + a transcript ref,
// never raw provider output).
//
// Phases:
//   - forum: the coordinator opens the conversation WITH a post_dialog_hook and
//     runs the round-robin discussion; forum_close closes it (the hook fires).
//   - nomination: each participant nominates one claim to retire + rationale via a
//     prune interrogation while still live; nomination_rollup gathers them.
//   - prune_tally: the adjudicator retires every ≥2-vote claim with coherent
//     rationale into the collaboration_ledger (shape synaptic_prune) as the
//     durable negative preamble. AC#5: a dead target is RECORDED, not hung on.
func compileSynapticPrune(spec Spec, maxCycles int) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	topic := collaborationTopic(spec)
	authorL := authorLane(spec)
	adjLane := collaborationAdjudicatorLane(spec)

	participants, err := pruneParticipantCount(spec)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	jobs := []map[string]any{}
	edges := []map[string]any{}
	phases := []map[string]any{}

	// --- forum ----------------------------------------------------------------
	forumOpen := job("forum_open", "build", "Open the forum with a post_dialog_hook", "coordinator", authorL, base+"/forum/open", "FORUM.md", "handoff", "forum_open", "prune_forum_open", "Open a round-robin conversation on "+topic+" via conversation.open, declaring a post_dialog_hook {deliver_to: <coordinator session>, packet_type: prune} so close emits the prune packet BEFORE the participants' preserved-context window releases (RFC 0094 §1). Run the discussion; record the transcript reference, not raw provider output.")
	forumOpen["phase_id"] = "forum"
	forumOpen["interrogable"] = true
	forumClose := job("forum_close", "phase_synthesis", "Close the forum (hook fires)", "coordinator", authorL, base+"/forum/close", "FORUM_CLOSE.md", "synthesis", "forum_close", "prune_forum_close", "Close the conversation for "+topic+". On close the post_dialog_hook emits exactly one prune packet to the coordinator carrying the participant session ids + the transcript ref, before any participant teardown. Publish the close summary referencing the curated dialogue trajectory.")
	forumClose["phase_id"] = "forum"
	jobs = append(jobs, forumOpen, forumClose)
	edges = append(edges, map[string]any{"from": "forum_open", "to": "forum_close", "on": "completed"})
	phases = append(phases, map[string]any{"id": "forum", "name": "Forum", "description": "Round-robin conversation opened with a post_dialog_hook so close emits the prune fan-out before teardown", "synthesis_job_id": "forum_close"})

	// --- nomination -----------------------------------------------------------
	// Each participant nominates one claim to retire while still live; the rollup
	// gathers the nominations (serialized per holder under round-robin, RFC 0094
	// §6).
	nominatorIDs := []string{}
	for idx := 1; idx <= participants; idx++ {
		id := fmt.Sprintf("nominator_%d", idx)
		nominatorIDs = append(nominatorIDs, id)
		nominator := job(id, "build", fmt.Sprintf("Prune nominator %d", idx), "pruner", collaborationReviewerLane(spec, idx), fmt.Sprintf("%s/nomination/%s", base, id), "NOMINATION.md", "handoff", id, "prune_nominator", "While still live in your preserved-context window, nominate exactly ONE claim from the "+topic+" forum to retire ('do not re-litigate'), with a coherent rationale. A claim is only retired if ≥2 nominators independently pick it.")
		nominator["phase_id"] = "nomination"
		nominator["parallel_group"] = "nomination"
		jobs = append(jobs, nominator)
	}
	nominationRollup := job("nomination_rollup", "phase_synthesis", "Gather prune nominations", "coordinator", adjLane, base+"/nomination/synthesis", "NOMINATION_ROLLUP.md", "findings_ledger", "nomination_rollup", "prune_nomination_rollup", "Gather every still-live participant's prune nomination for "+topic+". If a target participant already died, RECORD the dead target and continue (do not hang) — AC#5 clean refusal. Carry each nomination + rationale forward to the tally.")
	nominationRollup["phase_id"] = "nomination"
	jobs = append(jobs, nominationRollup)
	for _, id := range nominatorIDs {
		edges = append(edges, map[string]any{"from": id, "to": "nomination_rollup", "on": "completed"})
	}
	phases = append(phases, map[string]any{"id": "nomination", "name": "Nomination", "description": "Each still-live participant nominates one claim to retire via a prune interrogation", "synthesis_job_id": "nomination_rollup"})

	// --- prune_tally ----------------------------------------------------------
	tallyIntake := job("tally_intake", "build", "Stage tally inputs", "adjudicator", adjLane, base+"/prune_tally/intake", "TALLY_INTAKE.md", "handoff", "tally_intake", "prune_tally_intake", "Assemble the nominations for "+topic+" for the prune tally. Stage the curated trajectory only.")
	tallyIntake["phase_id"] = "prune_tally"
	tally := job("prune_tally", "phase_synthesis", "Tally and retire ≥2-vote claims", "adjudicator", adjLane, base+"/prune_tally/adjudicator", "COLLABORATION_LEDGER_${cycle}.md", "collaboration_ledger", "prune_ledger_${cycle}", "prune_tally", "Read only the curated nomination trajectory for "+topic+". Retire every claim nominated by ≥2 participants with coherent rationale as a nomination-kind entry in the collaboration_ledger (shape synaptic_prune); publish the verdict. The retired set is the durable NEGATIVE PREAMBLE ('do not re-litigate: …') injected into future runs on the same topic — provenance, not reputation.")
	tally["phase_id"] = "prune_tally"
	tally["fresh_session_required"] = true
	jobs = append(jobs, tallyIntake, tally)
	edges = append(edges, map[string]any{"from": "tally_intake", "to": "prune_tally", "on": "completed"})
	phases = append(phases, map[string]any{"id": "prune_tally", "name": "Prune tally", "description": "Retire ≥2-vote claims into the durable negative preamble ledger", "synthesis_job_id": "prune_tally"})

	// --- cross-phase sequencing edges -----------------------------------------
	edges = append(edges,
		map[string]any{"from": "forum_close", "to": nominatorIDs[0], "on": "completed"},
		map[string]any{"from": "nomination_rollup", "to": "tally_intake", "on": "completed"},
	)
	for idx := 1; idx < len(nominatorIDs); idx++ {
		edges = append(edges, map[string]any{"from": "forum_close", "to": nominatorIDs[idx], "on": "completed"})
	}

	// --- tally -> nomination revision cycle (RFC 0083 bounded) ----------------
	cycle := map[string]any{
		"from":            "prune_tally",
		"to":              nominatorIDs[0],
		"on_verdict":      "needs_revision",
		"max_iterations":  maxCycles,
		"allow_same_lane": true,
	}
	if spec.LaneSet == "local" {
		cycle["allow_same_model"] = true
	}
	cycles := []map[string]any{cycle}
	return jobs, edges, cycles, phases, nil
}

// fogReconstructorCount resolves how many disjoint-fragment reconstructor lanes
// the fog_of_war_review shape fans out. It mirrors crossExaminerCount: on a
// multi_review lane set it uses every reviewer lane (minus the judge), else 2 so
// peer interrogation has at least one peer to recover constraints from. An
// explicit reconstructor_count option overrides (≥2 — a single reconstructor has
// no peer to interrogate, defeating the shape).
func fogReconstructorCount(spec Spec) (int, error) {
	if raw, ok := spec.Options["reconstructor_count"]; ok {
		value, ok := intFrom(raw)
		if !ok || value < 2 {
			return 0, genErr("reconstructor_count must be an integer >= 2 (a single reconstructor has no peer to interrogate)", "spec.options.reconstructor_count")
		}
		return value, nil
	}
	if spec.LaneSet == "multi_review" {
		count := reviewerCount(spec) - 1
		if count >= 2 {
			return count, nil
		}
	}
	return 2, nil
}

// pruneParticipantCount resolves how many forum participants nominate a claim to
// retire. ≥2 is required so a ≥2-vote retirement is reachable. An explicit
// participant_count option overrides; the multi_review lane set uses its reviewer
// lanes, else the default 2.
func pruneParticipantCount(spec Spec) (int, error) {
	if raw, ok := spec.Options["participant_count"]; ok {
		value, ok := intFrom(raw)
		if !ok || value < 2 {
			return 0, genErr("participant_count must be an integer >= 2 (a >=2-vote retirement needs at least two nominators)", "spec.options.participant_count")
		}
		return value, nil
	}
	if spec.LaneSet == "multi_review" {
		count := reviewerCount(spec) - 1
		if count >= 2 {
			return count, nil
		}
	}
	return 2, nil
}
