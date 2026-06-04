package db

// WriteAuthorityClass classifies how a daemon-owned table may be written, per
// the RFC 0110 §13 write-authority inventory. Every striatumd.* table carries a
// classification so a future table cannot silently bypass the L1 model; the
// inventory guard (TestWriteAuthorityInventoryComplete) fails when a live table
// has no row here.
type WriteAuthorityClass string

const (
	// ClassSDGated: writes go only through an owner-owned SECURITY DEFINER
	// function from the phase named in the comment (RFC 0110 §7).
	ClassSDGated WriteAuthorityClass = "sd_gated"
	// ClassRuntimeDML: direct runtime-role DML retained (live coordination
	// state); some are append-only via triggers (UPDATE/DELETE revoked).
	ClassRuntimeDML WriteAuthorityClass = "runtime_dml"
	// ClassOwnerOnly: the runtime role holds no write privilege (authority
	// schema + migration bookkeeping).
	ClassOwnerOnly WriteAuthorityClass = "owner_only"
)

// writeAuthorityInventory classifies every daemon-owned table at the current
// phase (release N+1 slice 1 = Phase 0 audit_only). artifacts becomes sd_gated
// at P1 and events at P2; update those rows when each phase lands.
var writeAuthorityInventory = map[string]WriteAuthorityClass{
	// Phase 0: audit_log is SD-function-only.
	"audit_log": ClassSDGated,

	// Authority schema + migration bookkeeping: owner-only.
	"daemon_auth_registry": ClassOwnerOnly,
	"daemon_auth_log":      ClassOwnerOnly,
	"schema_authority":     ClassOwnerOnly,
	"owner_bundle_meta":    ClassOwnerOnly,
	"schema_meta":          ClassOwnerOnly,
	"schema_migrations":    ClassOwnerOnly,
	"repo_migrations":      ClassOwnerOnly,
	"daemon_meta":          ClassOwnerOnly,

	// Live coordination + durable state: direct runtime DML retained for now.
	// artifacts (-> sd_gated at P1) and events (-> sd_gated at P2) stay here in
	// slice 1; they are append-only via triggers.
	"artifacts":                      ClassRuntimeDML,
	"events":                         ClassRuntimeDML,
	"audit_chain_head":               ClassRuntimeDML,
	"audit_segments":                 ClassRuntimeDML,
	"audit_repositories":             ClassRuntimeDML,
	"apply_receipts":                 ClassRuntimeDML,
	"auto_finalize_circuit_breakers": ClassRuntimeDML,
	"blockers":                       ClassRuntimeDML,
	"client_capabilities":            ClassRuntimeDML,
	"client_sessions":                ClassRuntimeDML,
	"clients":                        ClassRuntimeDML,
	"command_requests":               ClassRuntimeDML,
	"conversations":                  ClassRuntimeDML,
	"cross_repo_cycle_counters":      ClassRuntimeDML,
	"cross_repo_run_repositories":    ClassRuntimeDML,
	"cross_repo_runs":                ClassRuntimeDML,
	"daemon_supervisors":             ClassRuntimeDML,
	"escalation_inbox":               ClassRuntimeDML,
	"interrogations":                 ClassRuntimeDML,
	"job_dependencies":               ClassRuntimeDML,
	"job_recovery_state":             ClassRuntimeDML,
	"job_worktrees":                  ClassRuntimeDML,
	"jobs":                           ClassRuntimeDML,
	"leases":                         ClassRuntimeDML,
	"principal_clients":              ClassRuntimeDML,
	"principals":                     ClassRuntimeDML,
	"process_executions":             ClassRuntimeDML,
	"process_supervisor_pointers":    ClassRuntimeDML,
	"process_supervisors":            ClassRuntimeDML,
	"queue_messages":                 ClassRuntimeDML,
	"repo_event_chain_heads":         ClassRuntimeDML,
	"repositories":                   ClassRuntimeDML,
	"rpc_methods":                    ClassRuntimeDML,
	"rpc_request_log":                ClassRuntimeDML,
	"runs":                           ClassRuntimeDML,
	"scheduler_cursors":              ClassRuntimeDML,
	"sessions":                       ClassRuntimeDML,
	"trajectory_segments":            ClassRuntimeDML,
	"verdicts":                       ClassRuntimeDML,
	"work_packets":                   ClassRuntimeDML,
	"workflow_accepted_risks":        ClassRuntimeDML,
	"workflow_snapshots":             ClassRuntimeDML,
}

// ClassifyTable returns the write-authority classification of a striatumd.*
// table and whether it is in the inventory.
func ClassifyTable(table string) (WriteAuthorityClass, bool) {
	class, ok := writeAuthorityInventory[table]
	return class, ok
}

// WriteAuthorityInventory returns a copy of the full classification.
func WriteAuthorityInventory() map[string]WriteAuthorityClass {
	out := make(map[string]WriteAuthorityClass, len(writeAuthorityInventory))
	for table, class := range writeAuthorityInventory {
		out[table] = class
	}
	return out
}
