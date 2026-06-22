package reads

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/db"
)

// schemaDriftDoctorBlock surfaces the RFC 0142 Layer 3 part 1 (P3) schema-
// fingerprint drift gate as a read-only doctor block. It reports the fingerprint
// this binary EXPECTS (the ordered runtime-migration + owner-bundle SHA set it
// embeds) versus the LIVE fingerprint recorded in striatumd.schema_state, and
// whether they diverge.
//
// Drift is reported as a WARNING (never a hard problem), so it is visible
// regardless of whether the operator has opted enforcement on with
// STRIATUM_SCHEMA_DRIFT_REFUSE. This matches the shadow-first contract: the gate
// itself defaults to log-and-continue, so doctor must make drift observable
// without escalating it to a red/blocking condition until an operator chooses to
// enforce. The block also reports refuse_enabled so the operator can see whether a
// drift would actually halt the next boot.
//
// Daemon-global (independent of repository_id); reads no secret. Like the other
// doctor blocks it never fails closed: a read error is captured in the block and
// returned as a warning so doctor itself still returns.
func schemaDriftDoctorBlock(ctx context.Context, runner db.Runner) (map[string]any, []string) {
	block := map[string]any{
		"checked":        false,
		"refuse_enabled": db.SchemaDriftRefuseEnabled(),
		"refuse_env":     db.EnvSchemaDriftRefuse,
	}
	expected, err := db.ExpectedFingerprint()
	if err != nil {
		block["error"] = err.Error()
		return block, []string{"schema_drift.expected_fingerprint_failed: " + err.Error()}
	}
	block["expected_fingerprint"] = expected

	if runner == nil {
		block["skipped"] = "no runner"
		return block, nil
	}
	live, err := db.LiveFingerprint(ctx, runner)
	if err != nil {
		block["checked"] = true
		block["error"] = err.Error()
		return block, []string{"schema_drift.read_failed: " + err.Error()}
	}
	block["checked"] = true
	block["live_fingerprint"] = live

	decision := db.EvaluateSchemaDrift(expected, live)
	switch {
	case live == "":
		// No fingerprint recorded yet (pre-P3 database, or fresh before first
		// self-record). Not drift — the next clean migrate records it.
		block["status"] = "unrecorded"
		block["drift"] = false
		block["note"] = "no live schema fingerprint recorded yet (pre-P3 database or first boot); the next successful migrate self-records it"
		return block, nil
	case decision.Drift:
		block["status"] = "drift"
		block["drift"] = true
		warning := "schema_drift: live schema fingerprint diverges from the set this binary embeds (RFC 0142 Layer 3); " +
			"reconcile the database to the binary (apply pending migrations/owner bundles, or roll the binary back to the matching frontier)"
		if db.SchemaDriftRefuseEnabled() {
			warning += " — " + db.EnvSchemaDriftRefuse + "=1 is set, so the daemon WILL refuse to serve on the next restart"
		} else {
			warning += " — shadow mode (" + db.EnvSchemaDriftRefuse + " unset): the daemon logs and continues; set it to 1 to refuse-to-serve"
		}
		return block, []string{warning}
	default:
		block["status"] = "in_sync"
		block["drift"] = false
		return block, nil
	}
}
