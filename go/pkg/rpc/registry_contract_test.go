package rpc

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

type contractMethod struct {
	Method              string  `json:"method"`
	RequiredCapability  *string `json:"required_capability"`
	RepositoryScope     *bool   `json:"repository_scope"`
	RepositoryScopeMode string  `json:"repository_scope_mode"`
	ParamsSchemaVersion int     `json:"params_schema_version"`
	AuditClass          string  `json:"audit_class"`
	MinEnvelope         int     `json:"min_envelope"`
	Deprecated          bool    `json:"deprecated"`
}

type contractMethodsDocument struct {
	Methods []contractMethod `json:"methods"`
}

func TestRegistryMatchesDaemonMethodsContract(t *testing.T) {
	contractPath := filepath.Join(findRepositoryRoot(t), "contracts", "daemon_methods.json")
	payload, err := os.ReadFile(contractPath)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("contracts/daemon_methods.json is not present in this checkout")
	}
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}

	expected := methodsByName(t, decodeContractMethods(t, payload))
	actual := methodsByName(t, registryContractView())

	if missing, extra := mapKeyDiff(expected, actual); len(missing) > 0 || len(extra) > 0 {
		t.Fatalf("Go registry method set drifts from contract; missing=%v extra=%v", missing, extra)
	}
	for method, want := range expected {
		got := actual[method]
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s metadata drifts from contract:\n got: %#v\nwant: %#v", method, got, want)
		}
	}
}

func decodeContractMethods(t *testing.T, payload []byte) []contractMethod {
	t.Helper()
	var document contractMethodsDocument
	if err := json.Unmarshal(payload, &document); err == nil && document.Methods != nil {
		return normalizeContractMethods(document.Methods)
	}

	var methods []contractMethod
	if err := json.Unmarshal(payload, &methods); err != nil {
		t.Fatalf("decode daemon method contract: %v", err)
	}
	return normalizeContractMethods(methods)
}

func normalizeContractMethods(methods []contractMethod) []contractMethod {
	normalized := make([]contractMethod, 0, len(methods))
	for _, method := range methods {
		if method.RepositoryScopeMode == "" && method.RepositoryScope == nil {
			method.RepositoryScopeMode = string(ScopeDaemonGlobal)
		}
		if method.RepositoryScopeMode == "" {
			if *method.RepositoryScope {
				method.RepositoryScopeMode = string(ScopeSingleRepo)
			} else {
				method.RepositoryScopeMode = string(ScopeDaemonGlobal)
			}
		}
		if method.RepositoryScope == nil {
			repositoryScope := method.RepositoryScopeMode == string(ScopeSingleRepo)
			method.RepositoryScope = &repositoryScope
		}
		if method.ParamsSchemaVersion == 0 {
			method.ParamsSchemaVersion = 1
		}
		if method.AuditClass == "" {
			method.AuditClass = "metadata"
		}
		if method.MinEnvelope == 0 {
			method.MinEnvelope = 1
		}
		normalized = append(normalized, method)
	}
	return normalized
}

func registryContractView() []contractMethod {
	entries := SortedMethods()
	methods := make([]contractMethod, 0, len(entries))
	for _, entry := range entries {
		var capability *string
		if entry.RequiredCapability != nil {
			value := string(*entry.RequiredCapability)
			capability = &value
		}
		repositoryScope := entry.RepositoryScope
		methods = append(methods, contractMethod{
			Method:              entry.Method,
			RequiredCapability:  capability,
			RepositoryScope:     &repositoryScope,
			RepositoryScopeMode: string(entry.RepositoryScopeMode),
			ParamsSchemaVersion: entry.ParamsSchemaVersion,
			AuditClass:          entry.AuditClass,
			MinEnvelope:         entry.MinEnvelope,
			Deprecated:          entry.Deprecated,
		})
	}
	return methods
}

func methodsByName(t *testing.T, methods []contractMethod) map[string]contractMethod {
	t.Helper()
	byName := make(map[string]contractMethod, len(methods))
	for _, method := range methods {
		if method.Method == "" {
			t.Fatalf("contract contains method without a name")
		}
		if _, exists := byName[method.Method]; exists {
			t.Fatalf("contract contains duplicate method %s", method.Method)
		}
		byName[method.Method] = method
	}
	return byName
}

func mapKeyDiff(expected map[string]contractMethod, actual map[string]contractMethod) ([]string, []string) {
	missing := make([]string, 0)
	extra := make([]string, 0)
	for key := range expected {
		if _, ok := actual[key]; !ok {
			missing = append(missing, key)
		}
	}
	for key := range actual {
		if _, ok := expected[key]; !ok {
			extra = append(extra, key)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go", "go.mod")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil && filepath.Base(dir) == "go" {
			return filepath.Dir(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root from %s", dir)
		}
		dir = parent
	}
}
