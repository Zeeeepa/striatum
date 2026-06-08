package workflowauthoring

import (
	"fmt"
	"sort"
	"strings"
)

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
