package mutations

import (
	"strings"
	"testing"
)

// TestSupervisedLaneEnvDropsBannedKeepsRequired asserts the exported
// SupervisedLaneEnv accessor composes the production allowlist filter
// (supervisedEnvPassThrough) with supervisedEnvEntries exactly as supervisedEnv
// does: required control-plane vars survive, banned secret-bearing vars are
// dropped (#87 / RFC 0096 §2). This is the hermetic surface the
// adapterconformance C2 golden asserts.
func TestSupervisedLaneEnvDropsBannedKeepsRequired(t *testing.T) {
	base := []string{
		// allowlisted pass-through:
		"STRIATUM_MCP_URL=http://127.0.0.1:1/mcp",
		"STRIATUM_MCP_TOKEN=bearer-not-real",
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
	env := SupervisedLaneEnv(base, "/repo", "repo_1", "run_1", "sess_1", "sup_1", "lane_1")

	got := map[string]string{}
	for _, entry := range env {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			got[k] = v
		}
	}

	// Required keys present (the freshly-set id vars + pass-through MCP wiring).
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
	// Pass-through MCP wiring preserved.
	if got["STRIATUM_MCP_TOKEN"] != "bearer-not-real" {
		t.Errorf("bearer token not preserved: %q", got["STRIATUM_MCP_TOKEN"])
	}
	// Banned keys dropped.
	for _, k := range []string{"DATABASE_URL", "POSTGRES_DB", "PGPASSWORD", "STRIATUM_POSTGRES_DSN", "STRIATUM_PG_TEST_URL"} {
		if _, ok := got[k]; ok {
			t.Errorf("banned key %q leaked into supervised env", k)
		}
	}
}
