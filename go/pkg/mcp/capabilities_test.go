package mcp

import (
	"context"
	"testing"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

type fixedAuthorizer struct {
	allowed rpc.Capability
}

func (a fixedAuthorizer) Authorize(required *rpc.Capability, repositoryID string, token string) rpc.AuthContext {
	if required != nil && *required == a.allowed && token == "tok" {
		return rpc.AuthContext{RepositoryID: repositoryID, Capability: *required, Decision: "allowed"}
	}
	return rpc.AuthContext{RepositoryID: repositoryID, Decision: "denied", DenialReason: "capability_missing"}
}

type allowAllAuthorizer struct{}

func (allowAllAuthorizer) Authorize(required *rpc.Capability, repositoryID string, token string) rpc.AuthContext {
	if required != nil && token == "tok" {
		return rpc.AuthContext{RepositoryID: repositoryID, Capability: *required, Decision: "allowed"}
	}
	return rpc.AuthContext{RepositoryID: repositoryID, Decision: "denied", DenialReason: "capability_missing"}
}

func TestVisibleToolsFiltersByCapability(t *testing.T) {
	tools := VisibleTools(context.Background(), fixedAuthorizer{allowed: rpc.CapabilityReview}, "tok", "repo_1")
	if len(tools) == 0 {
		t.Fatal("expected review tools")
	}
	for _, tool := range tools {
		if tool.RequiredCapability != string(rpc.CapabilityReview) {
			t.Fatalf("tool %s leaked capability %s", tool.Name, tool.RequiredCapability)
		}
		if tool.Name == "daemon.hello" || tool.Name == "daemon.describe" {
			t.Fatalf("internal method leaked into MCP tools: %s", tool.Name)
		}
	}
}

func TestVisibleToolsHideProductionUnsupportedMethods(t *testing.T) {
	tools := VisibleTools(context.Background(), allowAllAuthorizer{}, "tok", "repo_1")
	names := map[string]struct{}{}
	for _, tool := range tools {
		names[tool.Name] = struct{}{}
	}
	for _, hidden := range []string{
		"workflow.validate",
		"workflow.plan",
		"workflow.graph",
		"workflow.templates.list",
		"workflow.templates.show",
		"workflow.init",
		"workflow.generate",
		"workflow.upgrade",
	} {
		if _, ok := names[hidden]; ok {
			t.Fatalf("production-unsupported method leaked into MCP tools/list: %s", hidden)
		}
	}
	if _, ok := names["workflow.generate.preview"]; !ok {
		t.Fatalf("workflow.generate.preview should remain visible for authorized production MCP tokens")
	}
}
