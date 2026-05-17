package workflowgenerate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type PlannedWrite struct {
	Path           string `json:"path"`
	Status         string `json:"status"`
	ContentSHA256  string `json:"content_sha256"`
	ExistingSHA256 string `json:"existing_sha256,omitempty"`
	Diff           string `json:"diff,omitempty"`
}

func Preview(repoRoot string, generated Generated) ([]map[string]any, error) {
	plans := []map[string]any{}
	for _, entry := range generated.Files {
		rel, content, err := fileEntry(entry)
		if err != nil {
			return nil, err
		}
		target, err := SafeTarget(repoRoot, rel)
		if err != nil {
			return nil, err
		}
		status := "would_create"
		existingSHA := ""
		diff := ""
		if current, err := os.ReadFile(target); err == nil {
			status = "would_overwrite"
			existingSHA = sha256Hex(current)
			diff = simpleDiff(rel, string(current), content)
			if string(current) == content {
				status = "unchanged"
				diff = ""
			}
		} else if !os.IsNotExist(err) {
			return nil, &Error{Message: "generated path cannot be inspected: " + err.Error(), FieldPath: "files.path"}
		}
		plans = append(plans, map[string]any{
			"path":            rel,
			"status":          status,
			"content_sha256":  FileHash(content),
			"existing_sha256": existingSHA,
			"diff":            diff,
		})
	}
	return plans, nil
}

func Write(repoRoot string, generated Generated) (map[string]any, error) {
	targets := []struct {
		Abs     string
		Rel     string
		Content string
	}{}
	for _, entry := range generated.Files {
		rel, content, err := fileEntry(entry)
		if err != nil {
			return nil, err
		}
		target, err := SafeTarget(repoRoot, rel)
		if err != nil {
			return nil, err
		}
		targets = append(targets, struct {
			Abs     string
			Rel     string
			Content string
		}{Abs: target, Rel: rel, Content: content})
	}
	existing := []string{}
	for _, target := range targets {
		if _, err := os.Stat(target.Abs); err == nil {
			existing = append(existing, target.Rel)
		} else if !os.IsNotExist(err) {
			return nil, &Error{Message: "generated path cannot be inspected: " + err.Error(), FieldPath: "files.path"}
		}
	}
	if len(existing) > 0 {
		return nil, &Error{Message: "workflow generate refuses to overwrite existing files", FieldPath: "files", Hint: strings.Join(existing[:min(5, len(existing))], ", ")}
	}
	written := []string{}
	for _, target := range targets {
		if err := os.MkdirAll(filepath.Dir(target.Abs), 0o755); err != nil {
			cleanup(repoRoot, written)
			return nil, err
		}
		tmp := filepath.Join(filepath.Dir(target.Abs), fmt.Sprintf(".%s.tmp.%d", filepath.Base(target.Abs), os.Getpid()))
		if err := os.WriteFile(tmp, []byte(target.Content), 0o644); err != nil {
			cleanup(repoRoot, written)
			return nil, err
		}
		if err := os.Rename(tmp, target.Abs); err != nil {
			_ = os.Remove(tmp)
			cleanup(repoRoot, written)
			return nil, err
		}
		written = append(written, target.Rel)
	}
	workflowPath, _ := generated.Metadata["workflow_path"].(string)
	workflowTarget, err := SafeTarget(repoRoot, workflowPath)
	if err != nil {
		cleanup(repoRoot, written)
		return nil, err
	}
	raw, err := os.ReadFile(workflowTarget)
	if err != nil {
		cleanup(repoRoot, written)
		return nil, err
	}
	var workflow map[string]any
	if err := json.Unmarshal(raw, &workflow); err != nil {
		cleanup(repoRoot, written)
		return nil, &Error{Message: "written workflow JSON is invalid: " + err.Error(), FieldPath: "workflow"}
	}
	if err := ValidateWorkflow(workflow); err != nil {
		cleanup(repoRoot, written)
		return nil, err
	}
	return map[string]any{
		"status":        "created",
		"path":          filepath.Join(repoRoot, fmt.Sprint(generated.Metadata["scaffold_root"])),
		"workflow_path": workflowTarget,
		"files":         written,
		"generated":     generated.Map(),
	}, nil
}

func SafeTarget(repoRoot, relative string) (string, error) {
	if repoRoot == "" {
		return "", &Error{Message: "registered repository has no repo_root", FieldPath: "repository_id"}
	}
	safe, err := SafeRelativePath(relative, "files.path")
	if err != nil {
		return "", err
	}
	repoAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	target := filepath.Join(repoAbs, filepath.FromSlash(safe))
	resolved, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(repoAbs, resolved)
	if err != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", &Error{Message: "generated path escapes repository", FieldPath: "files.path"}
	}
	return resolved, nil
}

func fileEntry(entry map[string]any) (string, string, error) {
	rel, ok := entry["path"].(string)
	if !ok || rel == "" {
		return "", "", &Error{Message: "generated files must carry string path and content", FieldPath: "files.path"}
	}
	content, ok := entry["content"].(string)
	if !ok {
		return "", "", &Error{Message: "generated files must carry string path and content", FieldPath: "files.content"}
	}
	return rel, content, nil
}

func cleanup(repoRoot string, paths []string) {
	for _, rel := range paths {
		target, err := SafeTarget(repoRoot, rel)
		if err == nil {
			_ = os.Remove(target)
		}
	}
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func simpleDiff(path, old, next string) string {
	if old == next {
		return ""
	}
	oldLines := strings.SplitAfter(old, "\n")
	nextLines := strings.SplitAfter(next, "\n")
	var b strings.Builder
	b.WriteString("--- " + path + "\n")
	b.WriteString("+++ " + path + "\n")
	for _, line := range oldLines {
		if line != "" {
			b.WriteString("-" + line)
			if !strings.HasSuffix(line, "\n") {
				b.WriteString("\n")
			}
		}
	}
	for _, line := range nextLines {
		if line != "" {
			b.WriteString("+" + line)
			if !strings.HasSuffix(line, "\n") {
				b.WriteString("\n")
			}
		}
	}
	return b.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
