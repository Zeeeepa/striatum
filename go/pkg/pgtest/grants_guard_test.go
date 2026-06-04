package pgtest

import (
	"regexp"
	"strings"
	"testing"
)

// TestRoleSetupIssuesNoProtectedDML is G-PGTEST-GRANTS (RFC 0110 §10,
// C-PGTEST-NO-DML-GRANT): pgtest's per-test role setup must not GRANT/REVOKE DML
// on a protected append-only table (audit_log/artifacts/events), and must not
// blanket-(re)grant table DML across the schema. Those privileges come only from
// the production migration / owner-bundle SQL — so the 42501 negative-path gate
// runs against the real grant surface and cannot false-green on a hand-built one.
// The per-test login role inherits its DML surface from striatumd_rw membership.
func TestRoleSetupIssuesNoProtectedDML(t *testing.T) {
	protected := []string{"audit_log", "artifacts", "events"}
	grantRevoke := regexp.MustCompile(`(?i)\b(GRANT|REVOKE)\b`)
	// A blanket DML (re)grant across the schema would reopen the append-only
	// surfaces just as surely as naming them.
	blanketDML := regexp.MustCompile(`(?i)\b(GRANT|REVOKE)\b.*\b(INSERT|UPDATE|DELETE)\b.*\bON ALL TABLES\b`)

	for _, stmt := range roleSetupStatements("db_x", "striatumd_rw_db_x", "owner_user") {
		if !grantRevoke.MatchString(stmt) {
			continue
		}
		for _, tbl := range protected {
			if strings.Contains(stmt, "striatumd."+tbl) {
				t.Errorf("G-PGTEST-GRANTS: role setup GRANT/REVOKE names protected table %q: %q\n"+
					"protected-table DML must come from migration/owner-bundle SQL, not pgtest", tbl, stmt)
			}
		}
		if blanketDML.MatchString(stmt) {
			t.Errorf("G-PGTEST-GRANTS: role setup blanket-(re)grants table DML: %q\n"+
				"the append-only surfaces would reopen; rely on striatumd_rw membership instead", stmt)
		}
	}
}

// TestRoleSetupUsesMembership documents the positive contract: the per-test role
// is granted membership in striatumd_rw (so it inherits the migration-defined
// surface), and the owner is granted the per-test role (so the test can SET ROLE
// to it over the PEER socket without administering the cluster-wide striatumd_rw).
func TestRoleSetupUsesMembership(t *testing.T) {
	stmts := strings.Join(roleSetupStatements("db_x", "striatumd_rw_db_x", "owner_user"), "\n")
	if !strings.Contains(stmts, "GRANT striatumd_rw TO ") {
		t.Error("per-test role must be granted membership in striatumd_rw (inherits the migration-defined DML surface)")
	}
	if !strings.Contains(stmts, `GRANT "striatumd_rw_db_x" TO "owner_user"`) {
		t.Error("owner must be granted the per-test role so it can SET ROLE over the PEER socket")
	}
}
