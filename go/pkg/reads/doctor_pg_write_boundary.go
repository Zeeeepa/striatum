package reads

import "github.com/halbritt/striatum/go/pkg/db"

// connResetStormThreshold is the cumulative connection-reset-destroy count above
// which doctor warns of a possible reconnect storm (RFC 0110 §4.7 / OPS-12).
const connResetStormThreshold = 100

// pgWriteBoundaryDoctorBlock reports the RFC 0110 daemon→PostgreSQL write-boundary
// posture. In release N the authority plumbing is installed but no phase has
// closed any surface, so the posture is "none" and L0 rotation is inert
// (single-role); the conn-reset-destroy counter is surfaced as the bounded-
// discard signal. The "sole durable write path" claim is only valid once the
// posture string reaches "full" (phase P2).
func pgWriteBoundaryDoctorBlock() (map[string]any, []string) {
	destroys := db.ConnResetDestroyCount()
	block := map[string]any{
		"posture":             "none",
		"rotation":            "rotation_skipped_single_role",
		"conn_reset_destroys": destroys,
		"note": "RFC 0110 authority plumbing is in place (in-transaction authority/attribution prelude, " +
			"fail-closed mutation-coupled audit). DB-enforced write closure (P0 audit_only -> P2 full) and " +
			"L0 credential rotation land in a later release; no Striatum claim of a sole DB-enforced durable " +
			"write path is valid until this posture reads 'full'.",
	}
	var warnings []string
	if destroys > connResetStormThreshold {
		warnings = append(warnings, "pg_conn_reset_storm: many pooled connections were destroyed on release for "+
			"carrying leftover transaction state; check for a mass-cancel or PostgreSQL stress event")
	}
	return block, warnings
}
