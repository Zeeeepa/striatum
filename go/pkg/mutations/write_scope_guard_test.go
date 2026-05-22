package mutations

import (
	"reflect"
	"testing"
)

func TestWriteScopeViolationsRejectsOutsideAndForbiddenPaths(t *testing.T) {
	got := writeScopeViolations(
		[]string{"docs/rfc-0050/design/codex/DESIGN.md", "go/pkg/mcp/capabilities.go", ".striatum/scratch/pid"},
		[]string{"docs/rfc-0050/design/codex/"},
		[]string{".striatum/"},
	)
	want := []string{".striatum/scratch/pid", "go/pkg/mcp/capabilities.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %#v, want %#v", got, want)
	}
}

func TestWriteScopeViolationsAllowsBroadScopeButStillHonorsForbidden(t *testing.T) {
	got := writeScopeViolations(
		[]string{"src/striatum/workflow.py", ".striatum/state"},
		[]string{"."},
		[]string{".striatum/"},
	)
	want := []string{".striatum/state"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations = %#v, want %#v", got, want)
	}
}

func TestParseGitPorcelainZIncludesRenameOldAndNewPaths(t *testing.T) {
	got := parseGitPorcelainZ([]byte("R  docs/new.md\x00docs/old.md\x00?? tests/new_test.py\x00"))
	want := []string{"docs/new.md", "docs/old.md", "tests/new_test.py"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("paths = %#v, want %#v", got, want)
	}
}
