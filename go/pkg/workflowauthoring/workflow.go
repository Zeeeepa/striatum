package workflowauthoring

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	SchemaV1   = "striatum.workflow.v1"
	SchemaV11  = "striatum.workflow.v1.1"
	defaultFmt = "mermaid"
)

var requiredTopLevel = []string{
	"schema_version",
	"workflow_id",
	"workflow_version",
	"name",
	"branch",
	"coordinator",
	"lanes",
	"roles",
	"context_docs",
	"parallelism",
	"jobs",
	"edges",
	"cycles",
}

var allowedArtifactKinds = map[string]bool{
	"prompt": true, "finding": true, "findings_ledger": true, "synthesis": true,
	"marker": true, "handoff": true, "decision": true, "patch_summary": true,
	"test_report": true, "other": true, "support_ledger": true,
	"action_item_ledger": true, "harness_improvement_proposal": true,
	"escalation": true,
}

var verdictJobTypes = map[string]bool{"review": true, "phase_synthesis": true}

type Error struct {
	Message   string
	FieldPath string
}

func (e *Error) Error() string { return e.Message }

func errf(format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

func fieldErr(fieldPath string, format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), FieldPath: fieldPath}
}

func ResolveWorkflowPath(repoRoot string, workflowPath string) (string, string, error) {
	if strings.TrimSpace(workflowPath) == "" {
		return "", "", &Error{Message: "workflow_path must be a non-empty string", FieldPath: "workflow_path"}
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", err
	}
	root = filepath.Clean(root)
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}
	candidate := workflowPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", "", err
	}
	candidate = filepath.Clean(candidate)
	if !pathWithin(candidate, root) {
		return "", "", &Error{Message: "workflow path must stay inside the repository"}
	}
	if realCandidate, err := filepath.EvalSymlinks(candidate); err == nil {
		candidate = realCandidate
		if !pathWithin(candidate, root) {
			return "", "", &Error{Message: "workflow path must stay inside the repository"}
		}
	}
	rel, err := filepath.Rel(root, candidate)
	if err != nil || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." {
		return "", "", &Error{Message: "workflow path must stay inside the repository"}
	}
	return candidate, filepath.ToSlash(rel), nil
}

func LoadFile(repoRoot string, workflowPath string) (map[string]any, string, error) {
	path, sourcePath, err := ResolveWorkflowPath(repoRoot, workflowPath)
	if err != nil {
		return nil, "", err
	}
	workflow, err := Load(path)
	if err != nil {
		return nil, "", err
	}
	return workflow, sourcePath, nil
}

func Load(path string) (map[string]any, error) {
	suffix := strings.ToLower(filepath.Ext(path))
	if suffix == ".yaml" || suffix == ".yml" {
		return nil, &Error{Message: "workflow config must be JSON, not YAML"}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, &Error{Message: "read workflow: " + err.Error()}
	}
	if len(bytes.TrimLeft(raw, " \t\r\n")) == 0 || bytes.TrimLeft(raw, " \t\r\n")[0] != '{' {
		return nil, &Error{Message: "workflow config must be a JSON object"}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var workflow map[string]any
	if err := dec.Decode(&workflow); err != nil {
		return nil, &Error{Message: "workflow JSON is invalid: " + jsonErrorMessage(err)}
	}
	if workflow == nil {
		return nil, &Error{Message: "workflow config must be a JSON object"}
	}
	if err := Validate(workflow); err != nil {
		return nil, err
	}
	return workflow, nil
}

func Validate(workflow map[string]any) error {
	missing := []string{}
	for _, key := range requiredTopLevel {
		if _, ok := workflow[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return errf("workflow is missing required fields: %s", strings.Join(missing, ", "))
	}
	schema, _ := workflow["schema_version"].(string)
	if schema != SchemaV1 && schema != SchemaV11 {
		return fieldErr("schema_version", "workflow schema_version must be one of: %s, %s", SchemaV1, SchemaV11)
	}
	if _, err := object(workflow, "branch"); err != nil {
		return err
	}
	lanes, err := object(workflow, "lanes")
	if err != nil {
		return err
	}
	roles, err := object(workflow, "roles")
	if err != nil {
		return err
	}
	if _, err := list(workflow, "context_docs"); err != nil {
		return err
	}
	if _, err := object(workflow, "parallelism"); err != nil {
		return err
	}
	jobs, err := list(workflow, "jobs")
	if err != nil {
		return err
	}
	if _, err := list(workflow, "edges"); err != nil {
		return err
	}
	if _, err := list(workflow, "cycles"); err != nil {
		return err
	}
	jobMap := map[string]map[string]any{}
	for index, item := range jobs {
		job, ok := item.(map[string]any)
		if !ok {
			return fieldErr(fmt.Sprintf("jobs[%d]", index), "each job must be an object")
		}
		jobID, err := stringField(job, "id")
		if err != nil {
			return err
		}
		if _, exists := jobMap[jobID]; exists {
			return fieldErr(fmt.Sprintf("jobs[%d].id", index), "duplicate job id %q", jobID)
		}
		jobMap[jobID] = job
		roleID, err := stringField(job, "role_id")
		if err != nil {
			return err
		}
		if _, ok := roles[roleID]; !ok {
			return fieldErr(fmt.Sprintf("jobs[%d].role_id", index), "job %q references unknown role %q", jobID, roleID)
		}
		if laneID, ok := job["lane_id"].(string); ok && laneID != "" {
			if _, ok := lanes[laneID]; !ok {
				return fieldErr(fmt.Sprintf("jobs[%d].lane_id", index), "job %q references unknown lane %q", jobID, laneID)
			}
		}
		if err := validateJobPaths(index, jobID, job); err != nil {
			return err
		}
	}
	if err := validateEdges(workflow, jobMap); err != nil {
		return err
	}
	if err := validateCycles(workflow, jobMap); err != nil {
		return err
	}
	if err := validateArtifactUniqueness(jobs); err != nil {
		return err
	}
	if err := validateCycleTargets(workflow, jobMap); err != nil {
		return err
	}
	return nil
}

func Plan(workflow map[string]any) (map[string]any, error) {
	if err := Validate(workflow); err != nil {
		return nil, err
	}
	jobs := WorkflowJobMap(workflow)
	edges, err := EdgeDependencyPairs(workflow)
	if err != nil {
		return nil, err
	}
	downstream := map[string][]string{}
	indegree := map[string]int{}
	for jobID := range jobs {
		downstream[jobID] = []string{}
		indegree[jobID] = 0
	}
	for _, edge := range edges {
		fromID := stringValue(edge["from"])
		toID := stringValue(edge["to"])
		downstream[fromID] = append(downstream[fromID], toID)
		indegree[toID]++
	}
	ready := []string{}
	for jobID, count := range indegree {
		if count == 0 {
			ready = append(ready, jobID)
		}
	}
	sort.Strings(ready)
	visited := map[string]bool{}
	claimOrder := []map[string]any{}
	step := 1
	for len(ready) > 0 {
		wave := append([]string{}, ready...)
		ready = []string{}
		claimable := []map[string]any{}
		for _, jobID := range wave {
			claimable = append(claimable, PlannedJob(jobs[jobID]))
		}
		claimOrder = append(claimOrder, map[string]any{"step": step, "claimable": claimable})
		step++
		for _, jobID := range wave {
			visited[jobID] = true
			sort.Strings(downstream[jobID])
			for _, downstreamID := range downstream[jobID] {
				indegree[downstreamID]--
				if indegree[downstreamID] == 0 {
					ready = append(ready, downstreamID)
				}
			}
		}
		sort.Strings(ready)
	}
	if len(visited) != len(jobs) {
		remaining := []string{}
		for jobID := range jobs {
			if !visited[jobID] {
				remaining = append(remaining, jobID)
			}
		}
		sort.Strings(remaining)
		return nil, errf("workflow edges contain a dependency cycle involving: %s", strings.Join(remaining, ", "))
	}
	cycles := PlannedCycles(workflow)
	graph, err := GraphData(workflow)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"workflow_id":      workflow["workflow_id"],
		"workflow_version": workflow["workflow_version"],
		"valid":            true,
		"summary": map[string]any{
			"jobs":        len(jobs),
			"edges":       len(edges),
			"cycles":      len(cycles),
			"claim_steps": len(claimOrder),
		},
		"claim_order":  claimOrder,
		"review_gates": PlannedReviewGates(workflow, jobs, edges),
		"cycles":       cycles,
		"graph":        graph["graph"],
	}, nil
}

func GraphData(workflow map[string]any) (map[string]any, error) {
	if err := Validate(workflow); err != nil {
		return nil, err
	}
	jobs := WorkflowJobMap(workflow)
	edges, err := EdgeDependencyPairs(workflow)
	if err != nil {
		return nil, err
	}
	plannedEdges := []map[string]any{}
	for _, edge := range edges {
		plannedEdges = append(plannedEdges, PlannedEdge(jobs, stringValue(edge["from"]), stringValue(edge["to"])))
	}
	nodes := []map[string]any{}
	for _, item := range anySlice(workflow["jobs"]) {
		nodes = append(nodes, PlannedJob(item.(map[string]any)))
	}
	return map[string]any{
		"workflow_id":      workflow["workflow_id"],
		"workflow_version": workflow["workflow_version"],
		"graph": map[string]any{
			"nodes":  nodes,
			"edges":  plannedEdges,
			"cycles": PlannedCycles(workflow),
		},
	}, nil
}

func Graph(workflow map[string]any, format string) (map[string]any, error) {
	if format == "" {
		format = defaultFmt
	}
	data, err := GraphData(workflow)
	if err != nil {
		return nil, err
	}
	if format == "json" {
		return data, nil
	}
	graph := data["graph"].(map[string]any)
	switch format {
	case "mermaid":
		return map[string]any{"format": "mermaid", "source": Mermaid(graph)}, nil
	case "dot":
		return map[string]any{"format": "dot", "source": DOT(graph)}, nil
	default:
		return nil, &Error{Message: fmt.Sprintf("unknown format: %q (valid options: dot, json, mermaid)", format), FieldPath: "format"}
	}
}

func WorkflowJobMap(workflow map[string]any) map[string]map[string]any {
	result := map[string]map[string]any{}
	for _, item := range anySlice(workflow["jobs"]) {
		job := item.(map[string]any)
		result[stringValue(job["id"])] = job
	}
	return result
}

func EdgeDependencyPairs(workflow map[string]any) ([]map[string]any, error) {
	jobs := WorkflowJobMap(workflow)
	pairs := []map[string]any{}
	seen := map[string]bool{}
	for _, item := range anySlice(workflow["edges"]) {
		edge, ok := item.(map[string]any)
		if !ok {
			return nil, &Error{Message: "each edge must be an object"}
		}
		fromID, err := stringField(edge, "from")
		if err != nil {
			return nil, err
		}
		toID, err := stringField(edge, "to")
		if err != nil {
			return nil, err
		}
		if jobs[fromID] == nil || jobs[toID] == nil {
			return nil, &Error{Message: "workflow edge references an unknown job"}
		}
		if edge["on"] != "completed" {
			return nil, &Error{Message: "workflow edges must use on completed"}
		}
		key := fromID + "\x00" + toID
		if seen[key] {
			continue
		}
		seen[key] = true
		pairs = append(pairs, map[string]any{"on": "completed", "from": fromID, "to": toID})
	}
	return pairs, nil
}

func PlannedJob(job map[string]any) map[string]any {
	artifacts := []map[string]any{}
	for _, item := range anySlice(job["expected_artifacts"]) {
		if artifact, ok := item.(map[string]any); ok {
			artifacts = append(artifacts, map[string]any{
				"logical_name": artifact["logical_name"],
				"kind":         artifact["kind"],
				"path":         artifact["path"],
				"required":     artifact["required"] == true,
			})
		}
	}
	return map[string]any{
		"job_id":                 stringValue(job["id"]),
		"type":                   defaultString(job["type"], "generic"),
		"role_id":                stringValue(job["role_id"]),
		"lane_id":                nullableString(job["lane_id"]),
		"parallel_group":         nullableString(job["parallel_group"]),
		"fresh_session_required": job["fresh_session_required"] == true,
		"write_scope_mode":       writeScopeMode(job),
		"expected_artifacts":     artifacts,
	}
}

func PlannedEdge(jobs map[string]map[string]any, fromID string, toID string) map[string]any {
	gate := map[string]any{"on": "completed"}
	if verdictJobTypes[defaultString(jobs[fromID]["type"], "generic")] {
		gate["requires_verdict"] = []string{"accept", "accept_with_findings"}
	}
	return map[string]any{"from": fromID, "to": toID, "gate": gate}
}

func PlannedCycles(workflow map[string]any) []map[string]any {
	cycles := []map[string]any{}
	for _, item := range anySlice(workflow["cycles"]) {
		cycle := item.(map[string]any)
		cycles = append(cycles, map[string]any{
			"from":           cycle["from"],
			"to":             cycle["to"],
			"on_verdict":     cycle["on_verdict"],
			"max_iterations": cycle["max_iterations"],
		})
	}
	return cycles
}

func PlannedReviewGates(workflow map[string]any, jobs map[string]map[string]any, edges []map[string]any) []map[string]any {
	cycleByReview := map[string]map[string]any{}
	for _, cycle := range PlannedCycles(workflow) {
		cycleByReview[stringValue(cycle["from"])] = cycle
	}
	downstreamByReview := map[string][]string{}
	for _, edge := range edges {
		fromID := stringValue(edge["from"])
		if verdictJobTypes[defaultString(jobs[fromID]["type"], "generic")] {
			downstreamByReview[fromID] = append(downstreamByReview[fromID], stringValue(edge["to"]))
		}
	}
	rootPolicy := ""
	if policy, ok := workflow["review_revision_policy"].(map[string]any); ok {
		rootPolicy = stringValue(policy["root_review_needs_revision"])
	}
	gates := []map[string]any{}
	for jobID, job := range jobs {
		if defaultString(job["type"], "generic") != "review" {
			continue
		}
		var needsRevision map[string]any
		if cycle := cycleByReview[jobID]; cycle != nil {
			needsRevision = map[string]any{
				"action":         "cycle",
				"to":             cycle["to"],
				"max_iterations": cycle["max_iterations"],
			}
		} else if rootPolicy == "human_checkpoint" {
			needsRevision = map[string]any{"action": "human_checkpoint"}
		} else {
			needsRevision = map[string]any{"action": "no_declared_route"}
		}
		downstream := downstreamByReview[jobID]
		sort.Strings(downstream)
		gates = append(gates, map[string]any{
			"review_job_id":      jobID,
			"downstream_jobs":    downstream,
			"accepting_verdicts": []string{"accept", "accept_with_findings"},
			"needs_revision":     needsRevision,
			"reject":             map[string]any{"action": "fail_review"},
		})
	}
	sort.Slice(gates, func(i, j int) bool {
		return stringValue(gates[i]["review_job_id"]) < stringValue(gates[j]["review_job_id"])
	})
	return gates
}

func Mermaid(graph map[string]any) string {
	nodes := typedMaps(graph["nodes"])
	edges := typedMaps(graph["edges"])
	cycles := typedMaps(graph["cycles"])
	nodeNames := nodeNames(nodes)
	parallel := map[string][]map[string]any{}
	ungrouped := []map[string]any{}
	for _, node := range nodes {
		group := stringValue(node["parallel_group"])
		if group == "" {
			ungrouped = append(ungrouped, node)
		} else {
			parallel[group] = append(parallel[group], node)
		}
	}
	lines := []string{"flowchart TD"}
	for _, node := range ungrouped {
		lines = append(lines, mermaidNodeLine(node, nodeNames, "  "))
	}
	groups := sortedKeys(parallel)
	for index, groupID := range groups {
		lines = append(lines, fmt.Sprintf(`  subgraph pg%d["parallel: %s"]`, index, mermaidLabel(groupID)))
		sort.Slice(parallel[groupID], func(i, j int) bool {
			return stringValue(parallel[groupID][i]["job_id"]) < stringValue(parallel[groupID][j]["job_id"])
		})
		for _, node := range parallel[groupID] {
			lines = append(lines, mermaidNodeLine(node, nodeNames, "    "))
		}
		lines = append(lines, "  end")
	}
	for _, edge := range edges {
		label := "completed"
		if gate, ok := edge["gate"].(map[string]any); ok {
			if _, ok := gate["requires_verdict"]; ok {
				label = "accepted review"
			}
		}
		lines = append(lines, fmt.Sprintf("  %s -->|%s| %s", nodeNames[stringValue(edge["from"])], label, nodeNames[stringValue(edge["to"])]))
	}
	for _, cycle := range cycles {
		lines = append(lines, fmt.Sprintf("  %s -.->|needs_revision max %v| %s", nodeNames[stringValue(cycle["from"])], cycle["max_iterations"], nodeNames[stringValue(cycle["to"])]))
	}
	return strings.Join(lines, "\n") + "\n"
}

func DOT(graph map[string]any) string {
	nodes := typedMaps(graph["nodes"])
	edges := typedMaps(graph["edges"])
	cycles := typedMaps(graph["cycles"])
	nodeNames := nodeNames(nodes)
	parallel := map[string][]map[string]any{}
	ungrouped := []map[string]any{}
	for _, node := range nodes {
		group := stringValue(node["parallel_group"])
		if group == "" {
			ungrouped = append(ungrouped, node)
		} else {
			parallel[group] = append(parallel[group], node)
		}
	}
	lines := []string{"digraph striatum_workflow {", "  rankdir=TB;", `  node [shape=box, fontname="Helvetica"];`}
	for _, node := range ungrouped {
		lines = append(lines, dotNodeLine(node, nodeNames, "  "))
	}
	groups := sortedKeys(parallel)
	for index, groupID := range groups {
		lines = append(lines, fmt.Sprintf("  subgraph %s {", dotClusterID(groupID, index)))
		lines = append(lines, fmt.Sprintf(`    label="parallel: %s";`, dotLabel(groupID)))
		sort.Slice(parallel[groupID], func(i, j int) bool {
			return stringValue(parallel[groupID][i]["job_id"]) < stringValue(parallel[groupID][j]["job_id"])
		})
		for _, node := range parallel[groupID] {
			lines = append(lines, dotNodeLine(node, nodeNames, "    "))
		}
		lines = append(lines, "  }")
	}
	for _, edge := range edges {
		label := "completed"
		if gate, ok := edge["gate"].(map[string]any); ok {
			if _, ok := gate["requires_verdict"]; ok {
				label = "accepted review"
			}
		}
		lines = append(lines, fmt.Sprintf(`  %s -> %s [label="%s"];`, nodeNames[stringValue(edge["from"])], nodeNames[stringValue(edge["to"])], dotLabel(label)))
	}
	for _, cycle := range cycles {
		lines = append(lines, fmt.Sprintf(`  %s -> %s [style=dashed, label="%s"];`, nodeNames[stringValue(cycle["from"])], nodeNames[stringValue(cycle["to"])], dotLabel(fmt.Sprintf("needs_revision max %v", cycle["max_iterations"]))))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n") + "\n"
}

func validateJobPaths(jobIndex int, jobID string, job map[string]any) error {
	if scope, ok := job["write_scope"].(map[string]any); ok {
		allowed := stringsFromSlice(scope["allowed_paths"])
		forbidden := stringsFromSlice(scope["forbidden_paths"])
		for _, allowedPath := range allowed {
			if repoPathInvalid(allowedPath) {
				return errf("job %q has invalid write_scope allowed_path", jobID)
			}
			for _, forbiddenPath := range forbidden {
				if repoPathInvalid(forbiddenPath) {
					return errf("job %q has invalid write_scope forbidden_path", jobID)
				}
				if repoPathWithin(allowedPath, forbiddenPath) {
					return errf("job %q write_scope allowed_path %q is inside forbidden_path %q", jobID, allowedPath, forbiddenPath)
				}
			}
		}
	}
	for artifactIndex, item := range anySlice(job["expected_artifacts"]) {
		artifact, ok := item.(map[string]any)
		if !ok {
			return errf("job %q expected artifact must be an object", jobID)
		}
		path := stringValue(artifact["path"])
		if path == "" || repoPathInvalid(path) {
			return fieldErr(fmt.Sprintf("jobs[%d].expected_artifacts[%d].path", jobIndex, artifactIndex), "job %q has invalid artifact path", jobID)
		}
		if kind := stringValue(artifact["kind"]); kind != "" && !allowedArtifactKinds[kind] {
			return errf("job %s declares unknown artifact kind %s", jobID, kind)
		}
		if err := validateArtifactInWriteScope(jobID, job, path); err != nil {
			return err
		}
	}
	return nil
}

func validateArtifactInWriteScope(jobID string, job map[string]any, artifactPath string) error {
	scope, ok := job["write_scope"].(map[string]any)
	if !ok {
		return nil
	}
	allowed := stringsFromSlice(scope["allowed_paths"])
	forbidden := stringsFromSlice(scope["forbidden_paths"])
	if len(allowed) == 0 {
		return nil
	}
	for _, forbiddenPath := range forbidden {
		if repoPathWithin(artifactPath, forbiddenPath) {
			return errf("job %q expected artifact %q is inside forbidden_path %q", jobID, artifactPath, forbiddenPath)
		}
	}
	for _, allowedPath := range allowed {
		if repoPathWithin(artifactPath, allowedPath) {
			return nil
		}
	}
	return errf("job %q expected artifact %q is not inside any allowed_path", jobID, artifactPath)
}

func validateEdges(workflow map[string]any, jobMap map[string]map[string]any) error {
	seen := map[string]bool{}
	for _, item := range anySlice(workflow["edges"]) {
		edge, ok := item.(map[string]any)
		if !ok {
			return &Error{Message: "each edge must be an object"}
		}
		fromID, err := stringField(edge, "from")
		if err != nil {
			return err
		}
		toID, err := stringField(edge, "to")
		if err != nil {
			return err
		}
		if jobMap[fromID] == nil || jobMap[toID] == nil {
			return &Error{Message: "workflow edge references an unknown job"}
		}
		if edge["on"] != "completed" {
			return &Error{Message: "workflow edges must use on completed"}
		}
		seen[toID+"\x00"+fromID] = true
	}
	for jobID, job := range jobMap {
		needs, exists := job["needs"]
		if !exists {
			continue
		}
		declared := map[string]bool{}
		for _, item := range anySlice(needs) {
			dep, ok := item.(string)
			if !ok {
				return errf("job %q has non-string dependency", jobID)
			}
			declared[dep] = true
		}
		edgeNeeds := map[string]bool{}
		for key := range seen {
			parts := strings.Split(key, "\x00")
			if len(parts) == 2 && parts[0] == jobID {
				edgeNeeds[parts[1]] = true
			}
		}
		if !sameStringSet(declared, edgeNeeds) {
			return errf("job %q needs disagree with workflow edges", jobID)
		}
	}
	return nil
}

func validateCycles(workflow map[string]any, jobMap map[string]map[string]any) error {
	for index, item := range anySlice(workflow["cycles"]) {
		cycle, ok := item.(map[string]any)
		if !ok {
			return &Error{Message: "each cycle must be an object"}
		}
		fromID, err := stringField(cycle, "from")
		if err != nil {
			return err
		}
		toID, err := stringField(cycle, "to")
		if err != nil {
			return err
		}
		if jobMap[fromID] == nil || jobMap[toID] == nil {
			return fieldErr(fmt.Sprintf("cycles[%d].from", index), "workflow cycle references an unknown job")
		}
		if cycle["on_verdict"] != "needs_revision" {
			return &Error{Message: "workflow cycles must use on_verdict needs_revision"}
		}
		if !positiveWholeNumber(cycle["max_iterations"]) {
			return fieldErr(fmt.Sprintf("cycles[%d].max_iterations", index), "workflow cycles must declare max_iterations >= 1")
		}
	}
	return nil
}

func validateArtifactUniqueness(jobs []any) error {
	seen := map[string]string{}
	for _, item := range jobs {
		job, ok := item.(map[string]any)
		if !ok {
			continue
		}
		jobID := stringValue(job["id"])
		for _, artifactItem := range anySlice(job["expected_artifacts"]) {
			artifact, ok := artifactItem.(map[string]any)
			if !ok {
				continue
			}
			path := normalizeRepoPath(stringValue(artifact["path"]))
			if path == "" {
				continue
			}
			if previous, exists := seen[path]; exists && previous != jobID {
				return errf("jobs %q and %q both declare expected artifact path %q", previous, jobID, path)
			}
			seen[path] = jobID
		}
	}
	return nil
}

func validateCycleTargets(workflow map[string]any, jobMap map[string]map[string]any) error {
	edges, err := EdgeDependencyPairs(workflow)
	if err != nil {
		return err
	}
	pairs := [][2]string{}
	for _, edge := range edges {
		pairs = append(pairs, [2]string{stringValue(edge["from"]), stringValue(edge["to"])})
	}
	for _, item := range anySlice(workflow["cycles"]) {
		cycle, ok := item.(map[string]any)
		if !ok {
			continue
		}
		fromID := stringValue(cycle["from"])
		toID := stringValue(cycle["to"])
		if jobMap[fromID] == nil || jobMap[toID] == nil {
			continue
		}
		if !hasPath(pairs, toID, fromID) {
			return errf("workflow cycle from %q to %q is unsound: %q does not feed back into %q through workflow edges", fromID, toID, toID, fromID)
		}
	}
	return nil
}

func object(value map[string]any, key string) (map[string]any, error) {
	item, ok := value[key].(map[string]any)
	if !ok {
		return nil, errf("workflow field %q must be an object", key)
	}
	return item, nil
}

func list(value map[string]any, key string) ([]any, error) {
	item, ok := value[key].([]any)
	if !ok {
		return nil, errf("workflow field %q must be a list", key)
	}
	return item, nil
}

func stringField(value map[string]any, key string) (string, error) {
	item, ok := value[key].(string)
	if !ok || item == "" {
		return "", errf("workflow field %q must be a non-empty string", key)
	}
	return item, nil
}

func pathWithin(child string, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func repoPathInvalid(path string) bool {
	if path == "" || strings.HasPrefix(path, "/") {
		return true
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func repoPathWithin(child string, parent string) bool {
	childNorm := normalizeRepoPath(child)
	parentNorm := normalizeRepoPath(parent)
	if parentNorm == "" || childNorm == parentNorm {
		return true
	}
	return strings.HasPrefix(childNorm, parentNorm+"/")
}

func normalizeRepoPath(path string) string {
	parts := []string{}
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == "" || part == "." {
			continue
		}
		parts = append(parts, part)
	}
	return strings.TrimRight(strings.Join(parts, "/"), "/")
}

func hasPath(edges [][2]string, source string, target string) bool {
	stack := []string{source}
	seen := map[string]bool{}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == target {
			return true
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		for _, edge := range edges {
			if edge[0] == current {
				stack = append(stack, edge[1])
			}
		}
	}
	return false
}

func anySlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return []any{}
}

func typedMaps(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := []map[string]any{}
		for _, item := range items {
			if mapped, ok := item.(map[string]any); ok {
				out = append(out, mapped)
			}
		}
		return out
	default:
		return []map[string]any{}
	}
}

func stringsFromSlice(value any) []string {
	out := []string{}
	for _, item := range anySlice(value) {
		if text, ok := item.(string); ok {
			out = append(out, text)
		}
	}
	return out
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func defaultString(value any, fallback string) string {
	if text := stringValue(value); text != "" {
		return text
	}
	return fallback
}

func nullableString(value any) any {
	if text := stringValue(value); text != "" {
		return text
	}
	return nil
}

func writeScopeMode(job map[string]any) any {
	if scope, ok := job["write_scope"].(map[string]any); ok {
		if mode := stringValue(scope["mode"]); mode != "" {
			return mode
		}
	}
	return nil
}

func positiveWholeNumber(value any) bool {
	switch item := value.(type) {
	case json.Number:
		integer, err := item.Int64()
		return err == nil && integer >= 1
	case int:
		return item >= 1
	case int64:
		return item >= 1
	case float64:
		return item >= 1 && item == float64(int64(item))
	default:
		return false
	}
}

func sameStringSet(left map[string]bool, right map[string]bool) bool {
	if len(left) != len(right) {
		return false
	}
	for key := range left {
		if !right[key] {
			return false
		}
	}
	return true
}

func jsonErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func nodeNames(nodes []map[string]any) map[string]string {
	names := map[string]string{}
	for index, node := range nodes {
		names[stringValue(node["job_id"])] = fmt.Sprintf("n%d", index)
	}
	return names
}

func mermaidNodeLine(node map[string]any, names map[string]string, indent string) string {
	jobID := stringValue(node["job_id"])
	label := graphNodeLabel(node, "<br/>")
	return fmt.Sprintf(`%s%s["%s"]`, indent, names[jobID], mermaidLabel(label))
}

func dotNodeLine(node map[string]any, names map[string]string, indent string) string {
	jobID := stringValue(node["job_id"])
	label := graphNodeLabel(node, `\n`)
	return fmt.Sprintf(`%s%s [label="%s"];`, indent, names[jobID], dotLabel(label))
}

func graphNodeLabel(node map[string]any, separator string) string {
	jobID := stringValue(node["job_id"])
	typeText := defaultString(node["type"], "generic")
	roleID := stringValue(node["role_id"])
	laneID := stringValue(node["lane_id"])
	if laneID != "" {
		laneID = "/" + laneID
	}
	return jobID + separator + typeText + " " + roleID + laneID
}

func mermaidLabel(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}

func dotLabel(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}

func dotClusterID(groupID string, index int) string {
	replacer := strings.NewReplacer("-", "_", ".", "_", "/", "_", " ", "_")
	sanitized := strings.Trim(replacer.Replace(groupID), "_")
	if sanitized == "" {
		sanitized = fmt.Sprintf("pg%d", index)
	}
	return "cluster_" + sanitized
}

func sortedKeys(values map[string][]map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
