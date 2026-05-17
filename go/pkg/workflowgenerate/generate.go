package workflowgenerate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/halbritt/striatum/go/pkg/workflowtemplates"
)

const (
	GeneratorSchemaVersion = "striatum.workflow_generator.v1"
	PlanSchemaVersion      = "striatum.workflow_plan.v1"
	WorkflowSchemaVersion  = "striatum.workflow.v1"
)

var (
	shapes = set(
		"minimal", "review", "code_change", "human_checkpoint",
		"evidence_backed", "multi_review_synthesis", "custom",
	)
	laneSets      = set("local", "single_agent", "author_reviewer", "multi_review", "custom")
	laneModifiers = set("supervised", "worktree_isolated", "constrained", "harness_profiled")
	optionKeys    = set(
		"review_postures", "max_revision_cycles", "include_support_ledger",
		"constraints", "required_enforcement", "harness_profiles",
		"reviewer_count", "custom_job_artifacts", "supervision_compatible",
	)
	blockKinds = set(
		"draft", "review", "synthesis", "implementation", "test",
		"human_checkpoint", "support_ledger", "evidence_audit", "final_review",
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
		return Spec{}, err
	}
	if shape == "multi_phase" {
		return Spec{}, &Error{Message: "multi_phase workflow generation is not yet ported to the Go daemon; refusing rather than producing a partial rewrite", FieldPath: "spec.shape"}
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
	if spec.Shape == "custom" {
		jobs, edges, cycles, err = compileCustom(spec, lanes)
	} else {
		jobs, edges, cycles, err = compileShape(spec)
	}
	if err != nil {
		return Generated{}, err
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
	workflow := map[string]any{
		"schema_version":   WorkflowSchemaVersion,
		"workflow_id":      spec.WorkflowID,
		"workflow_version": spec.WorkflowVer,
		"name":             spec.Name,
		"branch":           cloneMap(spec.Branch),
		"coordinator":      coordinator(lanes),
		"lanes":            lanes,
		"roles":            roles,
		"context_docs":     append([]any(nil), spec.ContextDocs...),
		"parallelism":      parallelism,
		"jobs":             jobs,
		"edges":            edges,
		"cycles":           cycles,
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
	return Generated{
		Workflow: workflow,
		Files:    files,
		Metadata: map[string]any{
			"shape":             spec.Shape,
			"lane_set":          spec.LaneSet,
			"lane_modifiers":    append([]string(nil), spec.LaneModifiers...),
			"graph":             graph,
			"catalog_templates": []string{spec.Shape, spec.LaneSet},
			"scaffold_root":     spec.ScaffoldRoot,
			"workflow_path":     spec.ScaffoldRoot + "/workflow.json",
		},
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

func compileShape(spec Spec) ([]map[string]any, []map[string]any, []map[string]any, error) {
	base := spec.ArtifactRoot
	authorLane := authorLane(spec)
	reviewerLaneID := reviewerLane(spec, 1)
	switch spec.Shape {
	case "minimal":
		return []map[string]any{job("draft", "draft", "Draft starter artifact", "author", authorLane, base, "DRAFT.md", "handoff", "draft", "draft", "Produce the starter artifact for this workflow.")}, nil, nil, nil
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
				return nil, nil, nil, err
			}
			cycles = append(cycles, map[string]any{"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": max})
		}
		return jobs, edges, cycles, nil
	case "human_checkpoint":
		jobs := []map[string]any{
			job("analysis", "draft", "Analyze the requested decision", "author", authorLane, base, "ANALYSIS.md", "handoff", "analysis", "draft", ""),
			job("checkpoint", "human_checkpoint", "Open a human checkpoint", "reviewer", reviewerLaneID, base, "CHECKPOINT.md", "handoff", "checkpoint", "review", ""),
			job("apply", "synthesis", "Apply the owner decision", "author", authorLane, base, "SUMMARY.md", "synthesis", "summary", "apply", ""),
		}
		return jobs, []map[string]any{{"from": "analysis", "to": "checkpoint", "on": "completed"}, {"from": "checkpoint", "to": "apply", "on": "completed"}}, nil, nil
	case "evidence_backed":
		jobs := []map[string]any{
			job("draft", "draft", "Draft evidence-backed artifact", "author", authorLane, base, "DRAFT.md", "handoff", "draft", "draft", ""),
			job("support_ledger", "build", "Map claims to evidence", "author", authorLane, base+"/support", "SUPPORT_LEDGER.md", "support_ledger", "support_ledger", "support_ledger", ""),
			reviewJob("evidence_audit", reviewerLaneID, base+"/audit/EVIDENCE_AUDIT.md", "devils_advocate"),
			reviewJob("final_review", reviewerLaneID, base+"/final/FINAL_REVIEW.md", firstPosture(spec)),
		}
		return jobs, []map[string]any{{"from": "draft", "to": "support_ledger", "on": "completed"}, {"from": "support_ledger", "to": "evidence_audit", "on": "completed"}, {"from": "evidence_audit", "to": "final_review", "on": "completed"}}, nil, nil
	case "multi_review_synthesis":
		count := reviewerCount(spec)
		postures, err := postures(spec, count)
		if err != nil {
			return nil, nil, nil, err
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
		return jobs, edges, nil, nil
	default:
		return nil, nil, nil, genErr("unknown workflow shape", "spec.shape")
	}
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
	root := path.Dir(artifactPath)
	filename := path.Base(artifactPath)
	result := job(id, "review", "Review the draft", "reviewer", lane, root, filename, "finding", "review", "review", "Review the draft and record a finding.")
	result["fresh_session_required"] = true
	result["write_scope"] = map[string]any{"mode": "review_only_artifact", "repo_write": false, "allowed_paths": []string{root + "/"}, "forbidden_paths": []string{".striatum/"}}
	if posture != "neutral" {
		result["review_posture"] = posture
	}
	return result
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
	if workflow["schema_version"] != WorkflowSchemaVersion && workflow["schema_version"] != "striatum.workflow.v1.1" {
		return genErr("workflow has unsupported schema_version", "workflow.schema_version")
	}
	for _, key := range []string{"workflow_id", "workflow_version", "name"} {
		if value, ok := workflow[key].(string); !ok || value == "" {
			return genErr("workflow missing required field "+key, "workflow."+key)
		}
	}
	lanes := mapFrom(workflow["lanes"])
	roles := mapFrom(workflow["roles"])
	if len(lanes) == 0 {
		return genErr("workflow must declare at least one lane", "workflow.lanes")
	}
	jobs := listFrom(workflow["jobs"])
	if len(jobs) == 0 {
		return genErr("workflow must declare at least one job", "workflow.jobs")
	}
	jobIDs := map[string]struct{}{}
	reviewIDs := map[string]struct{}{}
	for idx, item := range jobs {
		job := mapFrom(item)
		jobID, _ := job["id"].(string)
		if jobID == "" {
			return genErr("workflow job is missing id", fmt.Sprintf("workflow.jobs[%d].id", idx))
		}
		if _, ok := jobIDs[jobID]; ok {
			return genErr("duplicate workflow job id", fmt.Sprintf("workflow.jobs[%d].id", idx))
		}
		jobIDs[jobID] = struct{}{}
		if job["type"] == "review" {
			reviewIDs[jobID] = struct{}{}
		}
		if role, _ := job["role_id"].(string); role == "" || roles[role] == nil {
			return genErr("job references missing role", fmt.Sprintf("workflow.jobs[%d].role_id", idx))
		}
		if lane, _ := job["lane_id"].(string); lane != "" && lanes[lane] == nil {
			return genErr("job references missing lane", fmt.Sprintf("workflow.jobs[%d].lane_id", idx))
		}
		for artIdx, art := range listFrom(job["expected_artifacts"]) {
			artifact := mapFrom(art)
			p, _ := artifact["path"].(string)
			if _, err := SafeRelativePath(p, fmt.Sprintf("workflow.jobs[%d].expected_artifacts[%d].path", idx, artIdx)); err != nil {
				return err
			}
		}
		scope := mapFrom(job["write_scope"])
		for _, ap := range stringListFrom(scope["allowed_paths"]) {
			if _, err := SafeRelativePath(strings.TrimSuffix(ap, "/"), fmt.Sprintf("workflow.jobs[%d].write_scope.allowed_paths", idx)); err != nil {
				return err
			}
		}
	}
	for idx, item := range listFrom(workflow["edges"]) {
		edge := mapFrom(item)
		if _, ok := jobIDs[fmt.Sprint(edge["from"])]; !ok {
			return genErr("edge references missing job", fmt.Sprintf("workflow.edges[%d].from", idx))
		}
		if _, ok := jobIDs[fmt.Sprint(edge["to"])]; !ok {
			return genErr("edge references missing job", fmt.Sprintf("workflow.edges[%d].to", idx))
		}
	}
	for idx, item := range listFrom(workflow["cycles"]) {
		cycle := mapFrom(item)
		from := fmt.Sprint(cycle["from"])
		if _, ok := reviewIDs[from]; !ok {
			return genErr("cycle source must be a review job", fmt.Sprintf("workflow.cycles[%d].from", idx))
		}
		if _, ok := jobIDs[fmt.Sprint(cycle["to"])]; !ok {
			return genErr("cycle references missing job", fmt.Sprintf("workflow.cycles[%d].to", idx))
		}
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
	default:
		return fmt.Sprintf("Complete the %s step declared by the workflow.\n", strings.ReplaceAll(strings.TrimSuffix(prompt, ".md"), "_", " "))
	}
}

func validateModifierMatrix(spec Spec, warnings *[]string) error {
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
	return set("generic", "codex", "claude_code", "gemini_cli")
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

func coordinator(lanes map[string]any) map[string]any {
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
	if spec.Shape == "multi_review_synthesis" {
		maxJobs = reviewerCount(spec)
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
		title = strings.Title(strings.ReplaceAll(blockID, "_", " "))
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
	warnings := []string{}
	lanes := mapFrom(workflow["lanes"])
	authorModel := ""
	reviewerModel := ""
	if lane, ok := mapFrom(lanes["author"])["display_model"].(string); ok {
		authorModel = lane
	}
	if lane, ok := mapFrom(lanes["reviewer"])["display_model"].(string); ok {
		reviewerModel = lane
	}
	if authorModel != "" && authorModel == reviewerModel {
		warnings = append(warnings, "author and reviewer lanes use the same display_model")
	}
	if lanes["local"] != nil {
		warnings = append(warnings, "local fixture lane is suitable for tests and operator-by-hand starts only")
	}
	level := "adequate"
	if len(warnings) > 0 {
		level = "weak"
	}
	return map[string]any{
		"valid":         true,
		"warnings":      warnings,
		"warning_count": len(warnings),
		"coverage":      map[string]any{"level": level, "score": 1, "max_score": 3},
	}
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
	}
	return 0, false
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

func sortedMapKeys(values map[string]any) []string {
	keys := []string{}
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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

func stringListFrom(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		result := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return nil
	}
}

var titleWordRE = regexp.MustCompile(`\b\w`)

func title(value string) string {
	return titleWordRE.ReplaceAllStringFunc(value, strings.ToUpper)
}
