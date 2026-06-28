package localcommands

import "testing"

func TestWorkflowValidateIsExplicitLocalCommand(t *testing.T) {
	command, ok := Lookup([]string{"workflow", "validate", "workflow.json"})
	if !ok {
		t.Fatal("workflow validate not local")
	}
	if command.Rationale == "" {
		t.Fatalf("missing rationale: %#v", command)
	}
}

func TestOperatorBootstrapIsExplicitLocalCommand(t *testing.T) {
	command, ok := Lookup([]string{"operator", "bootstrap", "--json"})
	if !ok {
		t.Fatal("operator bootstrap not local")
	}
	if command.Rationale == "" {
		t.Fatalf("missing rationale: %#v", command)
	}
}

func TestRecordsMigrationIsExplicitLocalCommand(t *testing.T) {
	command, ok := Lookup([]string{"records", "migration", "inventory"})
	if !ok {
		t.Fatal("records migration not local")
	}
	if command.Rationale == "" {
		t.Fatalf("missing rationale: %#v", command)
	}
}

func TestDaemonBackedWorkflowRoutesAreNotLocal(t *testing.T) {
	if command, ok := Lookup([]string{"workflow", "accepted-risks"}); ok {
		t.Fatalf("unexpected local route: %#v", command)
	}
}
