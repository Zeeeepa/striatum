package reads

import "github.com/halbritt/striatum/go/pkg/db"

// connResetStormThreshold is the cumulative connection-reset-destroy count above
// which doctor warns of a possible reconnect storm (RFC 0110 §4.7 / OPS-12).
const connResetStormThreshold = 100

// pgWriteBoundaryDoctorBlock reports the RFC 0110 daemon→PostgreSQL write-boundary
// posture. The DB-enforced phase posture is still "none" (P0/P1/P2 surface
// closure is a later release); slice 2 adds the live L0/authority signals — the
// daemon's bootstrap posture, the active audit hash format, and a rotator
// collision — read from process state (the doctor runs in the daemon, so it
// reports its own bootstrap posture without an owner read). No secret is read.
// The "sole durable write path" claim is only valid once posture reads "full".
func pgWriteBoundaryDoctorBlock() (map[string]any, []string) {
	destroys := db.ConnResetDestroyCount()
	rotation := db.AuthorityPosture()
	if rotation == "" {
		rotation = "inactive"
	}
	block := map[string]any{
		"posture":             "none",
		"rotation":            rotation,
		"audit_hash_format":   db.AuditHashFormat(),
		"rotator_collision":   db.AuthorityRotatorCollision(),
		"conn_reset_destroys": destroys,
		"note": "RFC 0110 authority plumbing + L0/v3 mechanism are in place. DB-enforced write " +
			"closure (P0 audit_only -> P2 full) lands in a later release; no Striatum claim of a sole " +
			"DB-enforced durable write path is valid until this posture reads 'full'.",
	}
	var warnings []string
	if destroys > connResetStormThreshold {
		warnings = append(warnings, "pg_conn_reset_storm: many pooled connections were destroyed on release for "+
			"carrying leftover transaction state; check for a mass-cancel or PostgreSQL stress event")
	}
	if db.AuthorityRotatorCollision() {
		warnings = append(warnings, "pg_rotator_collision: another instance recently rotated the same runtime role; "+
			"use per-instance roles (striatumd_rw_<instance>) for a shared PostgreSQL (RFC 0110 §9.4)")
	}
	return block, warnings
}
