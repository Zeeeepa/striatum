package workflowauthoring

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFileValidatePlanAndGraph(t *testing.T) {
	repo := t.TempDir()
	writeWorkflow(t, filepath.Join(repo, "workflow.json"), validWorkflow())

	workflow, sourcePath, err := LoadFile(repo, "workflow.json")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if sourcePath != "workflow.json" {
		t.Fatalf("sourcePath = %q", sourcePath)
	}

	plan, err := Plan(workflow)
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	summary := plan["summary"].(map[string]any)
	if summary["jobs"] != 2 || summary["edges"] != 1 || summary["claim_steps"] != 2 {
		t.Fatalf("summary = %#v", summary)
	}
	claimOrder := plan["claim_order"].([]map[string]any)
	firstWave := claimOrder[0]["claimable"].([]map[string]any)
	if firstWave[0]["job_id"] != "draft" {
		t.Fatalf("claim_order = %#v", claimOrder)
	}

	graph, err := Graph(workflow, "json")
	if err != nil {
		t.Fatalf("Graph json: %v", err)
	}
	if graph["workflow_id"] != "go-authoring" {
		t.Fatalf("graph header = %#v", graph)
	}
	mermaid, err := Graph(workflow, "mermaid")
	if err != nil {
		t.Fatalf("Graph mermaid: %v", err)
	}
	source := mermaid["source"].(string)
	for _, needle := range []string{"flowchart TD", "draft<br/>draft author/codex", "-->|completed|"} {
		if !strings.Contains(source, needle) {
			t.Fatalf("mermaid source missing %q:\n%s", needle, source)
		}
	}
	dot, err := Graph(workflow, "dot")
	if err != nil {
		t.Fatalf("Graph dot: %v", err)
	}
	if !strings.Contains(dot["source"].(string), "digraph striatum_workflow") {
		t.Fatalf("dot source = %s", dot["source"])
	}
}

func TestLoadFileRejectsTraversalAndYaml(t *testing.T) {
	repo := t.TempDir()
	parent := filepath.Dir(repo)
	outside := filepath.Join(parent, "workflow.yaml")
	if err := os.WriteFile(outside, []byte("{}"), 0o600); err != nil {
		t.Fatalf("write outside workflow: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(outside) })

	if _, _, err := LoadFile(repo, "../workflow.yaml"); err == nil || !strings.Contains(err.Error(), "inside the repository") {
		t.Fatalf("LoadFile traversal error = %v", err)
	}
	writeWorkflow(t, filepath.Join(repo, "workflow.yaml"), validWorkflow())
	if _, _, err := LoadFile(repo, "workflow.yaml"); err == nil || !strings.Contains(err.Error(), "must be JSON") {
		t.Fatalf("LoadFile yaml error = %v", err)
	}
}

func TestValidateReturnsAuthoringErrors(t *testing.T) {
	workflow := validWorkflow()
	jobs := workflow["jobs"].([]any)
	review := jobs[1].(map[string]any)
	review["role_id"] = "missing"
	err := Validate(workflow)
	if err == nil || !strings.Contains(err.Error(), "unknown role") {
		t.Fatalf("Validate unknown role error = %v", err)
	}

	workflow = validWorkflow()
	jobs = workflow["jobs"].([]any)
	review = jobs[1].(map[string]any)
	review["expected_artifacts"] = []any{map[string]any{
		"logical_name": "review",
		"kind":         "finding",
		"path":         "../REVIEW.md",
		"required":     true,
	}}
	err = Validate(workflow)
	if err == nil || !strings.Contains(err.Error(), "invalid artifact path") {
		t.Fatalf("Validate artifact path error = %v", err)
	}
}

func TestValidateRejectsInvalidLaneModel(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{"adapter": "process", "command": []any{"true"}, "model": 123} // Invalid model type (int)
	err := Validate(workflow)
	if err == nil || !strings.Contains(err.Error(), "model must be a non-empty string") {
		t.Fatalf("Validate invalid lane model error = %v", err)
	}

	workflow = validWorkflow()
	lanes = workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{"adapter": "process", "command": []any{"true"}, "model": ""} // Invalid empty model
	err = Validate(workflow)
	if err == nil || !strings.Contains(err.Error(), "model must be a non-empty string") {
		t.Fatalf("Validate invalid lane model error = %v", err)
	}

	workflow = validWorkflow()
	lanes = workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{"adapter": "process", "command": []any{"true"}, "model": "gpt-4"} // Valid model
	err = Validate(workflow)
	if err != nil {
		t.Fatalf("Validate valid lane model error = %v", err)
	}
}

func TestValidateUsesSharedArtifactKindsAndLaneConstraints(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{
		"adapter": "process",
		"command": []any{"true"},
		"constraints": map[string]any{
			"network":     "forbidden",
			"repo_scope":  "local_only",
			"transcripts": "off",
		},
		"required_enforcement": map[string]any{
			"network":     "advisory_strict",
			"repo_scope":  "advisory_strict",
			"transcripts": "enforced",
		},
	}
	jobs := workflow["jobs"].([]any)
	draft := jobs[0].(map[string]any)
	draft["expected_artifacts"] = []any{map[string]any{
		"logical_name": "brief",
		"kind":         "operator_brief",
		"path":         "src/BRIEF.md",
		"required":     true,
	}}
	if err := Validate(workflow); err != nil {
		t.Fatalf("Validate shared artifact kind and constraints: %v", err)
	}

	lanes["codex"].(map[string]any)["required_enforcement"] = map[string]any{"network": "enforced"}
	err := Validate(workflow)
	if err == nil || !strings.Contains(err.Error(), "adapter provides") {
		t.Fatalf("Validate invalid required_enforcement error = %v", err)
	}
}

func TestLintReportsReviewDiversityFindingsWithFingerprints(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{
		"adapter":       "process",
		"command":       []any{"true"},
		"display_model": "codex-gpt-5",
	}
	jobs := workflow["jobs"].([]any)
	draft := jobs[0].(map[string]any)
	draft["write_scope"] = map[string]any{
		"mode":            "repo_write",
		"repo_write":      true,
		"allowed_paths":   []any{"."},
		"forbidden_paths": []any{".striatum/"},
	}
	review := jobs[1].(map[string]any)
	delete(review, "fresh_session_required")

	payload, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if payload["valid"] != true {
		t.Fatalf("lint payload valid = %#v", payload["valid"])
	}
	warnings := payload["warnings"].([]map[string]any)
	rules := map[string]bool{}
	for _, warning := range warnings {
		rules[warning["rule"].(string)] = true
		if warning["fingerprint"] == "" {
			t.Fatalf("warning missing fingerprint: %#v", warning)
		}
	}
	for _, rule := range []string{
		"same_model_review_pair",
		"same_model_revision_cycle",
		"review_without_fresh_context",
		"broad_write_scope",
		"repo_write_without_worktree_isolation",
	} {
		if !rules[rule] {
			t.Fatalf("lint rules missing %s: %#v", rule, rules)
		}
	}
	coverage := payload["coverage"].(map[string]any)
	if coverage["level"] == "strong" {
		t.Fatalf("coverage unexpectedly strong: %#v", coverage)
	}
}

func TestLintReportsSameModelCollaborationAdjudicator(t *testing.T) {
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["codex"] = map[string]any{
		"adapter":       "process",
		"command":       []any{"true"},
		"display_model": "codex-gpt-5",
	}
	roles := workflow["roles"].(map[string]any)
	roles["adjudicator"] = map[string]any{}
	jobs := workflow["jobs"].([]any)
	jobs = append(jobs, map[string]any{
		"id":                     "adjudicate",
		"type":                   "review",
		"role_id":                "adjudicator",
		"lane_id":                "codex",
		"fresh_session_required": true,
		"write_scope": map[string]any{
			"mode":            "review_only_artifact",
			"allowed_paths":   []any{"reviews/adjudicator/"},
			"forbidden_paths": []any{".striatum/"},
		},
		"expected_artifacts": []any{map[string]any{
			"logical_name": "collaboration_ledger",
			"kind":         "collaboration_ledger",
			"path":         "reviews/adjudicator/COLLABORATION_LEDGER.md",
			"required":     true,
		}},
	})
	workflow["jobs"] = jobs
	edges := workflow["edges"].([]any)
	workflow["edges"] = append(edges, map[string]any{"from": "draft", "to": "adjudicate", "on": "completed"})

	payload, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	warnings := payload["warnings"].([]map[string]any)
	found := false
	for _, warning := range warnings {
		if warning["rule"] == "same_model_adjudicator_pair" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing same_model_adjudicator_pair warning: %#v", warnings)
	}
	coverage := payload["coverage"].(map[string]any)
	checks := coverage["checks"].([]map[string]any)
	independenceFailed := false
	for _, check := range checks {
		if check["id"] == "reviewer_independence" && check["passed"] == false {
			independenceFailed = true
		}
	}
	if !independenceFailed {
		t.Fatalf("coverage = %#v", coverage)
	}
}

func lintRuleSet(t *testing.T, workflow map[string]any) map[string]bool {
	t.Helper()
	payload, err := Lint(workflow)
	if err != nil {
		t.Fatalf("Lint: %v", err)
	}
	if payload["valid"] != true {
		t.Fatalf("lint payload valid = %#v", payload["valid"])
	}
	rules := map[string]bool{}
	for _, warning := range payload["warnings"].([]map[string]any) {
		rules[warning["rule"].(string)] = true
	}
	return rules
}

func TestLintFlagsAgyOneShotPipeLane(t *testing.T) {
	// Shell-shim one-shot agy lane without agent_loop must be flagged.
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["agy"] = map[string]any{
		"adapter":       "process",
		"display_model": "Antigravity",
		"command":       []any{"sh", "-c", "IFS= read -r prompt; exec agy --dangerously-skip-permissions --print-timeout 30m --print \"$prompt\" </dev/null"},
	}
	if !lintRuleSet(t, workflow)["agy_one_shot_pipe_lane"] {
		t.Fatalf("expected agy_one_shot_pipe_lane warning for shell-shim one-shot agy lane")
	}

	// Direct argv one-shot agy lane must be flagged too.
	workflow = validWorkflow()
	lanes = workflow["lanes"].(map[string]any)
	lanes["agy"] = map[string]any{
		"adapter": "process",
		"command": []any{"agy", "--print", "$prompt"},
	}
	if !lintRuleSet(t, workflow)["agy_one_shot_pipe_lane"] {
		t.Fatalf("expected agy_one_shot_pipe_lane warning for direct one-shot agy lane")
	}
}

func TestLintAllowsAgyAgentLoopAndOtherPipeLanes(t *testing.T) {
	// agy as an agent-loop lane must NOT be flagged.
	workflow := validWorkflow()
	lanes := workflow["lanes"].(map[string]any)
	lanes["agy"] = map[string]any{
		"adapter":              "process",
		"command":              []any{"agy", "--dangerously-skip-permissions"},
		"adapter_capabilities": map[string]any{"agent_loop": true},
	}
	if lintRuleSet(t, workflow)["agy_one_shot_pipe_lane"] {
		t.Fatalf("agy agent-loop lane must not be flagged as one-shot pipe")
	}

	// claude/codex one-shot pipe lanes must NOT be flagged (they self-claim).
	workflow = validWorkflow()
	lanes = workflow["lanes"].(map[string]any)
	lanes["claude_code"] = map[string]any{
		"adapter": "process",
		"command": []any{"claude", "--model", "claude-opus-4-7", "--print"},
	}
	lanes["codex"] = map[string]any{
		"adapter": "process",
		"command": []any{"sh", "-c", "IFS= read -r prompt; exec codex exec --model gpt-5.5 \"$prompt\" </dev/null"},
	}
	if lintRuleSet(t, workflow)["agy_one_shot_pipe_lane"] {
		t.Fatalf("claude/codex one-shot pipe lanes must not be flagged")
	}
}

func TestWorkflowFingerprintMatchesCanonicalJSON(t *testing.T) {
	workflow := validWorkflow()
	fingerprint, err := WorkflowFingerprint(workflow)
	if err != nil {
		t.Fatalf("WorkflowFingerprint: %v", err)
	}
	raw, err := json.Marshal(workflow)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	sum := sha256.Sum256(raw)
	if fingerprint != hex.EncodeToString(sum[:]) {
		t.Fatalf("fingerprint = %s, want canonical JSON sha", fingerprint)
	}
}

func writeWorkflow(t *testing.T, path string, workflow map[string]any) {
	t.Helper()
	raw, err := json.Marshal(workflow)
	if err != nil {
		t.Fatalf("marshal workflow: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write workflow: %v", err)
	}
}

func validWorkflow() map[string]any {
	return map[string]any{
		"schema_version":   SchemaV1,
		"workflow_id":      "go-authoring",
		"workflow_version": "1",
		"name":             "Go Authoring",
		"branch":           map[string]any{"mode": "confirm", "suggested_name": "striatum/go-authoring"},
		"coordinator":      map[string]any{"role_id": "author", "lane_id": "codex"},
		"lanes":            map[string]any{"codex": map[string]any{"adapter": "process", "command": []any{"true"}}},
		"roles":            map[string]any{"author": map[string]any{}, "reviewer": map[string]any{}},
		"context_docs":     []any{},
		"parallelism":      map[string]any{"mode": "declared", "max_active_jobs": 1},
		"edges":            []any{map[string]any{"from": "draft", "to": "review", "on": "completed"}},
		"cycles":           []any{map[string]any{"from": "review", "to": "draft", "on_verdict": "needs_revision", "max_iterations": 1}},
		"jobs": []any{
			map[string]any{
				"id":      "draft",
				"type":    "draft",
				"role_id": "author",
				"lane_id": "codex",
				"write_scope": map[string]any{
					"mode":            "repo_write",
					"allowed_paths":   []any{"src/"},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{map[string]any{
					"logical_name": "draft",
					"kind":         "handoff",
					"path":         "src/draft.md",
					"required":     true,
				}},
			},
			map[string]any{
				"id":                     "review",
				"type":                   "review",
				"role_id":                "reviewer",
				"lane_id":                "codex",
				"fresh_session_required": true,
				"write_scope": map[string]any{
					"mode":            "review_only_artifact",
					"allowed_paths":   []any{"reviews/"},
					"forbidden_paths": []any{".striatum/"},
				},
				"expected_artifacts": []any{map[string]any{
					"logical_name": "review",
					"kind":         "finding",
					"path":         "reviews/REVIEW.md",
					"required":     true,
				}},
			},
		},
	}
}
