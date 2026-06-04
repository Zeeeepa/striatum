package workflowgenerate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/workflowauthoring"
	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

const (
	GeneratorSchemaVersion   = "striatum.workflow_generator.v1"
	PlanSchemaVersion        = "striatum.workflow_plan.v1"
	WorkflowSchemaVersion    = "striatum.workflow.v1"
	WorkflowSchemaVersionV11 = "striatum.workflow.v1.1"
)

var (
	shapes = set(
		"minimal", "review", "code_change", "human_checkpoint",
		"evidence_backed", "implementation_panel", "multi_review_synthesis",
		"multi_phase", "custom", "conversation",
		"falsification_gate", "cross_examination",
		"adjudicated_constraint_extraction",
	)
	laneSets      = set("local", "single_agent", "author_reviewer", "multi_review", "custom")
	laneModifiers = set("supervised", "worktree_isolated", "constrained", "harness_profiled")
	optionKeys    = set(
		"review_postures", "max_revision_cycles", "include_support_ledger",
		"constraints", "required_enforcement", "harness_profiles",
		"reviewer_count", "role_pack", "role_packs", "adversary_pack",
		"adversary_packs", "proposal_count", "score_dimensions",
		"custom_job_artifacts", "supervision_compatible", "phases",
		"topic", "turns", "max_dialog_rounds", "falsifier_count", "include_scribe",
	)
	blockKinds = set(
		"draft", "review", "synthesis", "implementation", "test",
		"human_checkpoint", "support_ledger", "evidence_audit", "final_review",
		"conversation",
	)
	allowedPostures = set(
		"neutral", "devils_advocate", "security", "threat_model",
		"latency_performance", "ergonomics_dx", "accessibility",
		"compliance_license", "supply_chain",
	)
	constraintValues = map[string]map[string]struct{}{
		"network":             set("allowed", "disabled", "loopback_only"),
		"transcript_capture":  set("allowed", "disabled"),
		"repository_scope":    set("full", "write_scope_only"),
		"filesystem_writes":   set("allowed", "write_scope_only", "disabled"),
		"credential_exposure": set("allowed", "redacted", "disabled"),
	}
	enforcementLevels = set("not_enforced", "advisory", "best_effort", "enforced")
)

type Error struct {
	Message   string
	FieldPath string
	Hint      string
	Ref       string
}

func (e *Error) Error() string {
	return e.Message
}

type Spec struct {
	SchemaVersion string
	Shape         string
	LaneSet       string
	WorkflowID    string
	Name          string
	WorkflowVer   string
	Branch        map[string]any
	ScaffoldRoot  string
	ArtifactRoot  string
	Lanes         map[string]map[string]any
	Options       map[string]any
	LaneModifiers []string
	Plan          map[string]any
	ContextDocs   []any
	Parallelism   map[string]any
}

type Generated struct {
	Workflow   map[string]any
	Files      []map[string]any
	Metadata   map[string]any
	Warnings   []string
	Validation map[string]any
	Lint       map[string]any
}

func (g Generated) Map() map[string]any {
	return map[string]any{
		"workflow":   g.Workflow,
		"files":      g.Files,
		"metadata":   g.Metadata,
		"warnings":   g.Warnings,
		"validation": g.Validation,
		"lint":       g.Lint,
	}
}

func DefaultSpec(scaffoldRoot, artifactRoot, shape, laneSet string, lanes map[string]map[string]any, options map[string]any) (Spec, error) {
	safeSlug := path.Base(strings.TrimSuffix(scaffoldRoot, "/"))
	if safeSlug == "." || safeSlug == "/" || safeSlug == "" {
		safeSlug = "starter-workflow"
	}
	raw := map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            shape,
		"lane_set":         laneSet,
		"lane_modifiers":   []any{},
		"workflow_id":      safeSlug + "-starter",
		"name":             fmt.Sprintf("%s starter (%s)", safeSlug, strings.ReplaceAll(shape, "_", "-")),
		"workflow_version": time.Now().UTC().Format("2006-01-02"),
		"branch": map[string]any{
			"mode":           "confirm",
			"suggested_name": "striatum/" + safeSlug,
			"allow_dirty":    false,
		},
		"scaffold_root": scaffoldRoot,
		"artifact_root": artifactRoot,
		"lanes":         lanes,
		"options":       options,
		"context_docs":  []any{},
	}
	return SpecFromMap(raw)
}

func SpecFromMap(raw map[string]any) (Spec, error) {
	allowed := set("schema_version", "shape", "lane_set", "workflow_id", "name", "workflow_version", "branch", "scaffold_root", "artifact_root", "lanes", "options", "lane_modifiers", "plan", "context_docs", "parallelism")
	for key := range raw {
		if _, ok := allowed[key]; !ok {
			return Spec{}, genErr("unknown generator spec field: "+key, "spec."+key)
		}
	}
	schema, err := requiredString(raw, "schema_version", "spec")
	if err != nil {
		return Spec{}, err
	}
	if schema != GeneratorSchemaVersion {
		return Spec{}, genErr("unsupported generator schema_version", "spec.schema_version")
	}
	shape, err := choice(raw, "shape", shapes, "spec")
	if err != nil {
		// #111: when the requested shape is a catalog template that is
		// example-only (advertised for discovery but not generated), point the
		// operator at its example fixture instead of the generic "must be one of"
		// list — the catalog and the generator must otherwise agree (reconcile test).
		if rawShape, _ := raw["shape"].(string); rawShape != "" {
			if hint := exampleOnlyShapeHint(rawShape); hint != "" {
				return Spec{}, genErr(hint, "spec.shape")
			}
		}
		return Spec{}, err
	}
	laneSet, err := choice(raw, "lane_set", laneSets, "spec")
	if err != nil {
		return Spec{}, err
	}
	workflowID, err := requiredString(raw, "workflow_id", "spec")
	if err != nil {
		return Spec{}, err
	}
	name, err := requiredString(raw, "name", "spec")
	if err != nil {
		return Spec{}, err
	}
	version, err := requiredString(raw, "workflow_version", "spec")
	if err != nil {
		return Spec{}, err
	}
	branch, err := object(raw["branch"], "spec.branch")
	if err != nil {
		return Spec{}, err
	}
	scaffold, err := SafeRelativePath(mustString(raw["scaffold_root"]), "spec.scaffold_root")
	if err != nil {
		return Spec{}, err
	}
	artifact, err := SafeRelativePath(mustString(raw["artifact_root"]), "spec.artifact_root")
	if err != nil {
		return Spec{}, err
	}
	lanes, err := lanesFrom(raw["lanes"], laneSet)
	if err != nil {
		return Spec{}, err
	}
	options, err := object(defaultAny(raw["options"], map[string]any{}), "spec.options")
	if err != nil {
		return Spec{}, err
	}
	for key := range options {
		if _, ok := optionKeys[key]; !ok {
			return Spec{}, genErr("unknown generator option: "+key, "spec.options."+key)
		}
	}
	modifiers, err := stringList(defaultAny(raw["lane_modifiers"], []any{}), "spec.lane_modifiers")
	if err != nil {
		return Spec{}, err
	}
	for idx, modifier := range modifiers {
		if _, ok := laneModifiers[modifier]; !ok {
			return Spec{}, genErr("unknown lane modifier", fmt.Sprintf("spec.lane_modifiers[%d]", idx))
		}
	}
	contextDocs := []any{}
	if value, ok := raw["context_docs"]; ok {
		list, ok := value.([]any)
		if !ok {
			return Spec{}, genErr("value must be a list of objects", "spec.context_docs")
		}
		for idx, item := range list {
			if _, ok := item.(map[string]any); !ok {
				return Spec{}, genErr("value must be a list of objects", fmt.Sprintf("spec.context_docs[%d]", idx))
			}
		}
		contextDocs = append(contextDocs, list...)
	}
	var parallelism map[string]any
	if value, ok := raw["parallelism"]; ok {
		parallelism, err = object(value, "spec.parallelism")
		if err != nil {
			return Spec{}, err
		}
	}
	var plan map[string]any
	if value, ok := raw["plan"]; ok {
		plan, err = object(value, "spec.plan")
		if err != nil {
			return Spec{}, err
		}
	}
	if shape == "custom" && plan == nil {
		return Spec{}, genErr("custom shape requires a plan", "spec.plan")
	}
	if shape != "custom" && plan != nil {
		return Spec{}, genErr("plan is valid only for custom shape", "spec.plan")
	}
	return Spec{
		SchemaVersion: schema, Shape: shape, LaneSet: laneSet, WorkflowID: workflowID,
		Name: name, WorkflowVer: version, Branch: branch, ScaffoldRoot: scaffold,
		ArtifactRoot: artifact, Lanes: lanes, Options: options, LaneModifiers: modifiers,
		Plan: plan, ContextDocs: contextDocs, Parallelism: parallelism,
	}, nil
}

func Generate(spec Spec) (Generated, error) {
	warnings := []string{}
	if err := validateModifierMatrix(spec, &warnings); err != nil {
		return Generated{}, err
	}
	lanes, err := compileLanes(spec)
	if err != nil {
		return Generated{}, err
	}
	var jobs []map[string]any
	var edges []map[string]any
	var cycles []map[string]any
	var phases []map[string]any
	if spec.Shape == "custom" {
		jobs, edges, cycles, err = compileCustom(spec, lanes)
	} else {
		jobs, edges, cycles, phases, err = compileShape(spec)
	}
	if err != nil {
		return Generated{}, err
	}
	if jobs == nil {
		jobs = []map[string]any{}
	}
	if edges == nil {
		edges = []map[string]any{}
	}
	if cycles == nil {
		cycles = []map[string]any{}
	}
	if phases == nil {
		phases = []map[string]any{}
	}
	if spec.Shape == "implementation_panel" {
		warnings = append(warnings, "implementation_panel generates a high-artifact workflow; review proposal_count, score_dimensions, and lane costs before running.")
	}
	roleIDs := map[string]struct{}{}
	for _, job := range jobs {
		roleIDs[fmt.Sprint(job["role_id"])] = struct{}{}
	}
	roles := rolesFor(sortedKeys(roleIDs))
	parallelism := spec.Parallelism
	if parallelism == nil {
		parallelism = defaultParallelism(spec)
	}
	schemaVersion := WorkflowSchemaVersion
	if spec.Shape == "multi_phase" || isCollaborationShape(spec.Shape) {
		schemaVersion = WorkflowSchemaVersionV11
	}
	workflow := map[string]any{
		"schema_version":   schemaVersion,
		"workflow_id":      spec.WorkflowID,
		"workflow_version": spec.WorkflowVer,
		"name":             spec.Name,
		"branch":           cloneMap(spec.Branch),
		"coordinator":      coordinator(spec, lanes),
		"lanes":            lanes,
		"roles":            roles,
		"context_docs":     append([]any{}, spec.ContextDocs...),
		"parallelism":      parallelism,
		"jobs":             jobs,
		"edges":            edges,
		"cycles":           cycles,
	}
	if spec.Shape == "multi_phase" || isCollaborationShape(spec.Shape) {
		workflow["phases"] = phases
	}
	// RFC 0093 / RFC 0064: a collaboration shape on the single-lane `local`
	// fixture set runs the adjudicator on the same lane as the holder/proposer it
	// adjudicates, so the same_model_adjudicator_pair lint (now CLI-refused)
	// would otherwise reject the generated starter workflow. A local fixture lane
	// is inherently same-model and is exactly the documented legitimate override
	// case, so record the inline acceptance — matching the cycle.allow_same_model
	// the local collaboration cycle already sets.
	if isCollaborationShape(spec.Shape) && spec.LaneSet == "local" {
		workflow["allow_same_model_review_pairing"] = true
	}
	if hasModifier(spec, "harness_profiled") {
		profiles, err := harnessProfiles(spec)
		if err != nil {
			return Generated{}, err
		}
		workflow["harness_profiles"] = profiles
	}
	if err := ValidateWorkflow(workflow); err != nil {
		return Generated{}, err
	}
	graph := graphData(jobs, edges, cycles)
	files, err := renderFiles(spec, workflow, roles)
	if err != nil {
		return Generated{}, err
	}
	metadata := map[string]any{
		"shape":             spec.Shape,
		"lane_set":          spec.LaneSet,
		"lane_modifiers":    append([]string(nil), spec.LaneModifiers...),
		"graph":             graph,
		"catalog_templates": []string{spec.Shape, spec.LaneSet},
		"scaffold_root":     spec.ScaffoldRoot,
		"workflow_path":     spec.ScaffoldRoot + "/workflow.json",
	}
	if spec.Shape == "implementation_panel" {
		rolePacks, err := panelRolePacks(spec)
		if err != nil {
			return Generated{}, err
		}
		adversaryPacks, err := panelAdversaryPacks(spec)
		if err != nil {
			return Generated{}, err
		}
		proposalCount, err := panelProposalCount(spec)
		if err != nil {
			return Generated{}, err
		}
		scoreDimensions, err := panelScoreDimensions(spec)
		if err != nil {
			return Generated{}, err
		}
		metadata["role_packs"] = rolePacks
		metadata["adversary_packs"] = adversaryPacks
		metadata["proposal_count"] = proposalCount
		metadata["score_dimensions"] = scoreDimensions
	}
	if isCollaborationShape(spec.Shape) {
		metadata["shape_family"] = "collaboration"
		metadata["collaboration_shape_pack"] = "substance_gate_v1"
		metadata["topic"] = collaborationTopic(spec)
	}
	return Generated{
		Workflow:   workflow,
		Files:      files,
		Metadata:   metadata,
		Warnings:   warnings,
		Validation: map[string]any{"ok": true, "workflow_id": spec.WorkflowID},
		Lint:       lintWorkflow(workflow),
	}, nil
}

func GenerateFromMap(raw map[string]any) (Generated, error) {
	spec, err := SpecFromMap(raw)
	if err != nil {
		return Generated{}, err
	}
	return Generate(spec)
}

func compileLanes(spec Spec) (map[string]any, error) {
	if spec.LaneSet == "local" {
		lanes := map[string]any{
			"local": map[string]any{
				"adapter":       "process",
				"display_model": "Local Fixture",
				"command":       []string{"sh", "-c", "cat >/dev/null"},
				"capabilities":  []string{"write", "review"},
			},
		}
		return applyLaneModifiers(spec, lanes, set("local"))
	}
	ids := laneIDsFor(spec)
	lanes := map[string]any{}
	for _, laneID := range ids {
		body, ok := spec.Lanes[laneID]
		if !ok {
			return nil, genErr(fmt.Sprintf("lane_set %q requires lane %q", spec.LaneSet, laneID), "spec.lanes."+laneID)
		}
		command, err := stringList(body["command"], "spec.lanes."+laneID+".command")
		if err != nil || len(command) == 0 {
			return nil, genErr("lane command must be a non-empty JSON string array", "spec.lanes."+laneID+".command")
		}
		display := fmt.Sprint(body["display_model"])
		if display == "" || display == "<nil>" {
			display = laneID
		}
		adapter := fmt.Sprint(body["adapter"])
		if adapter == "" || adapter == "<nil>" {
			adapter = "process"
		}
		caps := []string{"write", "review", "synthesis"}
		if rawCaps, ok := body["capabilities"]; ok {
			caps, err = stringList(rawCaps, "spec.lanes."+laneID+".capabilities")
			if err != nil {
				return nil, err
			}
		}
		lanes[laneID] = map[string]any{
			"adapter":       adapter,
			"display_model": display,
			"command":       command,
			"capabilities":  caps,
		}
	}
	repoWrite := map[string]struct{}{}
	for _, id := range ids {
		if !strings.Contains(id, "reviewer") {
			repoWrite[id] = struct{}{}
		}
	}
	return applyLaneModifiers(spec, lanes, repoWrite)
}

func laneIDsFor(spec Spec) []string {
	switch spec.LaneSet {
	case "single_agent":
		return []string{"agent"}
	case "author_reviewer":
		return []string{"author", "reviewer"}
	case "multi_review":
		result := []string{"author"}
		for idx := 1; idx <= reviewerCount(spec); idx++ {
			result = append(result, fmt.Sprintf("reviewer_%d", idx))
		}
		return result
	case "custom":
		keys := []string{}
		for key := range spec.Lanes {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys
	default:
		return []string{"local"}
	}
}

func compileShape(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	authorLane := authorLane(spec)
	reviewerLaneID := reviewerLane(spec, 1)
	switch spec.Shape {
	case "minimal":
		return []map[string]any{job("draft", "draft", "Draft starter artifact", "author", authorLane, base, "DRAFT.md", "handoff", "draft", "draft", "Produce the starter artifact for this workflow.")}, nil, nil, nil, nil
	case "review", "code_change":
		jobs := []map[string]any{
			job("draft", "draft", "Draft starter artifact", "author", authorLane, base, "DRAFT.md", "handoff", "draft", "draft", "Produce the starter artifact for this workflow."),
			reviewJob("review", reviewerLaneID, base+"/review/REVIEW.md", firstPosture(spec)),
			job("apply", "synthesis", "Apply the accepted review", "author", authorLane, base, "SUMMARY.md", "synthesis", "summary", "apply", "Apply the accepted review findings."),
		}
		edges := []map[string]any{{"from": "draft", "to": "review", "on": "completed"}, {"from": "review", "to": "apply", "on": "completed"}}
		cycles := []map[string]any{}
		if spec.Shape == "code_change" {
			max, err := maxCycles(spec)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			cycles = append(cycles, map[string]any{"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": max})
		}
		return jobs, edges, cycles, nil, nil
	case "human_checkpoint":
		jobs := []map[string]any{
			job("analysis", "draft", "Analyze the requested decision", "author", authorLane, base, "ANALYSIS.md", "handoff", "analysis", "draft", ""),
			job("checkpoint", "human_checkpoint", "Open a human checkpoint", "reviewer", reviewerLaneID, base, "CHECKPOINT.md", "handoff", "checkpoint", "review", ""),
			job("apply", "synthesis", "Apply the owner decision", "author", authorLane, base, "SUMMARY.md", "synthesis", "summary", "apply", ""),
		}
		return jobs, []map[string]any{{"from": "analysis", "to": "checkpoint", "on": "completed"}, {"from": "checkpoint", "to": "apply", "on": "completed"}}, nil, nil, nil
	case "evidence_backed":
		jobs := []map[string]any{
			job("draft", "draft", "Draft evidence-backed artifact", "author", authorLane, base, "DRAFT.md", "handoff", "draft", "draft", ""),
			job("support_ledger", "build", "Map claims to evidence", "author", authorLane, base+"/support", "SUPPORT_LEDGER.md", "support_ledger", "support_ledger", "support_ledger", ""),
			reviewJob("evidence_audit", reviewerLaneID, base+"/audit/EVIDENCE_AUDIT.md", "devils_advocate"),
			reviewJob("final_review", reviewerLaneID, base+"/final/FINAL_REVIEW.md", firstPosture(spec)),
		}
		return jobs, []map[string]any{{"from": "draft", "to": "support_ledger", "on": "completed"}, {"from": "support_ledger", "to": "evidence_audit", "on": "completed"}, {"from": "evidence_audit", "to": "final_review", "on": "completed"}}, nil, nil, nil
	case "multi_review_synthesis":
		count := reviewerCount(spec)
		postures, err := postures(spec, count)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		jobs := []map[string]any{}
		edges := []map[string]any{}
		for idx := 1; idx <= count; idx++ {
			id := fmt.Sprintf("review_%d", idx)
			jobs = append(jobs, reviewJob(id, reviewerLane(spec, idx), fmt.Sprintf("%s/review_%d/REVIEW.md", base, idx), postures[idx-1]))
			edges = append(edges, map[string]any{"from": id, "to": "synthesis", "on": "completed"})
		}
		jobs = append(jobs,
			job("synthesis", "synthesis", "Synthesize independent reviews", "author", authorLane, base, "SYNTHESIS.md", "synthesis", "synthesis", "apply", ""),
			reviewJob("final_review", reviewerLane(spec, 1), base+"/final/FINAL_REVIEW.md", "neutral"),
		)
		edges = append(edges, map[string]any{"from": "synthesis", "to": "final_review", "on": "completed"})
		return jobs, edges, nil, nil, nil
	case "conversation":
		turns := 3
		if raw, ok := spec.Options["turns"]; ok {
			if v, ok := raw.(float64); ok {
				turns = int(v)
			} else if v, ok := raw.(int); ok {
				turns = v
			}
		}
		topic := "unspecified topic"
		if raw, ok := spec.Options["topic"]; ok {
			if v, ok := raw.(string); ok {
				topic = v
			}
		}
		jobs := []map[string]any{}
		edges := []map[string]any{}
		for i := 1; i <= turns; i++ {
			id := fmt.Sprintf("turn_%d", i)
			lane := "author"
			laneID := authorLane
			if i%2 == 0 {
				lane = "reviewer"
				laneID = reviewerLane(spec, 1)
			}
			label := fmt.Sprintf("Turn %d (%s)", i, topic)
			jobs = append(jobs, job(id, "conversation", label, lane, laneID, base, fmt.Sprintf("turn_%d.md", i), "handoff", id, "draft", ""))
			if i > 1 {
				edges = append(edges, map[string]any{"from": fmt.Sprintf("turn_%d", i-1), "to": id, "on": "completed"})
			}
		}
		return jobs, edges, nil, nil, nil
	case "falsification_gate", "cross_examination", "adjudicated_constraint_extraction":
		return compileCollaborationShape(spec)
	case "implementation_panel":
		jobs, edges, cycles, err := compileImplementationPanel(spec)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		return jobs, edges, cycles, nil, nil
	case "multi_phase":
		return compileMultiPhase(spec)
	default:
		return nil, nil, nil, nil, genErr("unknown workflow shape", "spec.shape")
	}
}

func compileImplementationPanel(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, error) {
	if _, err := panelRolePacks(spec); err != nil {
		return nil, nil, nil, err
	}
	proposalCount, err := panelProposalCount(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	scoreDimensions, err := panelScoreDimensions(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	base := spec.ArtifactRoot
	proposalLane := panelProposalLane(spec)
	reviewLane := panelReviewLane(spec, 1)
	dissentLane := panelReviewLane(spec, 2)
	arbitrationLane := panelArbitrationLane(spec)
	decisionLane := panelDecisionLane(spec)
	scorePosture := panelScorePosture(scoreDimensions)
	jobs := []map[string]any{
		job(
			"frame_problem",
			"synthesis",
			"Frame problem",
			"problem_framer",
			proposalLane,
			base,
			"PROBLEM_BRIEF.md",
			"handoff",
			"problem_brief",
			"frame_problem",
			"Publish a concise implementation problem brief with constraints, goals, non-goals, and decision criteria.",
		),
	}
	proposalIDs := []string{}
	scoreIDs := []string{}
	for idx := 0; idx < proposalCount; idx++ {
		suffix := string(rune('a' + idx))
		label := strings.ToUpper(suffix)
		proposalID := "propose_option_" + suffix
		scoreID := "score_option_" + suffix
		proposalIDs = append(proposalIDs, proposalID)
		scoreIDs = append(scoreIDs, scoreID)
		proposal := job(
			proposalID,
			"synthesis",
			"Propose option "+label,
			"proposer_"+suffix,
			proposalLane,
			base+"/proposals/option_"+suffix,
			"PROPOSAL_"+label+".md",
			"handoff",
			"proposal_"+suffix,
			"propose_option",
			"Develop implementation option "+label+" independently from the problem brief.",
		)
		proposal["parallel_group"] = "proposals"
		proposal["fresh_session_required"] = true
		jobs = append(jobs, proposal)
		score := reviewJobForRole(
			scoreID,
			"Score option "+label,
			"scorekeeper",
			reviewLane,
			base+"/scorecards/SCORECARD_"+label+".md",
			scorePosture,
			"scorecard_"+suffix,
			"score_option",
			"Score proposal "+label+" against the implementation-panel dimensions: "+strings.Join(scoreDimensions, ", ")+".",
		)
		score["parallel_group"] = "scorecards"
		jobs = append(jobs, score)
	}
	jobs = append(jobs,
		job(
			"compile_tradeoffs",
			"synthesis",
			"Compile tradeoffs",
			"tradeoff_ledger",
			proposalLane,
			base,
			"TRADEOFF_LEDGER.md",
			"findings_ledger",
			"tradeoff_ledger",
			"compile_tradeoffs",
			"Compile proposal and scorecard evidence into a normalized tradeoff ledger.",
		),
		job(
			"arbitrate",
			"synthesis",
			"Arbitrate preferred option",
			"arbitrator",
			arbitrationLane,
			base,
			"ARBITRATOR_SYNTHESIS.md",
			"synthesis",
			"arbitrator_synthesis",
			"arbitrate",
			"Select or compose the preferred implementation path from the tradeoff ledger and evidence.",
		),
		reviewJobForRole(
			"review_dissent",
			"Review dissent",
			"dissent_reviewer",
			dissentLane,
			base+"/DISSENT_REVIEW.md",
			"devils_advocate",
			"dissent_review",
			"review_dissent",
			"Try to falsify the arbitration before the final decision is recorded.",
		),
		job(
			"record_decision",
			"synthesis",
			"Record decision",
			"principal_decider",
			decisionLane,
			base,
			"DECISION.md",
			"decision",
			"decision",
			"record_decision",
			"Publish the final implementation decision and required follow-up work.",
		),
	)
	edges := []map[string]any{}
	for _, proposalID := range proposalIDs {
		edges = append(edges, map[string]any{"from": "frame_problem", "to": proposalID, "on": "completed"})
	}
	for idx, proposalID := range proposalIDs {
		edges = append(edges, map[string]any{"from": proposalID, "to": scoreIDs[idx], "on": "completed"})
	}
	for _, scoreID := range scoreIDs {
		edges = append(edges, map[string]any{"from": scoreID, "to": "compile_tradeoffs", "on": "completed"})
	}
	edges = append(edges,
		map[string]any{"from": "compile_tradeoffs", "to": "arbitrate", "on": "completed"},
		map[string]any{"from": "arbitrate", "to": "review_dissent", "on": "completed"},
		map[string]any{"from": "review_dissent", "to": "record_decision", "on": "completed"},
	)
	cycles := []map[string]any{
		{
			"from":            "review_dissent",
			"to":              "arbitrate",
			"on_verdict":      "needs_revision",
			"max_iterations":  1,
			"allow_same_lane": true,
		},
	}
	return jobs, edges, cycles, nil
}

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

func compileMultiPhase(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, []map[string]any, error) {
	rawPhases, err := objectList(defaultAny(spec.Options["phases"], []any{}), "spec.options.phases")
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if len(rawPhases) == 0 {
		return nil, nil, nil, nil, genErr("multi_phase requires at least one phase", "spec.options.phases")
	}
	jobs := []map[string]any{}
	edges := []map[string]any{}
	cycles := []map[string]any{}
	phases := []map[string]any{}
	var previousSynthesisID string
	seenPhaseIDs := map[string]struct{}{}
	for phaseIndex, rawPhase := range rawPhases {
		phasePath := fmt.Sprintf("spec.options.phases[%d]", phaseIndex)
		phaseID, err := requiredString(rawPhase, "id", phasePath)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if _, ok := seenPhaseIDs[phaseID]; ok {
			return nil, nil, nil, nil, genErr("duplicate phase id", phasePath+".id")
		}
		seenPhaseIDs[phaseID] = struct{}{}
		phaseName, err := requiredString(rawPhase, "name", phasePath)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		phase := map[string]any{"id": phaseID, "name": phaseName}
		if value, ok := rawPhase["description"]; ok {
			text, ok := value.(string)
			if !ok {
				return nil, nil, nil, nil, genErr("phase description must be a string", phasePath+".description")
			}
			phase["description"] = text
		}
		if value, ok := rawPhase["color"]; ok {
			text, ok := value.(string)
			if !ok {
				return nil, nil, nil, nil, genErr("phase color must be a string", phasePath+".color")
			}
			phase["color"] = text
		}
		tracks, err := objectList(defaultAny(rawPhase["tracks"], []any{}), phasePath+".tracks")
		if err != nil {
			return nil, nil, nil, nil, err
		}
		if len(tracks) == 0 {
			return nil, nil, nil, nil, genErr("phase requires at least one track", phasePath+".tracks")
		}
		phaseJobs := []map[string]any{}
		phaseEdges := []map[string]any{}
		phaseCycles := []map[string]any{}
		entryIDs := []string{}
		terminalIDs := []string{}
		seenTrackIDs := map[string]struct{}{}
		trackShapes := set("minimal", "review", "code_change", "human_checkpoint", "evidence_backed", "multi_review_synthesis")
		for trackIndex, track := range tracks {
			trackPath := fmt.Sprintf("%s.tracks[%d]", phasePath, trackIndex)
			trackID, err := requiredString(track, "id", trackPath)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			if _, ok := seenTrackIDs[trackID]; ok {
				return nil, nil, nil, nil, genErr("duplicate phase track id", trackPath+".id")
			}
			seenTrackIDs[trackID] = struct{}{}
			trackShape, err := choice(track, "shape", trackShapes, trackPath)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			trackSpec, err := trackSpec(spec, phaseID, trackID, trackShape, track)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			trackJobs, trackEdges, trackCycles, _, err := compileShape(trackSpec)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			prefix := phaseID + "__" + trackID + "__"
			idMap := map[string]string{}
			for _, trackJob := range trackJobs {
				jobID := fmt.Sprint(trackJob["id"])
				idMap[jobID] = prefix + jobID
			}
			trackEntryIDs := entryJobIDs(trackJobs, trackEdges, idMap)
			parallelEntries := set(trackEntryIDs...)
			trackLaneID, laneOverride := track["lane_id"].(string)
			for _, trackJob := range trackJobs {
				remapped := cloneMap(trackJob)
				remappedID := idMap[fmt.Sprint(trackJob["id"])]
				remapped["id"] = remappedID
				remapped["phase_id"] = phaseID
				if _, ok := parallelEntries[remappedID]; ok {
					remapped["parallel_group"] = phaseID + ":" + trackID
				}
				if laneOverride && remapped["type"] != "review" {
					remapped["lane_id"] = trackLaneID
				}
				phaseJobs = append(phaseJobs, remapped)
			}
			phaseEdges = append(phaseEdges, remapEdges(trackEdges, idMap)...)
			phaseCycles = append(phaseCycles, remapCycles(trackCycles, idMap)...)
			entryIDs = append(entryIDs, trackEntryIDs...)
			terminalIDs = append(terminalIDs, terminalJobIDs(trackJobs, trackEdges, idMap)...)
		}
		synthesisID := phaseID + "__synthesis"
		synthesisLane, err := phaseSynthesisLane(spec, rawPhase)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		phase["synthesis_job_id"] = synthesisID
		phaseJobs = append(phaseJobs, phaseSynthesisJob(phaseID, phaseName, synthesisID, synthesisLane, spec.ArtifactRoot))
		for _, terminalID := range uniqueSortedStrings(terminalIDs) {
			phaseEdges = append(phaseEdges, map[string]any{"from": terminalID, "to": synthesisID, "on": "completed"})
		}
		if previousSynthesisID != "" {
			for _, entryID := range uniqueSortedStrings(entryIDs) {
				edges = append(edges, map[string]any{"from": previousSynthesisID, "to": entryID, "on": "completed"})
			}
		}
		phases = append(phases, phase)
		jobs = append(jobs, phaseJobs...)
		edges = append(edges, phaseEdges...)
		cycles = append(cycles, phaseCycles...)
		previousSynthesisID = synthesisID
	}
	return jobs, edges, cycles, phases, nil
}

func trackSpec(spec Spec, phaseID, trackID, trackShape string, track map[string]any) (Spec, error) {
	lanes := map[string]map[string]any{}
	for laneID, lane := range spec.Lanes {
		lanes[laneID] = cloneMap(lane)
	}
	if laneID, ok := track["lane_id"].(string); ok {
		if _, ok := stringSet(laneIDsFor(spec))[laneID]; !ok {
			return Spec{}, genErr("track lane_id references missing lane", "spec.options.phases[].tracks[].lane_id")
		}
	}
	options := cloneMap(spec.Options)
	delete(options, "phases")
	if rawOptions, ok := track["options"].(map[string]any); ok {
		for key, value := range rawOptions {
			options[key] = value
		}
	}
	return Spec{
		SchemaVersion: spec.SchemaVersion,
		Shape:         trackShape,
		LaneSet:       spec.LaneSet,
		WorkflowID:    fmt.Sprintf("%s-%s-%s", spec.WorkflowID, phaseID, trackID),
		Name:          fmt.Sprintf("%s %s %s", spec.Name, phaseID, trackID),
		WorkflowVer:   spec.WorkflowVer,
		Branch:        cloneMap(spec.Branch),
		ScaffoldRoot:  spec.ScaffoldRoot,
		ArtifactRoot:  fmt.Sprintf("%s/%s/%s", spec.ArtifactRoot, phaseID, trackID),
		Lanes:         lanes,
		Options:       options,
		LaneModifiers: append([]string(nil), spec.LaneModifiers...),
		ContextDocs:   append([]any(nil), spec.ContextDocs...),
		Parallelism:   spec.Parallelism,
	}, nil
}

func remapEdges(edges []map[string]any, idMap map[string]string) []map[string]any {
	result := []map[string]any{}
	for _, edge := range edges {
		result = append(result, map[string]any{
			"from": idMap[fmt.Sprint(edge["from"])],
			"to":   idMap[fmt.Sprint(edge["to"])],
			"on":   fmt.Sprint(defaultAny(edge["on"], "completed")),
		})
	}
	return result
}

func remapCycles(cycles []map[string]any, idMap map[string]string) []map[string]any {
	result := []map[string]any{}
	for _, cycle := range cycles {
		remapped := cloneMap(cycle)
		remapped["from"] = idMap[fmt.Sprint(cycle["from"])]
		remapped["to"] = idMap[fmt.Sprint(cycle["to"])]
		result = append(result, remapped)
	}
	return result
}

func entryJobIDs(jobs, edges []map[string]any, idMap map[string]string) []string {
	incoming := map[string]int{}
	for _, job := range jobs {
		incoming[fmt.Sprint(job["id"])] = 0
	}
	for _, edge := range edges {
		incoming[fmt.Sprint(edge["to"])]++
	}
	result := []string{}
	for jobID, count := range incoming {
		if count == 0 {
			result = append(result, idMap[jobID])
		}
	}
	return result
}

func terminalJobIDs(jobs, edges []map[string]any, idMap map[string]string) []string {
	outgoing := map[string]int{}
	for _, job := range jobs {
		outgoing[fmt.Sprint(job["id"])] = 0
	}
	for _, edge := range edges {
		outgoing[fmt.Sprint(edge["from"])]++
	}
	result := []string{}
	for jobID, count := range outgoing {
		if count == 0 {
			result = append(result, idMap[jobID])
		}
	}
	return result
}

func phaseSynthesisLane(spec Spec, phase map[string]any) (string, error) {
	if laneID, ok := phase["synthesis_lane_id"].(string); ok {
		if _, ok := stringSet(laneIDsFor(spec))[laneID]; !ok {
			return "", genErr("synthesis_lane_id references missing lane", "spec.options.phases[].synthesis_lane_id")
		}
		return laneID, nil
	}
	return reviewerLane(spec, 1), nil
}

func phaseSynthesisJob(phaseID, phaseName, synthesisID, laneID, artifactRoot string) map[string]any {
	result := job(
		synthesisID,
		"phase_synthesis",
		"Synthesize "+phaseName,
		"reviewer",
		laneID,
		artifactRoot+"/"+phaseID,
		"SYNTHESIS.md",
		"synthesis",
		"phase_synthesis",
		"apply",
		"Synthesize the completed work in "+phaseName+" and record a phase verdict.",
	)
	result["phase_id"] = phaseID
	return result
}

func compileCustom(spec Spec, lanes map[string]any) ([]map[string]any, []map[string]any, []map[string]any, error) {
	if spec.Plan["schema_version"] != PlanSchemaVersion {
		return nil, nil, nil, genErr("custom plan has unsupported schema_version", "spec.plan.schema_version")
	}
	blocks, err := objectList(spec.Plan["blocks"], "spec.plan.blocks")
	if err != nil {
		return nil, nil, nil, err
	}
	bindings, err := object(defaultAny(spec.Plan["job_lane_bindings"], map[string]any{}), "spec.plan.job_lane_bindings")
	if err != nil {
		return nil, nil, nil, err
	}
	seen := map[string]struct{}{}
	jobs := []map[string]any{}
	for idx, block := range blocks {
		prefix := fmt.Sprintf("spec.plan.blocks[%d]", idx)
		blockID, err := requiredString(block, "id", prefix)
		if err != nil {
			return nil, nil, nil, err
		}
		if _, ok := seen[blockID]; ok {
			return nil, nil, nil, genErr("duplicate custom block id", prefix+".id")
		}
		seen[blockID] = struct{}{}
		kind, err := choice(block, "kind", blockKinds, prefix)
		if err != nil {
			return nil, nil, nil, err
		}
		laneID, ok := bindings[blockID].(string)
		if !ok || laneID == "" {
			return nil, nil, nil, genErr("custom block missing lane binding", "spec.plan.job_lane_bindings."+blockID)
		}
		if _, ok := lanes[laneID]; !ok {
			return nil, nil, nil, genErr("custom lane binding references missing lane", "spec.plan.job_lane_bindings."+blockID)
		}
		if _, ok := block["review_posture"]; ok && !isReviewKind(kind) {
			return nil, nil, nil, genErr("review-only fields on non-review block", prefix+".review_posture")
		}
		artifactPath := fmt.Sprintf("%s/%s.md", spec.ArtifactRoot, strings.ToUpper(blockID))
		if value, ok := block["artifact_path"].(string); ok && value != "" {
			artifactPath = value
		}
		if err := safeArtifactPath(artifactPath, spec.ArtifactRoot, prefix+".artifact_path"); err != nil {
			return nil, nil, nil, err
		}
		custom, err := customJob(blockID, kind, laneID, artifactPath, block, idx)
		if err != nil {
			return nil, nil, nil, err
		}
		jobs = append(jobs, custom)
	}
	ids := map[string]struct{}{}
	for _, job := range jobs {
		ids[fmt.Sprint(job["id"])] = struct{}{}
	}
	edges, err := customEdges(spec.Plan, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := assertAcyclic(ids, edges); err != nil {
		return nil, nil, nil, err
	}
	cycles, err := customCycles(spec.Plan, ids, jobs)
	if err != nil {
		return nil, nil, nil, err
	}
	return jobs, edges, cycles, nil
}

func job(id, jobType, title, role, lane, root, filename, artifactKind, logicalName, prompt, objective string) map[string]any {
	if objective == "" {
		objective = title
	}
	return map[string]any{
		"id":          id,
		"type":        jobType,
		"title":       title,
		"role_id":     role,
		"lane_id":     lane,
		"objective":   objective,
		"task_prompt": map[string]any{"path": "prompts/" + prompt + ".md"},
		"write_scope": map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []string{root + "/"}, "forbidden_paths": []string{".striatum/"}},
		"expected_artifacts": []map[string]any{{
			"logical_name": logicalName,
			"kind":         artifactKind,
			"path":         root + "/" + filename,
			"required":     true,
		}},
	}
}

func reviewJob(id, lane, artifactPath, posture string) map[string]any {
	return reviewJobForRole(id, "Review the draft", "reviewer", lane, artifactPath, posture, "review", "review", "Review the draft and record a finding.")
}

func reviewJobForRole(id, title, role, lane, artifactPath, posture, logicalName, prompt, objective string) map[string]any {
	root := path.Dir(artifactPath)
	filename := path.Base(artifactPath)
	result := job(id, "review", title, role, lane, root, filename, "finding", logicalName, prompt, objective)
	result["fresh_session_required"] = true
	result["write_scope"] = map[string]any{"mode": "review_only_artifact", "repo_write": false, "allowed_paths": []string{root + "/"}, "forbidden_paths": []string{".striatum/"}}
	if posture != "neutral" {
		result["review_posture"] = posture
	}
	return result
}

func panelRolePacks(spec Spec) ([]string, error) {
	packs, err := panelStringOptionList(spec, "role_packs", "role_pack", []string{"implementation_panel_roles"}, "spec.options.role_packs")
	if err != nil {
		return nil, err
	}
	if err := validateCatalogPacks(packs, "role_pack", "spec.options.role_packs"); err != nil {
		return nil, err
	}
	if _, ok := stringSet(packs)["implementation_panel_roles"]; !ok {
		return nil, genErr("implementation_panel requires implementation_panel_roles", "spec.options.role_packs")
	}
	return packs, nil
}

func panelAdversaryPacks(spec Spec) ([]string, error) {
	packs, err := panelStringOptionList(spec, "adversary_packs", "adversary_pack", []string{"maintainer_cost"}, "spec.options.adversary_packs")
	if err != nil {
		return nil, err
	}
	if err := validateCatalogPacks(packs, "adversary_pack", "spec.options.adversary_packs"); err != nil {
		return nil, err
	}
	return packs, nil
}

func panelStringOptionList(spec Spec, pluralKey, singularKey string, fallback []string, fieldPath string) ([]string, error) {
	raw, ok := spec.Options[pluralKey]
	if !ok {
		raw = spec.Options[singularKey]
	}
	if raw == nil {
		return append([]string(nil), fallback...), nil
	}
	if text, ok := raw.(string); ok && strings.TrimSpace(text) != "" {
		return []string{text}, nil
	}
	values, err := stringList(raw, fieldPath)
	if err != nil || len(values) == 0 {
		return nil, genErr(strings.TrimPrefix(pluralKey, "panel_")+" must be a non-empty string list", fieldPath)
	}
	return values, nil
}

func validateCatalogPacks(packs []string, kind, fieldPath string) error {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		return genErr("workflow template catalog could not be loaded", fieldPath)
	}
	for idx, packID := range packs {
		template, err := catalog.Get(packID)
		if err != nil {
			return genErr(fmt.Sprintf("unknown %s: %s", kind, packID), fmt.Sprintf("%s[%d]", fieldPath, idx))
		}
		if template["kind"] != kind {
			return genErr(fmt.Sprintf("template %q is not a %s", packID, kind), fmt.Sprintf("%s[%d]", fieldPath, idx))
		}
	}
	return nil
}

func panelProposalCount(spec Spec) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["proposal_count"], 3))
	if !ok || value < 2 || value > 3 {
		return 0, genErr("proposal_count must be 2 or 3 for implementation_panel", "spec.options.proposal_count")
	}
	return value, nil
}

func panelScoreDimensions(spec Spec) ([]string, error) {
	values := []string{}
	if raw, ok := spec.Options["score_dimensions"]; ok && raw != nil {
		list, err := stringList(raw, "spec.options.score_dimensions")
		if err != nil || len(list) == 0 {
			return nil, genErr("score_dimensions must be a non-empty string list", "spec.options.score_dimensions")
		}
		for _, item := range list {
			values = append(values, strings.TrimSpace(item))
		}
	}
	if len(values) == 0 {
		packs, err := panelAdversaryPacks(spec)
		if err != nil {
			return nil, err
		}
		catalog, err := workflowtemplates.Load()
		if err != nil {
			return nil, genErr("workflow template catalog could not be loaded", "spec.options.adversary_packs")
		}
		for _, packID := range packs {
			template, err := catalog.Get(packID)
			if err != nil {
				return nil, genErr(fmt.Sprintf("unknown adversary_pack: %s", packID), "spec.options.adversary_packs")
			}
			for _, item := range listFrom(template["postures"]) {
				if text, ok := item.(string); ok && strings.TrimSpace(text) != "" {
					values = append(values, text)
				}
			}
		}
	}
	if len(values) == 0 {
		values = []string{"maintainability", "migration_risk", "reversibility"}
	}
	for idx, value := range values {
		if !safePanelSlug(value) {
			return nil, genErr("score_dimensions entries must match ^[a-z0-9._-]{1,64}$", fmt.Sprintf("spec.options.score_dimensions[%d]", idx))
		}
	}
	return values, nil
}

func safePanelSlug(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, ch := range value {
		if ch > 127 {
			return false
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '.' || ch == '_' || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func panelScorePosture(scoreDimensions []string) string {
	return "custom:" + scoreDimensions[0]
}

func panelProposalLane(spec Spec) string {
	return authorLane(spec)
}

func panelReviewLane(spec Spec, idx int) string {
	if spec.LaneSet == "multi_review" {
		count := reviewerCount(spec)
		if idx > count {
			idx = count
		}
		return reviewerLane(spec, idx)
	}
	return reviewerLane(spec, 1)
}

func panelArbitrationLane(spec Spec) string {
	return authorLane(spec)
}

func panelDecisionLane(spec Spec) string {
	return authorLane(spec)
}

func renderFiles(spec Spec, workflow map[string]any, roles map[string]any) ([]map[string]any, error) {
	body, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return nil, err
	}
	files := []map[string]any{{"path": spec.ScaffoldRoot + "/workflow.json", "content": string(body) + "\n"}}
	for _, role := range sortedMapKeys(roles) {
		files = append(files, map[string]any{"path": spec.ScaffoldRoot + "/roles/" + role + ".md", "content": roleStub(role)})
	}
	prompts := map[string]struct{}{}
	for _, item := range listFrom(workflow["jobs"]) {
		job := mapFrom(item)
		task := mapFrom(job["task_prompt"])
		if p, ok := task["path"].(string); ok && strings.HasPrefix(p, "prompts/") {
			prompts[strings.TrimPrefix(p, "prompts/")] = struct{}{}
		}
	}
	for _, prompt := range sortedKeys(prompts) {
		files = append(files, map[string]any{"path": spec.ScaffoldRoot + "/prompts/" + prompt, "content": promptStub(prompt)})
	}
	return files, nil
}

func ValidateWorkflow(workflow map[string]any) error {
	if err := workflowauthoring.Validate(workflow); err != nil {
		if authoringErr, ok := err.(*workflowauthoring.Error); ok {
			fieldPath := authoringErr.FieldPath
			if fieldPath != "" && !strings.HasPrefix(fieldPath, "workflow.") {
				fieldPath = "workflow." + fieldPath
			}
			return genErr(authoringErr.Message, fieldPath)
		}
		return err
	}
	return nil
}

func SafeRelativePath(value, fieldPath string) (string, error) {
	if value == "" || strings.ContainsRune(value, '\x00') || strings.HasPrefix(value, "/") || strings.Contains(value, "\\") {
		return "", genErr("path must be repo-relative", fieldPath)
	}
	clean := path.Clean(value)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", genErr("path must not escape the repository or target .git/.striatum", fieldPath)
	}
	for _, part := range strings.Split(clean, "/") {
		if part == ".." || part == ".git" || part == ".striatum" {
			return "", genErr("path must not escape the repository or target .git/.striatum", fieldPath)
		}
	}
	return strings.TrimSuffix(clean, "/"), nil
}

func FileHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func roleStub(role string) string {
	panelRoles := map[string]string{
		"problem_framer":    "# Problem Framer Role\n\nYou frame the implementation problem before proposals begin. Publish constraints, goals, non-goals, and decision criteria at the declared artifact path.\n",
		"proposer_a":        "# Proposer A Role\n\nYou develop implementation option A independently from the other proposal roles. Stay inside the declared write scope.\n",
		"proposer_b":        "# Proposer B Role\n\nYou develop implementation option B independently from the other proposal roles. Stay inside the declared write scope.\n",
		"proposer_c":        "# Proposer C Role\n\nYou develop implementation option C independently from the other proposal roles. Stay inside the declared write scope.\n",
		"scorekeeper":       "# Scorekeeper Role\n\nYou score one proposal against the selected adversary-pack dimensions. Publish only the review artifact at the declared path.\n",
		"tradeoff_ledger":   "# Tradeoff Ledger Role\n\nYou normalize proposal and scorecard evidence into a tradeoff ledger at the declared artifact path.\n",
		"arbitrator":        "# Arbitrator Role\n\nYou select or compose the preferred implementation path from the tradeoff ledger and supporting evidence.\n",
		"dissent_reviewer":  "# Dissent Reviewer Role\n\nYou try to falsify the arbitration before final decision. Publish only the review artifact at the declared path.\n",
		"principal_decider": "# Principal Decider Role\n\nYou record the final implementation decision and required follow-up work at the declared artifact path.\n",
		"holder":            "# Holder Role\n\nYou publish the leading proposal as the claim falsifiers will challenge. Do not wait for live questions; the adjudicator ledger decides whether the static challenge/rebuttal gate clears.\n",
		"falsifier":         "# Falsifier Role\n\nYou challenge the published holder artifact. Write a concrete falsifying gap, the strongest rebuttal you can justify from the available artifacts, and do not publish the collaboration ledger.\n",
		"cross_examiner":    "# Cross-Examiner Role\n\nYou challenge the published finding or proposal before downstream publication. Record the challenge, the strongest rebuttal you can justify, and any unanswered gap in your declared artifact.\n",
		"adjudicator":       "# Adjudicator Role\n\nYou read only the curated dialogue trajectory, never raw terminal output. Publish the collaboration ledger and verdict according to the substance rubric. The `verdict` field MUST be one of: accept, accept_with_findings, needs_revision, reject. A clearing verdict (the one that lets the downstream phase publish) is `accept` or `accept_with_findings` — do not write `clear` or any other value.\n",
		"scribe":            "# Scribe Role\n\nYou record only the decision trail visible in the dialogue trajectory. Do not hypothesize, infer hidden reasoning, or add claims that are not present in the curated dialogue.\n",
		"committer":         "# Committer Role\n\nYou publish the downstream proposal or finding only after the collaboration ledger verdict clears the phase gate.\n",
		"convener":          "# Convener Role\n\nYou frame the problem, draft the candidate synthesis, and stay live for cross-examination. On a revision cycle you receive the prior cycle's constraints[] as binding input and must discharge each row explicitly. Do not treat dialogue completion as acceptance; the adjudicator ledger decides whether the gate clears.\n",
		"revision_convener": "# Revision Convener Role\n\nYou republish the synthesis after an adjudicated needs_revision. You take the prior cycle's constraints[] as first-class input and discharge each row explicitly (answer / fold-in / reject-with-rationale / accept-as-risk / defer-with-successor). Republished artifacts use the cycle-templated logical name.\n",
		"spec_author":       "# Spec Author Role\n\nYou write the RFC/spec using the latest cleared constraint ledger as binding input, not the original proposal. Every binding constraint must land in the spec as testable text or a gate.\n",
		"final_reviewer":    "# Final Reviewer Role\n\nYou verify discharge, you do not re-run the forum. Emit a constraint_discharge table marking each binding constraint discharged / partial / missing / accepted_risk with evidence. Final review is a typecheck that fails closed on any undischarged binding constraint.\n",
	}
	if content, ok := panelRoles[role]; ok {
		return content
	}
	if role == "reviewer" {
		return "# Reviewer Role\n\nYou are the reviewer for this workflow. Read the upstream draft and write a single review-only finding artifact at the declared path; do not modify other files.\n"
	}
	return "# Author Role\n\nYou are the author for this workflow. Produce the expected handoff artifact at the path declared in the workflow. Stay inside the declared write scope.\n"
}

func promptStub(prompt string) string {
	switch prompt {
	case "draft.md":
		return "Draft the initial artifact described by the workflow. Replace this stub with the concrete authoring instructions for your team.\n"
	case "review.md":
		return "Review the upstream draft and record a finding with one of the supported verdicts. Replace this stub with reviewer guidance.\n"
	case "apply.md":
		return "Apply the accepted review by producing the final synthesis artifact. Replace this stub with concrete apply instructions.\n"
	case "collaboration_holder.md":
		return "Produce the leading proposal as the published claim falsifiers will challenge. Do not treat challenge completion as acceptance; the adjudicator ledger decides whether the gate clears.\n"
	case "collaboration_falsifier.md":
		return "Read the published holder proposal and write a material falsifying challenge. Record the challenge, the strongest rebuttal you can justify, and any unanswered gap in the declared artifact.\n"
	case "collaboration_author_draft.md":
		return "Draft the finding or proposal as the published claim cross-examiners will challenge. The downstream publication is gated by the adjudicator's collaboration ledger.\n"
	case "collaboration_cross_examiner.md":
		return "Read the published draft and write one falsifying cross-examination challenge. Record the challenge, the strongest rebuttal you can justify, and any unanswered gap in the declared artifact.\n"
	case "adjudicate_collaboration.md":
		return "Read only the curated dialogue trajectory. Publish a collaboration_ledger whose verdict reflects whether a material challenge landed and was directly rebutted.\n"
	case "collaboration_scribe.md":
		return "Record only the visible dialogue decision trail. Do not invent missing reasoning or copy raw terminal/provider output.\n"
	case "collaboration_commit.md":
		return "Publish the downstream proposal or finding after the adjudicator ledger verdict clears the phase gate.\n"
	case "collaboration_final_summary.md":
		return "Summarize the collaboration gate result and downstream publication in a final synthesis artifact.\n"
	case "ace_survey.md":
		return "Survey the prior art, evidence, and existing constraints for the topic. Record what is already known and what is contested; do not synthesize a solution yet.\n"
	case "ace_survey_synthesis.md":
		return "Frame the problem, goals, non-goals, and decision criteria from the survey. This phase synthesis sets the scope the candidate synthesis must address.\n"
	case "ace_convener.md":
		return "Draft the candidate synthesis and stay live for cross-examination. On a revision cycle you receive the prior cycle's constraints[] as binding input; discharge each row explicitly. Do not treat dialogue completion as acceptance.\n"
	case "ace_convener_synthesis.md":
		return "Publish the candidate synthesis. On a revision cycle, every prior constraints[] row must be discharged explicitly (answer / fold-in / reject-with-rationale / accept-as-risk / defer-with-successor). This artifact is cycle-templated so it republishes cleanly.\n"
	case "ace_cross_examiner.md":
		return "Challenge the candidate synthesis from your assigned posture only (product / implementation / privacy / eval / operations or the configured override). Record findings[] rows with severity, the affected invariant, the closest acceptable answer, and the constraint shape you would require. An unanswered interrogation is evidence — record it.\n"
	case "ace_cross_exam_synthesis.md":
		return "Roll up every cross-examiner posture into one findings ledger. Preserve each finding's posture, severity, and status; carry unanswered interrogations forward as evidence for the adjudicator.\n"
	case "ace_adjudication_intake.md":
		return "Assemble the candidate synthesis and the cross-examination findings for adjudication. Do not add new challenges; stage the curated trajectory only.\n"
	case "ace_adjudicate.md":
		return "Read only the curated trajectory. Publish the collaboration_ledger verdict (accept / accept_with_findings / needs_revision / reject). On needs_revision you MUST convert each load-bearing challenge into a binding constraints[] row (or an explicit unresolved_question row); a naked refusal with an empty constraints[] is rejected (exit code 6). Each binding constraint needs a typed kind, a source_finding, a posture, severity, and a verification gate or expected_stage. Maintain the posture-disposition matrix in branches{}.\n"
	case "ace_revision_convener.md":
		return "Take the prior cycle's constraints[] as binding input. Discharge each row explicitly: answer / fold-in / reject-with-rationale / accept-as-risk / defer-with-successor. A high-severity challenge may only leave open via a recorded disposition. Republished artifacts use the cycle-templated logical name.\n"
	case "ace_revision_synthesis.md":
		return "Publish the revised synthesis that discharges the adjudicated constraints[]. Each binding constraint must be visibly addressed; this phase synthesis is the candidate the discharge review checks.\n"
	case "ace_discharge_review.md":
		return "Review the revised synthesis against the binding constraints[]. Confirm each constraint is discharged or flag it still open; this re-review is cycle-templated so each cycle republishes cleanly.\n"
	case "ace_discharge_review_synthesis.md":
		return "Confirm the latest cleared constraint ledger before spec publication. Record which ledger cycle is binding for the spec author.\n"
	case "ace_spec_author.md":
		return "Write the RFC/spec from the latest cleared constraint ledger as binding input — not from the original proposal. Every binding constraint must land in the spec as testable text or a gate.\n"
	case "ace_spec_publication.md":
		return "Publish the spec gated on the latest cleared collaboration ledger. The spec begins from adjudicated constraints, not the original proposal.\n"
	case "ace_final_review.md":
		return "Emit a constraint_discharge table: for each binding constraint, mark discharged / partial / missing / accepted_risk with evidence (a spec section or gate reference). Final review is a typecheck — do not re-run the forum. It fails closed on any binding constraint that is missing or partial-without-accepted-risk.\n"
	case "ace_final_review_synthesis.md":
		return "Summarize the discharge typecheck. The run fails closed on any undischarged binding constraint; record the coverage counts (raised / converted / discharged) for the dashboard.\n"
	default:
		return fmt.Sprintf("Complete the %s step declared by the workflow.\n", strings.ReplaceAll(strings.TrimSuffix(prompt, ".md"), "_", " "))
	}
}

func validateModifierMatrix(spec Spec, warnings *[]string) error {
	if isCollaborationShape(spec.Shape) && spec.LaneSet == "single_agent" {
		return &Error{Message: "collaboration shapes require at least a fixture or independent adjudication lane set", FieldPath: "spec.lane_set", Hint: "Use lane_set local for fixtures or author_reviewer/multi_review for real runs."}
	}
	for idx, modifier := range spec.LaneModifiers {
		if (modifier == "supervised" || modifier == "harness_profiled") && spec.LaneSet == "local" {
			return &Error{Message: "lane modifier is incompatible with lane set", FieldPath: fmt.Sprintf("spec.lane_modifiers[%d]", idx), Hint: fmt.Sprintf("modifier %q is forbidden for lane_set 'local'", modifier)}
		}
		if modifier == "worktree_isolated" && spec.Shape == "multi_review_synthesis" {
			*warnings = append(*warnings, "worktree_isolated has no effect on review-only jobs except synthesis")
		}
	}
	if hasModifier(spec, "harness_profiled") {
		if _, err := harnessProfiles(spec); err != nil {
			return err
		}
	}
	if hasModifier(spec, "constrained") {
		if _, err := constraints(spec); err != nil {
			return err
		}
		if _, err := requiredEnforcement(spec); err != nil {
			return err
		}
	}
	return nil
}

func applyLaneModifiers(spec Spec, lanes map[string]any, repoWrite map[string]struct{}) (map[string]any, error) {
	result := cloneNested(lanes)
	if hasModifier(spec, "worktree_isolated") {
		for laneID := range repoWrite {
			lane := mapFrom(result[laneID])
			lane["worktree_isolation"] = "per_job"
			result[laneID] = lane
		}
	}
	if hasModifier(spec, "constrained") {
		constraints, err := constraints(spec)
		if err != nil {
			return nil, err
		}
		required, err := requiredEnforcement(spec)
		if err != nil {
			return nil, err
		}
		for laneID, raw := range result {
			lane := mapFrom(raw)
			lane["constraints"] = constraints
			if len(required) > 0 {
				lane["required_enforcement"] = required
			}
			result[laneID] = lane
		}
	}
	if hasModifier(spec, "harness_profiled") {
		profiles, err := harnessProfiles(spec)
		if err != nil {
			return nil, err
		}
		profileIDs := sortedMapKeys(profiles)
		laneIDs := sortedMapKeys(result)
		for idx, laneID := range laneIDs {
			pick := idx
			if pick >= len(profileIDs) {
				pick = len(profileIDs) - 1
			}
			lane := mapFrom(result[laneID])
			lane["harness_profile_id"] = profileIDs[pick]
			result[laneID] = lane
		}
	}
	return result, nil
}

func harnessProfiles(spec Spec) (map[string]any, error) {
	profiles, err := object(spec.Options["harness_profiles"], "spec.options.harness_profiles")
	if err != nil || len(profiles) == 0 {
		return nil, genErr("harness_profiled requires options.harness_profiles", "spec.options.harness_profiles")
	}
	result := map[string]any{}
	for profileID, raw := range profiles {
		body, ok := raw.(map[string]any)
		if !ok {
			return nil, genErr("harness profile body must be an object", "spec.options.harness_profiles."+profileID)
		}
		family, _ := body["tool_family"].(string)
		if _, ok := workflowtemplatesHarnessFamilies()[family]; !ok {
			return nil, genErr("harness profile has unknown tool_family", "spec.options.harness_profiles."+profileID+".tool_family")
		}
		result[profileID] = enrichHarnessProfile(cloneMap(body))
	}
	return result, nil
}

func enrichHarnessProfile(body map[string]any) map[string]any {
	fragment := harnessFragmentByToolFamily(fmt.Sprint(body["tool_family"]))
	if fragment == nil {
		return body
	}
	native := map[string]any{}
	if raw, ok := body["native_delegation"].(map[string]any); ok {
		native = cloneMap(raw)
	}
	if value, ok := native["instruction"].(string); !ok || strings.TrimSpace(value) == "" {
		native["instruction"] = fragment["native_delegation_instruction"]
	}
	if _, ok := native["mode"]; !ok {
		if mode, ok := fragment["native_delegation_mode"].(string); ok && mode != "" {
			native["mode"] = mode
		}
	}
	body["native_delegation"] = native
	return body
}

func harnessFragmentByToolFamily(family string) map[string]any {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		return nil
	}
	for _, fragment := range catalog.HarnessProfileFragments {
		if fragment["tool_family"] == family {
			return fragment
		}
	}
	return nil
}

func workflowtemplatesHarnessFamilies() map[string]struct{} {
	return set("generic", "codex", "claude_code", "agy")
}

func constraints(spec Spec) (map[string]any, error) {
	constraints, err := object(defaultAny(spec.Options["constraints"], map[string]any{}), "spec.options.constraints")
	if err != nil {
		return nil, err
	}
	for key, value := range constraints {
		allowed, ok := constraintValues[key]
		if !ok {
			return nil, genErr("unknown adapter constraint value", "spec.options.constraints."+key)
		}
		if _, ok := allowed[fmt.Sprint(value)]; !ok {
			return nil, genErr("unknown adapter constraint value", "spec.options.constraints."+key)
		}
	}
	return constraints, nil
}

func requiredEnforcement(spec Spec) (map[string]any, error) {
	required, err := object(defaultAny(spec.Options["required_enforcement"], map[string]any{}), "spec.options.required_enforcement")
	if err != nil {
		return nil, err
	}
	for key, value := range required {
		if _, ok := constraintValues[key]; !ok {
			return nil, genErr("unknown required enforcement value", "spec.options.required_enforcement."+key)
		}
		if _, ok := enforcementLevels[fmt.Sprint(value)]; !ok {
			return nil, genErr("unknown required enforcement value", "spec.options.required_enforcement."+key)
		}
	}
	return required, nil
}

func rolesFor(roleIDs []string) map[string]any {
	result := map[string]any{}
	for _, role := range roleIDs {
		result[role] = map[string]any{"definition_path": "roles/" + role + ".md"}
	}
	return result
}

func coordinator(spec Spec, lanes map[string]any) map[string]any {
	if spec.Shape == "implementation_panel" {
		return map[string]any{"role_id": "problem_framer", "lane_id": panelProposalLane(spec)}
	}
	if spec.Shape == "falsification_gate" {
		return map[string]any{"role_id": "holder", "lane_id": authorLane(spec)}
	}
	if spec.Shape == "cross_examination" {
		return map[string]any{"role_id": "author", "lane_id": authorLane(spec)}
	}
	if spec.Shape == "adjudicated_constraint_extraction" {
		return map[string]any{"role_id": "convener", "lane_id": authorLane(spec)}
	}
	lane := "local"
	if lanes[lane] == nil {
		if lanes["author"] != nil {
			lane = "author"
		} else {
			keys := sortedMapKeys(lanes)
			if len(keys) > 0 {
				lane = keys[0]
			}
		}
	}
	return map[string]any{"role_id": "author", "lane_id": lane}
}

func defaultParallelism(spec Spec) map[string]any {
	maxJobs := 1
	switch spec.Shape {
	case "multi_review_synthesis":
		maxJobs = reviewerCount(spec)
	case "implementation_panel":
		if count, err := panelProposalCount(spec); err == nil {
			maxJobs = count
		}
	}
	return map[string]any{"mode": "declared", "max_active_jobs": maxJobs, "require_disjoint_write_scopes": true}
}

func authorLane(spec Spec) string {
	switch spec.LaneSet {
	case "local":
		return "local"
	case "single_agent":
		return "agent"
	default:
		return "author"
	}
}

func reviewerLane(spec Spec, idx int) string {
	switch spec.LaneSet {
	case "local":
		return "local"
	case "single_agent":
		return "agent"
	case "multi_review":
		return fmt.Sprintf("reviewer_%d", idx)
	default:
		return "reviewer"
	}
}

func reviewerCount(spec Spec) int {
	if value, ok := intFrom(spec.Options["reviewer_count"]); ok && value > 0 {
		return value
	}
	if postures, ok := spec.Options["review_postures"].([]any); ok && len(postures) > 0 {
		return len(postures)
	}
	if isCollaborationShape(spec.Shape) && spec.LaneSet == "multi_review" {
		if spec.Shape == "falsification_gate" {
			if falsifiers, err := falsifierCount(spec); err == nil {
				return falsifiers + 1
			}
			return 3
		}
		return 2
	}
	return 2
}

func postures(spec Spec, count int) ([]string, error) {
	values := []string{"devils_advocate", "security"}
	if raw, ok := spec.Options["review_postures"].([]any); ok && len(raw) > 0 {
		values = []string{}
		for idx, item := range raw {
			posture := fmt.Sprint(item)
			if err := validatePosture(posture, fmt.Sprintf("spec.options.review_postures[%d]", idx)); err != nil {
				return nil, err
			}
			values = append(values, posture)
		}
	}
	for len(values) < count {
		values = append(values, "neutral")
	}
	return values[:count], nil
}

func firstPosture(spec Spec) string {
	if _, ok := spec.Options["review_postures"]; !ok {
		return "neutral"
	}
	values, err := postures(spec, 1)
	if err != nil || len(values) == 0 {
		return "neutral"
	}
	return values[0]
}

func maxCycles(spec Spec) (int, error) {
	value, ok := intFrom(defaultAny(spec.Options["max_revision_cycles"], 1))
	if !ok || value < 1 {
		return 0, genErr("max_revision_cycles must be a positive integer", "spec.options.max_revision_cycles")
	}
	return value, nil
}

func isCollaborationShape(shape string) bool {
	return shape == "falsification_gate" || shape == "cross_examination" || shape == "adjudicated_constraint_extraction"
}

func validatePosture(posture, fieldPath string) error {
	if _, ok := allowedPostures[posture]; ok {
		return nil
	}
	if strings.HasPrefix(posture, "custom:") && strings.TrimSpace(strings.TrimPrefix(posture, "custom:")) != "" {
		return nil
	}
	return genErr("invalid review posture", fieldPath)
}

func customJob(blockID, kind, laneID, artifactPath string, block map[string]any, idx int) (map[string]any, error) {
	role := "author"
	jobType := kind
	artifactKind := "handoff"
	if isReviewKind(kind) {
		role = "reviewer"
		jobType = "review"
		artifactKind = "finding"
	} else if kind == "synthesis" {
		jobType = "synthesis"
	} else if kind == "implementation" || kind == "test" || kind == "support_ledger" {
		jobType = "build"
		if kind == "support_ledger" {
			artifactKind = "support_ledger"
		}
	}
	title := fmt.Sprint(block["title"])
	if title == "" || title == "<nil>" {
		title = titleFromBlockID(blockID)
	}
	result := job(blockID, jobType, title, role, laneID, path.Dir(artifactPath), path.Base(artifactPath), artifactKind, blockID, kind, "")
	if role == "reviewer" {
		posture := "neutral"
		if value, ok := block["review_posture"].(string); ok {
			posture = value
		} else if value, ok := block["posture"].(string); ok {
			posture = value
		}
		if err := validatePosture(posture, fmt.Sprintf("spec.plan.blocks[%d].posture", idx)); err != nil {
			return nil, err
		}
		result["review_posture"] = posture
		result["fresh_session_required"] = true
	}
	return result, nil
}

func customEdges(plan map[string]any, ids map[string]struct{}) ([]map[string]any, error) {
	raw, err := objectList(defaultAny(plan["edges"], []any{}), "spec.plan.edges")
	if err != nil {
		return nil, err
	}
	result := []map[string]any{}
	for idx, edge := range raw {
		from, err := requiredString(edge, "from", fmt.Sprintf("spec.plan.edges[%d]", idx))
		if err != nil {
			return nil, err
		}
		to, err := requiredString(edge, "to", fmt.Sprintf("spec.plan.edges[%d]", idx))
		if err != nil {
			return nil, err
		}
		if _, ok := ids[from]; !ok {
			return nil, genErr("edge references missing block", fmt.Sprintf("spec.plan.edges[%d].from", idx))
		}
		if _, ok := ids[to]; !ok {
			return nil, genErr("edge references missing block", fmt.Sprintf("spec.plan.edges[%d].to", idx))
		}
		on := "completed"
		if value, ok := edge["on"].(string); ok && value != "" {
			on = value
		}
		result = append(result, map[string]any{"from": from, "to": to, "on": on})
	}
	return result, nil
}

func customCycles(plan map[string]any, ids map[string]struct{}, jobs []map[string]any) ([]map[string]any, error) {
	raw, err := objectList(defaultAny(plan["cycles"], []any{}), "spec.plan.cycles")
	if err != nil {
		return nil, err
	}
	reviewIDs := map[string]struct{}{}
	for _, job := range jobs {
		if job["type"] == "review" {
			reviewIDs[fmt.Sprint(job["id"])] = struct{}{}
		}
	}
	result := []map[string]any{}
	for idx, cycle := range raw {
		from, err := requiredString(cycle, "from", fmt.Sprintf("spec.plan.cycles[%d]", idx))
		if err != nil {
			return nil, err
		}
		to, err := requiredString(cycle, "to", fmt.Sprintf("spec.plan.cycles[%d]", idx))
		if err != nil {
			return nil, err
		}
		if _, ok := ids[from]; !ok {
			return nil, genErr("cycle references missing block", fmt.Sprintf("spec.plan.cycles[%d].from", idx))
		}
		if _, ok := ids[to]; !ok {
			return nil, genErr("cycle references missing block", fmt.Sprintf("spec.plan.cycles[%d].to", idx))
		}
		if _, ok := reviewIDs[from]; !ok {
			return nil, genErr("cycle source must be a review block", fmt.Sprintf("spec.plan.cycles[%d].from", idx))
		}
		maxIterations, ok := intFrom(cycle["max_iterations"])
		if !ok || maxIterations < 1 {
			return nil, genErr("cycle max_iterations must be positive", fmt.Sprintf("spec.plan.cycles[%d].max_iterations", idx))
		}
		onVerdict := "needs_revision"
		if value, ok := cycle["on_verdict"].(string); ok && value != "" {
			onVerdict = value
		}
		result = append(result, map[string]any{"from": from, "to": to, "on_verdict": onVerdict, "max_iterations": maxIterations})
	}
	return result, nil
}

func safeArtifactPath(artifactPath, root, fieldPath string) error {
	safe, err := SafeRelativePath(artifactPath, fieldPath)
	if err != nil {
		return err
	}
	rootSafe, err := SafeRelativePath(root, fieldPath)
	if err != nil {
		return err
	}
	prefix := strings.TrimSuffix(rootSafe, "/") + "/"
	if safe != rootSafe && !strings.HasPrefix(safe, prefix) {
		return genErr("derived artifact path escapes artifact root", fieldPath)
	}
	return nil
}

func assertAcyclic(ids map[string]struct{}, edges []map[string]any) error {
	incoming := map[string]int{}
	outgoing := map[string][]string{}
	for id := range ids {
		incoming[id] = 0
	}
	for _, edge := range edges {
		from := fmt.Sprint(edge["from"])
		to := fmt.Sprint(edge["to"])
		outgoing[from] = append(outgoing[from], to)
		incoming[to]++
	}
	ready := []string{}
	for id, count := range incoming {
		if count == 0 {
			ready = append(ready, id)
		}
	}
	visited := 0
	for len(ready) > 0 {
		id := ready[len(ready)-1]
		ready = ready[:len(ready)-1]
		visited++
		for _, downstream := range outgoing[id] {
			incoming[downstream]--
			if incoming[downstream] == 0 {
				ready = append(ready, downstream)
			}
		}
	}
	if visited != len(ids) {
		return genErr("custom plan base edges contain a cycle", "spec.plan.edges")
	}
	return nil
}

func graphData(jobs, edges, cycles []map[string]any) map[string]any {
	nodes := []map[string]any{}
	for _, job := range jobs {
		nodes = append(nodes, map[string]any{"id": job["id"], "label": job["title"], "type": job["type"]})
	}
	return map[string]any{"nodes": nodes, "edges": edges, "cycles": cycles}
}

func lintWorkflow(workflow map[string]any) map[string]any {
	lint, err := workflowauthoring.Lint(workflow)
	if err != nil {
		return map[string]any{
			"valid":         false,
			"errors":        []map[string]any{{"message": err.Error()}},
			"warnings":      []map[string]any{},
			"warning_count": 0,
			"coverage":      map[string]any{"level": "weak", "score": 0, "max_score": 0},
		}
	}
	return lint
}

func isReviewKind(kind string) bool {
	return kind == "review" || kind == "evidence_audit" || kind == "final_review"
}

func hasModifier(spec Spec, modifier string) bool {
	for _, item := range spec.LaneModifiers {
		if item == modifier {
			return true
		}
	}
	return false
}

func lanesFrom(value any, laneSet string) (map[string]map[string]any, error) {
	if value == nil {
		if laneSet == "local" {
			return map[string]map[string]any{}, nil
		}
		return nil, genErr("lanes are required for this lane_set", "spec.lanes")
	}
	raw, err := object(value, "spec.lanes")
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]any{}
	for key, body := range raw {
		obj, err := object(body, "spec.lanes."+key)
		if err != nil {
			return nil, err
		}
		result[key] = obj
	}
	return result, nil
}

func genErr(message, fieldPath string) error {
	return &Error{Message: message, FieldPath: fieldPath}
}

func requiredString(raw map[string]any, key, prefix string) (string, error) {
	value, ok := raw[key].(string)
	if !ok || value == "" {
		return "", genErr(key+" must be a non-empty string", prefix+"."+key)
	}
	return value, nil
}

func choice(raw map[string]any, key string, choices map[string]struct{}, prefix string) (string, error) {
	value, err := requiredString(raw, key, prefix)
	if err != nil {
		return "", err
	}
	if _, ok := choices[value]; !ok {
		return "", genErr(fmt.Sprintf("%s must be one of %v", key, sortedKeys(choices)), prefix+"."+key)
	}
	return value, nil
}

func object(value any, fieldPath string) (map[string]any, error) {
	if obj, ok := value.(map[string]any); ok {
		return cloneMap(obj), nil
	}
	return nil, genErr("value must be an object", fieldPath)
}

func objectList(value any, fieldPath string) ([]map[string]any, error) {
	list, ok := value.([]any)
	if !ok {
		return nil, genErr("value must be a list of objects", fieldPath)
	}
	result := []map[string]any{}
	for idx, item := range list {
		obj, ok := item.(map[string]any)
		if !ok {
			return nil, genErr("value must be a list of objects", fmt.Sprintf("%s[%d]", fieldPath, idx))
		}
		result = append(result, cloneMap(obj))
	}
	return result, nil
}

func stringList(value any, fieldPath string) ([]string, error) {
	switch typed := value.(type) {
	case []string:
		for idx, item := range typed {
			if item == "" {
				return nil, genErr("value must be a list of non-empty strings", fmt.Sprintf("%s[%d]", fieldPath, idx))
			}
		}
		return append([]string(nil), typed...), nil
	case []any:
		result := []string{}
		for idx, item := range typed {
			text, ok := item.(string)
			if !ok || text == "" {
				return nil, genErr("value must be a list of non-empty strings", fmt.Sprintf("%s[%d]", fieldPath, idx))
			}
			result = append(result, text)
		}
		return result, nil
	default:
		return nil, genErr("value must be a list of non-empty strings", fieldPath)
	}
}

func mustString(value any) string {
	text, _ := value.(string)
	return text
}

func defaultAny(value, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}

func intFrom(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case json.Number:
		i, err := typed.Int64()
		if err == nil {
			return int(i), true
		}
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func boolFrom(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(typed))
		if err == nil {
			return parsed, true
		}
	}
	return false, false
}

// SupportedShapes returns the sorted shape ids `workflow generate` can produce.
// The template catalog must not advertise any other shape as generatable (#111);
// the reconcile test enforces catalog ↔ generator agreement.
func SupportedShapes() []string {
	return sortedKeys(shapes)
}

// IsSupportedShape reports whether the generator can produce the given shape.
func IsSupportedShape(shape string) bool {
	_, ok := shapes[shape]
	return ok
}

// exampleOnlyShapeHint returns a clear message when the requested shape is a
// catalog template marked example-only (generatable: false) rather than a
// generated shape, pointing the operator at its example fixture instead of the
// generic "must be one of" list (#111). Empty when the shape is not a known
// example-only template.
func exampleOnlyShapeHint(rawShape string) string {
	catalog, err := workflowtemplates.Load()
	if err != nil {
		return ""
	}
	entry, err := catalog.Get(rawShape)
	if err != nil {
		return ""
	}
	generatable, ok := entry["generatable"].(bool)
	if !ok || generatable {
		return ""
	}
	msg := fmt.Sprintf("shape %q is an example-only template, not a generated shape", rawShape)
	if path, _ := entry["example_workflow_path"].(string); path != "" {
		msg += fmt.Sprintf("; copy and adapt its example workflow at %s", path)
	}
	msg += fmt.Sprintf(", or pick a generated shape: %v", SupportedShapes())
	return msg
}

func set(values ...string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func sortedKeys(values map[string]struct{}) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func stringSet(values []string) map[string]struct{} {
	result := map[string]struct{}{}
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func uniqueSortedStrings(values []string) []string {
	seen := map[string]struct{}{}
	for _, value := range values {
		seen[value] = struct{}{}
	}
	return sortedKeys(seen)
}

func sortedMapKeys(values map[string]any) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func titleFromBlockID(blockID string) string {
	words := strings.Fields(strings.ReplaceAll(blockID, "_", " "))
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func cloneMap(input map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range input {
		result[key] = value
	}
	return result
}

func cloneNested(input map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range input {
		if obj, ok := value.(map[string]any); ok {
			result[key] = cloneMap(obj)
		} else {
			result[key] = value
		}
	}
	return result
}

func mapFrom(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{}
}

func listFrom(value any) []any {
	if value == nil {
		return []any{}
	}
	switch typed := value.(type) {
	case []any:
		return typed
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	default:
		return []any{}
	}
}

var titleWordRE = regexp.MustCompile(`\b\w`)

func title(value string) string {
	return titleWordRE.ReplaceAllStringFunc(value, strings.ToUpper)
}
