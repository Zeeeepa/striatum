package reads

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

var graphFormats = map[string]bool{"json": true, "mermaid": true, "dot": true, "ascii": true}
var graphOrients = map[string]bool{"tb": true, "lr": true}
var graphStyles = map[string]bool{"auto": true, "layered": true, "list": true, "fancy": true}

var mermaidStateFills = []struct {
	class string
	fill  string
}{
	{"state-completed", "#c8e6c9"},
	{"state-running", "#bbdefb"},
	{"state-claimed", "#bbdefb"},
	{"state-acked", "#bbdefb"},
	{"state-blocked", "#fff59d"},
	{"state-stale_lease", "#fff59d"},
	{"state-waiting_human", "#fff59d"},
	{"state-failed", "#ffcdd2"},
	{"state-canceled", "#ffcdd2"},
	{"state-queued", "#e0e0e0"},
	{"state-pending", "#f5f5f5"},
}

func HandleRunGraph(ctx context.Context, runner db.Runner, envelope rpc.Envelope) (map[string]any, error) {
	repositoryID, err := requireRepositoryID(envelope)
	if err != nil {
		return nil, err
	}
	runID := stringParam(envelope, "run_id")
	if runID == "" {
		return nil, rpc.NewError("schema_invalid", "run.graph requires run_id", nil)
	}
	format, err := graphChoice(envelope, "format", "mermaid", graphFormats)
	if err != nil {
		return nil, err
	}
	if _, err := graphChoice(envelope, "graph_orient", "tb", graphOrients); err != nil {
		return nil, err
	}
	if _, err := graphChoice(envelope, "graph_style", "layered", graphStyles); err != nil {
		return nil, err
	}

	workflow, err := graphWorkflowForRun(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	dependencies, err := graphDependencyEdges(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	graph := workflowGraph(workflow, dependencies)
	latest, err := graphLatestJobs(ctx, runner, repositoryID, runID)
	if err != nil {
		return nil, err
	}
	nodeStates := map[string]string{}
	reviewJobIDs := []string{}
	for workflowJobID, row := range latest {
		nodeStates[workflowJobID] = stringFrom(row, "state")
		if stringFrom(row, "job_type") == "review" {
			reviewJobIDs = append(reviewJobIDs, stringFrom(row, "job_id"))
		}
	}
	verdicts, err := graphLatestVerdicts(ctx, runner, repositoryID, reviewJobIDs)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return graphJSONPayload(runID, workflow, graph, latest, verdicts), nil
	case "dot":
		return map[string]any{"format": "dot", "source": renderGraphDOT(graph)}, nil
	case "ascii":
		return map[string]any{"format": "ascii", "source": renderGraphASCII(graph, nodeStates)}, nil
	default:
		return map[string]any{"format": "mermaid", "source": renderGraphMermaid(graph, nodeStates)}, nil
	}
}

func graphWorkflowForRun(ctx context.Context, runner db.Runner, repositoryID string, runID string) (map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT w.workflow_json, w.workflow_id, w.workflow_version
		   FROM striatumd.runs r
		   JOIN striatumd.workflow_snapshots w
		     ON w.repository_id = r.repository_id
		    AND w.workflow_snapshot_id = r.workflow_snapshot_id
		  WHERE r.repository_id = $1 AND r.run_id = $2`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, rpc.NewError("not_found", "run not found: "+runID, nil)
	}
	workflow := objectFromJSONish(rows[0]["workflow_json"])
	if workflow["workflow_id"] == nil {
		workflow["workflow_id"] = rows[0]["workflow_id"]
	}
	if workflow["workflow_version"] == nil {
		workflow["workflow_version"] = rows[0]["workflow_version"]
	}
	return workflow, nil
}

func graphDependencyEdges(ctx context.Context, runner db.Runner, repositoryID string, runID string) ([]map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT upstream.workflow_job_id AS from_id,
		        downstream.workflow_job_id AS to_id,
		        d.gate_json,
		        upstream.attempt AS upstream_attempt,
		        downstream.attempt AS downstream_attempt
		   FROM striatumd.job_dependencies d
		   JOIN striatumd.jobs downstream
		     ON downstream.repository_id = d.repository_id
		    AND downstream.job_id = d.job_id
		   JOIN striatumd.jobs upstream
		     ON upstream.repository_id = d.repository_id
		    AND upstream.job_id = d.depends_on_job_id
		  WHERE d.repository_id = $1
		    AND downstream.run_id = $2
		    AND upstream.run_id = $2
		  ORDER BY upstream.workflow_job_id, downstream.workflow_job_id,
		           downstream.attempt DESC, upstream.attempt DESC`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	edges := []map[string]any{}
	seen := map[string]bool{}
	for _, row := range rows {
		fromID := stringFrom(row, "from_id")
		toID := stringFrom(row, "to_id")
		key := fromID + "\x00" + toID
		if seen[key] {
			continue
		}
		seen[key] = true
		gateRow := objectFromJSONish(row["gate_json"])
		gate := map[string]any{"on": "completed"}
		if on, ok := gateRow["on"].(string); ok && on != "" {
			gate["on"] = on
		}
		if requires, ok := gateRow["requires_verdict"].([]any); ok {
			gate["requires_verdict"] = requires
		}
		edges = append(edges, map[string]any{"from": fromID, "to": toID, "gate": gate})
	}
	return edges, nil
}

func graphLatestJobs(ctx context.Context, runner db.Runner, repositoryID string, runID string) (map[string]map[string]any, error) {
	rows, err := collectRows(ctx, runner,
		`SELECT j.job_id, j.workflow_job_id, j.state, j.attempt, j.job_type
		   FROM striatumd.jobs j
		  WHERE j.repository_id = $1 AND j.run_id = $2
		  ORDER BY j.workflow_job_id, j.attempt DESC`,
		repositoryID, runID,
	)
	if err != nil {
		return nil, err
	}
	latest := map[string]map[string]any{}
	for _, row := range rows {
		workflowJobID := stringFrom(row, "workflow_job_id")
		if _, exists := latest[workflowJobID]; !exists {
			latest[workflowJobID] = row
		}
	}
	return latest, nil
}

func graphLatestVerdicts(ctx context.Context, runner db.Runner, repositoryID string, jobIDs []string) (map[string]map[string]any, error) {
	if len(jobIDs) == 0 {
		return map[string]map[string]any{}, nil
	}
	rows, err := collectRows(ctx, runner,
		`SELECT DISTINCT ON (v.job_id)
		        v.job_id, v.verdict_id, v.verdict, v.rationale,
		        v.findings_artifact_id, v.posture, v.created_at
		   FROM striatumd.verdicts v
		  WHERE v.repository_id = $1 AND v.job_id = ANY($2)
		  ORDER BY v.job_id, v.created_at DESC, v.verdict_id DESC`,
		repositoryID, jobIDs,
	)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]any{}
	for _, row := range rows {
		result[stringFrom(row, "job_id")] = row
	}
	return result, nil
}

func workflowGraph(workflow map[string]any, dependencies []map[string]any) map[string]any {
	nodes := plannedGraphNodes(workflow)
	edges := dependencies
	if len(edges) == 0 {
		edges = plannedGraphEdges(workflow)
	}
	return map[string]any{
		"nodes":  nodes,
		"edges":  edges,
		"cycles": plannedGraphCycles(workflow),
	}
}

func plannedGraphNodes(workflow map[string]any) []map[string]any {
	items, _ := workflow["jobs"].([]any)
	nodes := make([]map[string]any, 0, len(items))
	for _, item := range items {
		job := objectFromJSONish(item)
		jobID := stringValue(job["id"])
		artifacts := []map[string]any{}
		for _, artifactItem := range anySlice(job["expected_artifacts"]) {
			artifact := objectFromJSONish(artifactItem)
			artifacts = append(artifacts, map[string]any{
				"logical_name": artifact["logical_name"],
				"kind":         artifact["kind"],
				"path":         artifact["path"],
				"required":     artifact["required"] == true,
			})
		}
		nodes = append(nodes, map[string]any{
			"job_id":                 jobID,
			"type":                   defaultString(job["type"], "generic"),
			"role_id":                stringValue(job["role_id"]),
			"lane_id":                nullableStringValue(job["lane_id"]),
			"parallel_group":         nullableStringValue(job["parallel_group"]),
			"fresh_session_required": job["fresh_session_required"] == true,
			"write_scope_mode":       writeScopeMode(job),
			"expected_artifacts":     artifacts,
		})
	}
	return nodes
}

func plannedGraphEdges(workflow map[string]any) []map[string]any {
	edges := []map[string]any{}
	for _, item := range anySlice(workflow["edges"]) {
		edge := objectFromJSONish(item)
		fromID := stringValue(edge["from"])
		toID := stringValue(edge["to"])
		if fromID == "" || toID == "" {
			continue
		}
		gate := map[string]any{"on": defaultString(edge["on"], "completed")}
		edges = append(edges, map[string]any{"from": fromID, "to": toID, "gate": gate})
	}
	return edges
}

func plannedGraphCycles(workflow map[string]any) []map[string]any {
	cycles := []map[string]any{}
	for _, item := range anySlice(workflow["cycles"]) {
		cycle := objectFromJSONish(item)
		cycles = append(cycles, map[string]any{
			"from":           cycle["from"],
			"to":             cycle["to"],
			"on_verdict":     cycle["on_verdict"],
			"max_iterations": cycle["max_iterations"],
		})
	}
	return cycles
}

func graphJSONPayload(runID string, workflow map[string]any, graph map[string]any, latest map[string]map[string]any, verdicts map[string]map[string]any) map[string]any {
	nodes := graph["nodes"].([]map[string]any)
	annotated := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		item := copyMap(node)
		workflowJobID := stringValue(node["job_id"])
		row := latest[workflowJobID]
		if row == nil {
			item["current_state"] = "pending"
			item["attempt"] = nil
			item["state_class"] = mermaidStateClass("pending")
		} else {
			state := stringFrom(row, "state")
			item["current_state"] = state
			item["attempt"] = row["attempt"]
			item["state_class"] = mermaidStateClass(state)
			if stringFrom(row, "job_type") == "review" {
				verdict := verdicts[stringFrom(row, "job_id")]
				if verdict == nil {
					item["latest_verdict"] = nil
				} else {
					item["latest_verdict"] = map[string]any{
						"verdict_id":           verdict["verdict_id"],
						"verdict":              verdict["verdict"],
						"rationale":            verdict["rationale"],
						"findings_artifact_id": verdict["findings_artifact_id"],
						"posture":              defaultString(verdict["posture"], "neutral"),
					}
				}
			}
		}
		annotated = append(annotated, item)
	}
	graphOut := copyMap(graph)
	graphOut["nodes"] = annotated
	return map[string]any{
		"run_id":           runID,
		"workflow_id":      workflow["workflow_id"],
		"workflow_version": workflow["workflow_version"],
		"graph":            graphOut,
	}
}

func renderGraphMermaid(graph map[string]any, nodeStates map[string]string) string {
	nodes := graph["nodes"].([]map[string]any)
	edges := graph["edges"].([]map[string]any)
	cycles := graph["cycles"].([]map[string]any)
	nodeNames := graphNodeNames(nodes)
	lines := []string{"flowchart TD"}
	for _, node := range nodes {
		jobID := stringValue(node["job_id"])
		lines = append(lines, fmt.Sprintf(`  %s["%s"]`, nodeNames[jobID], mermaidLabel(graphNodeLabel(node, "<br/>"))))
	}
	for _, edge := range edges {
		fromID := stringValue(edge["from"])
		toID := stringValue(edge["to"])
		label := "completed"
		if _, ok := objectFromJSONish(edge["gate"])["requires_verdict"]; ok {
			label = "accepted review"
		}
		lines = append(lines, fmt.Sprintf("  %s -->|%s| %s", nodeNames[fromID], label, nodeNames[toID]))
	}
	for _, cycle := range cycles {
		fromID := stringValue(cycle["from"])
		toID := stringValue(cycle["to"])
		lines = append(lines, fmt.Sprintf("  %s -.->|needs_revision max %v| %s", nodeNames[fromID], cycle["max_iterations"], nodeNames[toID]))
	}
	for _, entry := range mermaidStateFills {
		lines = append(lines, fmt.Sprintf("  classDef %s fill:%s", entry.class, entry.fill))
	}
	for _, node := range nodes {
		jobID := stringValue(node["job_id"])
		state := nodeStates[jobID]
		if state == "" {
			state = "pending"
		}
		lines = append(lines, fmt.Sprintf("  class %s %s", nodeNames[jobID], mermaidStateClass(state)))
	}
	return strings.Join(lines, "\n") + "\n"
}

func renderGraphDOT(graph map[string]any) string {
	nodes := graph["nodes"].([]map[string]any)
	edges := graph["edges"].([]map[string]any)
	cycles := graph["cycles"].([]map[string]any)
	nodeNames := graphNodeNames(nodes)
	lines := []string{"digraph striatum_workflow {", "  rankdir=TB;", `  node [shape=box, fontname="Helvetica"];`}
	for _, node := range nodes {
		jobID := stringValue(node["job_id"])
		lines = append(lines, fmt.Sprintf(`  %s [label="%s"];`, nodeNames[jobID], dotLabel(graphNodeLabel(node, `\n`))))
	}
	for _, edge := range edges {
		fromID := stringValue(edge["from"])
		toID := stringValue(edge["to"])
		label := "completed"
		if _, ok := objectFromJSONish(edge["gate"])["requires_verdict"]; ok {
			label = "accepted review"
		}
		lines = append(lines, fmt.Sprintf(`  %s -> %s [label="%s"];`, nodeNames[fromID], nodeNames[toID], dotLabel(label)))
	}
	for _, cycle := range cycles {
		fromID := stringValue(cycle["from"])
		toID := stringValue(cycle["to"])
		lines = append(lines, fmt.Sprintf(`  %s -> %s [style=dashed, label="%s"];`, nodeNames[fromID], nodeNames[toID], dotLabel(fmt.Sprintf("needs_revision max %v", cycle["max_iterations"]))))
	}
	lines = append(lines, "}")
	return strings.Join(lines, "\n") + "\n"
}

func renderGraphASCII(graph map[string]any, nodeStates map[string]string) string {
	nodes := graph["nodes"].([]map[string]any)
	edges := graph["edges"].([]map[string]any)
	lines := []string{}
	for _, node := range nodes {
		jobID := stringValue(node["job_id"])
		state := nodeStates[jobID]
		if state == "" {
			state = "pending"
		}
		lines = append(lines, fmt.Sprintf("%s [%s]", jobID, state))
	}
	for _, edge := range edges {
		lines = append(lines, fmt.Sprintf("%s -> %s", edge["from"], edge["to"]))
	}
	return strings.Join(lines, "\n") + "\n"
}

func graphChoice(envelope rpc.Envelope, key string, fallback string, allowed map[string]bool) (string, error) {
	value, err := optionalNonEmptyString(envelope, key)
	if err != nil {
		return "", err
	}
	if value == "" {
		value = fallback
	}
	if !allowed[value] {
		options := make([]string, 0, len(allowed))
		for option := range allowed {
			options = append(options, option)
		}
		sort.Strings(options)
		return "", rpc.NewError("schema_invalid", fmt.Sprintf("unknown %s: %q (valid options: %s)", key, value, strings.Join(options, ", ")), nil)
	}
	return value, nil
}

func graphNodeNames(nodes []map[string]any) map[string]string {
	names := map[string]string{}
	for index, node := range nodes {
		names[stringValue(node["job_id"])] = fmt.Sprintf("n%d", index)
	}
	return names
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

func mermaidStateClass(state string) string {
	if state == "skipped" {
		return "state-canceled"
	}
	candidate := "state-" + state
	for _, entry := range mermaidStateFills {
		if entry.class == candidate {
			return candidate
		}
	}
	return "state-pending"
}

func mermaidLabel(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}

func dotLabel(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}

func objectFromJSONish(value any) map[string]any {
	switch item := value.(type) {
	case map[string]any:
		return item
	case []byte:
		var decoded map[string]any
		_ = json.Unmarshal(item, &decoded)
		return decoded
	case string:
		var decoded map[string]any
		_ = json.Unmarshal([]byte(item), &decoded)
		return decoded
	default:
		return map[string]any{}
	}
}

func anySlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return []any{}
}

func defaultString(value any, fallback string) string {
	text := stringValue(value)
	if text == "" {
		return fallback
	}
	return text
}

func nullableStringValue(value any) any {
	text := stringValue(value)
	if text == "" {
		return nil
	}
	return text
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func writeScopeMode(job map[string]any) any {
	scope := objectFromJSONish(job["write_scope"])
	if mode := stringValue(scope["mode"]); mode != "" {
		return mode
	}
	return nil
}
