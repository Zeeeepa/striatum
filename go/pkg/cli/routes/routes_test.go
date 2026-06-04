package routes

import (
	"strings"
	"testing"
)

func TestLookupIncludesRuntimeSuperviseRebridgeRoute(t *testing.T) {
	route, consumed, ok := Lookup([]string{"supervise", "rebridge", "sess_1"})
	if !ok {
		t.Fatalf("supervise rebridge route was not found")
	}
	if consumed != 2 {
		t.Fatalf("consumed = %d, want 2", consumed)
	}
	if route.Method != "supervise.rebridge" || route.ParamsGroup != "supervise_rebridge" || route.RequiredCapability != "claim" {
		t.Fatalf("route = %#v", route)
	}
}

func TestLookupSessionRegisterAlias(t *testing.T) {
	route, consumed, ok := Lookup([]string{"session", "register", "run_1", "author", "lane_a"})
	if !ok {
		t.Fatalf("session register alias was not found")
	}
	if consumed != 2 {
		t.Fatalf("consumed = %d, want 2", consumed)
	}
	if route.Method != "session.register" || route.ParamsGroup != "register_session" {
		t.Fatalf("route = %#v", route)
	}
}

func TestRenderHelpListsRequiredAndOptional(t *testing.T) {
	route, _, ok := Lookup([]string{"register-session"})
	if !ok {
		t.Fatalf("register-session route not found")
	}
	help := route.RenderHelp()
	for _, want := range []string{"usage: striatum register-session", "method: session.register", "required:", "optional:", "--capability", "(repeatable)", "--fresh", "session register"} {
		if !strings.Contains(help, want) {
			t.Fatalf("register-session help missing %q:\n%s", want, help)
		}
	}
}

func TestRenderHelpEnumValues(t *testing.T) {
	route, _, ok := Lookup([]string{"checkpoint", "resolve"})
	if !ok {
		t.Fatalf("checkpoint resolve route not found")
	}
	help := route.RenderHelp()
	if !strings.Contains(help, "continue|cancel") {
		t.Fatalf("checkpoint resolve help missing action enum:\n%s", help)
	}
}

func TestRenderHelpSuperviseTrajectoryListsTailControls(t *testing.T) {
	route, _, ok := Lookup([]string{"supervise", "trajectory"})
	if !ok {
		t.Fatalf("supervise trajectory route not found")
	}
	help := route.RenderHelp()
	for _, want := range []string{"usage: striatum supervise trajectory", "method: supervise.trajectory", "<session-id>", "--tail", "--tail-lines", ".striatum/scratch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("supervise trajectory help missing %q:\n%s", want, help)
		}
	}
}

// session close rejects an empty reason server-side ("session close reason
// must not be empty"), so `--help` must list --reason as REQUIRED instead of
// sending operators down a failing path with only <session-id> (issue #72).
func TestRenderHelpSessionCloseListsRequiredReason(t *testing.T) {
	route, _, ok := Lookup([]string{"session", "close"})
	if !ok {
		t.Fatalf("session close route not found")
	}
	if route.ParamsGroup != "session_close" {
		t.Fatalf("session close ParamsGroup = %q, want session_close", route.ParamsGroup)
	}
	usage, ok := UsageFor("session_close")
	if !ok {
		t.Fatalf("missing usage descriptor for session_close")
	}
	var reason Param
	found := false
	for _, p := range usage.Params {
		if p.Name == "reason" {
			reason = p
			found = true
		}
	}
	if !found {
		t.Fatalf("session_close usage missing --reason param: %#v", usage.Params)
	}
	if !reason.Required {
		t.Fatalf("session_close --reason must be Required, got %#v", reason)
	}
	help := route.RenderHelp()
	// Synopsis shows --reason as required (no surrounding brackets), and the
	// "required:" section lists it.
	if !strings.Contains(help, "--reason <value>") {
		t.Fatalf("session close synopsis missing required --reason token:\n%s", help)
	}
	requiredSection := help
	if idx := strings.Index(help, "optional:"); idx >= 0 {
		requiredSection = help[:idx]
	}
	if !strings.Contains(requiredSection, "required:") || !strings.Contains(requiredSection, "--reason") {
		t.Fatalf("session close help does not list --reason under required:\n%s", help)
	}
}

// Consumer-repo agents should be able to run `striatum <work-loop-verb> --help`
// without opening the source repo or command-authority matrix (#159).
func TestRenderHelpWorkLoopVerbsAreSelfContained(t *testing.T) {
	cases := []struct {
		args []string
		want []string
	}{
		{[]string{"claim-next"}, []string{"<session-id>", "--lease-seconds"}},
		{[]string{"publish-artifact"}, []string{"<session-id>", "<job-id>", "<lease-id>", "<kind>", "<logical-name>", "<path>", "--allow-no-process-execution"}},
		{[]string{"submit-review"}, []string{"<session-id>", "<job-id>", "<lease-id>", "<path>", "accept|accept_with_findings|needs_revision|reject", "--logical-name", "--kind"}},
		{[]string{"complete"}, []string{"<session-id>", "<job-id>", "<lease-id>", "--summary"}},
		{[]string{"repo", "patch-preview"}, []string{"<session-id>", "<job-id>", "<lease-id>", "--patch"}},
		{[]string{"process", "run"}, []string{"<session-id>", "<job-id>", "<lease-id>", "--command-json", "--timeout-seconds"}},
		{[]string{"worktree", "create"}, []string{"<session-id>", "<job-id>", "<lease-id>"}},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			route, _, ok := Lookup(tc.args)
			if !ok {
				t.Fatalf("route not found for %v", tc.args)
			}
			help := route.RenderHelp()
			if strings.Contains(help, "flags are derived from the daemon method") {
				t.Fatalf("help fell back to generic matrix pointer:\n%s", help)
			}
			for _, want := range tc.want {
				if !strings.Contains(help, want) {
					t.Fatalf("help missing %q:\n%s", want, help)
				}
			}
		})
	}
}

// Every operator verb named in issue #63 F9 must have a usage descriptor so
// `--help` lists its flags instead of surfacing them only as runtime errors.
func TestUsageCoversIssue63Verbs(t *testing.T) {
	for _, group := range []string{
		"supervise_start", "supervise_stop", "supervise_status", "supervise_send", "supervise_trajectory",
		"register_session", "checkpoint_resolve", "decision_record", "repo_add",
		"claim_next", "ack", "heartbeat", "release", "send", "block",
		"complete", "publish_artifact", "repo_write", "repo_patch", "process_run", "verdict", "submit_review",
		"worktree_create", "worktree_release",
	} {
		if _, ok := UsageFor(group); !ok {
			t.Fatalf("missing usage descriptor for %q", group)
		}
	}
}
