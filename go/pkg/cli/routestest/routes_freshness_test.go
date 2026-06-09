package routestest

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/halbritt/striatum/go/pkg/cli/routes"
)

type contract struct {
	CLIRoutes []contractRoute  `json:"cli_routes"`
	Methods   []contractMethod `json:"methods"`
}

type contractRoute struct {
	Command    string  `json:"command"`
	Subcommand *string `json:"subcommand"`
	Method     string  `json:"method"`
	Params     string  `json:"params"`
}

type contractMethod struct {
	Method              string  `json:"method"`
	RequiredCapability  *string `json:"required_capability"`
	RepositoryScopeMode string  `json:"repository_scope_mode"`
	Deprecated          bool    `json:"deprecated"`
}

func TestGeneratedRoutesMatchDaemonMethodContract(t *testing.T) {
	body, err := os.ReadFile("../../../../contracts/daemon_methods.json")
	if err != nil {
		t.Fatalf("read contract: %v", err)
	}
	var parsed contract
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("decode contract: %v", err)
	}
	methods := map[string]contractMethod{}
	for _, method := range parsed.Methods {
		methods[method.Method] = method
	}

	got := routes.All()
	if len(got) != len(parsed.CLIRoutes) {
		t.Fatalf("route count = %d, want %d; run go generate ./pkg/cli/routes", len(got), len(parsed.CLIRoutes))
	}
	for i, want := range parsed.CLIRoutes {
		method, ok := methods[want.Method]
		if !ok {
			t.Fatalf("contract route %d references missing method %s", i, want.Method)
		}
		subcommand := ""
		if want.Subcommand != nil {
			subcommand = *want.Subcommand
		}
		capability := ""
		if method.RequiredCapability != nil {
			capability = *method.RequiredCapability
		}
		if got[i].Command != want.Command ||
			got[i].Subcommand != subcommand ||
			got[i].Method != want.Method ||
			got[i].ParamsGroup != want.Params ||
			got[i].RequiredCapability != capability ||
			got[i].RepositoryScopeMode != method.RepositoryScopeMode ||
			got[i].Deprecated != method.Deprecated {
			t.Fatalf("route %d = %#v, want command=%q subcommand=%q method=%q params=%q capability=%q scope=%q deprecated=%t; run go generate ./pkg/cli/routes",
				i, got[i], want.Command, subcommand, want.Method, want.Params, capability, method.RepositoryScopeMode, method.Deprecated)
		}
	}
}

func TestWorkflowValidateIsNotGeneratedDaemonRoute(t *testing.T) {
	if route, _, ok := routes.Lookup([]string{"workflow", "validate"}); ok {
		t.Fatalf("workflow validate must remain a local command, got generated route %#v", route)
	}
}

func TestOperatorBootstrapIsNotGeneratedDaemonRoute(t *testing.T) {
	if route, _, ok := routes.Lookup([]string{"operator", "bootstrap"}); ok {
		t.Fatalf("operator bootstrap must remain a local read composite unless a product decision adds an RPC method, got route %#v", route)
	}
}
