package mutations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scanDirs are the packages whose handlers mutate daemon state and must open
// their transactions through db.BeginAuthorizedMutation (via withTx), so the
// RFC 0110 authority/attribution prelude is unavoidable. Paths are relative to
// this test's package directory (go/pkg/mutations).
var scanDirs = []string{".", "../admin"}

// beginTxAllowlist enumerates the call sites that legitimately open a raw
// transaction (read-only paths, migrations, recovery sweeps, daemon bootstrap)
// and therefore do not route through the authority prelude. Each entry carries
// a rationale. Keyed by path-as-globbed (see scanDirs).
var beginTxAllowlist = map[string]string{
	"../admin/bootstrap.go": "daemon bootstrap mints the first runtime token before the server threads any RPC auth/envelope onto ctx; no authority context exists yet and it runs before serving mutations",
}

// TestMutatingHandlersUseAuthorizedTransaction is G-MUTATION-TX (RFC 0110 §4.3,
// C-AUTH-TX-WRAPPER): no mutating handler may reach runner.BeginTx directly. The
// only sanctioned transaction opener for the mutations/admin packages is
// db.BeginAuthorizedMutation (which lives in package db and is reached via
// withTx / withTxRetryOnDeadlock); every other raw .BeginTx( call site must be
// explicitly allowlisted with a rationale.
func TestMutatingHandlersUseAuthorizedTransaction(t *testing.T) {
	var violations []string
	for _, dir := range scanDirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatalf("glob %s: %v", dir, err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			for i, line := range strings.Split(string(src), "\n") {
				if !strings.Contains(line, ".BeginTx(") {
					continue
				}
				if _, ok := beginTxAllowlist[file]; ok {
					continue
				}
				violations = append(violations, file+":"+itoa(i+1)+"  "+strings.TrimSpace(line))
			}
		}
	}
	if len(violations) > 0 {
		t.Fatalf("mutating handlers must open transactions via db.BeginAuthorizedMutation (withTx), "+
			"not runner.BeginTx directly:\n  %s\n\nIf a site legitimately begins its own transaction "+
			"(read-only/migration/recovery/bootstrap), add it to beginTxAllowlist with a rationale.",
			strings.Join(violations, "\n  "))
	}
}

// TestAuthorityContractIsDocumented is the PX3-005 doc-presence check that the
// G-MUTATION-TX guard run carries: the spec sentences this guard enforces must
// exist in the published docs, so code and contract cannot drift apart. Paths
// are relative to the package dir: go/pkg/mutations -> repo root is three up.
func TestAuthorityContractIsDocumented(t *testing.T) {
	checks := []struct {
		path     string
		contains string
	}{
		{"../../../docs/rfcs/0110-daemon-postgres-authentication-and-database-enforced-write-boundary.md", "BeginAuthorizedMutation"},
		{"../../../docs/reference/spec.md", "database-enforced write boundary"},
		{"../../../docs/decisions/decision-log.md", "daemon-authority gate"},
	}
	for _, c := range checks {
		src, err := os.ReadFile(c.path)
		if err != nil {
			t.Fatalf("read documented contract %s: %v", c.path, err)
		}
		if !strings.Contains(string(src), c.contains) {
			t.Errorf("%s must document the authority contract (missing %q); "+
				"docs and the G-MUTATION-TX guard must stay in sync (PX3-005)", c.path, c.contains)
		}
	}
}

// itoa avoids importing strconv for a single integer format.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
