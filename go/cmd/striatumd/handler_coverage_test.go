package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/halbritt/striatum/go/pkg/db"
	"github.com/halbritt/striatum/go/pkg/rpc"
)

var errCoverageRunner = errors.New("coverage runner does not execute SQL")

type coverageRunner struct{}

func (coverageRunner) Exec(context.Context, string, ...any) error {
	return errCoverageRunner
}

func (coverageRunner) QueryRow(context.Context, string, ...any) db.Row {
	return coverageRow{}
}

func (coverageRunner) QueryScalar(context.Context, string, ...any) (string, error) {
	return "", errCoverageRunner
}

func (coverageRunner) BeginTx(context.Context) (db.TxRunner, error) {
	return nil, errCoverageRunner
}

type coverageRow struct{}

func (coverageRow) Scan(...any) error {
	return errCoverageRunner
}

func TestGoDaemonMethodCoverageIsExplicit(t *testing.T) {
	server := rpc.NewServer()
	registerHandlers(server, coverageRunner{})

	var missingHandlers []string
	var notImplementedHandlers []string
	for _, entry := range rpc.SortedMethods() {
		if entry.Deprecated || entry.Method == "daemon.hello" || entry.Method == "daemon.describe" {
			continue
		}
		handler, ok := server.Handlers[entry.Method]
		if !ok {
			missingHandlers = append(missingHandlers, entry.Method)
			continue
		}
		_, err := handler(context.Background(), rpc.Envelope{
			SchemaVersion: rpc.SupportedEnvelopeVersion,
			RequestID:     "coverage-" + entry.Method,
			Method:        entry.Method,
			Params:        coverageParams(),
		})
		var rpcErr *rpc.Error
		if errors.As(err, &rpcErr) && rpcErr.Code == "not_implemented" {
			notImplementedHandlers = append(notImplementedHandlers, entry.Method)
		}
	}

	assertSameStrings(t, "missing Go daemon handlers", missingHandlers, nil)
	assertSameStrings(t, "Go daemon not_implemented handlers", notImplementedHandlers, nil)
}

// TestLiveMethodRegistryMatchesDaemonMethodsContract closes the registry-guard
// blind spot from #363.
//
// The in-package guard rpc.TestRegistryMatchesDaemonMethodsContract reconciles
// the contract against rpc.SortedMethods() — a COPY of the static methodEntries
// slice. Methods that the reads/mutations packages hand-register straight into
// the live map at runtime (`rpc.MethodRegistry[...] = rpc.NewMethod(...)`, e.g.
// the old supervise.rebridge / doctor.blob_block registrations) never appear in
// methodEntries, so SortedMethods() cannot see them and the in-package guard
// PASSES while the method is genuinely off-contract.
//
// This guard runs AFTER registerHandlers wires the full reads+mutations handler
// set, so rpc.MethodRegistry is fully populated — exactly the map the request-
// time authority check consults (rpc/server.go, webservice/service.go). Any
// method present in that live map but absent from contracts/daemon_methods.json
// (or whose capability/scope drifts) FAILS here. A future hand-registration that
// skips the contract is therefore caught immediately, not silently tolerated.
func TestLiveMethodRegistryMatchesDaemonMethodsContract(t *testing.T) {
	contractPath := filepath.Join(repositoryRootForTest(t), "contracts", "daemon_methods.json")
	payload, err := os.ReadFile(contractPath)
	if errors.Is(err, os.ErrNotExist) {
		t.Skip("contracts/daemon_methods.json is not present in this checkout")
	}
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}

	var document struct {
		Methods []struct {
			Method              string  `json:"method"`
			RequiredCapability  *string `json:"required_capability"`
			RepositoryScopeMode string  `json:"repository_scope_mode"`
		} `json:"methods"`
	}
	if err := json.Unmarshal(payload, &document); err != nil {
		t.Fatalf("decode contract: %v", err)
	}
	contractMethods := map[string]struct {
		capability *string
		scopeMode  string
	}{}
	for _, m := range document.Methods {
		scopeMode := m.RepositoryScopeMode
		if scopeMode == "" {
			scopeMode = string(rpc.ScopeDaemonGlobal)
		}
		contractMethods[m.Method] = struct {
			capability *string
			scopeMode  string
		}{capability: m.RequiredCapability, scopeMode: scopeMode}
	}

	// Populate the live registry exactly as the daemon does, including any
	// runtime map registrations performed inside reads.Register/mutations.Register.
	server := rpc.NewServer()
	registerHandlers(server, coverageRunner{})

	// daemon.hello / daemon.describe are intrinsic handshake methods that are
	// intentionally not enumerated as contract rows.
	intrinsic := map[string]bool{"daemon.hello": true, "daemon.describe": true}

	var offContract []string
	for method, entry := range rpc.MethodRegistry {
		if intrinsic[method] {
			continue
		}
		want, ok := contractMethods[method]
		if !ok {
			offContract = append(offContract, method)
			continue
		}
		var gotCap *string
		if entry.RequiredCapability != nil {
			value := string(*entry.RequiredCapability)
			gotCap = &value
		}
		if !reflect.DeepEqual(gotCap, want.capability) {
			t.Errorf("%s required_capability drifts: live=%v contract=%v", method, deref(gotCap), deref(want.capability))
		}
		if string(entry.RepositoryScopeMode) != want.scopeMode {
			t.Errorf("%s repository_scope_mode drifts: live=%q contract=%q", method, entry.RepositoryScopeMode, want.scopeMode)
		}
	}
	if len(offContract) > 0 {
		sort.Strings(offContract)
		t.Fatalf("methods are in the live rpc.MethodRegistry but absent from contracts/daemon_methods.json (off-contract hand-registrations): %v", offContract)
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func repositoryRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go", "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root from %s", dir)
		}
		dir = parent
	}
}

func TestRegisterHandlersWiresShutdownHook(t *testing.T) {
	server := rpc.NewServer()
	called := false
	registerHandlers(server, coverageRunner{}, handlerOptions{
		ShutdownHook: func(context.Context) error {
			called = true
			return nil
		},
	})

	handler := server.Handlers["daemon.shutdown"]
	if handler == nil {
		t.Fatalf("daemon.shutdown handler missing")
	}
	result, err := handler(context.Background(), rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "shutdown-test",
		Method:        "daemon.shutdown",
		Params:        map[string]any{},
	})
	if err != nil {
		t.Fatalf("daemon.shutdown returned error: %v", err)
	}
	if !called {
		t.Fatalf("shutdown hook was not called")
	}
	if result["status"] != "shutting_down" || result["accepted"] != true {
		t.Fatalf("unexpected shutdown result: %#v", result)
	}
}

func TestRegisterHandlersWiresKeyRotateHook(t *testing.T) {
	server := rpc.NewServer()
	called := false
	registerHandlers(server, coverageRunner{}, handlerOptions{
		KeyRotateHook: func(context.Context) (map[string]any, error) {
			called = true
			return map[string]any{
				"status":         "rotated",
				"signing_key_id": "ed25519:test",
			}, nil
		},
	})

	handler := server.Handlers["daemon.key.rotate"]
	if handler == nil {
		t.Fatalf("daemon.key.rotate handler missing")
	}
	result, err := handler(context.Background(), rpc.Envelope{
		SchemaVersion: rpc.SupportedEnvelopeVersion,
		RequestID:     "key-rotate-test",
		Method:        "daemon.key.rotate",
		Params:        map[string]any{},
	})
	if err != nil {
		t.Fatalf("daemon.key.rotate returned error: %v", err)
	}
	if !called {
		t.Fatalf("key rotate hook was not called")
	}
	if result["status"] != "rotated" || result["signing_key_id"] != "ed25519:test" {
		t.Fatalf("unexpected key rotation result: %#v", result)
	}
	if result["python_dependency"] != false || result["sqlite_dependency"] != false {
		t.Fatalf("dependency flags missing: %#v", result)
	}
}

func coverageParams() map[string]any {
	return map[string]any{
		"apply_receipt_id":     "receipt_1",
		"artifact_id":          "artifact_1",
		"blocker_id":           "blocker_1",
		"branch":               "main",
		"branch_name":          "main",
		"checkpoint_id":        "checkpoint_1",
		"decision_id":          "decision_1",
		"job_id":               "job_1",
		"lease_id":             "lease_1",
		"message_id":           "msg_1",
		"path":                 "docs/out.md",
		"receipt_id":           "receipt_1",
		"repository_id":        "repo_1",
		"review_id":            "review_1",
		"run_id":               "run_1",
		"session_id":           "sess_1",
		"supervisor_id":        "sup_1",
		"target_id":            "run_1",
		"worktree_id":          "worktree_1",
		"workflow_path":        "workflow.json",
		"workflow_template_id": "default",
	}
}

func assertSameStrings(t *testing.T, label string, got []string, want []string) {
	t.Helper()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s changed:\n got: %v\nwant: %v", label, got, want)
	}
}
