package reads

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
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
	// substrate_version must be the FIRST scalar query doctor issues. (RFC 0107
	// added the daemon-global principals scope query, so the total scalar-query
	// count is no longer exactly one.)
	if len(runner.scalarQueries) == 0 || !strings.Contains(runner.scalarQueries[0], "substrate_version") {
		t.Fatalf("doctor did not read schema_meta substrate_version first: %#v", runner.scalarQueries)
	}
}
