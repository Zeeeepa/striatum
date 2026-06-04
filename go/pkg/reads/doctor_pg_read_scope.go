package reads

const pgReadScopeBroadRuntimeSelect = "broad_runtime_select"

// pgReadScopeDoctorBlock reports the RFC 0110 / #164 read-scope posture. It is
// deliberately separate from pg_write_boundary: RFC 0110's shipped guarantees
// close mutation paths, not private reads by a live runtime credential.
func pgReadScopeDoctorBlock() map[string]any {
	return map[string]any{
		"posture":                   pgReadScopeBroadRuntimeSelect,
		"runtime_role_select_scope": "broad",
		"private_read_denial":       false,
		"bounded_by": []string{
			"L0 runtime credential rotation makes captured DSN strings stale after daemon restart",
			"L2 lane isolation prevents sandboxed lanes from reaching PostgreSQL once adopted",
		},
		"representative_sensitive_surfaces": []string{
			"artifacts",
			"events",
			"sessions",
			"queue_messages",
			"principals",
			"blockers",
		},
		"note": "RFC 0110 does not claim read confidentiality against a leaked live runtime credential; " +
			"#164 remains the successor for reducing this surface to an enumerated minimum.",
	}
}
