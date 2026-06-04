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

// Every operator verb named in issue #63 F9 must have a usage descriptor so
// `--help` lists its flags instead of surfacing them only as runtime errors.
func TestUsageCoversIssue63Verbs(t *testing.T) {
	for _, group := range []string{
		"supervise_start", "supervise_stop", "supervise_status", "supervise_send",
		"register_session", "checkpoint_resolve", "repo_add",
	} {
		if _, ok := UsageFor(group); !ok {
			t.Fatalf("missing usage descriptor for %q", group)
		}
	}
}
