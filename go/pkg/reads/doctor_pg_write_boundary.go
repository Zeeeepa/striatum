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
	phase := db.ActiveWriteBoundary()
	block := map[string]any{
		"posture":             string(phase),
		"rotation":            rotation,
		"audit_hash_format":   db.AuditHashFormat(),
		"rotator_collision":   db.AuthorityRotatorCollision(),
		"conn_reset_destroys": destroys,
		"note":                pgWriteBoundaryNote(phase),
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

// pgWriteBoundaryNote returns the operator-facing note for a write-boundary
// posture (RFC 0110 §7). Each phase's note states exactly the claim that phase
// licenses; only "full" licenses the sole-durable-write-path claim.
func pgWriteBoundaryNote(phase db.WriteBoundaryPhase) string {
	switch phase {
	case db.PhaseFull:
		return "RFC 0110 P2 full: audit_log, artifacts, and events are DB-enforced (SD-function-only). " +
			"The daemon's durable write paths are DB-enforced."
	case db.PhaseAuditArtifacts:
		return "RFC 0110 P1 audit_artifacts: the audit chain and artifact writes are DB-enforced " +
			"(SD-function-only). events is not yet closed; no sole-durable-write-path claim is valid until posture reads 'full'."
	case db.PhaseAuditOnly:
		return "RFC 0110 P0 audit_only: the audit chain is DB-enforced (SD-function-only). artifacts and " +
			"events are not yet closed; no sole-durable-write-path claim is valid until posture reads 'full'."
	default:
		return "RFC 0110 authority plumbing + L0/v3 mechanism are in place but no surface is DB-enforced yet. " +
			"No Striatum claim of a DB-enforced durable write path is valid until this posture reads 'full'."
	}
}
