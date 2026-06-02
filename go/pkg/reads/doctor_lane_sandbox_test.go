package reads

import (
	"strings"
	"testing"
)

// TestLaneSandboxDoctorBlock pins the #87 honest-proxy doctor logic: by default
// (no PG-less lane OS user configured) it warns that lanes can reach the daemon's
// PostgreSQL; a configured, existing, distinct lane user clears the warning; a
// configured-but-absent user warns. No live PostgreSQL is touched.
func TestLaneSandboxDoctorBlock(t *testing.T) {
	origUser := currentUsername
	origLookup := lookupOSUser
	t.Cleanup(func() { currentUsername = origUser; lookupOSUser = origLookup })
	currentUsername = func() string { return "daemonuser" }

	t.Run("default un-hardened warns", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "")
		lookupOSUser = func(string) bool { return true }
		block, warnings := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != false {
			t.Fatalf("expected lane_pg_isolated=false, got %#v", block["lane_pg_isolated"])
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "lane_pg_reachable") {
			t.Fatalf("expected one lane_pg_reachable warning, got %#v", warnings)
		}
	})

	t.Run("configured existing distinct user clears warning", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "striatum-lane")
		lookupOSUser = func(name string) bool { return name == "striatum-lane" }
		block, warnings := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != true {
			t.Fatalf("expected lane_pg_isolated=true, got %#v", block["lane_pg_isolated"])
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no warning for an adopted lane user, got %#v", warnings)
		}
	})

	t.Run("configured but absent user warns", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "ghost-lane")
		lookupOSUser = func(string) bool { return false }
		_, warnings := laneSandboxDoctorBlock()
		if len(warnings) != 1 || !strings.Contains(warnings[0], "no such OS user") {
			t.Fatalf("expected a 'no such OS user' warning, got %#v", warnings)
		}
	})

	t.Run("lane user equal to daemon user is not isolation", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "daemonuser")
		lookupOSUser = func(string) bool { return true }
		block, warnings := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != false {
			t.Fatalf("a lane user equal to the daemon user is not isolation: %#v", block)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected the default reachable warning, got %#v", warnings)
		}
	})
}
