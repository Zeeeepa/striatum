package reads

import "github.com/halbritt/striatum/go/pkg/db"

const pgReadScopeBroadRuntimeSelect = "broad_runtime_select"

// pgReadScopeDoctorBlock reports the RFC 0110 / #164 read-scope posture. It is
// deliberately separate from pg_write_boundary: RFC 0110's shipped guarantees
// close mutation paths, not private reads by a live runtime credential.
func pgReadScopeDoctorBlock() map[string]any {
	sensitiveSurfaces := db.RuntimeSensitiveReadTables()
	return map[string]any{
		"posture":                   pgReadScopeBroadRuntimeSelect,
		"runtime_role_select_scope": "broad",
		"private_read_denial":       false,
		"inventory_source":          "go/pkg/db/read_authority_inventory.go",
		"sensitive_surface_count":   len(sensitiveSurfaces),
		"partial_projection_gates": []map[string]any{
			{
				"surface":             "clients",
				"denied_columns":      db.RuntimeDeniedReadColumns()["clients"],
				"owner_bundle":        5,
				"authority_stamp":     "auth_projection_read",
				"private_read_denial": false,
			},
		},
		"bounded_by": []string{
			"L0 runtime credential rotation makes captured DSN strings stale after daemon restart",
			"L2 lane isolation prevents sandboxed lanes from reaching PostgreSQL once adopted",
		},
		"representative_sensitive_surfaces": sensitiveSurfaces,
		"note": "RFC 0110 does not claim read confidentiality against a leaked live runtime credential; " +
			"RFC 0113 R1 reduces token-secret columns first, but #164 remains open until every sensitive read surface is bounded.",
	}
}
