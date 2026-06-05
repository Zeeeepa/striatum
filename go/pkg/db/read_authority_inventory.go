package db

import "sort"

// ReadAuthorityClass classifies the current runtime-role read surface for a
// striatumd.* table. This is the #164 companion to the RFC 0110 write-authority
// inventory: it does not claim private-read denial, but it keeps the broad
// SELECT posture explicit and table-scoped until a successor narrows it.
type ReadAuthorityClass string

const (
	// ReadClassRuntimeSensitive: the runtime role currently holds SELECT and the
	// table can expose user/agent prose, repository paths, token-adjacent data,
	// principal/session identity, or other private workflow metadata if a live
	// runtime credential leaks.
	ReadClassRuntimeSensitive ReadAuthorityClass = "runtime_sensitive_select"
	// ReadClassRuntimeOperational: the runtime role currently holds SELECT for
	// daemon operation, but the table is not one of the representative sensitive
	// surfaces used in the #164 doctor posture.
	ReadClassRuntimeOperational ReadAuthorityClass = "runtime_operational_select"
	// ReadClassRuntimeParity: an owner-maintained table deliberately readable by
	// the runtime role for startup capability parity.
	ReadClassRuntimeParity ReadAuthorityClass = "runtime_parity_select"
	// ReadClassRuntimeDenied: the runtime role must not hold SELECT.
	ReadClassRuntimeDenied ReadAuthorityClass = "runtime_select_denied"
)

var readAuthorityInventory = map[string]ReadAuthorityClass{
	// Owner-only authority surfaces.
	"daemon_auth_registry": ReadClassRuntimeDenied,
	"daemon_auth_log":      ReadClassRuntimeDenied,
	"owner_bundle_meta":    ReadClassRuntimeDenied,

	// Startup parity: owner-maintained, intentionally runtime-readable.
	"schema_authority": ReadClassRuntimeParity,

	// Sensitive broad SELECT surfaces. These are the tables a future #164 split
	// should consider first for denial, projection, or row/column scoping.
	"artifacts":                   ReadClassRuntimeSensitive,
	"blockers":                    ReadClassRuntimeSensitive,
	"client_capabilities":         ReadClassRuntimeSensitive,
	"client_sessions":             ReadClassRuntimeSensitive,
	"clients":                     ReadClassRuntimeSensitive,
	"command_requests":            ReadClassRuntimeSensitive,
	"conversations":               ReadClassRuntimeSensitive,
	"cross_repo_run_repositories": ReadClassRuntimeSensitive,
	"cross_repo_runs":             ReadClassRuntimeSensitive,
	"daemon_supervisors":          ReadClassRuntimeSensitive,
	"escalation_inbox":            ReadClassRuntimeSensitive,
	"events":                      ReadClassRuntimeSensitive,
	"interrogations":              ReadClassRuntimeSensitive,
	"job_dependencies":            ReadClassRuntimeSensitive,
	"job_recovery_state":          ReadClassRuntimeSensitive,
	"job_worktrees":               ReadClassRuntimeSensitive,
	"jobs":                        ReadClassRuntimeSensitive,
	"leases":                      ReadClassRuntimeSensitive,
	"principal_clients":           ReadClassRuntimeSensitive,
	"principals":                  ReadClassRuntimeSensitive,
	"process_executions":          ReadClassRuntimeSensitive,
	"process_supervisor_pointers": ReadClassRuntimeSensitive,
	"process_supervisors":         ReadClassRuntimeSensitive,
	"queue_messages":              ReadClassRuntimeSensitive,
	"repositories":                ReadClassRuntimeSensitive,
	"rpc_request_log":             ReadClassRuntimeSensitive,
	"runs":                        ReadClassRuntimeSensitive,
	"scheduler_cursors":           ReadClassRuntimeSensitive,
	"sessions":                    ReadClassRuntimeSensitive,
	"trajectory_segments":         ReadClassRuntimeSensitive,
	"verdicts":                    ReadClassRuntimeSensitive,
	"work_packets":                ReadClassRuntimeSensitive,
	"workflow_accepted_risks":     ReadClassRuntimeSensitive,
	"workflow_snapshots":          ReadClassRuntimeSensitive,

	// Operational metadata and chain pointers. Still selected by the runtime
	// role in the current broad posture; not a private-read-denial claim.
	"apply_receipts":                 ReadClassRuntimeOperational,
	"audit_chain_head":               ReadClassRuntimeOperational,
	"audit_log":                      ReadClassRuntimeOperational,
	"audit_repositories":             ReadClassRuntimeOperational,
	"audit_segments":                 ReadClassRuntimeOperational,
	"auto_finalize_circuit_breakers": ReadClassRuntimeOperational,
	"cross_repo_cycle_counters":      ReadClassRuntimeOperational,
	"daemon_meta":                    ReadClassRuntimeOperational,
	"repo_event_chain_heads":         ReadClassRuntimeOperational,
	"repo_migrations":                ReadClassRuntimeOperational,
	"rpc_methods":                    ReadClassRuntimeOperational,
	"schema_meta":                    ReadClassRuntimeOperational,
	"schema_migrations":              ReadClassRuntimeOperational,
}

// ClassifyReadTable returns the read-authority classification of a striatumd.*
// table and whether it is in the inventory.
func ClassifyReadTable(table string) (ReadAuthorityClass, bool) {
	class, ok := readAuthorityInventory[table]
	return class, ok
}

// ReadAuthorityInventory returns a copy of the full classification.
func ReadAuthorityInventory() map[string]ReadAuthorityClass {
	out := make(map[string]ReadAuthorityClass, len(readAuthorityInventory))
	for table, class := range readAuthorityInventory {
		out[table] = class
	}
	return out
}

// RuntimeSensitiveReadTables returns the sorted tables currently called out as
// sensitive under the broad runtime SELECT posture.
func RuntimeSensitiveReadTables() []string {
	var out []string
	for table, class := range readAuthorityInventory {
		if class == ReadClassRuntimeSensitive {
			out = append(out, table)
		}
	}
	sort.Strings(out)
	return out
}

// RuntimeDeniedReadColumns returns the narrow column-level denials that have
// landed ahead of the full #164 table/projection split.
func RuntimeDeniedReadColumns() map[string][]string {
	return map[string][]string{
		"clients": {"token_hash", "token_salt"},
	}
}
