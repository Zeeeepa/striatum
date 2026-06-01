package adapterconformance

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/mutations"
)

// TestC2RequiredKeysPresentAndNoLeak is the hermetic LaneEnvHardened golden
// (#87): the production supervised env builder, fed a base env that contains
// both the allowlisted vars and a battery of banned secret-bearing vars, must
// carry every required key and drop every banned key.
func TestC2RequiredKeysPresentAndNoLeak(t *testing.T) {
	res := assertC2(AssertInput{Adapter: AdapterSpec{Name: "claude_code"}})
	if res.Status != StatusPass {
		t.Fatalf("C2 should pass with the production allowlist; got %s %q: %s",
			res.Status, res.FailureClass, res.Message)
	}
}

// TestC2DetectsLeakIfBuilderForwardedSecret proves the C2 golden is armed: if
// the builder forwarded a banned var, C2 emits EnvSecretLeak. We exercise the
// detector against a child env produced by the real builder plus a manually
// injected banned key (simulating a regression that re-adds a secret to the
// allowlist), confirming bannedKeysIn flags it.
func TestC2DetectsLeakIfBuilderForwardedSecret(t *testing.T) {
	base := c2GoldenBaseEnv()
	child := mutations.SupervisedLaneEnv(base, c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID)
	// Simulate a regression: a banned var leaks into the child env.
	leaky := append(append([]string(nil), child...), "DATABASE_URL="+c2DatabaseURL)
	leaked := bannedKeysIn(envKeySet(leaky))
	if len(leaked) == 0 {
		t.Fatalf("the leak detector must flag a forwarded DATABASE_URL")
	}
	found := false
	for _, k := range leaked {
		if k == "DATABASE_URL" {
			found = true
		}
	}
	if !found {
		t.Fatalf("DATABASE_URL leak not detected, got %v", leaked)
	}
}

// TestC2ProductionBuilderDropsBannedVars asserts the real builder, when handed a
// base env full of banned vars, drops all of them.
func TestC2ProductionBuilderDropsBannedVars(t *testing.T) {
	base := c2GoldenBaseEnv()
	child := mutations.SupervisedLaneEnv(base, c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID)
	keys := envKeySet(child)
	for _, banned := range []string{
		"DATABASE_URL", "POSTGRES_DB", "PGPASSWORD", "PGHOST",
		"STRIATUM_POSTGRES_DSN", "STRIATUM_PG_TEST_URL",
		"SOME_CLOUD_SECRET", "AWS_SECRET_ACCESS_KEY",
	} {
		if keys[banned] {
			t.Errorf("banned var %q reached the supervised child env (#87 leak)", banned)
		}
	}
}

// TestC2ProductionBuilderKeepsRequired asserts the required control-plane keys
// survive the allowlist filter.
func TestC2ProductionBuilderKeepsRequired(t *testing.T) {
	base := c2GoldenBaseEnv()
	child := mutations.SupervisedLaneEnv(base, c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID)
	keys := envKeySet(child)
	for _, required := range c2RequiredKeys {
		if !keys[required] {
			t.Errorf("required key %q dropped from the supervised child env", required)
		}
	}
	// The id vars must carry the values the builder was given (freshly set, not
	// inherited), and the bearer + endpoint must come through the pass-through.
	want := map[string]string{
		"STRIATUM_RUN_ID":     c2RunID,
		"STRIATUM_SESSION_ID": c2SessionID,
		"STRIATUM_REPO":       c2RepoRoot,
		"STRIATUM_MCP_TOKEN":  c2MCPToken,
		"STRIATUM_MCP_URL":    c2MCPURL,
	}
	for _, entry := range child {
		k, v, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if expect, tracked := want[k]; tracked && v != expect {
			t.Errorf("env %s = %q, want %q", k, v, expect)
		}
	}
}

// TestKeyIsBannedRules pins the banned-key matcher rules (exact, prefix,
// substring) so the detector cannot silently weaken.
func TestKeyIsBannedRules(t *testing.T) {
	banned := []string{
		"DATABASE_URL", "PGHOST", "PGUSER", "POSTGRES_PASSWORD",
		"STRIATUM_POSTGRES_DSN", "MY_DSN", "API_SECRET", "DB_PASSWORD",
	}
	for _, k := range banned {
		if !keyIsBanned(k) {
			t.Errorf("key %q should be banned", k)
		}
	}
	allowed := []string{
		"PATH", "HOME", "STRIATUM_MCP_URL", "STRIATUM_MCP_TOKEN",
		"STRIATUM_RUN_ID", "STRIATUM_SESSION_ID", "STRIATUM_REPO",
		"STRIATUM_REPOSITORY_ID", "STRIATUM_LANE_ID", "STRIATUM_SUPERVISOR_ID",
		"TERM", "USER", "LANG",
	}
	for _, k := range allowed {
		if keyIsBanned(k) {
			t.Errorf("allowed key %q should NOT be banned", k)
		}
	}
}
