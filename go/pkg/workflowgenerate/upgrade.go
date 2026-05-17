package workflowgenerate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type RunQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

type UpgradeOptions struct {
	Path      string
	Force     bool
	DryRun    bool
	AddPhases bool
	Apply     bool
}

func Upgrade(ctx context.Context, runner RunQueryer, repositoryID, repoRoot string, opts UpgradeOptions) (map[string]any, error) {
	if opts.Path == "" {
		return nil, &Error{Message: "workflow.upgrade requires path", FieldPath: "path"}
	}
	if opts.AddPhases {
		return nil, &Error{Message: "workflow upgrade --add-phases is not yet ported to the Go daemon; refusing rather than rewriting workflow without full parity", FieldPath: "add_phases"}
	}
	target, err := SafeTarget(repoRoot, opts.Path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &Error{Message: "workflow upgrade target does not exist: " + target, FieldPath: "path"}
		}
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, &Error{Message: "workflow upgrade target is not a regular file: " + target, FieldPath: "path"}
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	var workflow map[string]any
	if err := json.Unmarshal(raw, &workflow); err != nil {
		return nil, &Error{Message: "workflow JSON is invalid: " + err.Error(), FieldPath: "path"}
	}
	if err := ValidateWorkflowForUpgrade(workflow); err != nil {
		return nil, err
	}

	running, err := RunningRunsForWorkflow(ctx, runner, repositoryID, repoRoot, target)
	if err != nil {
		return nil, err
	}
	if len(running) > 0 && !opts.DryRun {
		return nil, &Error{Message: "workflow upgrade refuses to mutate a workflow with non-terminal runs: " + strings.Join(running, ", "), FieldPath: "path"}
	}

	profiles, _ := workflow["harness_profiles"].(map[string]any)
	if len(profiles) == 0 {
		status := "no_changes"
		if opts.DryRun {
			status = "would_no_changes"
		}
		return map[string]any{
			"workflow_path": target,
			"status":        status,
			"changes":       []map[string]any{},
			"conflicts":     []map[string]any{},
			"note":          "workflow has no harness_profiles section; nothing to upgrade",
		}, nil
	}

	changes := []map[string]any{}
	conflicts := []map[string]any{}
	updatedProfiles := map[string]any{}
	for _, profileID := range sortedMapKeys(profiles) {
		body, ok := profiles[profileID].(map[string]any)
		if !ok {
			updatedProfiles[profileID] = profiles[profileID]
			continue
		}
		newBody := cloneMap(body)
		fragment := harnessFragmentByToolFamily(fmt.Sprint(body["tool_family"]))
		if fragment == nil {
			updatedProfiles[profileID] = newBody
			continue
		}
		native := map[string]any{}
		if rawNative, ok := newBody["native_delegation"].(map[string]any); ok {
			native = cloneMap(rawNative)
		}
		catalogInstruction := fmt.Sprint(fragment["native_delegation_instruction"])
		currentInstruction, hasInstruction := native["instruction"]
		currentText, currentIsString := currentInstruction.(string)
		switch {
		case hasInstruction && currentInstruction == catalogInstruction:
		case !hasInstruction || (currentIsString && strings.TrimSpace(currentText) == ""):
			native["instruction"] = catalogInstruction
			changes = append(changes, map[string]any{"profile_id": profileID, "field": "native_delegation.instruction", "old_value": nullableValue(currentInstruction, hasInstruction), "new_value": catalogInstruction})
		case opts.Force:
			native["instruction"] = catalogInstruction
			changes = append(changes, map[string]any{"profile_id": profileID, "field": "native_delegation.instruction", "old_value": currentInstruction, "new_value": catalogInstruction, "forced": true})
		default:
			conflicts = append(conflicts, map[string]any{"profile_id": profileID, "field": "native_delegation.instruction", "current_value": currentInstruction, "catalog_value": catalogInstruction})
		}
		if _, ok := native["mode"]; !ok {
			if catalogMode, ok := fragment["native_delegation_mode"].(string); ok && catalogMode != "" {
				native["mode"] = catalogMode
				changes = append(changes, map[string]any{"profile_id": profileID, "field": "native_delegation.mode", "old_value": nil, "new_value": catalogMode})
			}
		}
		if len(native) > 0 {
			newBody["native_delegation"] = native
		}
		updatedProfiles[profileID] = newBody
	}

	if len(conflicts) > 0 && !opts.Force {
		return map[string]any{
			"workflow_path": target,
			"status":        "refused_conflict",
			"changes":       changes,
			"conflicts":     conflicts,
			"hint":          "rerun with force to overwrite, or edit the profile to match the catalog first",
		}, nil
	}
	if len(running) > 0 && opts.DryRun {
		return map[string]any{
			"workflow_path": target,
			"status":        "would_refuse_running",
			"changes":       changes,
			"conflicts":     conflicts,
			"running_runs":  running,
		}, nil
	}
	if len(changes) == 0 {
		status := "no_changes"
		if opts.DryRun {
			status = "would_no_changes"
		}
		return map[string]any{"workflow_path": target, "status": status, "changes": []map[string]any{}, "conflicts": []map[string]any{}}, nil
	}
	if opts.DryRun {
		return map[string]any{"workflow_path": target, "status": "would_update", "changes": changes, "conflicts": []map[string]any{}}, nil
	}

	updated := cloneMap(workflow)
	updated["harness_profiles"] = updatedProfiles
	body, err := json.MarshalIndent(updated, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(target, append(body, '\n'), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"workflow_path": target, "status": "updated", "changes": changes, "conflicts": []map[string]any{}}, nil
}

func ValidateWorkflowForUpgrade(workflow map[string]any) error {
	if workflow["schema_version"] == nil {
		return &Error{Message: "workflow config must declare schema_version", FieldPath: "schema_version"}
	}
	return nil
}

func RunningRunsForWorkflow(ctx context.Context, runner RunQueryer, repositoryID, repoRoot, workflowPath string) ([]string, error) {
	if runner == nil {
		return nil, &Error{Message: "workflow upgrade cannot verify non-terminal runs because daemon PostgreSQL query access is unavailable", FieldPath: "path"}
	}
	candidates := workflowSourcePathCandidates(repoRoot, workflowPath)
	rows, err := runner.Query(ctx, `
		SELECT r.run_id
		  FROM striatumd.runs r
		  JOIN striatumd.workflow_snapshots w
		    ON w.repository_id = r.repository_id
		   AND w.workflow_snapshot_id = r.workflow_snapshot_id
		 WHERE r.repository_id = $1
		   AND r.state NOT IN ('completed', 'failed', 'canceled')
		   AND w.source_path = ANY($2)
		 ORDER BY r.run_id`, repositoryID, candidates)
	if err != nil {
		return nil, &Error{Message: "workflow upgrade cannot verify non-terminal runs using daemon PostgreSQL: " + err.Error(), FieldPath: "path"}
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return nil, err
		}
		result = append(result, runID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func workflowSourcePathCandidates(repoRoot, workflowPath string) []string {
	target, _ := filepath.Abs(workflowPath)
	candidates := map[string]struct{}{target: {}}
	repoAbs, err := filepath.Abs(repoRoot)
	if err == nil {
		if rel, err := filepath.Rel(repoAbs, target); err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".." {
			candidates[filepath.ToSlash(rel)] = struct{}{}
		}
	}
	out := []string{}
	for candidate := range candidates {
		out = append(out, candidate)
	}
	sort.Strings(out)
	return out
}

func nullableValue(value any, exists bool) any {
	if !exists {
		return nil
	}
	return value
}
