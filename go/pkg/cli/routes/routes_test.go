package routes

import "testing"

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
