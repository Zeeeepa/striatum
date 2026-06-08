package workflowauthoring

import (
	"sort"
	"strings"
)

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
	result := map[string]any{
		"job_id":                 stringValue(job["id"]),
		"type":                   defaultString(job["type"], "generic"),
		"role_id":                stringValue(job["role_id"]),
		"lane_id":                nullableString(job["lane_id"]),
		"parallel_group":         nullableString(job["parallel_group"]),
		"fresh_session_required": job["fresh_session_required"] == true,
		"write_scope_mode":       writeScopeMode(job),
		"expected_artifacts":     artifacts,
	}
	if resources := plannedSharedResources(job); len(resources) > 0 {
		result["shared_resources"] = resources
	}
	return result
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

func plannedSharedResources(job map[string]any) []map[string]any {
	resources := []map[string]any{}
	for _, item := range anySlice(job["shared_resources"]) {
		switch resource := item.(type) {
		case string:
			id := strings.TrimSpace(resource)
			if id == "" {
				continue
			}
			resources = append(resources, map[string]any{"id": id, "mode": "exclusive"})
		case map[string]any:
			id := strings.TrimSpace(stringValue(resource["id"]))
			if id == "" {
				continue
			}
			mode := stringValue(resource["mode"])
			if mode == "" {
				mode = "exclusive"
			}
			entry := map[string]any{"id": id, "mode": mode}
			if description := strings.TrimSpace(stringValue(resource["description"])); description != "" {
				entry["description"] = description
			}
			if namespace := strings.TrimSpace(stringValue(resource["namespace"])); namespace != "" {
				entry["namespace"] = namespace
			}
			resources = append(resources, entry)
		}
	}
	return resources
}
