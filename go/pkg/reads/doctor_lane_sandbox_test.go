package reads

import (
	"context"
	"strings"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
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
		t.Setenv(lanePGSocketHardenedEnv, "")
		lookupOSUser = func(string) bool { return true }
		block, warnings, problems := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != false {
			t.Fatalf("expected lane_pg_isolated=false, got %#v", block["lane_pg_isolated"])
		}
		if block["enforcement"] != "advisory" {
			t.Fatalf("expected advisory enforcement, got %#v", block["enforcement"])
		}
		if len(warnings) != 1 || !strings.Contains(warnings[0], "lane_pg_reachable") {
			t.Fatalf("expected one lane_pg_reachable warning, got %#v", warnings)
		}
		if len(problems) != 0 {
			t.Fatalf("expected no hard problem by default, got %#v", problems)
		}
	})

	t.Run("configured existing distinct user clears warning", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "striatum-lane")
		t.Setenv(lanePGSocketHardenedEnv, "")
		lookupOSUser = func(name string) bool { return name == "striatum-lane" }
		block, warnings, problems := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != true {
			t.Fatalf("expected lane_pg_isolated=true, got %#v", block["lane_pg_isolated"])
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no warning for an adopted lane user, got %#v", warnings)
		}
		if len(problems) != 0 {
			t.Fatalf("expected no problem for an adopted lane user, got %#v", problems)
		}
	})

	t.Run("configured but absent user warns", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "ghost-lane")
		t.Setenv(lanePGSocketHardenedEnv, "")
		lookupOSUser = func(string) bool { return false }
		_, warnings, problems := laneSandboxDoctorBlock()
		if len(warnings) != 1 || !strings.Contains(warnings[0], "no such OS user") {
			t.Fatalf("expected a 'no such OS user' warning, got %#v", warnings)
		}
		if len(problems) != 0 {
			t.Fatalf("expected no hard problem by default, got %#v", problems)
		}
	})

	t.Run("lane user equal to daemon user is not isolation", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "daemonuser")
		t.Setenv(lanePGSocketHardenedEnv, "")
		lookupOSUser = func(string) bool { return true }
		block, warnings, problems := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != false {
			t.Fatalf("a lane user equal to the daemon user is not isolation: %#v", block)
		}
		if len(warnings) != 1 {
			t.Fatalf("expected the default reachable warning, got %#v", warnings)
		}
		if len(problems) != 0 {
			t.Fatalf("expected no hard problem by default, got %#v", problems)
		}
	})

	t.Run("hardened socket profile blocks instead of warning", func(t *testing.T) {
		t.Setenv(laneOSUserEnv, "")
		t.Setenv(lanePGSocketHardenedEnv, "1")
		lookupOSUser = func(string) bool { return true }
		block, warnings, problems := laneSandboxDoctorBlock()
		if block["lane_pg_isolated"] != false || block["enforcement"] != "blocking" {
			t.Fatalf("expected blocking unisolated lane posture, got %#v", block)
		}
		if len(warnings) != 0 {
			t.Fatalf("expected no advisory warning under hardened profile, got %#v", warnings)
		}
		if len(problems) != 1 || !strings.Contains(problems[0], "lane_pg_reachable") {
			t.Fatalf("expected one blocking lane_pg_reachable problem, got %#v", problems)
		}
	})
}

func TestHandleDoctorBlocksLanePGReachableUnderHardenedSocketProfile(t *testing.T) {
	origUser := currentUsername
	origLookup := lookupOSUser
	t.Cleanup(func() { currentUsername = origUser; lookupOSUser = origLookup })
	currentUsername = func() string { return "daemonuser" }
	lookupOSUser = func(string) bool { return true }
	t.Setenv(laneOSUserEnv, "")
	t.Setenv(lanePGSocketHardenedEnv, "true")

	result, err := HandleDoctor(context.Background(), &doctorFakeRunner{}, rpc.Envelope{Params: map[string]any{}})
	if err != nil {
		t.Fatalf("HandleDoctor: %v", err)
	}
	if result["ok"] != false {
		t.Fatalf("expected doctor to fail closed under hardened lane profile: %#v", result)
	}
	problems := result["problems"].([]string)
	if len(problems) != 1 || !strings.Contains(problems[0], "lane_pg_reachable") {
		t.Fatalf("expected lane_pg_reachable problem, got %#v", problems)
	}
	laneSandbox := result["lane_sandbox"].(map[string]any)
	if laneSandbox["pg_socket_hardened"] != true || laneSandbox["enforcement"] != "blocking" {
		t.Fatalf("lane_sandbox hardened fields = %#v", laneSandbox)
	}
}
