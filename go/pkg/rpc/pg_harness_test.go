package rpc

import "testing"

func TestStatefulDaemonMethodsStayRepositoryScoped(t *testing.T) {
	statefulPrefixes := []string{
		"work.",
		"artifact.",
		"review.",
		"run.",
		"branch.",
		"checkpoint.",
		"recovery.",
		"supervise.",
		"workflow.",
		"worktree.",
	}
	for _, entry := range SortedMethods() {
		if entry.Deprecated || !hasAnyPrefix(entry.Method, statefulPrefixes) {
			continue
		}
		if entry.Method == "dashboard.all" {
			continue
		}
		if entry.RepositoryScopeMode != ScopeSingleRepo {
			t.Fatalf("%s scope = %s, want single_repo", entry.Method, entry.RepositoryScopeMode)
		}
		if !entry.RepositoryScope {
			t.Fatalf("%s must require repository scope", entry.Method)
		}
		if entry.RequiredCapability == nil {
			t.Fatalf("%s must declare a required capability", entry.Method)
		}
	}
}

func hasAnyPrefix(value string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
