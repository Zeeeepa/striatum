package reads

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/installers"
	"github.com/halbritt/striatum/go/pkg/rpc"
	"github.com/jackc/pgx/v5"
)

type doctorFakeRow struct{}

func (doctorFakeRow) Scan(dest ...any) error {
	return nil
}

type doctorFakeRunner struct {
	scalarQueries []string
}

func (r *doctorFakeRunner) Exec(_ context.Context, _ string, _ ...any) error {
	return nil
}

func (r *doctorFakeRunner) QueryRow(_ context.Context, _ string, _ ...any) db.Row {
	return doctorFakeRow{}
}

func (r *doctorFakeRunner) QueryScalar(_ context.Context, sql string, _ ...any) (string, error) {
	r.scalarQueries = append(r.scalarQueries, sql)
	if strings.Contains(sql, "substrate_version") {
		return "8", nil
	}
	return "", nil
}

func (r *doctorFakeRunner) BeginTx(_ context.Context) (db.TxRunner, error) {
	return nil, errors.New("not implemented")
}

func TestHandleDoctorReadsSubstrateVersionFromSchemaMetaKey(t *testing.T) {
	runner := &doctorFakeRunner{}

	result, err := HandleDoctor(context.Background(), runner, rpc.Envelope{Params: map[string]any{}})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}

	if result["schema_version"] != 8 {
		t.Fatalf("schema_version = %v, want 8", result["schema_version"])
	}
	if result["ok"] != true {
		t.Fatalf("ok = %v, want true; problems=%v", result["ok"], result["problems"])
	}
	readScope, ok := result["pg_read_scope"].(map[string]any)
	if !ok {
		t.Fatalf("doctor result missing pg_read_scope block: %#v", result["pg_read_scope"])
	}
	if readScope["posture"] != pgReadScopeBroadRuntimeSelect {
		t.Fatalf("pg_read_scope posture = %v, want %s", readScope["posture"], pgReadScopeBroadRuntimeSelect)
	}
	// substrate_version must be the FIRST scalar query doctor issues. (RFC 0107
	// added the daemon-global principals scope query, so the total scalar-query
	// count is no longer exactly one.)
	if len(runner.scalarQueries) == 0 || !strings.Contains(runner.scalarQueries[0], "substrate_version") {
		t.Fatalf("doctor did not read schema_meta substrate_version first: %#v", runner.scalarQueries)
	}
}

type doctorSkillsFakeRunner struct {
	doctorFakeRunner
	repoRoot string
}

func (r *doctorSkillsFakeRunner) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.repositories"):
		return dashboardAllRowsFromMaps([]map[string]any{{"repo_root": r.repoRoot}}), nil
	case strings.Contains(sql, "COUNT(*) AS c"):
		return dashboardAllRowsFromMaps([]map[string]any{{"c": int64(0)}}), nil
	default:
		return dashboardAllRowsFromMaps(nil), nil
	}
}

func TestHandleDoctorWarnsOnStaleGeneratedSkills(t *testing.T) {
	home := t.TempDir()
	target := t.TempDir()
	t.Setenv("HOME", home)
	if _, err := installers.InstallSkills(installers.SkillsParams{
		Target: target, Home: home, Profile: "claude_code", Scope: "project", Namespace: "striatum-", Version: "2.8.0",
	}); err != nil {
		t.Fatal(err)
	}

	oldVersion := packageStriatumVersion
	packageStriatumVersion = "2.24.0"
	defer func() { packageStriatumVersion = oldVersion }()

	result, err := HandleDoctor(context.Background(), &doctorSkillsFakeRunner{repoRoot: target}, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_1"},
	})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	skills, ok := result["skills"].(map[string]any)
	if !ok || skills["checked"] != true {
		t.Fatalf("skills doctor block = %#v", result["skills"])
	}
	warnings := strings.Join(result["warnings"].([]string), "\n")
	for _, want := range []string{"skills_outdated", "2.8.0", "2.24.0", "skills install"} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q:\n%s", want, warnings)
		}
	}
	if _, err := os.Stat(filepath.Join(target, ".claude", "skills", "striatum-workflow", "SKILL.md")); err != nil {
		t.Fatalf("stale skill should remain for doctor-only warning: %v", err)
	}
}

type doctorWorktreeAnchorFakeRunner struct {
	doctorFakeRunner
	rows []map[string]any
}

func (r *doctorWorktreeAnchorFakeRunner) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	switch {
	case strings.Contains(sql, "FROM striatumd.job_worktrees"):
		return dashboardAllRowsFromMaps(r.rows), nil
	case strings.Contains(sql, "COUNT(*) AS c"):
		return dashboardAllRowsFromMaps([]map[string]any{{"c": int64(0)}}), nil
	default:
		return dashboardAllRowsFromMaps(nil), nil
	}
}

func TestDoctorFlagsUnreachableWorktreeHead(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not on PATH: %v", err)
	}
	repoRoot := t.TempDir()
	baseSHA := readsGitInit(t, repoRoot)
	runID := "run_doctor_anchor"
	jobID := "job_doctor_anchor"
	worktreeID := "wt_doctor_anchor"
	runBranch := "wf/doctor-anchor"
	worktreeRel := filepath.ToSlash(filepath.Join(".striatum", "worktrees", worktreeID))
	worktreeRoot := filepath.Join(repoRoot, filepath.FromSlash(worktreeRel))

	readsGitRun(t, repoRoot, "branch", runBranch, baseSHA)
	readsGitRun(t, repoRoot, "worktree", "add", "--detach", worktreeRoot, runBranch)
	if err := os.WriteFile(filepath.Join(worktreeRoot, "worktree.txt"), []byte("worktree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	readsGitRun(t, worktreeRoot, "add", "worktree.txt")
	readsGitRun(t, worktreeRoot, "commit", "-q", "-m", "worktree change")
	worktreeHead := readsGitRevParse(t, worktreeRoot, "HEAD")

	runner := &doctorWorktreeAnchorFakeRunner{rows: []map[string]any{{
		"worktree_id":     worktreeID,
		"run_id":          runID,
		"job_id":          jobID,
		"lease_id":        "lease_doctor_anchor",
		"base_branch":     runBranch,
		"branch_name":     runBranch,
		"repo_root":       repoRoot,
		"worktree_path":   worktreeRel,
		"state":           "active",
		"workflow_job_id": "author_draft",
		"job_state":       "completed",
	}}}

	result, err := HandleDoctor(context.Background(), runner, rpc.Envelope{
		Params: map[string]any{"repository_id": "repo_doctor_anchor", "verbose": true},
	})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	if result["ok"] == true {
		t.Fatalf("doctor ok = true, want false")
	}
	problems := strings.Join(result["problems"].([]string), "\n")
	for _, want := range []string{"worktree_head_unreachable." + worktreeID, "job_completed_without_anchor." + jobID, worktreeHead} {
		if !strings.Contains(problems, want) {
			t.Fatalf("doctor problems missing %q:\n%s", want, problems)
		}
	}
	records := result["problem_records"].([]map[string]any)
	checks := map[string]bool{}
	for _, record := range records {
		checks[stringFrom(record, "check")] = true
	}
	if !checks["worktree_head_unreachable"] || !checks["job_completed_without_anchor"] {
		t.Fatalf("problem_records = %#v", records)
	}
}
