package reads

import (
	"context"
	"errors"
	"os"
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
