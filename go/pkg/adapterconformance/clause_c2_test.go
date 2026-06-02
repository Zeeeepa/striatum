package adapterconformance

import (
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/mutations"
)

// claudeAdapterSpec is the production claude_code adapter spec (Name +
// bare-interactive Command), so the C2 golden exercises the real per-adapter
// env hardening path (cliAdapter == "claude") rather than an empty command.
func claudeAdapterSpec() AdapterSpec {
	for _, a := range Adapters() {
		if a.Name == "claude_code" {
			return a
		}
	}
	return AdapterSpec{Name: "claude_code", Command: []string{"claude"}}
}

// TestC2RequiredKeysPresentAndNoLeak is the hermetic LaneEnvHardened golden
// (#87): the production supervised env builder, fed a base env that contains
// both the allowlisted vars and a battery of banned secret-bearing vars, must
// carry every required key and drop every banned key.
func TestC2RequiredKeysPresentAndNoLeak(t *testing.T) {
	res := assertC2(AssertInput{Adapter: claudeAdapterSpec()})
	if res.Status != StatusPass {
		t.Fatalf("C2 should pass with the production allowlist; got %s %q: %s",
			res.Status, res.FailureClass, res.Message)
	}
}

// TestC2ClaudeLaneEnvCarriesWelcomeSuppression is the #101 self-hosting golden:
// the production supervised claude lane env MUST carry the welcome/update-nag
// suppression keys (DISABLE_AUTOUPDATER + CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC).
// A revert that drops them from supervisedAdapterEnvEntries makes assertC2 fail
// with AdapterContractViolation (proven by TestC2RevertDropsWelcomeSuppression).
func TestC2ClaudeLaneEnvCarriesWelcomeSuppression(t *testing.T) {
	child := mutations.SupervisedLaneEnv(
		"claude", c2GoldenBaseEnv(),
		c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID,
		c2BoundToken,
	)
	keys := envKeySet(child)
	for _, key := range []string{"DISABLE_AUTOUPDATER", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"} {
		if !keys[key] {
			t.Errorf("supervised claude lane env missing #101 suppression key %q", key)
		}
	}
	// Each is the documented disable value "1".
	want := map[string]string{
		"DISABLE_AUTOUPDATER":                      "1",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
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

// TestC2NonClaudeAdaptersDoNotGetClaudeSuppression confirms the suppression keys
// are scoped per-adapter: a codex/agy lane env must NOT carry the claude-only
// welcome/update-nag keys.
func TestC2NonClaudeAdaptersDoNotGetClaudeSuppression(t *testing.T) {
	for _, adapter := range []string{"codex", "agy"} {
		child := mutations.SupervisedLaneEnv(
			adapter, c2GoldenBaseEnv(),
			c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID,
			c2BoundToken,
		)
		keys := envKeySet(child)
		for _, key := range []string{"DISABLE_AUTOUPDATER", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"} {
			if keys[key] {
				t.Errorf("adapter %q env should NOT carry claude-only key %q", adapter, key)
			}
		}
	}
}

// TestC2RevertDropsWelcomeSuppression proves the #101 golden is armed: if the
// production builder did NOT carry the suppression keys (the revert), assertC2
// fails with AdapterContractViolation. We simulate the revert by removing the
// keys the production builder added, then re-running the same per-adapter check
// assertC2 performs.
func TestC2RevertDropsWelcomeSuppression(t *testing.T) {
	child := mutations.SupervisedLaneEnv(
		"claude", c2GoldenBaseEnv(),
		c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID,
		c2BoundToken,
	)
	// Reverted env: strip the per-adapter suppression keys.
	var reverted []string
	dropped := map[string]bool{"DISABLE_AUTOUPDATER": true, "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": true}
	for _, entry := range child {
		k, _, _ := strings.Cut(entry, "=")
		if dropped[k] {
			continue
		}
		reverted = append(reverted, entry)
	}
	keys := envKeySet(reverted)
	var missing []string
	for _, key := range c2AdapterRequiredKeys["claude"] {
		if !keys[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) != 2 {
		t.Fatalf("revert should drop both #101 suppression keys; missing=%v", missing)
	}
}

// TestC2DetectsLeakIfBuilderForwardedSecret proves the C2 golden is armed: if
// the builder forwarded a banned var, C2 emits EnvSecretLeak. We exercise the
// detector against a child env produced by the real builder plus a manually
// injected banned key (simulating a regression that re-adds a secret to the
// allowlist), confirming bannedKeysIn flags it.
func TestC2DetectsLeakIfBuilderForwardedSecret(t *testing.T) {
	base := c2GoldenBaseEnv()
	child := mutations.SupervisedLaneEnv("claude", base, c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID, c2BoundToken)
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
	child := mutations.SupervisedLaneEnv("claude", base, c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID, c2BoundToken)
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
	child := mutations.SupervisedLaneEnv("claude", base, c2RepoRoot, c2RepositoryID, c2RunID, c2SessionID, c2SupervisorID, c2LaneID, c2BoundToken)
	keys := envKeySet(child)
	for _, required := range c2RequiredKeys {
		if !keys[required] {
			t.Errorf("required key %q dropped from the supervised child env", required)
		}
	}
	// The id vars must carry the values the builder was given (freshly set, not
	// inherited); the endpoint comes through the pass-through; and the bearer is
	// the injected session-BOUND token (#135), NOT the shared c2MCPToken in the
	// base env (which the allowlist now drops).
	want := map[string]string{
		"STRIATUM_RUN_ID":     c2RunID,
		"STRIATUM_SESSION_ID": c2SessionID,
		"STRIATUM_REPO":       c2RepoRoot,
		"STRIATUM_MCP_TOKEN":  c2BoundToken,
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
