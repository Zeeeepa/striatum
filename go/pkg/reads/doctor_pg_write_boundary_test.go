package reads

import (
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
)

// TestPgWriteBoundaryDoctorBlock pins the default posture: the daemon→PG write
// boundary reports "none" (no phase has closed a surface), the active audit hash
// format, and the bounded-discard counter, with no warnings on a clean process.
func TestPgWriteBoundaryDoctorBlock(t *testing.T) {
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })
	db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false)

	block, warnings := pgWriteBoundaryDoctorBlock()
	if block["posture"] != "none" {
		t.Errorf("phase posture must be \"none\", got %v", block["posture"])
	}
	if block["rotation"] != "inactive" {
		t.Errorf("rotation = %v, want inactive before bootstrap", block["rotation"])
	}
	if block["audit_hash_format"] != "v2" {
		t.Errorf("audit_hash_format = %v, want v2", block["audit_hash_format"])
	}
	if _, ok := block["conn_reset_destroys"]; !ok {
		t.Error("block must surface conn_reset_destroys")
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings on a clean process: %v", warnings)
	}
}

// TestDoctorSingleRolePosture is T-DOCTOR-SINGLEROLE (RFC 0110 §9.3): when
// bootstrap skipped rotation (single-role), the doctor surfaces the
// rotation_skipped_single_role posture.
func TestDoctorSingleRolePosture(t *testing.T) {
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })
	db.SetAuthorityRuntime("secret", db.AuditHashFormatV3, db.PostureSingleRoleSkip, false)

	block, _ := pgWriteBoundaryDoctorBlock()
	if block["rotation"] != db.PostureSingleRoleSkip {
		t.Errorf("rotation posture = %v, want %s", block["rotation"], db.PostureSingleRoleSkip)
	}
	if block["audit_hash_format"] != "v3" {
		t.Errorf("audit_hash_format = %v, want v3", block["audit_hash_format"])
	}
}

// TestDoctorRotatorCollisionWarns: a bootstrap rotator collision surfaces a
// warning and the rotator_collision flag (RFC 0110 §9.4).
func TestDoctorRotatorCollisionWarns(t *testing.T) {
	t.Cleanup(func() { db.SetAuthorityRuntime("", db.AuditHashFormatV2, "", false) })
	db.SetAuthorityRuntime("secret", db.AuditHashFormatV3, db.PostureRotated, true)

	block, warnings := pgWriteBoundaryDoctorBlock()
	if block["rotator_collision"] != true {
		t.Errorf("rotator_collision = %v, want true", block["rotator_collision"])
	}
	found := false
	for _, w := range warnings {
		if len(w) >= 21 && w[:21] == "pg_rotator_collision:" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a pg_rotator_collision warning, got %v", warnings)
	}
}
