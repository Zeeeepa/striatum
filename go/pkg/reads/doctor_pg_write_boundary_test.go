package reads

import "testing"

// TestPgWriteBoundaryDoctorBlock pins the release-N posture: the daemon→PG write
// boundary reports "none" (no phase has closed a surface) with the single-role
// rotation note and the bounded-discard counter, and warns only above the
// reconnect-storm threshold.
func TestPgWriteBoundaryDoctorBlock(t *testing.T) {
	block, warnings := pgWriteBoundaryDoctorBlock()

	if block["posture"] != "none" {
		t.Errorf("release N posture must be \"none\", got %v", block["posture"])
	}
	if block["rotation"] != "rotation_skipped_single_role" {
		t.Errorf("rotation posture = %v, want rotation_skipped_single_role", block["rotation"])
	}
	if _, ok := block["conn_reset_destroys"]; !ok {
		t.Error("block must surface conn_reset_destroys")
	}
	// A clean process has a low destroy count, so no storm warning.
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings at a low destroy count: %v", warnings)
	}
}
