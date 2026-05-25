package workflowgenerate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPreviewReturnsPlannedWritesWithoutWriting(t *testing.T) {
	repo := t.TempDir()
	generated := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "review",
		"lane_set":         "local",
		"workflow_id":      "demo",
		"name":             "Demo",
		"workflow_version": "2026-05-17",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": false},
		"scaffold_root":    "workflows/demo",
		"artifact_root":    "striatum/demo",
		"lanes":            map[string]any{},
		"options":          map[string]any{},
		"lane_modifiers":   []any{},
		"context_docs":     []any{},
	})
	planned, err := Preview(repo, generated)
	if err != nil {
		t.Fatal(err)
	}
	if len(planned) == 0 {
		t.Fatal("expected planned writes")
	}
	if planned[0]["status"] != "would_create" {
		t.Fatalf("first status = %#v", planned[0])
	}
	if _, err := os.Stat(filepath.Join(repo, "workflows", "demo", "workflow.json")); !os.IsNotExist(err) {
		t.Fatalf("preview wrote workflow.json or stat failed: %v", err)
	}
}

func TestWriteCreatesOnlySafeRepoRelativeTargets(t *testing.T) {
	repo := t.TempDir()
	generated := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "minimal",
		"lane_set":         "local",
		"workflow_id":      "demo",
		"name":             "Demo",
		"workflow_version": "2026-05-17",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": false},
		"scaffold_root":    "workflows/demo",
		"artifact_root":    "striatum/demo",
		"lanes":            map[string]any{},
		"options":          map[string]any{},
	})
	result, err := Write(repo, generated)
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "created" {
		t.Fatalf("status = %#v", result["status"])
	}
	if _, err := os.Stat(filepath.Join(repo, "workflows", "demo", "workflow.json")); err != nil {
		t.Fatalf("workflow.json not created: %v", err)
	}
	if _, err := Write(repo, generated); err == nil {
		t.Fatal("second write should refuse overwrite")
	}
}

func TestTraversalAndScratchPathsRejected(t *testing.T) {
	repo := t.TempDir()
	generated := Generated{
		Files:    []map[string]any{{"path": "../evil.md", "content": "bad\n"}},
		Metadata: map[string]any{"workflow_path": "../evil.md", "scaffold_root": ".."},
	}
	if _, err := Preview(repo, generated); err == nil || !strings.Contains(err.Error(), "path must not escape") {
		t.Fatalf("traversal preview error = %v", err)
	}
	generated.Files[0]["path"] = ".striatum/workflow.json"
	if _, err := Write(repo, generated); err == nil || !strings.Contains(err.Error(), ".git/.striatum") {
		t.Fatalf("scratch write error = %v", err)
	}
}

func TestMultiPhaseShapeEmitsV11PhasedGraph(t *testing.T) {
	spec := multiPhaseGeneratorSpec()
	generated, err := GenerateFromMap(spec)
	if err != nil {
		t.Fatal(err)
	}
	workflow := generated.Workflow
	if err := ValidateWorkflow(workflow); err != nil {
		t.Fatal(err)
	}
	if workflow["schema_version"] != "striatum.workflow.v1.1" {
		t.Fatalf("schema_version = %#v", workflow["schema_version"])
	}
	phases := listFrom(workflow["phases"])
	if len(phases) != 2 {
		t.Fatalf("phases = %#v", workflow["phases"])
	}
	phase1 := mapFrom(phases[0])
	if phase1["id"] != "phase_1_design" || phase1["name"] != "Design" || phase1["description"] != "Parallel design tracks" || phase1["color"] != "#6b7280" || phase1["synthesis_job_id"] != "phase_1_design__synthesis" {
		t.Fatalf("first phase = %#v", phase1)
	}
	phase2 := mapFrom(phases[1])
	if phase2["id"] != "phase_2_build" || phase2["name"] != "Build" || phase2["synthesis_job_id"] != "phase_2_build__synthesis" {
		t.Fatalf("second phase = %#v", phase2)
	}

	jobs := jobsByID(workflow["jobs"])
	if jobs["phase_1_design__python__draft"]["phase_id"] != "phase_1_design" {
		t.Fatalf("phase_1_design python draft = %#v", jobs["phase_1_design__python__draft"])
	}
	if jobs["phase_1_design__python__draft"]["parallel_group"] != "phase_1_design:python" {
		t.Fatalf("phase_1_design python draft = %#v", jobs["phase_1_design__python__draft"])
	}
	if jobs["phase_1_design__synthesis"]["type"] != "phase_synthesis" || jobs["phase_1_design__synthesis"]["phase_id"] != "phase_1_design" {
		t.Fatalf("phase_1_design synthesis = %#v", jobs["phase_1_design__synthesis"])
	}
	if jobs["phase_2_build__synthesis"]["type"] != "phase_synthesis" {
		t.Fatalf("phase_2_build synthesis = %#v", jobs["phase_2_build__synthesis"])
	}

	for _, edge := range [][2]string{
		{"phase_1_design__synthesis", "phase_2_build__python__draft"},
		{"phase_1_design__synthesis", "phase_2_build__docs__draft"},
		{"phase_1_design__python__draft", "phase_1_design__synthesis"},
	} {
		if !hasEdge(workflow["edges"], edge[0], edge[1]) {
			t.Fatalf("missing edge %v in %#v", edge, workflow["edges"])
		}
	}
}

func TestMultiPhaseInvalidCasesCarryFieldPath(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(map[string]any)
		fieldPath string
	}{
		{
			name: "missing phases",
			mutate: func(spec map[string]any) {
				spec["options"] = map[string]any{}
			},
			fieldPath: "spec.options.phases",
		},
		{
			name: "duplicate phase id",
			mutate: func(spec map[string]any) {
				phases := listFrom(mapFrom(spec["options"])["phases"])
				mapFrom(phases[1])["id"] = "phase_1_design"
			},
			fieldPath: "spec.options.phases[1].id",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec := multiPhaseGeneratorSpec()
			tc.mutate(spec)
			_, err := GenerateFromMap(spec)
			if err == nil {
				t.Fatal("expected error")
			}
			var genError *Error
			if !errors.As(err, &genError) {
				t.Fatalf("error type = %T, %v", err, err)
			}
			if genError.FieldPath != tc.fieldPath {
				t.Fatalf("field path = %q, want %q; err = %v", genError.FieldPath, tc.fieldPath, err)
			}
		})
	}
}

func TestImplementationPanelShapeUsesRoleAndAdversaryPacks(t *testing.T) {
	spec := implementationPanelGeneratorSpec()
	generated, err := GenerateFromMap(spec)
	if err != nil {
		t.Fatal(err)
	}
	workflow := generated.Workflow
	if err := ValidateWorkflow(workflow); err != nil {
		t.Fatal(err)
	}

	jobs := jobsByID(workflow["jobs"])
	if _, ok := jobs["propose_option_a"]; !ok {
		t.Fatalf("missing propose_option_a in %#v", jobs)
	}
	if _, ok := jobs["propose_option_b"]; !ok {
		t.Fatalf("missing propose_option_b in %#v", jobs)
	}
	if _, ok := jobs["propose_option_c"]; ok {
		t.Fatalf("unexpected propose_option_c in %#v", jobs)
	}
	if jobs["score_option_a"]["review_posture"] != "custom:operator_experience" {
		t.Fatalf("score_option_a = %#v", jobs["score_option_a"])
	}
	if jobs["review_dissent"]["role_id"] != "dissent_reviewer" {
		t.Fatalf("review_dissent = %#v", jobs["review_dissent"])
	}
	if mapFrom(workflow["coordinator"])["role_id"] != "problem_framer" {
		t.Fatalf("coordinator = %#v", workflow["coordinator"])
	}
	if mapFrom(workflow["parallelism"])["max_active_jobs"] != 2 {
		t.Fatalf("parallelism = %#v", workflow["parallelism"])
	}
	if strings.Join(generated.Metadata["role_packs"].([]string), ",") != "implementation_panel_roles" {
		t.Fatalf("role_packs = %#v", generated.Metadata["role_packs"])
	}
	if strings.Join(generated.Metadata["adversary_packs"].([]string), ",") != "operator_ergonomics" {
		t.Fatalf("adversary_packs = %#v", generated.Metadata["adversary_packs"])
	}
	if strings.Join(generated.Metadata["score_dimensions"].([]string), ",") != "operator_experience,recovery,documentation" {
		t.Fatalf("score_dimensions = %#v", generated.Metadata["score_dimensions"])
	}
	if len(generated.Warnings) == 0 || !strings.Contains(generated.Warnings[0], "high-artifact") {
		t.Fatalf("warnings = %#v", generated.Warnings)
	}
}

func TestGenerateUsesSharedAuthoringLintPayload(t *testing.T) {
	generated := mustGenerate(t, map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "review",
		"lane_set":         "author_reviewer",
		"workflow_id":      "demo",
		"name":             "Demo",
		"workflow_version": "2026-05-17",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": false},
		"scaffold_root":    "workflows/demo",
		"artifact_root":    "striatum/demo",
		"lanes": map[string]any{
			"author":   map[string]any{"adapter": "process", "command": []any{"true"}, "display_model": "codex-gpt-5"},
			"reviewer": map[string]any{"adapter": "process", "command": []any{"true"}, "display_model": "codex-gpt-5.1"},
		},
		"options": map[string]any{},
	})
	warnings, ok := generated.Lint["warnings"].([]map[string]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("lint warnings = %#v", generated.Lint["warnings"])
	}
	if warnings[0]["fingerprint"] == "" {
		t.Fatalf("lint warning missing fingerprint: %#v", warnings[0])
	}
	coverage := generated.Lint["coverage"].(map[string]any)
	if coverage["checks"] == nil {
		t.Fatalf("lint coverage did not come from workflowauthoring: %#v", coverage)
	}
}

func TestUpgradeAddPhasesPreviewWritesNothing(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "workflow.json")
	if err := os.WriteFile(path, mustJSON(t, parallelGroupWorkflow()), 0o644); err != nil {
		t.Fatal(err)
	}
	snapshot, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Upgrade(context.Background(), workflowUpgradeQueryer{}, "repo_1", repo, UpgradeOptions{Path: "workflow.json", AddPhases: true})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "would_update" || result["mode"] != "add_phases" {
		t.Fatalf("unexpected result = %#v", result)
	}
	phases, ok := result["phases_added"].([]map[string]any)
	if !ok || len(phases) != 2 {
		t.Fatalf("phases = %#v", result["phases_added"])
	}
	if phases[0]["id"] != "phase_design" || phases[0]["synthesis_job_id"] != "phase_design__synthesis" {
		t.Fatalf("first phase = %#v", phases[0])
	}
	if phases[1]["id"] != "phase_build" || phases[1]["synthesis_job_id"] != "phase_build__synthesis" {
		t.Fatalf("second phase = %#v", phases[1])
	}
	if !containsMap(result["jobs_relabelled"], map[string]any{"job_id": "design_python", "phase_id": "phase_design"}) {
		t.Fatalf("jobs_relabelled = %#v", result["jobs_relabelled"])
	}
	current, err := os.ReadFile(path)
	if err == nil {
		if string(current) != string(snapshot) {
			t.Fatal("preview rewrote workflow")
		}
	} else {
		t.Fatal(err)
	}
}

func TestUpgradeAddPhasesApplyRewritesToV11(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "workflow.json")
	if err := os.WriteFile(path, mustJSON(t, parallelGroupWorkflow()), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Upgrade(context.Background(), workflowUpgradeQueryer{}, "repo_1", repo, UpgradeOptions{Path: "workflow.json", AddPhases: true, Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "updated" {
		t.Fatalf("status = %#v", result["status"])
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk map[string]any
	if err := json.Unmarshal(raw, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk["schema_version"] != "striatum.workflow.v1.1" {
		t.Fatalf("schema_version = %#v", onDisk["schema_version"])
	}
	jobs := map[string]map[string]any{}
	for _, item := range listFrom(onDisk["jobs"]) {
		job := mapFrom(item)
		jobs[fmt.Sprint(job["id"])] = job
	}
	if jobs["design_python"]["phase_id"] != "phase_design" || jobs["build_python"]["phase_id"] != "phase_build" {
		t.Fatalf("phase ids = %#v %#v", jobs["design_python"], jobs["build_python"])
	}
	if jobs["phase_design__synthesis"]["type"] != "phase_synthesis" || jobs["phase_build__synthesis"]["type"] != "phase_synthesis" {
		t.Fatalf("synthesis jobs = %#v %#v", jobs["phase_design__synthesis"], jobs["phase_build__synthesis"])
	}
	edges := map[[2]string]struct{}{}
	for _, item := range listFrom(onDisk["edges"]) {
		edge := mapFrom(item)
		edges[[2]string{fmt.Sprint(edge["from"]), fmt.Sprint(edge["to"])}] = struct{}{}
	}
	for _, edge := range [][2]string{{"design_python", "phase_design__synthesis"}, {"phase_design__synthesis", "build_python"}, {"build_python", "phase_build__synthesis"}} {
		if _, ok := edges[edge]; !ok {
			t.Fatalf("missing edge %v in %#v", edge, edges)
		}
	}
}

func TestUpgradeAddPhasesPreviewReportsRunningRuns(t *testing.T) {
	repo := t.TempDir()
	path := filepath.Join(repo, "workflow.json")
	if err := os.WriteFile(path, mustJSON(t, parallelGroupWorkflow()), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Upgrade(context.Background(), workflowUpgradeQueryer{running: []string{"run_active"}}, "repo_1", repo, UpgradeOptions{Path: "workflow.json", AddPhases: true})
	if err != nil {
		t.Fatal(err)
	}
	if result["status"] != "would_refuse_running" {
		t.Fatalf("result = %#v", result)
	}
}

type workflowUpgradeQueryer struct {
	running []string
}

func (q workflowUpgradeQueryer) Query(context.Context, string, ...any) (pgx.Rows, error) {
	rows := []string{}
	rows = append(rows, q.running...)
	return &workflowUpgradeRows{rows: rows, index: -1}, nil
}

type workflowUpgradeRows struct {
	rows  []string
	index int
}

func (r *workflowUpgradeRows) Close()     {}
func (r *workflowUpgradeRows) Err() error { return nil }
func (r *workflowUpgradeRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}
func (r *workflowUpgradeRows) FieldDescriptions() []pgconn.FieldDescription {
	return []pgconn.FieldDescription{{Name: "run_id"}}
}
func (r *workflowUpgradeRows) Next() bool {
	r.index++
	return r.index < len(r.rows)
}
func (r *workflowUpgradeRows) Scan(dest ...any) error {
	if len(dest) != 1 {
		return errors.New("expected one scan destination")
	}
	ptr, ok := dest[0].(*string)
	if !ok {
		return errors.New("expected string scan destination")
	}
	*ptr = r.rows[r.index]
	return nil
}
func (r *workflowUpgradeRows) Values() ([]any, error) {
	if r.index < 0 || r.index >= len(r.rows) {
		return nil, errors.New("row index out of range")
	}
	return []any{r.rows[r.index]}, nil
}
func (r *workflowUpgradeRows) RawValues() [][]byte { return nil }
func (r *workflowUpgradeRows) Conn() *pgx.Conn     { return nil }

var _ pgx.Rows = (*workflowUpgradeRows)(nil)

func mustGenerate(t *testing.T, spec map[string]any) Generated {
	t.Helper()
	generated, err := GenerateFromMap(spec)
	if err != nil {
		t.Fatal(err)
	}
	return generated
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(body, '\n')
}

func parallelGroupWorkflow() map[string]any {
	workflow := map[string]any{
		"schema_version":   "striatum.workflow.v1",
		"workflow_id":      "demo",
		"workflow_version": "2026-05-17",
		"name":             "Demo",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/demo", "allow_dirty": false},
		"coordinator":      map[string]any{"role_id": "author", "lane_id": "claude_code"},
		"lanes": map[string]any{
			"claude_code": map[string]any{
				"adapter":       "process",
				"display_model": "Claude",
				"command":       []string{"claude", "--print"},
				"capabilities":  []string{"write"},
			},
		},
		"roles":        map[string]any{"author": map[string]any{"definition_path": "roles/author.md"}, "reviewer": map[string]any{"definition_path": "roles/reviewer.md"}},
		"context_docs": []any{},
		"parallelism":  map[string]any{"mode": "declared", "max_active_jobs": 1, "require_disjoint_write_scopes": true},
		"jobs": []any{
			map[string]any{
				"id":             "design_python",
				"type":           "draft",
				"title":          "Python Design",
				"role_id":        "author",
				"lane_id":        "claude_code",
				"parallel_group": "design_python",
				"objective":      "Draft design.",
				"task_prompt":    map[string]any{"path": "prompts/design.md"},
				"write_scope":    map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []string{"scratch/design/"}, "forbidden_paths": []string{".striatum/"}},
				"expected_artifacts": []any{
					map[string]any{"logical_name": "design", "kind": "handoff", "path": "scratch/design/DESIGN.md", "required": true},
				},
			},
			map[string]any{
				"id":             "build_python",
				"type":           "build",
				"title":          "Python Build",
				"role_id":        "author",
				"lane_id":        "claude_code",
				"parallel_group": "build_python",
				"objective":      "Build implementation.",
				"task_prompt":    map[string]any{"path": "prompts/build.md"},
				"write_scope":    map[string]any{"mode": "repo_write", "repo_write": true, "allowed_paths": []string{"scratch/build/"}, "forbidden_paths": []string{".striatum/"}},
				"expected_artifacts": []any{
					map[string]any{"logical_name": "build", "kind": "handoff", "path": "scratch/build/BUILD.md", "required": true},
				},
			},
		},
		"edges":  []any{map[string]any{"from": "design_python", "to": "build_python", "on": "completed"}},
		"cycles": []any{},
	}
	return workflow
}

func multiPhaseGeneratorSpec() map[string]any {
	return map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "multi_phase",
		"lane_set":         "author_reviewer",
		"workflow_id":      "multi_phase-test",
		"name":             "multi_phase test",
		"workflow_version": "2026-05-12",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/multi_phase", "allow_dirty": false},
		"scaffold_root":    "workflows/multi_phase",
		"artifact_root":    "striatum/multi_phase",
		"lanes": map[string]any{
			"author":   map[string]any{"command": []any{"author", "run"}, "display_model": "Author"},
			"reviewer": map[string]any{"command": []any{"reviewer", "run"}, "display_model": "Reviewer"},
		},
		"options": map[string]any{
			"phases": []any{
				map[string]any{
					"id":          "phase_1_design",
					"name":        "Design",
					"description": "Parallel design tracks",
					"color":       "#6b7280",
					"tracks": []any{
						map[string]any{"id": "python", "shape": "minimal", "lane_id": "author"},
						map[string]any{"id": "docs", "shape": "review", "lane_id": "author"},
					},
					"synthesis_lane_id": "reviewer",
				},
				map[string]any{
					"id":   "phase_2_build",
					"name": "Build",
					"tracks": []any{
						map[string]any{"id": "python", "shape": "minimal", "lane_id": "author"},
						map[string]any{"id": "docs", "shape": "minimal", "lane_id": "author"},
					},
					"synthesis_lane_id": "reviewer",
				},
			},
		},
	}
}

func implementationPanelGeneratorSpec() map[string]any {
	return map[string]any{
		"schema_version":   GeneratorSchemaVersion,
		"shape":            "implementation_panel",
		"lane_set":         "multi_review",
		"workflow_id":      "implementation_panel-test",
		"name":             "implementation_panel test",
		"workflow_version": "2026-05-12",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/implementation_panel", "allow_dirty": false},
		"scaffold_root":    "workflows/implementation_panel",
		"artifact_root":    "striatum/implementation_panel",
		"lanes": map[string]any{
			"author":     map[string]any{"command": []any{"author", "run"}, "display_model": "Author"},
			"reviewer_1": map[string]any{"command": []any{"reviewer1", "run"}, "display_model": "Reviewer 1"},
			"reviewer_2": map[string]any{"command": []any{"reviewer2", "run"}, "display_model": "Reviewer 2"},
		},
		"options": map[string]any{
			"role_packs":      []any{"implementation_panel_roles"},
			"adversary_packs": []any{"operator_ergonomics"},
			"proposal_count":  2,
		},
	}
}

func containsMap(value any, expected map[string]any) bool {
	for _, item := range listFrom(value) {
		actual := mapFrom(item)
		matches := true
		for key, expectedValue := range expected {
			if actual[key] != expectedValue {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func jobsByID(value any) map[string]map[string]any {
	jobs := map[string]map[string]any{}
	for _, item := range listFrom(value) {
		job := mapFrom(item)
		jobs[fmt.Sprint(job["id"])] = job
	}
	return jobs
}

func hasEdge(value any, from, to string) bool {
	for _, item := range listFrom(value) {
		edge := mapFrom(item)
		if edge["from"] == from && edge["to"] == to {
			return true
		}
	}
	return false
}
