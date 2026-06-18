package workflowgenerate

import (
	"fmt"
	"strings"
)

func compileCollaborationShape(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	max, err := maxCycles(spec)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	switch spec.Shape {
	case "falsification_gate":
		return compileFalsificationGate(spec, max)
	case "cross_examination":
		return compileCrossExamination(spec, max)
	case "adjudicated_constraint_extraction":
		return compileAdjudicatedConstraintExtraction(spec, max)
	case "fog_of_war_review":
		return compileFogOfWarReview(spec, max)
	case "synaptic_prune":
		return compileSynapticPrune(spec, max)
	default:
		return nil, nil, nil, nil, genErr("unknown collaboration shape", "spec.shape")
	}
}

func compileFalsificationGate(spec Spec, maxCycles int) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	topic := collaborationTopic(spec)
	holder := job("holder", "build", "Hold leading proposal", "holder", authorLane(spec), base+"/dialogue/holder", "HOLDER.md", "handoff", "holder_handoff", "collaboration_holder", "Produce the leading proposal for "+topic+" as the published claim that falsifiers will challenge.")
	holder["phase_id"] = "dialogue"
	jobs := []map[string]any{holder}
	edges := []map[string]any{}
	previousID := "holder"
	falsifiers, err := falsifierCount(spec)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	firstDialogueTarget := ""
	for idx := 1; idx <= falsifiers; idx++ {
		id := fmt.Sprintf("falsifier_%d", idx)
		falsifier := job(id, "build", fmt.Sprintf("Falsifier %d", idx), "falsifier", collaborationReviewerLane(spec, idx), fmt.Sprintf("%s/dialogue/falsifier_%d", base, idx), "FALSIFIER.md", "handoff", id, "collaboration_falsifier", "Read the published holder proposal and write a falsifying challenge for "+topic+", including the strongest rebuttal or unanswered gap you can justify.")
		falsifier["phase_id"] = "dialogue"
		jobs = append(jobs, falsifier)
		edges = append(edges, map[string]any{"from": previousID, "to": id, "on": "completed"})
		if firstDialogueTarget == "" {
			firstDialogueTarget = id
		}
		previousID = id
	}
	if includeScribe(spec) {
		scribe := job("scribe_note", "build", "Scribe dialogue trail", "scribe", collaborationAdjudicatorLane(spec), base+"/dialogue/scribe", "PROGRESS_NOTE.md", "progress_note", "scribe_note", "collaboration_scribe", "Record only the decision trail visible in the dialogue trajectory for "+topic+".")
		scribe["phase_id"] = "dialogue"
		jobs = append(jobs, scribe)
		edges = append(edges, map[string]any{"from": previousID, "to": "scribe_note", "on": "completed"})
		previousID = "scribe_note"
	}
	jobs = append(jobs, collaborationAdjudicatorJob(spec, topic))
	edges = append(edges, map[string]any{"from": previousID, "to": "adjudicate", "on": "completed"})
	commit, final := collaborationCommitJobs(spec, "Commit falsification-cleared proposal", "Publish the proposal only after the adjudicator ledger records a clearing verdict.")
	jobs = append(jobs, commit, final)
	edges = append(edges,
		map[string]any{"from": "adjudicate", "to": "commit_proposal", "on": "completed"},
		map[string]any{"from": "commit_proposal", "to": "final_summary", "on": "completed"},
	)
	cycle := map[string]any{
		"from":            "adjudicate",
		"to":              firstDialogueTarget,
		"on_verdict":      "needs_revision",
		"max_iterations":  maxCycles,
		"allow_same_lane": true,
	}
	if spec.LaneSet == "local" {
		cycle["allow_same_model"] = true
	}
	cycles := []map[string]any{cycle}
	return jobs, edges, cycles, collaborationPhases(), nil
}

func compileCrossExamination(spec Spec, maxCycles int) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	topic := collaborationTopic(spec)
	draft := job("author_draft", "build", "Draft finding for cross-examination", "author", authorLane(spec), base+"/dialogue/author", "DRAFT.md", "handoff", "author_draft", "collaboration_author_draft", "Draft the finding or proposal for "+topic+" as the published claim that cross-examiners will challenge.")
	draft["phase_id"] = "dialogue"
	jobs := []map[string]any{draft}
	edges := []map[string]any{}
	previousID := "author_draft"
	examiners := crossExaminerCount(spec)
	firstDialogueTarget := ""
	for idx := 1; idx <= examiners; idx++ {
		id := fmt.Sprintf("cross_examiner_%d", idx)
		examiner := job(id, "build", fmt.Sprintf("Cross-examiner %d", idx), "cross_examiner", collaborationReviewerLane(spec, idx), fmt.Sprintf("%s/dialogue/cross_examiner_%d", base, idx), "CROSS_EXAM.md", "handoff", id, "collaboration_cross_examiner", "Read the published draft for "+topic+" and write one falsifying cross-examination challenge with the strongest rebuttal or unanswered gap you can justify.")
		examiner["phase_id"] = "dialogue"
		jobs = append(jobs, examiner)
		edges = append(edges, map[string]any{"from": previousID, "to": id, "on": "completed"})
		if firstDialogueTarget == "" {
			firstDialogueTarget = id
		}
		previousID = id
	}
	if includeScribe(spec) {
		scribe := job("scribe_note", "build", "Scribe cross-examination trail", "scribe", collaborationAdjudicatorLane(spec), base+"/dialogue/scribe", "PROGRESS_NOTE.md", "progress_note", "scribe_note", "collaboration_scribe", "Record only the decision trail visible in the dialogue trajectory for "+topic+".")
		scribe["phase_id"] = "dialogue"
		jobs = append(jobs, scribe)
		edges = append(edges, map[string]any{"from": previousID, "to": "scribe_note", "on": "completed"})
		previousID = "scribe_note"
	}
	jobs = append(jobs, collaborationAdjudicatorJob(spec, topic))
	edges = append(edges, map[string]any{"from": previousID, "to": "adjudicate", "on": "completed"})
	commit, final := collaborationCommitJobs(spec, "Publish cross-examined finding", "Publish the finding only after the cross-examination ledger records a clearing verdict.")
	jobs = append(jobs, commit, final)
	edges = append(edges,
		map[string]any{"from": "adjudicate", "to": "commit_proposal", "on": "completed"},
		map[string]any{"from": "commit_proposal", "to": "final_summary", "on": "completed"},
	)
	cycle := map[string]any{
		"from":            "adjudicate",
		"to":              firstDialogueTarget,
		"on_verdict":      "needs_revision",
		"max_iterations":  maxCycles,
		"allow_same_lane": true,
	}
	if spec.LaneSet == "local" {
		cycle["allow_same_model"] = true
	}
	cycles := []map[string]any{cycle}
	return jobs, edges, cycles, collaborationPhases(), nil
}

// adjudicatedConstraintPostures returns the cross-examiner posture set for the
// adjudicated_constraint_extraction shape. RFC 0098 §2 ships a default five-posture
// pack (product / implementation / privacy / eval / operations) and lets a workflow
// override it via options.review_postures. Postures here are free-form shape labels
// recorded in the ledger, not the lint posture vocabulary, so they are slugified
// rather than validated against allowedPostures.
func adjudicatedConstraintPostures(spec Spec) ([]string, error) {
	defaults := []string{"product", "implementation", "privacy", "eval", "operations"}
	raw, ok := spec.Options["review_postures"].([]any)
	if !ok || len(raw) == 0 {
		return defaults, nil
	}
	values := []string{}
	for idx, item := range raw {
		posture := strings.TrimSpace(fmt.Sprint(item))
		if !safePanelSlug(posture) {
			return nil, genErr("review_postures entries must match ^[a-z0-9._-]{1,64}$", fmt.Sprintf("spec.options.review_postures[%d]", idx))
		}
		values = append(values, posture)
	}
	return values, nil
}

// compileAdjudicatedConstraintExtraction emits the RFC 0098 eight-phase
// productive-refusal loop as a striatum.workflow.v1.1 phased graph. Each declared
// phase carries exactly one phase_synthesis job plus a peer, so the shared
// ValidatePhaseShapes rules (workflow validate AND run.prepare, GH #66) accept it.
//
// The load-bearing structure (RFC 0098 §1):
//   - adjudication's phase_synthesis publishes the collaboration_ledger gate; a
//     needs_revision verdict re-opens convener_synthesis as an RFC 0083 bounded
//     cycle (max_cycles) so the constraint table is carried forward.
//   - every artifact re-published inside the revision cycle (the convener synthesis
//     ledger, the adjudication ledger, the revision synthesis, the discharge
//     re-review) uses a ${cycle}-templated logical_name/path so republish does not
//     collide on the append-only artifacts table (RFC 0098 Acceptance #4 / GH #84).
//   - spec_publication consumes the latest cleared ledger; final_review reads the
//     binding constraints + spec and is a discharge typecheck (slice 3 gates it).
func compileAdjudicatedConstraintExtraction(spec Spec, maxCycles int) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	topic := collaborationTopic(spec)
	postures, err := adjudicatedConstraintPostures(spec)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	authorL := authorLane(spec)
	adjLane := collaborationAdjudicatorLane(spec)

	jobs := []map[string]any{}
	edges := []map[string]any{}
	phases := []map[string]any{}

	// --- survey ---------------------------------------------------------------
	surveyScan := job("survey_scan", "build", "Survey prior art and constraints", "convener", authorL, base+"/survey/scan", "SURVEY.md", "handoff", "survey_scan", "ace_survey", "Survey the prior art, evidence, and existing constraints relevant to "+topic+".")
	surveyScan["phase_id"] = "survey"
	surveySynth := job("survey_synthesis", "phase_synthesis", "Frame the survey", "convener", authorL, base+"/survey/synthesis", "SURVEY_SYNTHESIS.md", "synthesis", "survey_synthesis", "ace_survey_synthesis", "Frame the problem, goals, non-goals, and decision criteria for "+topic+" from the survey.")
	surveySynth["phase_id"] = "survey"
	jobs = append(jobs, surveyScan, surveySynth)
	edges = append(edges, map[string]any{"from": "survey_scan", "to": "survey_synthesis", "on": "completed"})
	phases = append(phases, map[string]any{"id": "survey", "name": "Survey", "description": "Frame the problem and existing constraints before synthesis", "synthesis_job_id": "survey_synthesis"})

	// --- convener_synthesis ---------------------------------------------------
	// Cycle-aware: each needs_revision re-opens this phase, so the candidate
	// synthesis logical_name/path are ${cycle}-templated to republish cleanly.
	convenerDraft := job("convener_draft", "build", "Draft candidate synthesis", "convener", authorL, base+"/convener_synthesis/draft", "CANDIDATE_${cycle}.md", "handoff", "convener_candidate_${cycle}", "ace_convener", "Draft the candidate synthesis for "+topic+" and stay live for cross-examination.")
	convenerDraft["phase_id"] = "convener_synthesis"
	convenerDraft["interrogable"] = true
	convenerSynth := job("convener_synthesis", "phase_synthesis", "Publish candidate synthesis", "convener", authorL, base+"/convener_synthesis/synthesis", "SYNTHESIS_${cycle}.md", "synthesis", "convener_synthesis_${cycle}", "ace_convener_synthesis", "Publish the candidate synthesis for "+topic+"; on a revision cycle, discharge each prior constraints[] row explicitly.")
	convenerSynth["phase_id"] = "convener_synthesis"
	jobs = append(jobs, convenerDraft, convenerSynth)
	edges = append(edges, map[string]any{"from": "convener_draft", "to": "convener_synthesis", "on": "completed"})
	phases = append(phases, map[string]any{"id": "convener_synthesis", "name": "Convener synthesis", "description": "Publish the candidate synthesis; re-opened by needs_revision to discharge constraints", "synthesis_job_id": "convener_synthesis"})

	// --- cross_exam -----------------------------------------------------------
	crossSynthInputs := []string{}
	for idx, posture := range postures {
		id := fmt.Sprintf("cross_examiner_%d", idx+1)
		examiner := job(id, "build", fmt.Sprintf("Cross-examiner (%s)", posture), "cross_examiner", collaborationReviewerLane(spec, idx+1), fmt.Sprintf("%s/cross_exam/%s", base, posture), "CROSS_EXAM.md", "handoff", id, "ace_cross_examiner", "Challenge the candidate synthesis for "+topic+" from the "+posture+" posture: record findings[] rows with severity, the affected invariant, and the closest acceptable answer.")
		examiner["phase_id"] = "cross_exam"
		examiner["parallel_group"] = "cross_exam"
		examiner["interrogation_targets"] = []map[string]any{
			{"workflow_job_id": "convener_draft", "required": true},
		}
		jobs = append(jobs, examiner)
		crossSynthInputs = append(crossSynthInputs, id)
	}
	crossSynth := job("cross_exam_synthesis", "phase_synthesis", "Roll up cross-examination", "convener", adjLane, base+"/cross_exam/synthesis", "CROSS_EXAM_SYNTHESIS_${cycle}.md", "findings_ledger", "cross_exam_findings_${cycle}", "ace_cross_exam_synthesis", "Roll up every cross-examiner posture into one findings ledger; preserve unanswered interrogations as evidence.")
	crossSynth["phase_id"] = "cross_exam"
	jobs = append(jobs, crossSynth)
	for _, id := range crossSynthInputs {
		edges = append(edges, map[string]any{"from": id, "to": "cross_exam_synthesis", "on": "completed"})
	}
	phases = append(phases, map[string]any{"id": "cross_exam", "name": "Cross-examination", "description": "Posture-specific adversarial challenge of the candidate synthesis", "synthesis_job_id": "cross_exam_synthesis"})

	// --- adjudication ---------------------------------------------------------
	// The adjudication ledger is the RFC 0093 gate, cycle-aware, and the source of
	// the constraints[] table carried into revision.
	adjudicatePeer := job("adjudication_intake", "build", "Stage adjudication inputs", "adjudicator", adjLane, base+"/adjudication/intake", "INTAKE_${cycle}.md", "handoff", "adjudication_intake_${cycle}", "ace_adjudication_intake", "Assemble the candidate synthesis and cross-examination findings for adjudication of "+topic+".")
	adjudicatePeer["phase_id"] = "adjudication"
	adjudicate := job("adjudicate", "phase_synthesis", "Adjudicate and extract constraints", "adjudicator", adjLane, base+"/adjudication/adjudicator", "COLLABORATION_LEDGER_${cycle}.md", "collaboration_ledger", "collaboration_ledger_${cycle}", "ace_adjudicate", "Read only the curated trajectory for "+topic+"; publish the collaboration_ledger verdict and, on needs_revision, convert load-bearing challenges into a non-empty constraints[] table.")
	adjudicate["phase_id"] = "adjudication"
	adjudicate["fresh_session_required"] = true
	jobs = append(jobs, adjudicatePeer, adjudicate)
	edges = append(edges, map[string]any{"from": "adjudication_intake", "to": "adjudicate", "on": "completed"})
	phases = append(phases, map[string]any{"id": "adjudication", "name": "Adjudication", "description": "Productive refusal: convert load-bearing challenges into binding constraints", "synthesis_job_id": "adjudicate"})

	// --- revision_synthesis ---------------------------------------------------
	// Receives the prior cycle's constraints[] as first-class inputs and discharges
	// each one; cycle-aware logical names so republish is collision-free.
	revisionDraft := job("revision_draft", "build", "Discharge constraints", "revision_convener", authorL, base+"/revision_synthesis/draft", "REVISION_${cycle}.md", "handoff", "revision_draft_${cycle}", "ace_revision_convener", "Take the prior cycle's constraints[] as binding input and discharge each row explicitly (answer / fold-in / reject-with-rationale / accept-as-risk / defer-with-successor) for "+topic+".")
	revisionDraft["phase_id"] = "revision_synthesis"
	revisionSynth := job("revision_synthesis", "phase_synthesis", "Publish revised synthesis", "revision_convener", authorL, base+"/revision_synthesis/synthesis", "REVISION_SYNTHESIS_${cycle}.md", "synthesis", "revision_synthesis_${cycle}", "ace_revision_synthesis", "Publish the revised synthesis that discharges the adjudicated constraints[] for "+topic+".")
	revisionSynth["phase_id"] = "revision_synthesis"
	jobs = append(jobs, revisionDraft, revisionSynth)
	edges = append(edges, map[string]any{"from": "revision_draft", "to": "revision_synthesis", "on": "completed"})
	phases = append(phases, map[string]any{"id": "revision_synthesis", "name": "Revision synthesis", "description": "Republish the synthesis discharging each adjudicated constraint", "synthesis_job_id": "revision_synthesis"})

	// --- constraint_discharge_review ------------------------------------------
	// A re-review of the discharge; cycle-aware so each re-publish is distinct.
	dischargeReview := job("discharge_review", "review", "Review constraint discharge", "adjudicator", adjLane, base+"/constraint_discharge_review/review", "DISCHARGE_REVIEW_${cycle}.md", "finding", "discharge_review_${cycle}", "ace_discharge_review", "Verify the revised synthesis discharges each binding constraint for "+topic+"; flag any constraint still open.")
	dischargeReview["phase_id"] = "constraint_discharge_review"
	dischargeReview["fresh_session_required"] = true
	dischargeReview["write_scope"] = map[string]any{"mode": "review_only_artifact", "repo_write": false, "allowed_paths": []string{base + "/constraint_discharge_review/review/"}, "forbidden_paths": []string{".striatum/"}}
	dischargeSynth := job("discharge_review_synthesis", "phase_synthesis", "Confirm discharge", "adjudicator", adjLane, base+"/constraint_discharge_review/synthesis", "DISCHARGE_SYNTHESIS_${cycle}.md", "synthesis", "discharge_review_synthesis_${cycle}", "ace_discharge_review_synthesis", "Confirm the latest cleared constraint ledger before spec publication for "+topic+".")
	dischargeSynth["phase_id"] = "constraint_discharge_review"
	jobs = append(jobs, dischargeReview, dischargeSynth)
	edges = append(edges, map[string]any{"from": "discharge_review", "to": "discharge_review_synthesis", "on": "completed"})
	phases = append(phases, map[string]any{"id": "constraint_discharge_review", "name": "Constraint discharge review", "description": "Confirm each binding constraint cleared before publication", "synthesis_job_id": "discharge_review_synthesis"})

	// --- spec_publication -----------------------------------------------------
	specDraft := job("spec_draft", "build", "Author the spec", "spec_author", authorL, base+"/spec_publication/draft", "SPEC.md", "handoff", "spec_draft", "ace_spec_author", "Write the RFC/spec for "+topic+" from the latest cleared constraint ledger as binding input, not from the original proposal.")
	specDraft["phase_id"] = "spec_publication"
	specSynth := job("spec_publication", "phase_synthesis", "Publish the spec", "spec_author", authorL, base+"/spec_publication/synthesis", "SPEC_PUBLICATION.md", "synthesis", "spec_publication", "ace_spec_publication", "Publish the spec gated on the latest cleared collaboration ledger for "+topic+".")
	specSynth["phase_id"] = "spec_publication"
	jobs = append(jobs, specDraft, specSynth)
	edges = append(edges, map[string]any{"from": "spec_draft", "to": "spec_publication", "on": "completed"})
	phases = append(phases, map[string]any{"id": "spec_publication", "name": "Spec publication", "description": "Author the spec from the latest cleared constraint ledger", "synthesis_job_id": "spec_publication"})

	// --- final_review ---------------------------------------------------------
	// A discharge typecheck: emits a constraint_discharge table and fails closed on
	// any undischarged binding constraint (slice 3 gates it; the prompt describes it).
	finalCheck := job("final_discharge_check", "review", "Typecheck constraint discharge", "final_reviewer", adjLane, base+"/final_review/check", "CONSTRAINT_DISCHARGE.md", "finding", "constraint_discharge", "ace_final_review", "Emit a constraint_discharge table for "+topic+": for each binding constraint mark discharged / partial / missing / accepted_risk with evidence. This is a typecheck — do not re-run the forum.")
	finalCheck["phase_id"] = "final_review"
	finalCheck["fresh_session_required"] = true
	finalCheck["write_scope"] = map[string]any{"mode": "review_only_artifact", "repo_write": false, "allowed_paths": []string{base + "/final_review/check/"}, "forbidden_paths": []string{".striatum/"}}
	finalSynth := job("final_review_synthesis", "phase_synthesis", "Finalize the run", "final_reviewer", adjLane, base+"/final_review/synthesis", "FINAL_SUMMARY.md", "synthesis", "final_review_synthesis", "ace_final_review_synthesis", "Summarize the discharge typecheck result; the run fails closed on any undischarged binding constraint.")
	finalSynth["phase_id"] = "final_review"
	jobs = append(jobs, finalCheck, finalSynth)
	edges = append(edges, map[string]any{"from": "final_discharge_check", "to": "final_review_synthesis", "on": "completed"})
	phases = append(phases, map[string]any{"id": "final_review", "name": "Final review", "description": "Discharge typecheck; fails closed on undischarged binding constraints", "synthesis_job_id": "final_review_synthesis"})

	// --- cross-phase sequencing edges (synthesis -> next phase entry) ----------
	// Each edge originates at a phase's synthesis job and targets the immediate
	// next phase's non-synthesis entry, satisfying ValidatePhaseShapes.
	sequence := [][2]string{
		{"survey_synthesis", "convener_draft"},
		{"convener_synthesis", "cross_examiner_1"},
		{"cross_exam_synthesis", "adjudication_intake"},
		{"adjudicate", "revision_draft"},
		{"revision_synthesis", "discharge_review"},
		{"discharge_review_synthesis", "spec_draft"},
		{"spec_publication", "final_discharge_check"},
	}
	for _, pair := range sequence {
		edges = append(edges, map[string]any{"from": pair[0], "to": pair[1], "on": "completed"})
	}
	// convener_synthesis also fans out to the remaining cross-examiners so the whole
	// posture panel opens together (parallel_group cross_exam).
	for idx := range postures {
		if idx == 0 {
			continue
		}
		edges = append(edges, map[string]any{"from": "convener_synthesis", "to": fmt.Sprintf("cross_examiner_%d", idx+1), "on": "completed"})
	}

	// --- revision cycle (RFC 0083 bounded; absorbed by adjudication, RFC 0098 #77)
	cycle := map[string]any{
		"from":            "adjudicate",
		"to":              "convener_draft",
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

func collaborationAdjudicatorJob(spec Spec, topic string) map[string]any {
	// Build finding 1: the adjudicator's collaboration_ledger is cycle-scoped.
	// Each `needs_revision` revision cycle re-runs this job with a bumped attempt
	// and the daemon resolves the ${cycle} placeholder to a distinct
	// cycle_<attempt> segment, so the attempt-2 ledger publishes under a new
	// logical name + path instead of colliding with attempt-1's content-hash
	// guard (which would deadlock the revision cycle). See RFC 0093 design
	// synthesis §4.6 (cycle_<N> naming) and pkg/mutations/collaboration_ledger.go
	// for the runtime resolver.
	result := job("adjudicate", "phase_synthesis", "Adjudicate dialogue substance", "adjudicator", collaborationAdjudicatorLane(spec), spec.ArtifactRoot+"/dialogue/adjudicator", "COLLABORATION_LEDGER_${cycle}.md", "collaboration_ledger", "collaboration_ledger_${cycle}", "adjudicate_collaboration", "Read only the dialogue trajectory for "+topic+" and publish the collaboration ledger verdict (one of accept, accept_with_findings, needs_revision, reject — a clearing verdict is accept or accept_with_findings, never `clear`).")
	result["phase_id"] = "dialogue"
	result["fresh_session_required"] = true
	return result
}

func collaborationCommitJobs(spec Spec, title, objective string) (map[string]any, map[string]any) {
	commit := job("commit_proposal", "synthesis", title, "committer", authorLane(spec), spec.ArtifactRoot+"/commit/proposal", "PROPOSAL.md", "synthesis", "commit_proposal", "collaboration_commit", objective)
	commit["phase_id"] = "commit"
	final := job("final_summary", "phase_synthesis", "Finalize collaboration run", "adjudicator", collaborationAdjudicatorLane(spec), spec.ArtifactRoot+"/commit/final", "FINAL_SUMMARY.md", "synthesis", "final_summary", "collaboration_final_summary", "Summarize the cleared collaboration gate and downstream publication.")
	final["phase_id"] = "commit"
	return commit, final
}

func collaborationPhases() []map[string]any {
	return []map[string]any{
		{"id": "dialogue", "name": "Dialogue", "description": "Preserved-context challenge and adjudication", "synthesis_job_id": "adjudicate"},
		{"id": "commit", "name": "Commit", "description": "Downstream work gated by the collaboration ledger", "synthesis_job_id": "final_summary"},
	}
}

func collaborationTopic(spec Spec) string {
	if topic, ok := spec.Options["topic"].(string); ok && strings.TrimSpace(topic) != "" {
		return strings.TrimSpace(topic)
	}
	return "unspecified collaboration topic"
}

func falsifierCount(spec Spec) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["falsifier_count"], 2))
	if !ok || value < 1 {
		return 0, genErr("falsifier_count must be a positive integer", "spec.options.falsifier_count")
	}
	return value, nil
}

func crossExaminerCount(spec Spec) int {
	if spec.LaneSet == "multi_review" {
		count := reviewerCount(spec) - 1
		if count > 0 {
			return count
		}
	}
	return 1
}

func collaborationReviewerLane(spec Spec, idx int) string {
	if spec.LaneSet != "multi_review" {
		return reviewerLane(spec, 1)
	}
	maxReviewer := reviewerCount(spec) - 1
	if maxReviewer < 1 {
		maxReviewer = 1
	}
	if idx > maxReviewer {
		idx = maxReviewer
	}
	return reviewerLane(spec, idx)
}

func collaborationAdjudicatorLane(spec Spec) string {
	if spec.LaneSet == "multi_review" {
		return reviewerLane(spec, reviewerCount(spec))
	}
	return reviewerLane(spec, 1)
}

func includeScribe(spec Spec) bool {
	raw, ok := spec.Options["include_scribe"]
	if !ok {
		return false
	}
	value, ok := boolFrom(raw)
	return ok && value
}
