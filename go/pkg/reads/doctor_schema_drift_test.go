package reads

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

// schemaDriftDoctorRunner is a canned Runner that drives schemaDriftDoctorBlock
// (and HandleDoctor through it) by answering the two queries LiveFingerprint
// issues: the to_regclass presence probe for schema_state and the singleton-row
// fingerprint SELECT. substrate_version and the owner_bundle_meta probe keep the
// rest of HandleDoctor happy (no owner-bundle shortfall noise).
type schemaDriftDoctorRunner struct {
	statePresent bool
	live         string
}

func (r *schemaDriftDoctorRunner) Exec(_ context.Context, _ string, _ ...any) error { return nil }

func (r *schemaDriftDoctorRunner) QueryRow(_ context.Context, _ string, _ ...any) db.Row {
	return doctorFakeRow{}
}

func (r *schemaDriftDoctorRunner) QueryScalar(_ context.Context, sql string, _ ...any) (string, error) {
	switch {
	case strings.Contains(sql, "substrate_version"):
		return "43", nil
	case strings.Contains(sql, "to_regclass") && strings.Contains(sql, "schema_state"):
		if r.statePresent {
			return "true", nil
		}
		return "false", nil
	case strings.Contains(sql, "FROM striatumd.schema_state"):
		return r.live, nil
	case strings.Contains(sql, "to_regclass") && strings.Contains(sql, "owner_bundle_meta"):
		// Report owner-bundle in sync so HandleDoctor stays clean for these tests.
		return "true", nil
	case strings.Contains(sql, "MAX(version)"):
		return strconv.Itoa(db.RequiredOwnerBundleVersion), nil
	default:
		return "", nil
	}
}

func (r *schemaDriftDoctorRunner) BeginTx(_ context.Context) (db.TxRunner, error) {
	return nil, errors.New("not implemented")
}

// TestSchemaDriftDoctorBlockDrift covers the RFC 0142 P3 drift case: a recorded
// live fingerprint that diverges from the binary's expected fingerprint reports
// status "drift" and emits a WARNING (not a hard problem). In default shadow mode
// the warning says the daemon logs and continues.
func TestSchemaDriftDoctorBlockDrift(t *testing.T) {
	t.Setenv(db.EnvSchemaDriftRefuse, "")
	runner := &schemaDriftDoctorRunner{
		statePresent: true,
		live:         "0000000000000000000000000000000000000000000000000000000000000000",
	}
	block, warnings := schemaDriftDoctorBlock(context.Background(), runner)
	if block["status"] != "drift" {
		t.Fatalf("status = %v; want drift", block["status"])
	}
	if block["drift"] != true {
		t.Fatalf("drift = %v; want true", block["drift"])
	}
	if len(warnings) != 1 {
		t.Fatalf("want exactly one warning, got %v", warnings)
	}
	for _, needle := range []string{"schema_drift", "shadow mode", db.EnvSchemaDriftRefuse} {
		if !strings.Contains(warnings[0], needle) {
			t.Fatalf("warning missing %q; got: %s", needle, warnings[0])
		}
	}
}

// TestSchemaDriftDoctorBlockDriftRefuseEnabled: with enforcement on, the warning
// states the daemon WILL refuse on the next restart.
func TestSchemaDriftDoctorBlockDriftRefuseEnabled(t *testing.T) {
	t.Setenv(db.EnvSchemaDriftRefuse, "1")
	runner := &schemaDriftDoctorRunner{
		statePresent: true,
		live:         "0000000000000000000000000000000000000000000000000000000000000000",
	}
	block, warnings := schemaDriftDoctorBlock(context.Background(), runner)
	if block["status"] != "drift" || block["refuse_enabled"] != true {
		t.Fatalf("block = %#v; want drift + refuse_enabled true", block)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "WILL refuse to serve") {
		t.Fatalf("warning must say the daemon will refuse; got: %v", warnings)
	}
}

// TestSchemaDriftDoctorBlockHealthy covers in-sync and unrecorded: neither emits a
// warning, and a correctly-recorded fingerprint reports in_sync.
func TestSchemaDriftDoctorBlockHealthy(t *testing.T) {
	t.Setenv(db.EnvSchemaDriftRefuse, "")
	expected, err := db.ExpectedFingerprint()
	if err != nil {
		t.Fatalf("ExpectedFingerprint: %v", err)
	}
	cases := []struct {
		name         string
		statePresent bool
		live         string
		wantStatus   string
	}{
		{"in_sync", true, expected, "in_sync"},
		{"unrecorded_absent_table", false, "", "unrecorded"},
		{"unrecorded_empty_row", true, "", "unrecorded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &schemaDriftDoctorRunner{statePresent: tc.statePresent, live: tc.live}
			block, warnings := schemaDriftDoctorBlock(context.Background(), runner)
			if len(warnings) != 0 {
				t.Fatalf("healthy case emitted warnings: %v", warnings)
			}
			if block["status"] != tc.wantStatus {
				t.Fatalf("status = %v; want %s", block["status"], tc.wantStatus)
			}
			if block["drift"] != false {
				t.Fatalf("drift = %v; want false", block["drift"])
			}
		})
	}
}

// TestHandleDoctorSurfacesSchemaDrift confirms the block flows through the full
// HandleDoctor surface as a WARNING (drift does NOT flip ok=false — it is advisory
// in shadow mode) and the schema_drift block is present in the result.
func TestHandleDoctorSurfacesSchemaDrift(t *testing.T) {
	t.Setenv(db.EnvSchemaDriftRefuse, "")
	runner := &schemaDriftDoctorRunner{
		statePresent: true,
		live:         "0000000000000000000000000000000000000000000000000000000000000000",
	}
	result, err := HandleDoctor(context.Background(), runner, rpc.Envelope{Params: map[string]any{}})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	block, ok := result["schema_drift"].(map[string]any)
	if !ok {
		t.Fatalf("doctor result missing schema_drift block: %#v", result["schema_drift"])
	}
	if block["status"] != "drift" {
		t.Fatalf("schema_drift.status = %v; want drift", block["status"])
	}
	warnings, _ := result["warnings"].([]string)
	found := false
	for _, w := range warnings {
		if strings.HasPrefix(w, "schema_drift") {
			found = true
		}
	}
	if !found {
		t.Fatalf("doctor warnings missing the schema drift warning: %v", warnings)
	}
}
