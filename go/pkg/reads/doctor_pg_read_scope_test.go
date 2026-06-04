package reads

import "testing"

// TestPgReadScopeDoctorBlock pins the #164 posture string: until read-scope
// least privilege lands, doctor must be explicit that the runtime role retains
// broad SELECT and no private-read denial claim is valid.
func TestPgReadScopeDoctorBlock(t *testing.T) {
	block := pgReadScopeDoctorBlock()
	if block["posture"] != pgReadScopeBroadRuntimeSelect {
		t.Fatalf("posture = %v, want %s", block["posture"], pgReadScopeBroadRuntimeSelect)
	}
	if block["private_read_denial"] != false {
		t.Fatalf("private_read_denial = %v, want false", block["private_read_denial"])
	}
	if block["runtime_role_select_scope"] != "broad" {
		t.Fatalf("runtime_role_select_scope = %v, want broad", block["runtime_role_select_scope"])
	}
	surfaces, ok := block["representative_sensitive_surfaces"].([]string)
	if !ok || len(surfaces) == 0 {
		t.Fatalf("expected representative sensitive surfaces, got %#v", block["representative_sensitive_surfaces"])
	}
	if !containsStringItem(surfaces, "artifacts") || !containsStringItem(surfaces, "events") {
		t.Fatalf("expected artifacts and events in representative surfaces, got %#v", surfaces)
	}
}

func containsStringItem(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
