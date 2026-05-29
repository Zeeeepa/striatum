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

// Every operator verb named in issue #63 F9 must have a usage descriptor so
// `--help` lists its flags instead of surfacing them only as runtime errors.
func TestUsageCoversIssue63Verbs(t *testing.T) {
	for _, group := range []string{
		"supervise_start", "supervise_stop", "supervise_status", "supervise_send",
		"register_session", "checkpoint_resolve",
	} {
		if _, ok := UsageFor(group); !ok {
			t.Fatalf("missing usage descriptor for %q", group)
		}
	}
}
