package workflowtemplates

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RenderMarkdown(catalog *Catalog) (string, error) {
	if catalog == nil {
		var err error
		catalog, err = Load()
		if err != nil {
			return "", err
		}
	}
	var lines []string
	lines = append(lines,
		"# Workflow Template Catalog",
		"",
		"Generated from the bundled Striatum workflow template catalog.",
		"",
		"## Workflow Shapes",
		"",
	)
	for _, entry := range sortedEntries(catalog.Shapes) {
		section, err := shapeSection(entry)
		if err != nil {
			return "", err
		}
		lines = append(lines, section...)
		lines = append(lines, "")
	}
	lines = append(lines, "## Lane Sets", "")
	for _, entry := range sortedEntries(catalog.LaneSets) {
		lines = append(lines, templateSection(entry, []string{"recommended_for", "required_options"})...)
		lines = append(lines, "")
	}
	if len(catalog.RolePacks) > 0 {
		lines = append(lines, "## Role Packs", "")
		for _, entry := range sortedEntries(catalog.RolePacks) {
			lines = append(lines, templateSection(entry, []string{"recommended_for", "default_shapes", "roles"})...)
			lines = append(lines, "")
		}
	}
	if len(catalog.AdversaryPacks) > 0 {
		lines = append(lines, "## Adversary Packs", "")
		for _, entry := range sortedEntries(catalog.AdversaryPacks) {
			lines = append(lines, templateSection(entry, []string{"recommended_for", "default_shapes", "postures"})...)
			lines = append(lines, "")
		}
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n", nil
}

type MarkdownWriteOptions struct {
	Force bool
	Check bool
}

func WriteMarkdown(repoRoot string, targetPath string, options MarkdownWriteOptions) (map[string]any, error) {
	body, err := RenderMarkdown(nil)
	if err != nil {
		return nil, err
	}
	target, relative, err := safeMarkdownTarget(repoRoot, targetPath)
	if err != nil {
		return nil, err
	}
	digest := sha256Hex([]byte(body))
	existing, err := os.ReadFile(target)
	if err == nil {
		if string(existing) == body {
			return map[string]any{"status": "up_to_date", "path": target, "relative_path": relative, "sha256": digest}, nil
		}
		if options.Check {
			return map[string]any{"status": "stale", "path": target, "relative_path": relative, "sha256": digest}, nil
		}
		if !options.Force {
			return nil, &Error{
				Message:   "workflow templates render-md refuses to overwrite an existing Markdown file",
				FieldPath: "path",
				Hint:      "rerun with --force to update the file, or --check to test for drift",
			}
		}
	} else if os.IsNotExist(err) {
		if options.Check {
			return map[string]any{"status": "missing", "path": target, "relative_path": relative, "sha256": digest}, nil
		}
	} else {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, err
	}
	tmp := filepath.Join(filepath.Dir(target), fmt.Sprintf(".%s.tmp.%d", filepath.Base(target), os.Getpid()))
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return nil, err
	}
	status := "created"
	if len(existing) > 0 {
		status = "updated"
	}
	return map[string]any{"status": status, "path": target, "relative_path": relative, "sha256": digest}, nil
}

func shapeSection(entry map[string]any) ([]string, error) {
	lines := templateSection(entry, []string{"recommended_for", "default_lane_sets", "required_options"})
	// #111: an example-only shape is advertised for discovery but is NOT a
	// `workflow generate --shape` value; say so and point at the example fixture
	// so operators do not select it as a generated shape.
	if generatable, ok := entry["generatable"].(bool); ok && !generatable {
		note := "**Example-only** — not a `workflow generate --shape` value."
		if path, ok := entry["example_workflow_path"].(string); ok && path != "" {
			note += fmt.Sprintf(" Copy and adapt the example workflow at `%s`.", path)
		}
		lines = append(lines, note, "")
	}
	preview, _ := entry["graph_preview"].(map[string]any)
	if preview == nil {
		return stripTrailingBlank(lines), nil
	}
	mermaid, err := RenderGraphPreviewMermaid(preview)
	if err != nil {
		return nil, err
	}
	if mermaid == "" {
		lines = append(lines, "No fixed graph preview.", "")
	} else {
		lines = append(lines, "```mermaid", strings.TrimRight(mermaid, "\n"), "```", "")
	}
	return stripTrailingBlank(lines), nil
}

func templateSection(entry map[string]any, keys []string) []string {
	templateID := fmt.Sprint(entry["template_id"])
	displayName := fmt.Sprint(entry["display_name"])
	if displayName == "" || displayName == "<nil>" {
		displayName = templateID
	}
	lines := []string{fmt.Sprintf("### %s (`%s`)", displayName, templateID), ""}
	if summary, ok := entry["summary"].(string); ok && summary != "" {
		lines = append(lines, summary, "")
	}
	lines = append(lines, metadataLines(entry, keys)...)
	return stripTrailingBlank(lines)
}

func metadataLines(entry map[string]any, keys []string) []string {
	labels := map[string]string{
		"recommended_for":   "Recommended for",
		"default_lane_sets": "Default lane sets",
		"required_options":  "Required options",
		"default_shapes":    "Default shapes",
		"roles":             "Roles",
		"postures":          "Postures",
	}
	lines := []string{}
	for _, key := range keys {
		values := stringList(entry[key])
		if len(values) == 0 {
			continue
		}
		rendered := strings.Join(backtickValues(values), ", ")
		if key == "recommended_for" {
			rendered = strings.Join(values, "; ")
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", labels[key], rendered))
	}
	if len(lines) > 0 {
		lines = append(lines, "")
	}
	return lines
}

func RenderGraphPreviewMermaid(graphPreview map[string]any) (string, error) {
	nodes := mapList(graphPreview["nodes"])
	if len(nodes) == 0 {
		return "", nil
	}
	edges := mapList(graphPreview["edges"])
	cycles := mapList(graphPreview["cycles"])
	nodeNames := map[string]string{}
	for index, node := range nodes {
		nodeID := fmt.Sprint(node["id"])
		if nodeID == "" {
			return "", &Error{Message: "workflow template catalog graph_preview node id must be non-empty", FieldPath: fmt.Sprintf("graph_preview.nodes[%d].id", index)}
		}
		if _, exists := nodeNames[nodeID]; exists {
			return "", &Error{Message: fmt.Sprintf("workflow template catalog graph_preview.nodes[%d] duplicates id %q", index, nodeID), FieldPath: fmt.Sprintf("graph_preview.nodes[%d].id", index)}
		}
		nodeNames[nodeID] = fmt.Sprintf("n%d", index)
	}
	lines := []string{"flowchart TD"}
	for _, node := range nodes {
		nodeID := fmt.Sprint(node["id"])
		label := fmt.Sprint(node["label"])
		if label == "" || label == "<nil>" {
			label = nodeID
		}
		lines = append(lines, fmt.Sprintf(`  %s["%s"]`, nodeNames[nodeID], mermaidLabel(label)))
	}
	for index, edge := range edges {
		fromID := fmt.Sprint(edge["from"])
		toID := fmt.Sprint(edge["to"])
		if err := requireKnownNode(fromID, nodeNames, fmt.Sprintf("graph_preview.edges[%d].from", index)); err != nil {
			return "", err
		}
		if err := requireKnownNode(toID, nodeNames, fmt.Sprintf("graph_preview.edges[%d].to", index)); err != nil {
			return "", err
		}
		label := fmt.Sprint(edge["label"])
		if label == "" || label == "<nil>" {
			lines = append(lines, fmt.Sprintf("  %s --> %s", nodeNames[fromID], nodeNames[toID]))
		} else {
			lines = append(lines, fmt.Sprintf("  %s -->|%s| %s", nodeNames[fromID], mermaidLabel(label), nodeNames[toID]))
		}
	}
	for index, cycle := range cycles {
		fromID := fmt.Sprint(cycle["from"])
		toID := fmt.Sprint(cycle["to"])
		if err := requireKnownNode(fromID, nodeNames, fmt.Sprintf("graph_preview.cycles[%d].from", index)); err != nil {
			return "", err
		}
		if err := requireKnownNode(toID, nodeNames, fmt.Sprintf("graph_preview.cycles[%d].to", index)); err != nil {
			return "", err
		}
		lines = append(lines, fmt.Sprintf("  %s -.->|%s| %s", nodeNames[fromID], mermaidLabel(cycleLabel(cycle)), nodeNames[toID]))
	}
	return strings.Join(lines, "\n") + "\n", nil
}

func safeMarkdownTarget(repoRoot string, targetPath string) (string, string, error) {
	if strings.ContainsRune(targetPath, '\x00') {
		return "", "", &Error{Message: "Markdown output path cannot contain NUL bytes", FieldPath: "path"}
	}
	if strings.ToLower(filepath.Ext(targetPath)) != ".md" {
		return "", "", &Error{Message: "Markdown output path must end with .md", FieldPath: "path"}
	}
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", "", err
	}
	target := targetPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, targetPath)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", "", err
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", "", &Error{Message: "Markdown output path must stay inside the repository", FieldPath: "path"}
	}
	for _, part := range strings.Split(filepath.ToSlash(rel), "/") {
		if part == ".git" || part == ".striatum" {
			return "", "", &Error{Message: "Markdown output path cannot be inside .git or .striatum", FieldPath: "path"}
		}
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		return "", "", &Error{Message: "Markdown output path exists and is not a regular file", FieldPath: "path"}
	}
	return target, filepath.ToSlash(rel), nil
}

func requireKnownNode(nodeID string, nodeNames map[string]string, fieldPath string) error {
	if _, ok := nodeNames[nodeID]; !ok {
		return &Error{Message: fmt.Sprintf("workflow template catalog graph preview references unknown node %q", nodeID), FieldPath: fieldPath}
	}
	return nil
}

func cycleLabel(cycle map[string]any) string {
	if label, ok := cycle["label"].(string); ok && label != "" {
		return label
	}
	onVerdict, _ := cycle["on_verdict"].(string)
	maxIterations := fmt.Sprint(cycle["max_iterations"])
	if onVerdict != "" && maxIterations != "" && maxIterations != "<nil>" {
		return onVerdict + " max " + maxIterations
	}
	if onVerdict != "" {
		return onVerdict
	}
	return "cycle"
}

func sortedEntries(entries []map[string]any) []map[string]any {
	result := cloneEntries(entries)
	sort.SliceStable(result, func(i, j int) bool {
		return fmt.Sprint(result[i]["template_id"]) < fmt.Sprint(result[j]["template_id"])
	})
	return result
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		result := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	}
	return nil
}

func mapList(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := []map[string]any{}
		for _, item := range typed {
			if mapped, ok := item.(map[string]any); ok {
				result = append(result, mapped)
			}
		}
		return result
	}
	return nil
}

func backtickValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, "`"+value+"`")
	}
	return result
}

func stripTrailingBlank(lines []string) []string {
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func mermaidLabel(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`)
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
