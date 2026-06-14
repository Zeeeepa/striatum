package rpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoMemoryMethodsOrCapabilities(t *testing.T) {
	for _, entry := range SortedMethods() {
		if strings.HasPrefix(entry.Method, "memory.") {
			t.Fatalf("forbidden memory.* daemon method registered: %s", entry.Method)
		}
		if entry.RequiredCapability != nil && strings.HasPrefix(string(*entry.RequiredCapability), "memory.") {
			t.Fatalf("forbidden memory.* capability registered on %s", entry.Method)
		}
	}

	recall, ok := MethodRegistry["recall.search"]
	if !ok {
		t.Fatal("recall.search is not registered")
	}
	if recall.RequiredCapability == nil || *recall.RequiredCapability != CapabilityRead {
		t.Fatalf("recall.search capability = %#v, want read", recall.RequiredCapability)
	}
	if recall.RepositoryScopeMode != ScopeSingleRepo {
		t.Fatalf("recall.search scope = %s, want single_repo", recall.RepositoryScopeMode)
	}
}

func TestContractHasNoMemoryMethodsOrCapabilities(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(findRepositoryRoot(t), "contracts", "daemon_methods.json"))
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var doc contractMethodsDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("decode contract: %v", err)
	}
	foundRecall := false
	for _, entry := range doc.Methods {
		if strings.HasPrefix(entry.Method, "memory.") {
			t.Fatalf("forbidden memory.* contract method: %s", entry.Method)
		}
		if entry.RequiredCapability != nil && strings.HasPrefix(*entry.RequiredCapability, "memory.") {
			t.Fatalf("forbidden memory.* contract capability on %s", entry.Method)
		}
		if entry.Method == "recall.search" {
			foundRecall = true
			if entry.RequiredCapability == nil || *entry.RequiredCapability != string(CapabilityRead) {
				t.Fatalf("recall.search contract capability = %#v, want read", entry.RequiredCapability)
			}
			if entry.RepositoryScopeMode != string(ScopeSingleRepo) {
				t.Fatalf("recall.search contract scope = %s, want single_repo", entry.RepositoryScopeMode)
			}
		}
	}
	if !foundRecall {
		t.Fatal("recall.search missing from contract")
	}
}
