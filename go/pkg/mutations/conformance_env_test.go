package mutations

import (
	"strings"
	"testing"
)

// TestSupervisedLaneEnvDropsBannedKeepsRequired asserts the exported
// SupervisedLaneEnv accessor composes the production allowlist filter
// (supervisedEnvPassThrough) with supervisedEnvEntries exactly as supervisedEnv
// does: required control-plane vars survive, banned secret-bearing vars are
// dropped (#87 / RFC 0096 §2), and the lane's bearer is the injected
// session-BOUND token, NOT any shared STRIATUM_MCP_TOKEN in the base env (#135 /
// RFC 0096 V2). This is the hermetic surface the adapterconformance C2 golden
// asserts.
func TestSupervisedLaneEnvDropsBannedKeepsRequired(t *testing.T) {
	const boundToken = "stok_bound.session-secret-not-real"
	base := []string{
		// allowlisted pass-through:
		"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
		// the daemon's SHARED operator-override token — must be DROPPED, not
		// inherited by the lane (#135):
		"STRIATUM_MCP_TOKEN=shared-override-not-real",
		"STRIATUM_MCP_TOKEN_FILE=/run/striatum/client-token",
		"HOME=/home/x",
		"PATH=/usr/bin",
		"TERM=xterm",
		// banned:
		"DATABASE_URL=postgres://leak@127.0.0.1/striatumd",
		"POSTGRES_DB=striatumd",
		"PGPASSWORD=leak",
		"STRIATUM_POSTGRES_DSN=host=x",
		"STRIATUM_PG_TEST_URL=postgres://leak",
	}
	env := SupervisedLaneEnv("claude", base, "/repo", "repo_1", "run_1", "sess_1", "sup_1", "lane_1", boundToken)

	got := map[string]string{}
	for _, entry := range env {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			got[k] = v
		}
	}

	// Required keys present (the freshly-set id vars + endpoint + injected bearer).
	for _, k := range []string{
		"PATH", "HOME", "STRIATUM_MCP_URL", "STRIATUM_MCP_TOKEN",
		"STRIATUM_RUN_ID", "STRIATUM_SESSION_ID", "STRIATUM_SUPERVISOR_ID",
		"STRIATUM_REPO", "STRIATUM_REPOSITORY_ID", "STRIATUM_LANE_ID",
	} {
		if _, ok := got[k]; !ok {
			t.Errorf("required key %q missing from supervised env", k)
		}
	}
	// Id vars are set freshly from the builder arguments.
	if got["STRIATUM_RUN_ID"] != "run_1" || got["STRIATUM_REPO"] != "/repo" {
		t.Errorf("id vars not set from builder args: run=%q repo=%q", got["STRIATUM_RUN_ID"], got["STRIATUM_REPO"])
	}
	// The bearer is the injected session-bound token, NOT the shared override.
	if got["STRIATUM_MCP_TOKEN"] != boundToken {
		t.Errorf("STRIATUM_MCP_TOKEN = %q, want the injected bound token %q", got["STRIATUM_MCP_TOKEN"], boundToken)
	}
	// The shared operator-override token + token-file are dropped (no passthrough).
	if _, ok := got["STRIATUM_MCP_TOKEN_FILE"]; ok {
		t.Errorf("shared STRIATUM_MCP_TOKEN_FILE leaked into supervised env: %q", got["STRIATUM_MCP_TOKEN_FILE"])
	}
	// Banned keys dropped.
	for _, k := range []string{"DATABASE_URL", "POSTGRES_DB", "PGPASSWORD", "STRIATUM_POSTGRES_DSN", "STRIATUM_PG_TEST_URL"} {
		if _, ok := got[k]; ok {
			t.Errorf("banned key %q leaked into supervised env", k)
		}
	}
}

// TestSupervisedLaneEnvWithoutBoundTokenInjectsNone asserts the fail-loud
// posture: when no bound token is supplied (a dev/test path that does not mint),
// the lane env carries NO STRIATUM_MCP_TOKEN — it never silently falls back to a
// shared override present in the base env (#135).
func TestSupervisedLaneEnvWithoutBoundTokenInjectsNone(t *testing.T) {
	base := []string{
		"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
		"STRIATUM_MCP_TOKEN=shared-override-not-real",
		"PATH=/usr/bin",
	}
	env := SupervisedLaneEnv("claude", base, "/repo", "repo_1", "run_1", "sess_1", "sup_1", "lane_1", "")
	for _, entry := range env {
		if k, _, ok := strings.Cut(entry, "="); ok && k == "STRIATUM_MCP_TOKEN" {
			t.Fatalf("no bound token supplied but STRIATUM_MCP_TOKEN present in lane env: %q", entry)
		}
	}
}
