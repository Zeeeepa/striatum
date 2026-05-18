package mcp

import (
	"context"

	"github.com/halbritt/striatum/go/pkg/rpc"
)

type Tool struct {
	Name                string `json:"name"`
	Description         string `json:"description"`
	RequiredCapability  string `json:"required_capability"`
	RepositoryScopeMode string `json:"repository_scope_mode"`
}

func VisibleTools(ctx context.Context, authorizer rpc.Authorizer, token string, repositoryID string) []Tool {
	tools := []Tool{}
	for _, entry := range rpc.SortedMethods() {
		if entry.RequiredCapability == nil || isInternal(entry.Method) || isHiddenProductionTool(entry.Method) || entry.Deprecated {
			continue
		}
		scopeRepo := ""
		if entry.RepositoryScopeMode == rpc.ScopeSingleRepo {
			scopeRepo = repositoryID
		}
		auth := authorizer.Authorize(entry.RequiredCapability, scopeRepo, token)
		if auth.Decision != "allowed" {
			continue
		}
		tools = append(tools, Tool{
			Name:                entry.Method,
			Description:         "Striatum daemon RPC method " + entry.Method,
			RequiredCapability:  string(*entry.RequiredCapability),
			RepositoryScopeMode: string(entry.RepositoryScopeMode),
		})
	}
	return tools
}

func isInternal(method string) bool {
	return method == "daemon.hello" || method == "daemon.describe"
}

func isHiddenProductionTool(method string) bool {
	switch method {
	case "workflow.validate",
		"workflow.plan",
		"workflow.graph",
		"workflow.templates.list",
		"workflow.templates.show",
		"workflow.init",
		"workflow.generate",
		"workflow.upgrade":
		return true
	default:
		return false
	}
}
