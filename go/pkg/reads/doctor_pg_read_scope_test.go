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
	if block["inventory_source"] != "go/pkg/db/read_authority_inventory.go" {
		t.Fatalf("inventory_source = %v", block["inventory_source"])
	}
	surfaces, ok := block["representative_sensitive_surfaces"].([]string)
	if !ok || len(surfaces) == 0 {
		t.Fatalf("expected representative sensitive surfaces, got %#v", block["representative_sensitive_surfaces"])
	}
	if block["sensitive_surface_count"] != len(surfaces) {
		t.Fatalf("sensitive_surface_count = %v, want %d", block["sensitive_surface_count"], len(surfaces))
	}
	gates, ok := block["partial_projection_gates"].([]map[string]any)
	if !ok || len(gates) != 1 {
		t.Fatalf("partial_projection_gates = %#v, want one token-secret gate", block["partial_projection_gates"])
	}
	if gates[0]["surface"] != "clients" || gates[0]["authority_stamp"] != "auth_projection_read" {
		t.Fatalf("partial projection gate = %#v, want clients/auth_projection_read", gates[0])
	}
	deniedColumns, ok := gates[0]["denied_columns"].([]string)
	if !ok || !containsStringItem(deniedColumns, "token_hash") || !containsStringItem(deniedColumns, "token_salt") {
		t.Fatalf("partial projection denied_columns = %#v, want token_hash/token_salt", gates[0]["denied_columns"])
	}
	if !containsStringItem(surfaces, "artifacts") || !containsStringItem(surfaces, "events") {
		t.Fatalf("expected artifacts and events in representative surfaces, got %#v", surfaces)
	}
	if !containsStringItem(surfaces, "clients") || !containsStringItem(surfaces, "work_packets") {
		t.Fatalf("expected clients and work_packets in representative surfaces, got %#v", surfaces)
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
