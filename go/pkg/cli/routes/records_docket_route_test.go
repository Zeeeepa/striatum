package routes

import (
	"strings"
	"testing"
)

func TestRecordsDocketRouteIsReachable(t *testing.T) {
	route, consumed, ok := Lookup([]string{"records", "docket", "run_1"})
	if !ok {
		t.Fatalf("records docket route was not found")
	}
	if consumed != 2 {
		t.Fatalf("consumed = %d, want 2", consumed)
	}
	if route.Method != "records.docket" {
		t.Fatalf("route.Method = %q, want records.docket", route.Method)
	}
	if route.ParamsGroup != "records_docket" {
		t.Fatalf("route.ParamsGroup = %q, want records_docket", route.ParamsGroup)
	}
	if route.RequiredCapability != "read" {
		t.Fatalf("route.RequiredCapability = %q, want read", route.RequiredCapability)
	}
	if route.RepositoryScopeMode != "single_repo" {
		t.Fatalf("route.RepositoryScopeMode = %q, want single_repo", route.RepositoryScopeMode)
	}
	inGenerated := false
	for _, r := range generatedRoutes {
		if r.Method == "records.docket" {
			inGenerated = true
			break
		}
	}
	if !inGenerated {
		t.Fatalf("records.docket must be an on-contract generated route")
	}
	if help := route.RenderHelp(); !strings.Contains(help, "run-id") || !strings.Contains(help, "format") {
		t.Fatalf("help does not advertise expected params: %q", help)
	}
}
